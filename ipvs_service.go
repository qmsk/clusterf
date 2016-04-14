package clusterf

import (
    "github.com/qmsk/clusterf/config"
	"fmt"
    "github.com/qmsk/clusterf/ipvs"
	"net"
	"syscall"
)

type Service struct {
	ipvs.Service

	dests       ServiceDests
}

type Services map[string]Service

// Lookup or initialize ipvsService from a kernel ipvs.Service
func (services Services) get(ipvsService ipvs.Service) Service {
	if service, exists := services[ipvsService.String()]; exists {
		return service
	} else {
		service := Service{
			Service:	ipvsService,
			dests:		make(ServiceDests),
		}

		services[service.String()] = service

		return service
	}
}

func (services Services) sync(ipvsService ipvs.Service, ipvsDests []ipvs.Dest) {
	// for side-effect
	service := services.get(ipvsService)

	for _, ipvsDest := range ipvsDests {
		service.dests.sync(ipvsDest)
	}
}

func (services Services) config(ipvsService ipvs.Service) Service {
	return services.get(ipvsService)
}

// Build a new services state from Config
func configServices(configServices map[string]config.Service, routes Routes, options IPVSOptions) (Services, error) {
	services := make(Services)

	for _, configService := range configServices {
		for _, ipvsType := range ipvsTypes {
			if ipvsService, err := configServiceFrontend(ipvsType, configService.Frontend, options); err != nil {
				return nil, err
			} else if ipvsService != nil {
				service := services.config(*ipvsService)

				for _, configBackend := range configService.Backends {
					if ipvsDest, err := configServiceBackend(*ipvsService, configBackend, routes, options); err != nil {
						return nil, err
					} else if ipvsDest != nil {
						// TODO: dest merging
						service.dests.config(*ipvsDest)
					}
				}
			}
		}
	}

	return services, nil
}

// Lookup or initialize an ipvsService from a config ServiceFrontend
func configServiceFrontend (ipvsType ipvsType, frontend config.ServiceFrontend, options IPVSOptions) (*ipvs.Service, error) {
    ipvsService := ipvs.Service{
		Af:         ipvsType.Af,
		Protocol:   ipvsType.Protocol,

		SchedName:  options.SchedName,
		Timeout:    0,
		Flags:      ipvs.Flags{Flags: 0, Mask: 0xffffffff},
		Netmask:    0xffffffff,
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
