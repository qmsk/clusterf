package clusterf

import (
    "github.com/qmsk/clusterf/config"
	"fmt"
    "github.com/qmsk/clusterf/ipvs"
	"net"
	"syscall"
)

const IPVS_WEIGHT uint32 = 10

type Dest struct {
	ipvs.Dest
}

type ServiceDests map[string]Dest

func (dests ServiceDests) get(ipvsDest ipvs.Dest) Dest {
	if dest, exists := dests[ipvsDest.String()]; exists {
		return dest
	} else {
		dest := Dest{
			Dest:		ipvsDest,
		}

		dests[dest.String()] = dest

		return dest
	}
}

func (dests ServiceDests) sync(ipvsDest ipvs.Dest) {
	// for side-effect
	_ = dests.get(ipvsDest)
}

func (dests ServiceDests) config(ipvsDest ipvs.Dest) Dest {
	return dests.get(ipvsDest)
}

func routeDest (ipvsDest ipvs.Dest, ipvsService ipvs.Service, routes Routes) (*ipvs.Dest, error) {
    route := routes.Lookup(ipvsDest.Addr)
    if route == nil {
        return &ipvsDest, nil
    }

    if route.ipvs_filter {
        // ignore
        return nil, nil
    }

    if route.ipvs_fwdMethod != nil {
        ipvsDest.FwdMethod = *route.ipvs_fwdMethod
    }

    switch ipvsService.Af {
    case syscall.AF_INET:
        if route.Gateway4 != nil {
            // chaining
            ipvsDest.Addr = route.Gateway4
            ipvsDest.Port = ipvsService.Port
        }
    }

	return &ipvsDest, nil
}

func configServiceBackend (ipvsService ipvs.Service, backend config.ServiceBackend, routes Routes, options IPVSOptions) (*ipvs.Dest, error) {
    ipvsDest := ipvs.Dest{
        FwdMethod:  options.FwdMethod,
    }

    switch ipvsService.Af {
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

    switch ipvsService.Protocol {
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

    if backend.Weight == 0 {
		// XXX: we also need to support 0-weight backends!
		ipvsDest.Weight = IPVS_WEIGHT

    } else {
        ipvsDest.Weight = uint32(backend.Weight)
    }

	if routeDest, err := routeDest(ipvsDest, ipvsService, routes); err != nil {
		return nil, err
	} else if routeDest == nil {
		return nil, nil
	} else {
		ipvsDest = *routeDest
	}

	return &ipvsDest, nil
}
