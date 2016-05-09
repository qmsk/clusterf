package clusterf

import (
	"fmt"
	"github.com/qmsk/clusterf/config"
	"github.com/qmsk/clusterf/ipvs"
	"net"
	"syscall"
)

type Service struct {
	ipvs.Service

	dests ServiceDests
}

type ServiceDests map[string]Dest

func (dests ServiceDests) get(ipvsDest ipvs.Dest) Dest {
	if dest, exists := dests[ipvsDest.String()]; exists {
		return dest
	} else {
		dest := Dest{
			Dest: ipvsDest,
		}

		dests[dest.String()] = dest

		return dest
	}
}

func (dests ServiceDests) sync(ipvsDest ipvs.Dest) {
	// for side-effect
	_ = dests.get(ipvsDest)
}

func (dests ServiceDests) config(ipvsDest ipvs.Dest) {
	dest, exists := dests[ipvsDest.String()]
	if exists {
		// merge
		dest.Weight += ipvsDest.Weight

	} else {
		dest = Dest{
			Dest: ipvsDest,
		}
	}

	dests[ipvsDest.String()] = dest
}

// Lookup or initialize an ipvsService from a config ServiceFrontend
func configServiceFrontend(ipvsType ipvsType, frontend *config.ServiceFrontend, options IPVSOptions) (*ipvs.Service, error) {
	if frontend == nil {
		return nil, nil
	}

	ipvsService := ipvs.Service{
		Af:       ipvsType.Af,
		Protocol: ipvsType.Protocol,

		SchedName: options.SchedName,
		Timeout:   0,
		Flags:     ipvs.Flags{Flags: 0, Mask: 0xffffffff},
		Netmask:   0xffffffff,
	}

	switch ipvsType.Af {
	case syscall.AF_INET:
		if frontend.IPv4 == "" {
			return nil, nil
		} else if ip := net.ParseIP(frontend.IPv4); ip == nil {
			return nil, fmt.Errorf("Invalid IPv4: %v", frontend.IPv4)
		} else if ip4 := ip.To4(); ip4 == nil {
			return nil, fmt.Errorf("Invalid IPv4: %v", ip)
		} else {
			ipvsService.Addr = ip4
		}
	case syscall.AF_INET6:
		if frontend.IPv6 == "" {
			return nil, nil
		} else if ip := net.ParseIP(frontend.IPv6); ip == nil {
			return nil, fmt.Errorf("Invalid IPv6: %v", frontend.IPv6)
		} else if ip16 := ip.To16(); ip16 == nil {
			return nil, fmt.Errorf("Invalid IPv6: %v", ip)
		} else {
			ipvsService.Addr = ip16
		}
	}

	switch ipvsType.Protocol {
	case syscall.IPPROTO_TCP:
		if frontend.TCP == 0 {
			return nil, nil
		} else {
			ipvsService.Port = frontend.TCP
		}
	case syscall.IPPROTO_UDP:
		if frontend.UDP == 0 {
			return nil, nil
		} else {
			ipvsService.Port = frontend.UDP
		}
	default:
		panic("invalid proto")
	}

	return &ipvsService, nil
}
