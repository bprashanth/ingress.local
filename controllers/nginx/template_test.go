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
	"bufio"
	"bytes"
	"fmt"
	"github.com/bprashanth/Ingress/lib"
	"text/template"
)

func ExampleTemplate() {
	tp := template.New("test")
	temp, _ := tp.Parse(rawConf)
	update := Conf{
		WorkerConnections: 1024,
		Services: []*lib.Service{
			{Host: "_", Port: 443, Crt: "/etc/nginx/wildcard.crt", Key: "/etc/nginx/wildcard.key", ClusterDNSName: "catchall"},
			{Host: "foo", Port: 80, Crt: "", Key: "/etc/nginx/foo.key", ClusterDNSName: "foosvc"},
		},
	}
	var b bytes.Buffer
	out := bufio.NewWriter(&b)
	temp.Execute(out, update)
	out.Flush()
	fmt.Println(b.String())
	// Output:
	// events {
	//   worker_connections 1024;
	// }
	// http {
	//
	//   server {
	//     listen 443;
	//     server_name _;
	//     resolver 127.0.0.1;
	//
	//     ssl on;
	//     ssl_certificate /etc/nginx/wildcard.crt;
	//     ssl_certificate_key /etc/nginx/wildcard.key;
	//
	//     location / {
	//       proxy_pass https://catchall;
	//     }
	//   }
	//   server {
	//     listen 80;
	//     server_name foo;
	//     resolver 127.0.0.1;
	//
	//     location / {
	//       proxy_pass https://foosvc;
	//     }
	//   }
	// }
}
