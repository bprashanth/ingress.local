#! /bin/bash

# This test is for dev purposes. It reads like golang, deal with it.

set -e
source ../../hack/testlib.sh
APP=${APP:-nginxhttps}
PUSH=${PUSH:-false}
HOSTS=${HOST:-nginxhttps}

function setup {
    cleanup "${APP}"
    makeCerts ${APP} ${HOSTS[*]}
    if "${PUSH}"; then
        make push
    fi
    "${K}" create -f nginx-https.yaml
    waitForPods "${APP}"
}

function run {
    frontendIP=`getNodeIPs frontend`
    echo Frontend ip ${frontendIP[*]}

    for h in ${HOSTS[*]}; do
        for ip in ${frontendIP[*]}; do
            curlHTTPSWithHost $h 8082 $ip $h.crt
            curlNodePort "${APP}"
        done
    done
    cleanup "${APP}"
}

setup
run
