package main

import (
    "flag"
    "fmt"
    "qmsk.net/clusterf/ipvs"
    "log"
    "net"
    "os"
    "qmsk.net/clusterf/server"
    "syscall"
)

var (
    etcdConfig server.EtcdConfig
    ipvsDebug   bool
)

func init() {
    flag.StringVar(&etcdConfig.Machines, "etcd-machines", "http://127.0.0.1:2379",
        "Client endpoint for etcd")
    flag.StringVar(&etcdConfig.Prefix, "etcd-prefix", "/clusterf",
        "Etcd tree prefix")

    flag.BoolVar(&ipvsDebug, "ipvs-debug", false,
        "IPVS debugging")
}

type serviceType struct {
    af      uint16
    proto   uint16
}

type ipvsKey struct {
    serviceName string
    serviceType serviceType

    backendName string
}

type ipvsState struct {
    client      *ipvs.Client

    services    map[ipvsKey]ipvs.Service
    dests       map[ipvsKey]ipvs.Dest
}

func (self *ipvsState) init() {
    self.services = make(map[ipvsKey]ipvs.Service)
    self.dests = make(map[ipvsKey]ipvs.Dest)
}

type Server struct {
    etcd        *server.Etcd

    ipvs        ipvsState
}

var serviceTypes = []serviceType {
    { syscall.AF_INET,      syscall.IPPROTO_TCP },
    { syscall.AF_INET6,     syscall.IPPROTO_TCP },
    { syscall.AF_INET,      syscall.IPPROTO_UDP },
    { syscall.AF_INET6,     syscall.IPPROTO_UDP },
}

func protoString (proto uint16) string {
    switch (proto) {
    case syscall.IPPROTO_TCP:   return "TCP"
    case syscall.IPPROTO_UDP:   return "UDP"
    default: return fmt.Sprintf("%d", proto)
    }
}

func (self *ipvsState) printIPVS () {
    if services, err := self.client.ListServices(); err != nil {
        log.Fatalf("ipvs.ListServices: %v\n", err)
    } else {
        fmt.Printf("Proto                           Addr:Port\n")
        for _, service := range services {
            fmt.Printf("%-5s %30s:%-5d %s\n",
                protoString(service.Protocol),
                service.Addr, service.Port,
                service.SchedName,
            )

            if dests, err := self.client.ListDests(service); err != nil {
                log.Fatalf("ipvs.ListDests: %v\n", err)
            } else {
                for _, dest := range dests {
                    fmt.Printf("%5s %30s:%-5d %#04x\n",
                        "",
                        dest.Addr, dest.Port,
                        dest.FwdMethod,
                    )
                }
            }
        }
    }
}

func syncServiceType (ipvsService *ipvs.Service, frontend *server.ServiceFrontend) (bool, error) {
    switch ipvsService.Af {
    case syscall.AF_INET:
        if frontend.IPv4 == "" {
            return false, nil
        } else if ip := net.ParseIP(frontend.IPv4); ip == nil {
            return false, fmt.Errorf("Invalid IPv4: %s", frontend.IPv4)
        } else if ip4 := ip.To4(); ip4 == nil {
            return false, fmt.Errorf("Invalid IPv4: %s", ip)
        } else {
            ipvsService.Addr = ip4
        }
    case syscall.AF_INET6:
        if frontend.IPv6 == "" {
            return false, nil
        } else if ip := net.ParseIP(frontend.IPv6); ip == nil {
            return false, fmt.Errorf("Invalid IPv6: %s", frontend.IPv6)
        } else if ip16 := ip.To16(); ip16 == nil {
            return false, fmt.Errorf("Invalid IPv6: %s", ip)
        } else {
            ipvsService.Addr = ip16
        }
    }

    switch ipvsService.Protocol {
    case syscall.IPPROTO_TCP:
        if frontend.TCP == 0 {
            return false, nil
        } else {
            ipvsService.Port = frontend.TCP
        }
    case syscall.IPPROTO_UDP:
        if frontend.UDP == 0 {
            return false, nil
        } else {
            ipvsService.Port = frontend.UDP
        }
    default:
        panic("invalid proto")
    }

    return true, nil
}

func syncDestType (ipvsDest *ipvs.Dest, backend *server.ServiceBackend, ipvsService ipvs.Service) (bool, error) {
    // only set the dest if the service is set
    if ipvsService.Addr == nil || ipvsService.Port == 0 {
        ipvsDest.Addr = nil
        ipvsDest.Port = 0

        return false, nil
    }

    switch ipvsService.Af {
    case syscall.AF_INET:
        if backend.IPv4 == "" {
            return false, nil
        } else if ip := net.ParseIP(backend.IPv4); ip == nil {
            return false, fmt.Errorf("Invalid IPv4: %s", backend.IPv4)
        } else if ip4 := ip.To4(); ip4 == nil {
            return false, fmt.Errorf("Invalid IPv4: %s", ip)
        } else {
            ipvsDest.Addr = ip4
        }
    case syscall.AF_INET6:
        if backend.IPv6 == "" {
            return false, nil
        } else if ip := net.ParseIP(backend.IPv6); ip == nil {
            return false, fmt.Errorf("Invalid IPv6: %s", backend.IPv6)
        } else if ip16 := ip.To16(); ip16 == nil {
            return false, fmt.Errorf("Invalid IPv6: %s", ip)
        } else {
            ipvsDest.Addr = ip16
        }
    default:
        panic("invalid af")
    }

    switch ipvsService.Protocol {
    case syscall.IPPROTO_TCP:
        if backend.TCP == 0 {
            return false, nil
        } else {
            ipvsDest.Port = backend.TCP
        }
    case syscall.IPPROTO_UDP:
        if backend.UDP == 0 {
            return false, nil
        } else {
            ipvsDest.Port = backend.UDP
        }
    default:
        panic("invalid proto")
    }

    return true, nil
}


func (self *ipvsState) syncService (service *server.Service, frontend *server.ServiceFrontend) {
    log.Printf("Sync %s: Frontend %+v\n", service.Name, frontend)

    ipvsService := ipvs.Service{
        SchedName:  "wlc",
        Timeout:    0,
        Flags:      ipvs.Flags{Flags: 0, Mask: 0xffffffff},
        Netmask:    0xffffffff,
    }

    for _, serviceType := range serviceTypes {
        ipvsService.Af = serviceType.af
        ipvsService.Protocol = serviceType.proto

        serviceActive, err := syncServiceType(&ipvsService, frontend)
        if err != nil {
            log.Printf("syncServiceType %s (%+v): %s\n", service.Name, serviceType, err)
            continue
        }

        serviceKey := ipvsKey{serviceName: service.Name, serviceType: serviceType}
        getService, serviceExists := self.services[serviceKey]

        if serviceExists && !serviceActive {
            // clear any existing instance
            if err := self.client.DelService(getService); err != nil  {
                log.Printf("ipvs.DelService %s: %s\n", service.Name, err)
            }

            delete(self.services, serviceKey)

        } else if serviceActive { // XXX: && (!serviceExists || ipvsService != getService) {
            // ensure service exists
            if err := self.client.NewService(ipvsService); err != nil  {
                log.Printf("ipvs.NewService %s: %s\n", service.Name, err)
            }

            self.services[serviceKey] = ipvsService

            // backing servers
            for backendName, backend := range service.Backends {
                // XXX: not with inner serviceType's
                self.syncServiceBackend(service, backendName, backend)
            }

            // flush old
            if serviceExists {
                if err := self.client.DelService(getService); err != nil  {
                    log.Printf("ipvs.DelService %s: %s\n", service.Name, err)
                }
            }
        }
    }
}

func (self *ipvsState) delService (service *server.Service, frontend *server.ServiceFrontend) {
    log.Printf("Delete %s: Frontend %+v\n", service.Name, frontend)

    for _, serviceType := range serviceTypes {
        serviceKey := ipvsKey{serviceName: service.Name, serviceType: serviceType}
        getService, serviceExists := self.services[serviceKey]

        if serviceExists {
            // clear any existing instance
            if err := self.client.DelService(getService); err != nil  {
                log.Printf("ipvs.DelService %s: %s\n", service.Name, err)
            }

            delete(self.services, serviceKey)
        }
    }
}

func (self *ipvsState) syncServiceBackend (service *server.Service, backendName string, backend *server.ServiceBackend) {
    log.Printf("Sync Service %s: Backend %s: %+v\n", service.Name, backendName, backend)

    ipvsDest := ipvs.Dest{
        FwdMethod:  ipvs.IP_VS_CONN_F_MASQ,
        Weight:     10,
    }

    for _, serviceType := range serviceTypes {
        serviceKey := ipvsKey{serviceName: service.Name, serviceType: serviceType}
        ipvsService := self.services[serviceKey]

        destKey := ipvsKey{serviceName: service.Name, serviceType: serviceType, backendName: backendName}
        getDest, destExists := self.dests[destKey]

        destActive, err := syncDestType(&ipvsDest, backend, ipvsService) // XXX: serviceExists
        if err != nil {
            log.Printf("syncDestType %s/%s (%+v): %s\n", service.Name, backendName, serviceType, err)
            continue
        }

        if destExists && !destActive {
            if err := self.client.DelDest(ipvsService, getDest); err != nil  {
                log.Printf("ipvs.DelDest %s %s: %s\n", service.Name, backendName, err)
            }

            delete(self.services, destKey)
        } else if destActive { // XXX: && (!destExists || ipvsDest != getDest) {
            if err := self.client.NewDest(ipvsService, ipvsDest); err != nil {
                log.Printf("ipvs.NewDest %s %s: %s\n", service.Name, backendName, err)
                continue
            }

            self.dests[destKey] = ipvsDest

            // flush old
            if destExists {
                if err := self.client.DelDest(ipvsService, getDest); err != nil  {
                    log.Printf("ipvs.DelDest %s %s: %s\n", service.Name, backendName, err)
                }
            }
        }
    }
}

func (self *ipvsState) delServiceBackend (service *server.Service, backendName string, backend *server.ServiceBackend) {
    log.Printf("Delete Service %s: Backend %s: %+v\n", service.Name, backendName, backend)

    for _, serviceType := range serviceTypes {
        serviceKey := ipvsKey{serviceName: service.Name, serviceType: serviceType}
        ipvsService := self.services[serviceKey]

        destKey := ipvsKey{serviceName: service.Name, serviceType: serviceType, backendName: backendName}
        getDest, destExists := self.dests[destKey]

        if destExists {
            if err := self.client.DelDest(ipvsService, getDest); err != nil  {
                log.Printf("ipvs.DelDest %s %s: %s\n", service.Name, backendName, err)
            }

            delete(self.services, destKey)
        }
    }
}

func main() {
    flag.Parse()

    if len(flag.Args()) > 0 {
        flag.Usage()
        os.Exit(1)
    }

    var self Server
    self.ipvs.init()

    // IPVS
    if ipvsClient, err := ipvs.Open(); err != nil {
        log.Fatalf("ipvs.Open: %v\n", err)
    } else {
        log.Printf("ipvs.Open\n")

        if ipvsDebug {
            ipvsClient.SetDebug()
        }

        if info, err := ipvsClient.GetInfo(); err != nil {
            log.Fatalf("ipvs.GetInfo: %v\n", err)
        } else {
            log.Printf("ipvs.GetInfo: version=%s, conn_tab_size=%d\n", info.Version, info.ConnTabSize)
        }

        self.ipvs.client = ipvsClient
    }

    if err := self.ipvs.client.Flush(); err != nil {
        log.Fatalf("ipvs.Flush: %v\n", err)
    }

    // etcd
    if etcdClient, err := etcdConfig.Open(); err != nil {
        log.Fatalf("etcd.Open: %s\n", err)
    } else {
        log.Printf("etcd.Open: %s\n", etcdClient)

        self.etcd = etcdClient

        // start
        if services, err := self.etcd.Scan(); err != nil {
            log.Fatalf("etcd.Sync: %s\n", err)
        } else {
            // iterate initial set of services
            for _, service := range services {
                self.ipvs.syncService(service, service.Frontend)
            }
        }

        // read channel for changes
        for event := range self.etcd.Sync() {
            log.Printf("etcd.Sync: %+v\n", event)
            switch event.Type {
            case server.NewService, server.SetService:
                self.ipvs.syncService(event.Service, event.Frontend)
            case server.DelService:
                self.ipvs.delService(event.Service, event.Frontend)
            case server.NewBackend, server.SetBackend:
                self.ipvs.syncServiceBackend(event.Service, event.BackendName, event.Backend)
            case server.DelBackend:
                self.ipvs.delServiceBackend(event.Service, event.BackendName, event.Backend)
            }
        }
    }

    // self.printIPVS()

    log.Printf("Exit\n")
}
