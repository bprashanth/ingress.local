/*
Copyright 2015 The Kubernetes Authors All rights reserved.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package lib

import (
	"time"

	"github.com/golang/glog"
	"k8s.io/kubernetes/pkg/util"
)

// Service has all information to route inbound requests to a backend service.
type Service struct {
	// DNS Hostname used to resolve the service, eg: foo.bar.com. This hostname
	// must match the name in your SSL certificates. "_" implies that the
	// subscriber should apply a catch all rule appropriate to the loadbalancer.
	Host string
	// The frontend port on which to listen for new requests. Defaults to 443.
	Port int

	// URL path used to route traffic to the endpoints. "/" implies that the
	// subscriber should apply a catch all rule appropriate to the loadbalancer.
	Path string
	// Endpoints that receive traffic for Path
	Endpoints []string

	// The DNS name of the service within the cluster.
	ClusterDNSName string

	// SSL Certificates: Empty implies that the subscriber can choose weather
	// to use the default (wildcard/SAN) certificates of the loadbalancer, or
	// just drop to HTTP for the given host/port.

	// Path to a .crt file used for SSL
	Crt string
	// Path to a .key file used for SSL
	Key string
}

// Snapshot contains a snapshot of services given to subscribers through
// a PubSubClient.
type Snapshot struct {
	// Services is a snapshot of all Services
	Services []*Service
	// Delta is the subset of Services that changed, empty on delete.
	Delta []*Service
}

// PubSub manages an input and output channel typically meant for pub/sub.
// The input channel is read only, and given to a writer/publisher. The writer
// never blocks. The output channel is buffered to the size of the ring. Eg:
// Go channel: pub -> sub, pub will block if sub is busy.
// Ring Buffer: pub -> rb <- sub, rb drops old writes to make room for new ones.
type PubSub struct {
	inputCh  <-chan Snapshot
	outputCh chan Snapshot
}

// NewPubSub creates a new ring buffer used to pub/sub Snapshots.
func NewPubSub(inputCh <-chan Snapshot, outputCh chan Snapshot) *PubSub {
	return &PubSub{inputCh, outputCh}
}

// Run starts the goroutine that manages the pub/sub ring buffer.
func (r *PubSub) Run() {
	for v := range r.inputCh {
		glog.Infof("Received new value on input ch %+v", v)
		select {
		case r.outputCh <- v:
		default:
			<-r.outputCh
			r.outputCh <- v
		}
	}
	close(r.outputCh)
}

// StartHub starts a new pub/sub hub.
// namespace: the Kubernetes namespace to subscribe to for updates.
// stopCh: Closing this channel tells the publisher to go away.
// Returns A channel the subscriber can block on for Snapshots.
func StartHub(namespace string, stopCh chan struct{}) chan Snapshot {
	in := make(chan Snapshot)
	out := make(chan Snapshot)
	ps := NewPubSub(in, out)
	go ps.Run()
	// TODO: Pipe up inCluster
	go startPublisher(in, stopCh, false, namespace)
	return out
}

// startPublisher starts watching kubernetes and published updates.
// pubCh: A channel on which Snapshot updates are published.
// stopCh: A subscriber closing this channel tells the publisher to go away.
// inCluster: Is this function being invoked from a pod in a kubernetes cluster?
// namespace: The namespace the subscriber cares about.
func startPublisher(pubCh chan Snapshot, stopCh chan struct{}, inCluster bool, namespace string) error {
	iw, err := NewIngressWatcher(pubCh, inCluster, namespace)
	if err != nil {
		return err
	}
	go iw.epController.Run(stopCh)
	go iw.ingController.Run(stopCh)
	go func() {
		util.Until(iw.worker, time.Second, stopCh)
		// Closing the publish channel signals to the subscriber that the
		// publisher is done. The normal way this happens is if the
		// subscriber closed stopCh.
		close(pubCh)
	}()
	return nil
}
