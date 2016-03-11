---
title: Using the Weave Net CNI Plugin
layout: default
---

The most popular container system that uses CNI is Kubernetes.

To create a network which can span multiple hosts, the Weave peers must be connected to each other, by specifying the other hosts during `weave launch` or via
[`weave connect`](/site/using-weave/finding-adding-hosts-dynamically.md).

See [Deploying Applications to Weave Net](/site/using-weave/deploying-applications.md#peer-connections) for a discussion on peer connections. 

After you've launched Weave and peered your hosts, you can configure
Kubernetes to use Weave.  There are two steps:

Step 1: configure `kubelet` to use CNI by starting it with the following options:

    --network-plugin=cni --network-plugin-dir=/etc/cni/net.d

See the [`kubelet` documentation](http://kubernetes.io/v1.1/docs/admin/kubelet.html)
for more details.

Step 2: create a CNI configuration file:

```
mkdir -p /etc/cni/net.d
$ cat >/etc/cni/net.d/10-weave.conf <<EOF
{
    "name": "weave-net",
    "type": "weave",
}
EOF
```

Now, whenever Kubernetes starts a pod, it will be attached to the Weave network.

By default, the Weave CNI plugin will add a default route out via the Weave bridge, so your containers can access resources on the internet.  If you do not want this, add a section to the config file specifying no routes:

```
    "ipam": {
        "routes": [ ]
    }
```


The following other fields in the [CNI Spec]](https://github.com/appc/cni/blob/master/SPEC.md#network-configuration) are supported:

- `ipam / gateway`
- `ipam / subnet`

###Caveats

The CNI plugin does not add entries to Weave DNS.
