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

## Additional features

The `clusterf` code additionally supports the use of *routed backends*, to redirect traffic to a specific backend via some middle tier.
This feature enables the separaration of the IPVS traffic handling into two tiers: a scaleable and fault-tolerant stateless frontend using IPVS `droute` forwarding, plus a simple-to-configure stateful backend using IPVS `masq` forwarding.

## Known issues

*   Dead service backends are not cleaned up.
    The `clusterf-docker` daemon will remove any containers that are stopped, but a dead `clusterf-docker` daemon or docker host will result in
    ghost backends in etcd.
*   Multiple ServiceBackends routed to the same ipvsBackend are not coleasced.
    This will cause issues with the IPVS backend state for all affected service backends being removed as soon as the first colliding service backend is removed.
*   Route updates are not propagated to backends.
    Adding/Updating/Removing a route will not be reflected in the IPVS state until the service backends are updated.
*   The `clusterf-docker` daemon is limited in terms of the policy configuration available.
    It needs support for different network topologies, such as Docker's traditional "published" NAT ports.
*   Hairpinning to allow access to local backends from docker containers on the same host requires some work to deal with asymmetric routing on the docker host bridge.
*   IPv6 configuration is partially supported in the `clusterf-ipvs` code, but untested. The `clusterf-docker` code is lacking IPv6 configuration support.

## Future ideas

*   Implement a docker networking extension to configure the public VIP directly within the docker container.
    Removes the need for DNAT on the docker host, as forwaded traffic can be routed directly to the container.
