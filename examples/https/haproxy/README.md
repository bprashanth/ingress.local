Placeholder

## Running haproxy

Docker:
```shell
$ cat haproxyhttps.crt haproxyhttps.key > haproxyhttps.key
$ docker run -it -v "/tmp/haproxyhttps.pem:/etc/haproxy/ssl/haproxy.pem" -p 8082:443 bprashanth/haproxyhttps:0.0
```

```
servicelb [NOTICE] Proxy insecure started.
servicelb [NOTICE] Proxy insecure started.
servicelb [NOTICE] Proxy secure started.
servicelb [NOTICE] Proxy secure started.
servicelb [NOTICE] Proxy secure started.
servicelb [NOTICE] Proxy secure started.
servicelb [INFO] Connect from 127.0.0.1:34840 to 127.0.0.1:443 (secure/HTTP)
servicelb [INFO] Connect from 127.0.0.1:34970 to 127.0.0.1:443 (secure/HTTP)
servicelb [INFO] Connect from 127.0.0.1:35111 to 127.0.0.1:443 (secure/HTTP)
```


## Trouble shooting

If your logs show:
```
[ALERT] 284/232651 (13) : parsing [/etc/haproxy/haproxy.cfg:21] : 'bind *:443' :
  unable to load SSL private key from PEM file '/etc/haproxy/ssl/haproxyhttps.crt'.
  unable to load SSL certificate from PEM file '/etc/haproxy/ssl/haproxyhttps.key'.
```

If you see errors of the sort:
```
curl: (56) Recv failure: Connection reset by peer
```
you probably haven't opened up your container ports, or you have overlapping selectors in a way that kube-proxy is confused about where to send your requests.
