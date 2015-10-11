#! /bin/bash

# This test is for dev purposes. It reads like golang, deal with it.

set -e
source ../../testlib.sh
app=${APP:-nginxsni}
hosts=(nginx1 nginx2 nginx3)
push=${PUSH:-false}

cleanup nginxsni
makeCerts ${hosts[*]}
if $push; then
    make push
fi

$k create -f nginx-sni.yaml
waitForPods $app

frontendIP=`getNodeIPs frontend`
echo Frontend ip ${frontendIP[*]}

for h in ${hosts[*]}; do
    for ip in ${frontendIP[*]}; do
        curlHTTPSWithHost $h 8082 $ip $h.crt
    done
done


