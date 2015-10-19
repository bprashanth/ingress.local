# Ingress

Ingress a word derived from the verb *ingredi*, and means:
> ### in·gress
__ˈinˌɡres/__
  *noun*
 1. a place or means of access; an entrance.
    *synonyms*:	entry, entrance, entryway, entrée, access, admittance, admission; More

An Ingress in Kubernetes is a REST object that allows you to configure an entrance to your cluster. Typically, Services have private IPs and the internet is oblivious to pods in your cluster. Any traffic that ends up at an edge routers is dropped or sent elsewhere, because nothing in the cluster has a public address.

```
    internet
        |
   -----------
  [ Services ]
```
Where:
* `|` is a pipe with bi-directional traffic
* A logical grouping is denoted by `[ ]`
* Everything after `----` has a cluster private IP

This is not very useful unless all your clients are going to access the Services from within the cluster. The role of Ingress is to punch holes through the edge.

```
    internet
        |
   [ Ingress ]
   --|-----|--
   [ Services ]
```

You must be very strategic when doing so, because the internet is big. You need to tell the Ingress whome to admit and where to send them (or perhaps more importantly, whome *not* to admit, and where *not* to allow them). Typically, one does this through a loadbalancer such as HAProxy, Nginx, or one from your cloud provider.

```
    internet
        |
   [ HAProxy ]
   --|-----|--
   [ Services ]
```

Doing this is as simple as deploying a [single pod](#service-loadbalancer/README.md). But that doesn't give you much control over the "holes", every Service gets a single path into the cluster, they *must* all share a single Hostname, and if the node the HAProxy pod is running on (for it has to run *somewhere*) goes down, your Services are once again dead to the world. This is the problem the Ingress resource solves. Ideally we would like to tell the HAProxy instance (referred to from here on as an L7 loadbalancer) how to configure itself without waiting for a Service on the other side, or perhaps more importantly, how to reconfigure itself even when the Service hasn't changed.


```
    internet
        |
   [L4/router]
    /      \
 [L7]     [L7]
  |        |
[  frontends  ]
--|---------|--
```


![Build Status](https://travis-ci.org/bprashanth/Ingress.svg)
