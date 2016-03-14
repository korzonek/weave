---
title: Using the Weave Net CNI Plugin
layout: default
---

The most popular container system that uses CNI is Kubernetes.

To create a network which can span multiple hosts, the Weave peers must be connected to each other, by specifying the other hosts during `weave launch` or via
[`weave connect`](/site/using-weave/finding-adding-hosts-dynamically.md).

See [Deploying Applications to Weave Net](/site/using-weave/deploying-applications.md#peer-connections) for a discussion on peer connections. 

As well as launching Weave Net, you have to run an extra command to
configure the Weave bridge:

    weave launch <peer hosts>
    weave expose

After you've launched Weave and peered your hosts, you can configure
Kubernetes to use Weave.  There are two steps:

Step 1: configure `kubelet` to use CNI by starting it with the following options:

    --network-plugin=cni --network-plugin-dir=/etc/cni/net.d

See the [`kubelet` documentation](http://kubernetes.io/v1.1/docs/admin/kubelet.html)
for more details.

Step 2: create a CNI configuration file:

```
mkdir -p /etc/cni/net.d
cat >/etc/cni/net.d/10-weave.conf <<EOF
{
    "name": "weave-net",
    "type": "weave"
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

- `ipam / type` - default is to use Weave's own IPAM
- `ipam / subnet` - default is to use Weave's IPAM default subnet
- `ipam / gateway` - default is to use the Weave bridge IP address (allocated by `weave expose`)

###Caveats

- The Weave router container must be running for CNI to allocate addresses
- The CNI plugin does not add entries to Weave DNS.

###Walkthrough of trying out Weave

Starting from http://kubernetes.io/docs/getting-started-guides/vagrant/,
and using Kubernetes at least version 1.2:

Run:

    export KUBERNETES_MEMORY=2048
    curl -sS https://get.k8s.io | bash
    cd kubernetes

[I decided to raise the amount of RAM allocated to the Vagrant VMs
because processes were getting killed due to out-of-memory errors]

Shut down `flanneld`:

    vagrant ssh master
    rm /etc/systemd/system/docker.service.requires/flanneld.service
    sudo systemctl stop flanneld
    sudo systemctl disable flanneld
    exit

    vagrant ssh node-1
    rm /etc/systemd/system/docker.service.requires/flanneld.service
    sudo systemctl stop flanneld
    sudo systemctl disable flanneld
    exit

Install Weave Net (all running as root)

    curl https://raw.githubusercontent.com/weaveworks/weave/master/weave -o /usr/local/bin/weave
    chmod a+x /usr/local/bin/weave
    mkdir -p /opt/cni/bin
    WEAVE_VERSION=git-3ada8e540b1f /usr/local/bin/weave setup
    WEAVE_VERSION=git-3ada8e540b1f /usr/local/bin/weave launch 10.245.1.2 10.245.1.3
    WEAVE_VERSION=git-3ada8e540b1f /usr/local/bin/weave expose
    docker cp weaveplugin:/home/weave/plugin /opt/cni/bin/weave
    docker cp weaveplugin:/home/weave/plugin /opt/cni/bin/weave-ipam
    mkdir -p /etc/cni/net.d
    cat  >/etc/cni/net.d/10-weave.conf <<EOF
    {
        "name": "weave-net",
        "type": "weave"
    }
    EOF
    sed -i -e's%--api-servers%--network-plugin=cni --network-plugin-dir=/etc/cni/net.d --api-servers%' /etc/sysconfig/kubelet
    systemctl daemon-reload
    systemctl restart kubelet




???

[root@kubernetes-node-1 vagrant]# WEAVE_VERSION=git-3ada8e540b1f /usr/local/bin/weave launch 10.245.1.2 10.245.1.3
Usage of loopback devices is strongly discouraged for production use. Either use `--storage-opt dm.thinpooldev` or use `--storage-opt dm.no_warn_on_loop_devices=true` to suppress this warning.
