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
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"strings"

	"github.com/golang/glog"
	"k8s.io/kubernetes/pkg/apis/extensions"
	client "k8s.io/kubernetes/pkg/client/unversioned"

	// This installs the legacy v1 API
	_ "k8s.io/kubernetes/pkg/api/install"
)

const receiversKey = "Ingress.receivers"

// Receiver holds ssl/tls information for the Ingress.
type Receiver struct {
	Host   string `json:"host"`
	Port   int    `json:"port"`
	Secret string `json:"secret"`
	SSLDir string `json:"sslDir"`
}

type ingAnnotations map[string]string

func (i ingAnnotations) getReceivers() (string, bool) {
	s, ok := i[receiversKey]
	return s, ok
}

// ReceiverSecrets contains information about secrets in the filesystem.
// One converts a Receiver to a ReceiverSecret by retrieving and writing the
// corresponding Kubernetes secret to the ssldir as a .key/.crt file.
type ReceiverSecrets struct {
	Host    string
	Port    int
	KeyPath string
	CrtPath string
}

// ReceiverClient manages receiver annotations.
type ReceiverClient struct {
	Client *client.Client
}

// LoadSecrets loads all the secrets of the Ingress as .crt and .key files and returns a map of host to ReceiverSecrets.
func (r *ReceiverClient) LoadSecrets(ing extensions.Ingress) (map[string]ReceiverSecrets, error) {
	receiverSecrets := map[string]ReceiverSecrets{}
	receivers := []Receiver{}

	if jsonRec, ok := ingAnnotations(ing.Annotations).getReceivers(); ok {
		if err := json.Unmarshal([]byte(jsonRec), &receivers); err != nil {
			return receiverSecrets, err
		}
	}

	for _, rcv := range receivers {
		if err := os.MkdirAll(rcv.SSLDir, 0644); err != nil {
			return receiverSecrets, err
		}
		secret, err := r.Client.Secrets(ing.Namespace).Get(rcv.Secret)
		if err != nil {
			return receiverSecrets, err
		}
		rs := ReceiverSecrets{Host: rcv.Host, Port: rcv.Port}
		// TODO: Get rid of all this when the kubelet supports dynamic
		// secret loading.
		for k, v := range secret.Data {
			parts := strings.Split(k, ".")
			if len(parts) != 2 {
				continue
			}
			ext := parts[1]
			path := fmt.Sprintf("%v/%v.%v", rcv.SSLDir, parts[0], ext)
			if ext == "crt" {
				rs.CrtPath = path
			} else if ext == "key" {
				rs.KeyPath = path
			} else {
				continue
			}
			if _, err := os.Stat(path); err == nil {
				glog.Infof("%v already exists, not overwriting", path)
				continue
			}
			if err := ioutil.WriteFile(path, []byte(v), 0644); err != nil {
				return receiverSecrets, err
			}
		}
		if rs.KeyPath == "" || rs.CrtPath == "" {
			return receiverSecrets, fmt.Errorf("%v only had one of key/crt path.", rcv.Secret)
		}
		receiverSecrets[rcv.Host] = rs
	}
	return receiverSecrets, nil
}

// Get returns the receiver annotations of an Ingress.
func (r *ReceiverClient) Get(ingName, ingNamespace string) (rec []Receiver, err error) {
	// Get the Ingress, lookup it's receivers from annotations and return a decoded list.
	var ing *extensions.Ingress
	ing, err = r.Client.Extensions().Ingress(ingNamespace).Get(ingName)
	if err != nil {
		return
	}
	if jsonRec, ok := ingAnnotations(ing.Annotations).getReceivers(); ok {
		err = json.Unmarshal([]byte(jsonRec), &rec)
	}
	return
}

// Update updates receiver annotations on an Ingress.
func (r *ReceiverClient) Update(ingName, ingNamespace string, rec Receiver) error {
	// Get the Ingress, decode it's receivers, add new receiver, encode receiver list,
	// update annotations. We could call GetReceivers but there's a condition where we might
	// clobber during the update if we don't reuse this Ingress.
	var ing *extensions.Ingress
	var err error
	ing, err = r.Client.Extensions().Ingress(ingNamespace).Get(ingName)
	if err != nil {
		return err
	}
	var receivers []Receiver
	if jsonRec, ok := ingAnnotations(ing.Annotations).getReceivers(); ok {
		if err := json.Unmarshal([]byte(jsonRec), &receivers); err != nil {
			return err
		}
	}
	newReceiver := true
	for i := range receivers {
		if receivers[i].Host == rec.Host {
			receivers[i] = rec
			newReceiver = false
		}
	}
	if newReceiver {
		receivers = append(receivers, rec)
	}
	jsonReceivers, err := json.Marshal(receivers)
	if err != nil {
		return err
	}
	ing.Annotations[receiversKey] = string(jsonReceivers)
	if _, err := r.Client.Extensions().Ingress(ingNamespace).Update(ing); err != nil {
		return err
	}
	return nil
}
