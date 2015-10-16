package clusterf

import (
    "fmt"
    "qmsk.net/clusterf/ipvs"
    "log"
    "syscall"
)

type ipvsType struct {
    Af          ipvs.Af
    Protocol    ipvs.Protocol
}

var ipvsTypes = []ipvsType {
    { syscall.AF_INET,      syscall.IPPROTO_TCP },
    { syscall.AF_INET6,     syscall.IPPROTO_TCP },
    { syscall.AF_INET,      syscall.IPPROTO_UDP },
    { syscall.AF_INET6,     syscall.IPPROTO_UDP },
}

type IpvsConfig struct {
    Debug       bool
    FwdMethod   string
    SchedName   string
}

type IPVSDriver struct {
    ipvsClient *ipvs.Client

    // global state
    routes      Routes

    // global defaults
    fwdMethod   ipvs.FwdMethod
    schedName   string
}

func (self IpvsConfig) setup(routes Routes) (*IPVSDriver, error) {
    driver := &IPVSDriver{
        routes: routes,
    }

    if fwdMethod, err := ipvs.ParseFwdMethod(self.FwdMethod); err != nil {
        return nil, err
    } else {
        driver.fwdMethod = fwdMethod
    }

    driver.schedName = self.SchedName

    // IPVS
    if ipvsClient, err := ipvs.Open(); err != nil {
        return nil, err
    } else {
        log.Printf("ipvs.Open: %+v\n", ipvsClient)

        driver.ipvsClient = ipvsClient
    }

    if self.Debug {
        driver.ipvsClient.SetDebug()
    }

    if info, err := driver.ipvsClient.GetInfo(); err != nil {
        return nil, err
    } else {
        log.Printf("ipvs.GetInfo: version=%s, conn_tab_size=%d\n", info.Version, info.ConnTabSize)
    }

    return driver, nil
}

// Begin initial config sync by flushing the system state
func (self *IPVSDriver) sync() error {
    if err := self.ipvsClient.Flush(); err != nil {
        return err
    } else {
        log.Printf("ipvs.Flush")
    }

    return nil
}

func (self *IPVSDriver) newFrontend() *ipvsFrontend {
    return &ipvsFrontend{
        driver: self,
        state:  make(map[ipvsType]*ipvs.Service),
    }
}

func (self *IPVSDriver) Print() {
    if services, err := self.ipvsClient.ListServices(); err != nil {
        log.Fatalf("ipvs.ListServices: %v\n", err)
    } else {
        fmt.Printf("Proto                           Addr:Port\n")
        for _, service := range services {
            fmt.Printf("%-5v %30s:%-5d %s\n",
                service.Protocol,
                service.Addr, service.Port,
                service.SchedName,
            )

            if dests, err := self.ipvsClient.ListDests(service); err != nil {
                log.Fatalf("ipvs.ListDests: %v\n", err)
            } else {
                for _, dest := range dests {
                    fmt.Printf("%5s %30s:%-5d %v\n",
                        "",
                        dest.Addr, dest.Port,
                        dest.FwdMethod,
                    )
                }
            }
        }
    }
}
