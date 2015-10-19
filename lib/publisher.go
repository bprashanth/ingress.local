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
	"fmt"
	"reflect"
	"time"

	"github.com/golang/glog"
	flag "github.com/spf13/pflag"
	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/apis/extensions"
	"k8s.io/kubernetes/pkg/client/cache"
	client "k8s.io/kubernetes/pkg/client/unversioned"
	"k8s.io/kubernetes/pkg/controller/framework"
	"k8s.io/kubernetes/pkg/fields"
	kubectl_util "k8s.io/kubernetes/pkg/kubectl/cmd/util"
	"k8s.io/kubernetes/pkg/labels"
	"k8s.io/kubernetes/pkg/runtime"
	"k8s.io/kubernetes/pkg/util"
	"k8s.io/kubernetes/pkg/util/workqueue"
	"k8s.io/kubernetes/pkg/watch"
)

const (
	reloadQPS     = 10.0
	resyncPeriod  = 10 * time.Second
	defaultSSLDir = "/etc/kubernetes/ingress/certs"
)

// keyFunc for inserting api objects into a Store.
var (
	keyFunc = framework.DeletionHandlingMetaNamespaceKeyFunc
)

// IngressWatcher watches the kubernetes api.
type IngressWatcher struct {
	queue          *workqueue.Type
	client         *client.Client
	epController   *framework.Controller
	ingController  *framework.Controller
	epLister       cache.StoreToEndpointsLister
	ingLister      StoreToIngressLister
	pubCh          chan Snapshot
	receiverClient *ReceiverClient
}

// getEndpoints returns a list of <endpoint ip>:<port> for a given service/target port combination.
func (iw *IngressWatcher) getEndpoints(s *api.Service, servicePort util.IntOrString) (endpoints []string) {
	ep, err := iw.epLister.GetServiceEndpoints(s)
	if err != nil {
		return
	}

	// The intent here is to create a union of all subsets that match a targetPort.
	// We know the endpoint already matches the service, so all pod ips that have
	// the target port are capable of service traffic for it.
	for _, ss := range ep.Subsets {
		for _, epPort := range ss.Ports {
			var targetPort int
			switch servicePort.Kind {
			case util.IntstrInt:
				if epPort.Port == servicePort.IntVal {
					targetPort = epPort.Port
				}
			case util.IntstrString:
				if epPort.Name == servicePort.StrVal {
					targetPort = epPort.Port
				}
			}
			if targetPort == 0 {
				continue
			}
			for _, epAddress := range ss.Addresses {
				endpoints = append(endpoints, fmt.Sprintf("%v:%v", epAddress.IP, targetPort))
			}
		}
	}
	return
}

// getLBService constructs a loadbalancer service from the given Kubernetes specific information.
func (iw *IngressWatcher) getLBService(svc *api.Service, svcPort util.IntOrString, host, path string, rs ReceiverSecrets) *Service {
	clusterName := fmt.Sprintf("%v.%v.svc.cluster.local", svc.Name, svc.Namespace)
	service := Service{
		Host:           host,
		Path:           path,
		Port:           0,
		Endpoints:      iw.getEndpoints(svc, svcPort),
		Key:            rs.KeyPath,
		Crt:            rs.CrtPath,
		ClusterDNSName: clusterName,
	}
	if service.Port == 0 {
		service.Port = 443
	}
	return &service
}

func wrapWithService(name, namespace string) *api.Service {
	return &api.Service{
		ObjectMeta: api.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
	}
}

// publish publishes a Snapshot to the pub channel.
func (iw *IngressWatcher) publish(key string) error {
	ings, _ := iw.ingLister.List()
	snap := Snapshot{Services: []*Service{}}
	for _, ing := range ings.Items {
		// Retrieve receiver annotations from Ingress, write secrets from
		// receiver annotations to file system. If this fails, we can't
		// reload most loadbalancers, so just requeue.
		receivers, err := iw.receiverClient.LoadSecrets(ing)
		if err != nil {
			return err
		}
		newServices := []*Service{}
		for _, rule := range ing.Spec.Rules {
			for _, path := range rule.HTTP.Paths {
				svc := wrapWithService(path.Backend.ServiceName, ing.Namespace)
				svcPort := path.Backend.ServicePort
				lbSvc := iw.getLBService(svc, svcPort, rule.Host, path.Path, receivers[rule.Host])
				newServices = append(newServices, lbSvc)
			}
		}
		defBackend := ing.Spec.Backend
		if defBackend != nil {
			defSvc := wrapWithService(defBackend.ServiceName, ing.Namespace)
			// TODO: Pipe wildcard cert to default backend.
			lbSvc := iw.getLBService(defSvc, defBackend.ServicePort, "", "", ReceiverSecrets{})
			newServices = append(newServices, lbSvc)
		}
		if currKey, err := keyFunc(&ing); err != nil {
			return err
		} else if currKey == key {
			snap.Delta = newServices
		}
		snap.Services = append(snap.Services, newServices...)
	}
	iw.pubCh <- snap
	return nil
}

// worker handles the work queue.
func (iw *IngressWatcher) worker() {
	for {
		keyObj, _ := iw.queue.Get()
		key := keyObj.(string)
		glog.Infof("Publish triggered by %v", key)
		if err := iw.publish(key); err != nil {
			glog.Infof("Requeuing %v because of error: %v", key, err)
			iw.queue.Add(key)
		} else {
			iw.queue.Done(key)
		}
	}
}

// NewIngressWatcher creates a new watcher for Ingress related resources.
// pubCh: The channel to publish Snapshots to.
// inCluster: Is this watcher running in a pod in a kubernetes cluster?
// namespace: The namespace the subscriber cares about.
func NewIngressWatcher(pubCh chan Snapshot, inCluster bool, namespace string) (*IngressWatcher, error) {
	flags := flag.NewFlagSet("", flag.ContinueOnError)
	clientConfig := kubectl_util.DefaultClientConfig(flags)
	var kubeClient *client.Client
	var err error
	if inCluster {
		if kubeClient, err = client.NewInCluster(); err != nil {
			return nil, err
		}
	} else {
		config, err := clientConfig.ClientConfig()
		if err != nil {
			return nil, err
		}
		kubeClient, err = client.New(config)
	}
	iw := IngressWatcher{
		client:         kubeClient,
		pubCh:          pubCh,
		queue:          workqueue.New(),
		receiverClient: &ReceiverClient{kubeClient},
	}

	enqueue := func(obj interface{}) {
		key, err := keyFunc(obj)
		if err != nil {
			glog.Errorf("Couldn't get key for object %+v: %v", obj, err)
			return
		}
		glog.Infof("Queuing key %v", key)
		iw.queue.Add(key)
	}
	eventHandlers := framework.ResourceEventHandlerFuncs{
		AddFunc:    enqueue,
		DeleteFunc: enqueue,
		UpdateFunc: func(old, cur interface{}) {
			if !reflect.DeepEqual(old, cur) {
				enqueue(cur)
			}
		},
	}

	iw.epLister.Store, iw.epController = framework.NewInformer(
		cache.NewListWatchFromClient(
			iw.client, "endpoints", namespace, fields.Everything()),
		&api.Endpoints{}, resyncPeriod, eventHandlers)

	iw.ingLister.Store, iw.ingController = framework.NewInformer(
		&cache.ListWatch{
			ListFunc:  ingressListFunc(iw.client),
			WatchFunc: ingressWatchFunc(iw.client),
		},
		&extensions.Ingress{}, resyncPeriod, eventHandlers)
	return &iw, nil
}

func ingressListFunc(c *client.Client) func() (runtime.Object, error) {
	return func() (runtime.Object, error) {
		return c.Extensions().Ingress(api.NamespaceAll).List(labels.Everything(), fields.Everything())
	}
}

func ingressWatchFunc(c *client.Client) func(options api.ListOptions) (watch.Interface, error) {
	return func(options api.ListOptions) (watch.Interface, error) {
		return c.Extensions().Ingress(api.NamespaceAll).Watch(
			labels.Everything(), fields.Everything(), options)
	}
}

// StoreToIngressLister makes a Store that lists Ingress.
// TODO: Move this to cache/listers post 1.1.
type StoreToIngressLister struct {
	cache.Store
}

// List lists all Ingress' in the store.
func (s *StoreToIngressLister) List() (ing extensions.IngressList, err error) {
	for _, m := range s.Store.List() {
		ing.Items = append(ing.Items, *(m.(*extensions.Ingress)))
	}
	return ing, nil
}
