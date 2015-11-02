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
	"github.com/bprashanth/Ingress/lib"
	"github.com/golang/glog"
	"os"
	"text/template"
)

const workers = 1024

var (
	rawConf = `
events {
  worker_connections {{.WorkerConnections}};
}
http {
{{range $i, $svc := .Services}}
  server {
    listen {{$svc.Port}};
    server_name {{$svc.Host}};
    resolver 127.0.0.1;
{{if $svc.Crt }}
    ssl on;
    ssl_certificate {{$svc.Crt}};
    ssl_certificate_key {{$svc.Key}};
{{end}}
    location / {
      proxy_pass https://{{$svc.ClusterDNSName}};
    }
  }{{end}}
}`
)

type Conf struct {
	WorkerConnections int
	Services          []*lib.Service
}

func main() {
	conf := Conf{workers, []*lib.Service{}}
	snapshots := lib.StartHub("default", make(chan struct{}))
	glog.Infof("Subscribing to snapshots")
	for {
		snap := <-snapshots
		for _, svc := range snap.Services {
			glog.Infof("Received snapshot %+v", *svc)
		}
		conf.Services = snap.Services
		tp := template.New("test")
		temp, _ := tp.Parse(rawConf)
		temp.Execute(os.Stdout, conf)
	}
}
