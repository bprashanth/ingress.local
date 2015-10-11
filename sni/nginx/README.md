for h in nginx nginx1 nginx2; do make keys secret CERT=$h.crt KEY=$h.key HOST=$h SECRET=$h.json; kl create -f $h.json; done
docker build -t bprashanth/nginxrouter:1.0 .
docker push bprashanth/nginxrouter:1.0
k create -f .
curl --resolve nginx:8082:104.197.79.157 https://nginx:8082 --cacert nginx.crt

