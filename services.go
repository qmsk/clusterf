package clusterf

import (
    "github.com/qmsk/clusterf/config"
	"fmt"
    "github.com/qmsk/clusterf/ipvs"
)

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

	for serviceName, configService := range configServices {
		for _, ipvsType := range ipvsTypes {
			if ipvsService, err := configServiceFrontend(ipvsType, configService.Frontend, options); err != nil {
				return nil, fmt.Errorf("Invalid config for service %v: %v", serviceName, err)
			} else if ipvsService != nil {
				service := services.config(*ipvsService)

				for backendName, configBackend := range configService.Backends {
					if ipvsDest, err := configServiceBackend(*ipvsService, configBackend, routes, options); err != nil {
						return nil, fmt.Errorf("Invalid config for service %v backend %v: %v", serviceName, backendName, err)
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
