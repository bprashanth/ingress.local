
### Testing

#### Setup

First clean your environment so you get accurate results by delete all services, rcs and pods.
```shell
$ kubectl delete rc,svc,pods --all --namespace=kube-system
```

Check the nodes, you should see something like:
```shell
$ kubectl describe node nodename
Capacity:
 cpu:		2
 memory:	7679608Ki
 pods:		100
...

Non-terminated Pods:		(2 in total)
  Namespace			Name								CPU Requests	CPU Limits	Memory Requests	Memory Limits
  ─────────			────								────────────	──────────	───────────────	─────────────
  kube-system			fluentd-elasticsearch-e2e-test-beeps-minion-ugv1		100m (5%)	100m (5%)	0 (0%)		0 (0%)
  kube-system			kube-registry-proxy-e2e-test-beeps-minion-ugv1			100m (5%)	100m (5%)	50Mi (0%)	50Mi (0%)
```

Indicating that we have a node with 2 cpus and ~7G memory at our disposal. The only pods on the node are the 2 static pods.

#### Single webserver

Create the nginx test app:
```shell
$ kubectl create -f nginx-app.yaml
$ kubectl get pods
NAME              READY     STATUS    RESTARTS   AGE
nginxtest-6s73g   1/1       Running   0          1m
$ kubectl get svc
NAME         CLUSTER_IP    EXTERNAL_IP   PORT(S)   SELECTOR        AGE
kubernetes   10.0.0.1      <none>        443/TCP   <none>          9m
nginxtest    10.0.16.100   nodes         80/TCP    app=nginxtest   1m

$ kubectl get svc -o yaml | grep -i "nodeport: " -C 4
spec:
  clusterIP: 10.0.16.100
  ports:
  - name: http
    nodePort: 30504
    port: 80
    protocol: TCP
    targetPort: 80

$ kubectl get nodes -o yaml | grep -i externalip -B 2
      type: InternalIP
    - address: 104.154.43.116
      type: ExternalIP
--
      type: InternalIP
    - address: 23.236.59.51
      type: ExternalIP
--
      type: InternalIP
    - address: 104.197.5.239
      type: ExternalIP

$ curl http://104.154.43.116:30504
<pre>
INTRODUCTION
============
...
```

So we have a webserver to test. The root page should give you enough context about the server itself.
Lets see how fast we can fetch an image:
```shell
$ docker run jordi/ab ab -n 100 -ck 100 "http://104.154.43.116:30504/fs/images/nginx.jpg"
Concurrency Level:      100
Time taken for tests:   1.262 seconds
Complete requests:      100
Failed requests:        0
Total transferred:      2622100 bytes
HTML transferred:       2597800 bytes
Requests per second:    79.24 [#/sec] (mean)

```

That creates 100 requests in parallel with http keep alive, and measures the qps. You can see we're able to handle ~80 qps.
Now keep bumping up the concurrency till the webserver falls over. For me this happens at 800 concurrent requests:
```shell
$ docker run jordi/ab ab -n 1000 -c 700 "http://104.154.43.116:30504/fs/images/nginx.jpg"
Failed requests:        0
Total transferred:      26221000 bytes
HTML transferred:       25978000 bytes
Requests per second:    316.98 [#/sec] (mean)

This is ApacheBench, Version 2.3 <$Revision: 1604373 $>
Copyright 1996 Adam Twiss, Zeus Technology Ltd, http://www.zeustech.net/
Licensed to The Apache Software Foundation, http://www.apache.org/

Benchmarking 104.154.43.116 (be patient)
Completed 100 requests
Completed 200 requests
Completed 300 requests
Completed 400 requests
Completed 500 requests
Completed 600 requests
Completed 700 requests
apr_socket_recv: Connection reset by peer (104)
Total of 778 requests completed
```

There are multiple things you can do to tune the system. We will not get into any of them. The point is,
it is pretty easy to render a system temporarily unusable by pushing it to the limit. By way of example, you can force
the virtual memory manager to thrash (milage may vary depending on your pod resource limits):
```
$ docker run jordi/ab ab -n 1000 -c 700 "http://104.154.43.116:30504/stress?cpu=3&bytes=100M"
```

Try accessing the same /fs/images endpoint while that's running, or even after it's done.

