package clusterf

import (
    "qmsk.net/clusterf/config"
    "fmt"
    "qmsk.net/clusterf/ipvs"
    "log"
    "net"
    "syscall"
)

type ipvsBackend struct {
    driver      *IPVSDriver
    frontend    *ipvsFrontend
    state       map[ipvsType]*ipvs.Dest
}

func (self *ipvsBackend) buildDest (ipvsType ipvsType, backend config.ServiceBackend) (*ipvs.Dest, error) {
    ipvsDest := &ipvs.Dest{
        FwdMethod:  self.driver.fwdMethod,
        Weight:     10,
    }

    switch ipvsType.Af {
    case syscall.AF_INET:
        if backend.IPv4 == "" {
            return nil, nil
        } else if ip := net.ParseIP(backend.IPv4); ip == nil {
            return nil, fmt.Errorf("Invalid IPv4: %v", backend.IPv4)
        } else if ip4 := ip.To4(); ip4 == nil {
            return nil, fmt.Errorf("Invalid IPv4: %v", ip)
        } else {
            ipvsDest.Addr = ip4
        }
    case syscall.AF_INET6:
        if backend.IPv6 == "" {
            return nil, nil
        } else if ip := net.ParseIP(backend.IPv6); ip == nil {
            return nil, fmt.Errorf("Invalid IPv6: %v", backend.IPv6)
        } else if ip16 := ip.To16(); ip16 == nil {
            return nil, fmt.Errorf("Invalid IPv6: %v", ip)
        } else {
            ipvsDest.Addr = ip16
        }
    default:
        panic("invalid af")
    }

    switch ipvsType.Protocol {
    case syscall.IPPROTO_TCP:
        if backend.TCP == 0 {
            return nil, nil
        } else {
            ipvsDest.Port = backend.TCP
        }
    case syscall.IPPROTO_UDP:
        if backend.UDP == 0 {
            return nil, nil
        } else {
            ipvsDest.Port = backend.UDP
        }
    default:
        panic("invalid proto")
    }

    return ipvsDest, nil
}

// create any instances of this backend, assuming there is no active state
func (self *ipvsBackend) add(backend config.ServiceBackend) error {
    for _, ipvsType := range ipvsTypes {
        if ipvsService := self.frontend.state[ipvsType]; ipvsService != nil {
            if ipvsDest, err := self.buildDest(ipvsType, backend); err != nil {
                return err
            } else if ipvsDest != nil {
                log.Printf("clusterf:ipvsBackend.add: new %v %v\n", ipvsService, ipvsDest)

                if err := self.driver.ipvsClient.NewDest(*ipvsService, *ipvsDest); err != nil  {
                    return err
                } else {
                    self.state[ipvsType] = ipvsDest
                }
            }
        }
    }

    return nil
}

// update any instances of this backend
// - removes any active instances that are no longer configured
// - replaces any active instances that have changed
// - adds new active isntances that are now configured
//
// TODO: sets any active instances that have changed parameters
func (self *ipvsBackend) set(backend config.ServiceBackend) error {
    for _, ipvsType := range ipvsTypes {
        if ipvsService := self.frontend.state[ipvsType]; ipvsService != nil {
            var setDest, getDest *ipvs.Dest
            var match bool

            getDest = self.state[ipvsType]

            if ipvsDest, err := self.buildDest(ipvsType, backend); err != nil {
                return err
            } else if ipvsDest != nil {
                log.Printf("clusterf:ipvsBackend.set: new %v %v\n", ipvsService, ipvsDest)

                setDest = ipvsDest
            }

            // compare for matching id, but changed value
            if setDest != nil && getDest != nil {
                match = false // XXX: setDest.Match(getDest)
            } else {
                match = false
            }

            if setDest == nil {
                // configured as inactive
            } else if match {
                log.Printf("clusterf:ipvsBackend.set: set %v %v\n", ipvsService, setDest)

                // reconfigure active in-place
                if err := self.driver.ipvsClient.SetDest(*ipvsService, *setDest); err != nil  {
                    return err
                }
            } else {
                log.Printf("clusterf:ipvsBackend.set: new %v %v\n", ipvsService, setDest)

                // replace active
                if err := self.driver.ipvsClient.NewDest(*ipvsService, *setDest); err != nil  {
                    return err
                }
            }

            if getDest == nil {
                // not active

            } else if match {
                // remains active

            } else {
                log.Printf("clusterf:ipvsBackend.set: del %v %v\n", ipvsService, getDest)

                // replace active
                if err := self.driver.ipvsClient.DelDest(*ipvsService, *getDest); err != nil {
                    // XXX: inconsistent, we now have two dest's
                    return err
                }
            }

            // may be nil, if the new backend did not have this ipvsType
            self.state[ipvsType] = setDest
        }
    }

    return nil
}

// remove any active instances of this backend, clearing the active state
func (self *ipvsBackend) del() error {
    for _, ipvsType := range ipvsTypes {
        if ipvsService := self.frontend.state[ipvsType]; ipvsService != nil {
            if ipvsDest := self.state[ipvsType]; ipvsDest != nil {
                log.Printf("clusterf:ipvsBackend.del: del %v %v\n", ipvsService, ipvsDest)

                if err := self.driver.ipvsClient.DelDest(*ipvsService, *ipvsDest); err != nil  {
                    return err
                } else {
                    self.state[ipvsType] = nil
                }
            }
        }
    }

    return nil
}
