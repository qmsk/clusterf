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
)

func init() {
    flag.StringVar(&etcdConfig.Machines, "etcd-machines", "http://127.0.0.1:2379",
        "Client endpoint for etcd")
    flag.StringVar(&etcdConfig.Prefix, "etcd-prefix", "/clusterf",
        "Etcd tree prefix")
}

type Server struct {
    etcd    *server.Etcd
    ipvs    *ipvs.Client
}

func main() {
    log.Println("Hello World")

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
        log.Printf("ipvs.Open: %+v\n", ipvsClient)

        if err := ipvsClient.GetInfo(); err != nil {
            log.Fatalf("ipvs.GetInfo: %v\n", err)
        }

        self.ipvs = ipvsClient

        if services, err := ipvsClient.ListServices(); err != nil {
            log.Fatalf("ipvs.ListServices: %v\n", err)
        } else {
            fmt.Printf("Proto                           Addr:Port  Sched\n")
            for _, service := range services {
                fmt.Printf("%5d %30s:%-5d %s\n",
                    service.Protocol,
                    service.Addr, service.Port,
                    service.SchedName,
                )

                if dests, err := ipvsClient.ListDests(service); err != nil {
                    log.Fatalf("ipvs.ListDests: %v\n", err)
                } else {
                    for _, dest := range dests {
                        fmt.Printf("%5s %30s:%-5d %#04x\n",
                            "->",
                            dest.Addr, dest.Port,
                            dest.FwdMethod,
                        )
                    }
                }
            }
        }

        if err := ipvsClient.Flush(); err != nil {
            log.Fatalf("ipvs.Flush: %v\n", err)
        }
    }

    // etcd
    if etcdClient, err := etcdConfig.Open(); err != nil {
        log.Fatalf("etcd.Open: %s\n", err)
    } else {
        log.Printf("etcd.Open: %s\n", etcdClient)

        self.etcd = etcdClient

        // start
        err := self.etcd.Sync(func (service server.Service) {
            log.Printf("etcd.Sync %s: Frontend %+v\n", service.Name, service.Frontend)

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
                log.Printf("etcd.Sync %s: Server %s: %+v\n", service.Name, serverName, server)

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
        })
        if err != nil {
            log.Fatalf("etcd.Sync: %s\n", err)
        }
    }
}
