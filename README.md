# Cluster Frontend

`clusterf` is a clustered L3 loadbalancer control plane.
It uses [CoreOS etcd](https://github.com/coreos/etcd) as a configuration backend, supports the Linux built-in [IPVS](http://www.linuxvirtualserver.org/software/ipvs.html) TCP/UDP load balancer, and provides [Docker](https://www.docker.com/) integration.

The `clusterf-docker` daemon runs on the docker hosts, and enumerates the Docker API for running containers to synchronizes any labeled services into the etcd `/clusterf` configuration store. The daemon continues to listen for Docker events to update any container state changes to the backend configurations, adding and removing backends as existing containers go away or new containers are started.

The `clusterf-ipvs` daemon runs on the cluster frontend hosts with external connectivity, and enumerates configured service frontend+backends from the etcd `/clusterf` configuration store to synchronizes the in-kernel IPVS configuration. The daemon continues to watch for etcd changes to update the live IPVS service state.

The system essentially acts as a L4-aware L3 routed network, routing packets at the L3 layer based on L4 information.

## Highlights

The use of `etcd` as a distributed share configuration backend allows the seamless operation of multiple `clusterf-docker` hosts and multiple `clusterf-ipvs` hosts, with changes to service state on backend nodes being immediately propagated to all frontend nodes.

In terms of performance, the `clusterf` daemons act as a control-plane only: the actual packet-handling data plane is implemented by the IPVS code inside the Linux kernel, and forwarded packets do not need to pass through user-space.

## Example

    $ sudo ipvsadm
    IP Virtual Server version 1.2.1 (size=4096)
    Prot LocalAddress:Port Scheduler Flags
      -> RemoteAddress:Port           Forward Weight ActiveConn InActConn
      
    $ etcdctl set /clusterf/services/test/frontend '{"ipv4": "10.107.107.107", "tcp": 1337}'
    {"ipv4": "10.107.107.107", "tcp": 1337}
    $ sudo ipvsadm
    IP Virtual Server version 1.2.1 (size=4096)
    Prot LocalAddress:Port Scheduler Flags
      -> RemoteAddress:Port           Forward Weight ActiveConn InActConn
    TCP  10.107.107.107:1337 wlc

    $ etcdctl set /clusterf/services/test/backends/test3-1 '{"ipv4": "10.3.107.1", "tcp": 1337}'
    {"ipv4": "10.3.107.1", "tcp": 1337}
    $ sudo ipvsadm
    IP Virtual Server version 1.2.1 (size=4096)
    Prot LocalAddress:Port Scheduler Flags
      -> RemoteAddress:Port           Forward Weight ActiveConn InActConn
    TCP  10.107.107.107:1337 wlc
      -> 10.3.107.1:1337              Masq    10     0          0         

    $ etcdctl set /clusterf/services/test/backends/test3-2 '{"ipv4": "10.3.107.2", "tcp": 1337}'
    {"ipv4": "10.3.107.2", "tcp": 1337}
    $ sudo ipvsadm -L -n
    IP Virtual Server version 1.2.1 (size=4096)
    Prot LocalAddress:Port Scheduler Flags
      -> RemoteAddress:Port           Forward Weight ActiveConn InActConn
    TCP  10.107.107.107:1337 wlc
      -> 10.3.107.1:1337              Masq    10     0          0         
      -> 10.3.107.2:1337              Masq    10     0          0         

    $ etcdctl set /clusterf/services/test/backends/test3-2 '{"ipv4": "10.3.107.2", "tcp": 1338}'
    {"ipv4": "10.3.107.2", "tcp": 1338}
    $ sudo ipvsadm -L -n
    IP Virtual Server version 1.2.1 (size=4096)
    Prot LocalAddress:Port Scheduler Flags
      -> RemoteAddress:Port           Forward Weight ActiveConn InActConn
    TCP  10.107.107.107:1337 wlc
      -> 10.3.107.1:1337              Masq    10     0          0         
      -> 10.3.107.2:1338              Masq    10     0          0         

    $ etcdctl rm --recursive /clusterf/services/test
    $ sudo ipvsadm -L -n
    IP Virtual Server version 1.2.1 (size=4096)
    Prot LocalAddress:Port Scheduler Flags
      -> RemoteAddress:Port           Forward Weight ActiveConn InActConn

## Docker integration

The docker integration uses container (image) labels:

    net.qmsk.clusterf.service=$service
    net.qmsk.clusterf.backend.tcp=$port
    net.qmsk.clusterf.backend.udp=$port

A container can also be a backend in multiple different services:

    net.qmsk.clusterf.service="$service1 $service2"
    net.qmsk.clusterf.backend:$service.tcp=$port
    net.qmsk.clusterf.backend:$service.udp=$port

As an example:

    docker run --rm -it --expose 8080 -l net.qmsk.clusterf.service=test -l net.qmsk.clusterf.backend.tcp=8080 ...

The ports must be EXPOSE'd on the container, but do not necessarily need to be published. The backend will be configured using the internal address of the container.

## Additional features

### Local configuration

The `clusterf-ipvs --config-source=file:///...` flag can be used to load configuration from a local filesystem tree, which is merged with the configuration in etcd. The structure of the configuration nodes is the same as in etcd.

This can be used to customize the set of services/routes per node.

### Forwarding configuration

The forwarding method for IPVS destinations can be configured in aggregate for different sets of backends via `/clusterf/routes/...`, using IPv4 address *prefix* information to represent the network topology:

    $ etcdctl get /clusterf/routes/test3
    {"Prefix":"10.3.107.0/24","IPVSMethod":"masq"}

This means that any backends configured under `10.3.107.0/24` will be configured with an IPVS *masq* forwarding-method.

### Routed backends

The `clusterf` code additionally supports the use of *routed backends*, to redirect traffic to a set of backends via some intermediate *gateway*:

    {"Prefix":"10.6.107.0/24",Gateway":"10.107.107.6","IPVSMethod":"droute"}

The backend's IPVS dest will be added using the given *gateway* address (retaining the service's frontend port) in place of the dest's *host:port* address.

This feature enables the separaration of the IPVS traffic handling into two tiers: a scaleable and fault-tolerant stateless frontend tier using IPVS `droute` forwarding, plus a simple-to-configure stateful intermediate tier using IPVS `masq` forwarding.

The `clusterf-ipvs --filter-routes=file://` flag can be used to override any routes in etcd on the intermediate tier, which can be used to limit IPVS destinations to local backends only.

The `clusterf-docker --route-*` flags can be used to advertise routes for local docker networks into etcd for use by the frontend IPVS tier.

### Weighted backends

Each backend can define its own weight, which can be updated at runtime. Backends with a higher weight will recieve proportionally more connections.

A backend weight of zero will prevent new connections being scheduled for the backend, allowing existing connections to continue.

This is used in `clusterf-docker` for graceful container shutdowns. Containers going through the *kill* -> *die* -> *stop* lifecycle will be marked as not running and have their weight set to zero while stopping, before being removed. See [Issue #5](https://github.com/qmsk/clusterf/issues/5) for an example.

### Backend merging

Overlapping backends are merged. This will happen if multiple backends for a given service resolve to the same IPVS host:port, typically as a result of a route aggregating a set of backends to an intermediate frontend.

The merging is based on the backend weight. The IPVS weight of the merged destination is calculated from the weights of all merged backends, and updated as backends are added/removed/reweighted.

## Known issues

*   The `clusterf-docker` daemon is limited in terms of the policy configuration available. It assumes the docker networks are globally addressed and routable from the frontend.
*   `{"IPVSMethod":"masq"}` does not work with hairpinning from docker containers to backends running on the same host. This would require workarounds to deal with the asymmetric routing across the docker host bridge.
*   IPv6 configuration is supported, but untested. IPv4 -> IPv6 frontend/backends are in theory supported, but untested and pending IPVS support for the kernel interface.

## Future ideas

*   Implement a docker networking extension to configure the public VIP directly within the docker container.
    Removes the need for DNAT on the docker host, as forwaded traffic can be routed directly to the container.
*   Implement health-checking for service backends within `clusterf-ipvs`. Currently we rely on `clusterf-docker` removing backends for failed docker containers, and etcd TTL expiry if the docker host fails. See [Issue #1](https://github.com/qmsk/clusterf/issues/1).

## Acknowledgments

This work was supported by the Academy of Finland project ["Cloud Security Services" (CloSe)](https://wiki.aalto.fi/display/CloSeProject/CloSe+Project+Public+Homepage) at Aalto University Department of Communications and Networking.
