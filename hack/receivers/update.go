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

package main

import (
	flag "github.com/spf13/pflag"
	"os"
	"strings"

	client "k8s.io/kubernetes/pkg/client/unversioned"
	kubectl_util "k8s.io/kubernetes/pkg/kubectl/cmd/util"

	"github.com/bprashanth/Ingress/lib"
	"github.com/golang/glog"
)

var (
	flags = flag.NewFlagSet("", flag.ContinueOnError)

	host   = flags.String("host", "", "Hostname.")
	port   = flags.Int("port", 0, "Port number.")
	secret = flags.String("secret", "",
		`Name of secret. This secret must contain a .crt and .key.`)
	sslDir  = flags.String("sslDir", lib.defaultSSLDir, "Dir to put .crt and .key.")
	ingress = flags.String("ing", "", "Namespace/Name of ingress.")
)

func main() {
	flags.Parse(os.Args)
	if *port == 0 || *secret == "" || *host == "" || *ingress == "" {
		glog.Fatalf("Need more information to add receiver.")
	}
	fullName := strings.Split(*ingress, "/")
	if len(fullName) != 2 {
		glog.Fatalf("--ingress should take the form namespace/name.")
	}
	ingName := fullName[1]
	ingNamespace := fullName[0]
	rec := lib.Receiver{Host: *host, Port: *port, Secret: *secret, SSLDir: *sslDir}

	clientConfig := kubectl_util.DefaultClientConfig(flags)
	config, err := clientConfig.ClientConfig()
	if err != nil {
		glog.Fatalf("error connecting to the client: %v", err)
	}
	kubeClient, err := client.New(config)
	if err != nil {
		glog.Fatalf("error creating kube client %v", err)
	}

	ar := lib.ReceiverClient{kubeClient}
	if err := ar.Update(ingName, ingNamespace, rec); err != nil {
		glog.Fatalf("%v", err)
	}
	receivers, err := ar.Get(ingName, ingNamespace)
	if err != nil {
		glog.Fatalf("%v", err)
	}
	glog.Infof("Receivers of %v: %+v", *ingress, receivers)
}
