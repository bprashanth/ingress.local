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

	"k8s.io/kubernetes/pkg/apis/extensions"
	client "k8s.io/kubernetes/pkg/client/unversioned"
)

const receiversKey = "Ingress.receivers"

type Receiver struct {
	Host string `json:"host"`
	Port int    `json:"port"`
	Cert string `json:"cert"`
}

type ingAnnotations map[string]string

func (i ingAnnotations) getReceivers() (string, bool) {
	s, ok := i[receiversKey]
	return s, ok
}

type AnnotatedReceivers struct {
	Client *client.Client
}

func (r *AnnotatedReceivers) Get(ingName, ingNamespace string) (rec []Receiver, err error) {
	// Get the Ingress, lookup it's receivers from annotations and return a decoded list.
	var ing *extensions.Ingress
	ing, err = r.Client.Experimental().Ingress(ingNamespace).Get(ingName)
	if err != nil {
		return
	}
	if jsonRec, ok := ingAnnotations(ing.Annotations).getReceivers(); ok {
		err = json.Unmarshal([]byte(jsonRec), &rec)
	}
	return
}

func (r *AnnotatedReceivers) Update(ingName, ingNamespace string, rec Receiver) error {
	// Get the Ingress, decode it's receivers, add new receiver, encode receiver list,
	// update annotations. We could call GetReceivers but there's a condition where we might
	// clobber during the update if we don't reuse this Ingress.
	var ing *extensions.Ingress
	var err error
	ing, err = r.Client.Experimental().Ingress(ingNamespace).Get(ingName)
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
	if _, err := r.Client.Experimental().Ingress(ingNamespace).Update(ing); err != nil {
		return err
	}
	return nil
}
