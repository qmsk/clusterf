package clusterf
/*
 * Internal services states, maintained from config changes.
 */

import (
    "qmsk.net/clusterf/config"
    "fmt"
    "log"
)

type Services struct {
    services    map[string]*Service

    driver      *IPVSDriver
}

func NewServices(driver *IPVSDriver) *Services {
    return &Services{
        services:   make(map[string]*Service),
        driver:     driver,
    }
}

// Return Service for named service, possibly creating a new (empty) Service.
func (self *Services) get(name string) *Service {
    service, serviceExists := self.services[name]

    if !serviceExists {
        service = newService(name,
            self.driver,
        )
        self.services[name] = service
    }

    return service
}

// Return all currently valid Services
func (self *Services) Services() []*Service {
    services := make([]*Service, 0, len(self.services))

    for _, service := range self.services {
        if service.Frontend == nil {
            continue
        }

        services = append(services, service)
    }

    return services
}

/* Configuration actions */

// Configuration action on a service itself
// handle service-delete actions
// new service creation is implicitly handled when calling this
func (self *Services) configService(service *Service, action config.Action, serviceConfig *config.ConfigService) {
    log.Printf("clusterf:Services: Service %s: %s %+v\n", service.Name, action, serviceConfig)

    switch action {
    case config.DelConfig:
        delete(self.services, service.Name)

        service.delFrontend()
    }
}

func (self *Services) ApplyConfig(action config.Action, baseConfig config.Config) {
    log.Printf("clusterf:Services: config %s %#v\n", action, baseConfig)

    switch baseConfig.(type) {
    case *config.ConfigService:
        serviceConfig := baseConfig.(*config.ConfigService)

        if serviceConfig.ServiceName == "" {
            // all services
            for _, service := range self.services {
                self.configService(service, action, serviceConfig)
            }
        } else {
            service := self.get(serviceConfig.ServiceName)

            self.configService(service, action, serviceConfig)
        }

    case *config.ConfigServiceFrontend:
        frontendConfig := baseConfig.(*config.ConfigServiceFrontend)

        service := self.get(frontendConfig.ServiceName)

        service.configFrontend(action, frontendConfig)

    case *config.ConfigServiceBackend:
        backendConfig := baseConfig.(*config.ConfigServiceBackend)

        service := self.get(backendConfig.ServiceName)

        if backendConfig.BackendName == "" {
            // all service backends
            for backendName, _ := range service.Backends {
                service.configBackend(backendName, action, backendConfig)
            }
        } else {
            service.configBackend(backendConfig.BackendName, action, backendConfig)
        }

    default:
        panic(fmt.Errorf("Unknown config type: %#v", baseConfig))
    }
}
