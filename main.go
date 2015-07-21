package main

import (
    "flag"
    "fmt"
    "qmsk.net/clusterf/ipvs"
    "log"
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

type Server struct {
    etcd    *server.Etcd
    ipvs    *ipvs.Client
}

func protoString (proto uint16) string {
    switch (proto) {
    case syscall.IPPROTO_TCP:   return "TCP"
    case syscall.IPPROTO_UDP:   return "UDP"
    default: return fmt.Sprintf("%d", proto)
    }
}

func (self *Server) printIPVS () {
    if services, err := self.ipvs.ListServices(); err != nil {
        log.Fatalf("ipvs.ListServices: %v\n", err)
    } else {
        fmt.Printf("Proto                           Addr:Port\n")
        for _, service := range services {
            fmt.Printf("%-5s %30s:%-5d %s\n",
                protoString(service.Protocol),
                service.Addr, service.Port,
                service.SchedName,
            )

            if dests, err := self.ipvs.ListDests(service); err != nil {
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

func (self *Server) syncService (service server.Service) {
    log.Printf("Sync %s: Frontend %+v\n", service.Name, service.Frontend)

    ipvsService := ipvs.Service{
        Af:         syscall.AF_INET,
        Protocol:   syscall.IPPROTO_TCP,
        Addr:       service.Frontend.IPv4,
        Port:       service.Frontend.TCP,

        SchedName:  "wlc",
        Timeout:    0,
        Flags:      ipvs.IPVSFlags{Flags: 0, Mask: 0xffffffff},
        Netmask:    0xffffffff,
    }

    if err := self.ipvs.NewService(ipvsService); err != nil  {
        log.Fatalf("ipvs.NewService %s: %s\n", service.Name, err)
    }

    for serverName, server := range service.Servers {
        log.Printf("Sync %s: Server %s: %+v\n", service.Name, serverName, server)

        ipvsDest := ipvs.Dest{
            Addr:       server.IPv4,
            Port:       server.TCP,
            FwdMethod:  ipvs.IP_VS_CONN_F_MASQ,
            Weight:     10,
        }

        if err := self.ipvs.NewDest(ipvsService, ipvsDest); err != nil {
            log.Fatalf("ipvs.NewDest %s %s: %s\n", service.Name, serverName, err)
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

        self.ipvs = ipvsClient
    }

    if err := self.ipvs.Flush(); err != nil {
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
                self.syncService(service)
            }
        }

        // read channel for changes
        for event := range self.etcd.Sync() {
            log.Printf("etcd.Sync: %+v\n", event)
        }
    }

    // self.printIPVS()

    log.Printf("Exit\n")
}
