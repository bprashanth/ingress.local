#! /bin/bash

# This test is for dev purposes. It reads like golang, deal with it.

set -e
k=${KUBECTL:-kubectl}
app=${APP:-nginxtest}

# curlNodePort gets all NodePorts for all ports of Services matching label app=$app and curls 1 node's external ip.
function curlNodePort {
  port=`$k get svc -l app=$1 -o template --template='{{range $.items}}{{range .spec.ports}}{{.nodePort}} {{end}}{{end}}'`
  node=`$k get nodes --template='{{range .items}}{{range .status.addresses}}{{if eq .type "ExternalIP"}}{{.address}} {{end}}{{end}}{{end}}' | awk '{print $1}'`
  for p in $port; do
      for n in $node; do
          echo curling $n:$p
          # TODO: check return code, i.e curl -s -o /dev/null -w "%{http_code}"
          curl $n:$p
      done
  done
}

# waitForPods waits till all pods with label app=$app leave Pending or Terminating.
function waitForPods {
    while [ `$k get pods -l app=$1 | grep "Terminating\|Pending" | wc -l` -ne 0 ]; do
        echo waiting for $1 pods
    done
    echo $1 pods no longer pending
}

function cleanup {
    $k delete rc,svc,pods -l app=$1
}


echo $k
$k cluster-info
cleanup $app
$k create -f nginx-app.yaml
$k get rc,svc,pods -l app=$app
waitForPods $app
curlNodePort $app
cleanup $app

