package clusterf

import (
    "github.com/qmsk/clusterf/config"
	"fmt"
    "github.com/qmsk/clusterf/ipvs"
	"net"
	"syscall"
)

type Dest struct {
	ipvs.Dest
}

func configServiceBackend (ipvsService ipvs.Service, backend config.ServiceBackend, routes Routes, options IPVSOptions) (*ipvs.Dest, error) {
    ipvsDest := ipvs.Dest{
        FwdMethod:  options.FwdMethod,		// default, overriden by route
        Weight:     uint32(backend.Weight),
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

	// apply routes
    route := routes.Lookup(ipvsDest.Addr)
    if route == nil {
		// as-is
        return &ipvsDest, nil
    } else if route.IPVSMethod == nil {
        // ignore
        return nil, nil
    } else {
        ipvsDest.FwdMethod = *route.IPVSMethod
    }

	// IPVS chaning to next frontend
	if route.Gateway != nil {
		// TODO: mixed-family routes
		ipvsDest.Addr = route.Gateway
		ipvsDest.Port = ipvsService.Port
	}

	return &ipvsDest, nil
}
