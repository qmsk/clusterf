package clusterf

import (
    "fmt"
    "qmsk.net/clusterf/ipvs"
    "log"
    "syscall"
)

type ipvsType struct {
    Af          uint16
    Protocol    uint16
}

var ipvsTypes = []ipvsType {
    { syscall.AF_INET,      syscall.IPPROTO_TCP },
    { syscall.AF_INET6,     syscall.IPPROTO_TCP },
    { syscall.AF_INET,      syscall.IPPROTO_UDP },
    { syscall.AF_INET6,     syscall.IPPROTO_UDP },
}

type IpvsConfig struct {
    Debug   bool
}

type IPVSDriver struct {
    ipvsClient *ipvs.Client
}

func (self IpvsConfig) Open() (*IPVSDriver, error) {
    ipvs := &IPVSDriver{}

    if err := ipvs.open(self); err != nil {
        return nil, err
    } else {
        return ipvs, nil
    }
}

func (self *IPVSDriver) open(config IpvsConfig) error {
    // IPVS
    if ipvsClient, err := ipvs.Open(); err != nil {
        return err
    } else {
        log.Printf("ipvs.Open: %+v\n", ipvsClient)

        self.ipvsClient = ipvsClient
    }

    if config.Debug {
        self.ipvsClient.SetDebug()
    }

    if info, err := self.ipvsClient.GetInfo(); err != nil {
        return err
    } else {
        log.Printf("ipvs.GetInfo: version=%s, conn_tab_size=%d\n", info.Version, info.ConnTabSize)
    }

    return nil
}

// Begin initial config sync
func (self *IPVSDriver) StartSync() error {
    if err := self.ipvsClient.Flush(); err != nil {
        return err
    } else {
        log.Printf("ipvs.Flush")
    }

    return nil
}

func (self *IPVSDriver) newFrontend() *ipvsFrontend {
    return &ipvsFrontend{
        ipvs:   self.ipvsClient,
        state:  make(map[ipvsType]*ipvs.Service),
    }
}

// XXX
func protoString (proto uint16) string {
    switch (proto) {
    case syscall.IPPROTO_TCP:   return "TCP"
    case syscall.IPPROTO_UDP:   return "UDP"
    default: return fmt.Sprintf("%d", proto)
    }
}

func (self *IPVSDriver) printIPVS () {
    if services, err := self.ipvsClient.ListServices(); err != nil {
        log.Fatalf("ipvs.ListServices: %v\n", err)
    } else {
        fmt.Printf("Proto                           Addr:Port\n")
        for _, service := range services {
            fmt.Printf("%-5s %30s:%-5d %s\n",
                protoString(service.Protocol),
                service.Addr, service.Port,
                service.SchedName,
            )

            if dests, err := self.ipvsClient.ListDests(service); err != nil {
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


