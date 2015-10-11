#! /bin/bash

# This test is for dev purposes. It reads like golang, deal with it.

set -e
source ../../testlib.sh
app=${APP:-nginxtest}

echo $k
checkCluster
cleanup $app
$k create -f nginx-app.yaml
$k get rc,svc,pods -l app=$app
waitForPods $app
curlNodePort $app
cleanup $app

