package config

import (
    "fmt"
    "strings"
)

/*
 * Configuration sources: where the config is coming from
 */
type ConfigSource interface {
    // uniquely identifying
    String()    string
}

/* Config objects */

type ServiceFrontend struct {
    IPv4    string  `json:"ipv4,omitempty"`
    IPv6    string  `json:"ipv6,omitempty"`
    TCP     uint16  `json:"tcp,omitempty"`
    UDP     uint16  `json:"udp,omitempty"`
}

type ServiceBackend struct {
    IPv4    string  `json:"ipv4,omitempty"`
    IPv6    string  `json:"ipv6,omitempty"`
    TCP     uint16  `json:"tcp,omitempty"`
    UDP     uint16  `json:"udp,omitempty"`

    Weight  uint    `json:"weight,omitempty"`   // default: 10
}

type Service struct {
    Frontend        ServiceFrontend
    Backends        map[string]ServiceBackend
}

// Copy-on-write update for .Backends
func (service *Service) setBackend(backendName string, serviceBackend ServiceBackend, remove bool) {
    serviceBackends := make(map[string]ServiceBackend)

    for backendName, backend := range service.Backends {
        serviceBackends[backendName] = backend
    }

    if remove {
        delete(serviceBackends, backendName)
    } else {
        serviceBackends[backendName] = serviceBackend
    }

    service.Backends = serviceBackends

}

type Route struct {
    // IPv4 prefix to match
    // empty for default match
    Prefix4     string

    // Override backend IPv4 address for ipvs
    Gateway4    string

    // Configure IPVS fwd-method to use for destination
    //  droute tunnel masq
    // Filter out backend if set to
    //  filter
    IpvsMethod  string
}

// Top-level config object
type Config struct {
    Routes      map[string]Route
    Services    map[string]Service
}

// Copy-on-write update for .Services 
func (config *Config) setService(serviceName string, service Service, remove bool) {
    services := make(map[string]Service)

    for serviceName, service := range config.Services {
        services[serviceName] = service
    }

    if remove {
        delete(services, serviceName)
    } else {
        services[serviceName] = service
    }

    config.Services = services
}

func (config *Config) updateServices(node Node) error {
    if node.Remove {
        // reset
        config.Services = make(map[string]Service)
    }

    return nil
}

func (config *Config) updateService(node Node, serviceName string) error {
    service := config.Services[serviceName]

    config.setService(serviceName, service, node.Remove)

    return nil
}

func (config *Config) updateServiceFrontend(node Node, serviceName string, serviceFrontend ServiceFrontend) error {
    service := config.Services[serviceName]

    service.Frontend = serviceFrontend

    config.setService(serviceName, service, node.Remove)

    return nil
}

func (config *Config) updateServiceBackends(node Node, serviceName string) error {
    service := config.Services[serviceName]

    if node.Remove {
        // reset
        service.Backends = make(map[string]ServiceBackend)
    }

    config.setService(serviceName, service, false)

    return nil
}

func (config *Config) updateServiceBackend(node Node, serviceName string, backendName string, serviceBackend ServiceBackend) error {
    service := config.Services[serviceName]

    service.setBackend(backendName, serviceBackend, node.Remove)
    config.setService(serviceName, service, false)

    return nil
}

func (config *Config) updateRoutes(node Node) error {
    if node.Remove {
        // reset
        config.Routes = make(map[string]Route)
    }

    return nil
}

func (config *Config) updateRoute(node Node, routeName string, route Route) error {
    routes := make(map[string]Route)

    for name, route := range config.Routes {
        routes[name] = route
    }

    if node.Remove {
        // clear
        delete(routes, routeName)
    } else {
        routes[routeName] = route
    }

    config.Routes = routes

    return nil
}

// Update config from Node
func (config *Config) update(node Node) error {
    nodePath := strings.Split(node.Path, "/")

    if len(node.Path) == 0 {
        // Split("", "/") would give [""]
        nodePath = nil
    }

    // match config tree path
    if len(nodePath) == 0 && node.IsDir {
        // XXX: just ignore? Undefined if it makes sense to do anything here
        return nil

    } else if len(nodePath) == 1 && nodePath[0] == "services" && node.IsDir {
        // recursive on all services
        return config.updateServices(node)

    } else if len(nodePath) >= 2 && nodePath[0] == "services" {
        serviceName := nodePath[1]

        if len(nodePath) == 2 && node.IsDir {
            return config.updateService(node, serviceName)

        } else if len(nodePath) == 3 && nodePath[2] == "frontend" && !node.IsDir {
            var serviceFrontend ServiceFrontend

            if err := node.unmarshal(&serviceFrontend); err != nil {
                return fmt.Errorf("service %s frontend: %s", serviceName, err)
            }

            return config.updateServiceFrontend(node, serviceName, serviceFrontend)

        } else if len(nodePath) == 3 && nodePath[2] == "backends" && node.IsDir {
            // recursive on all backends
            return config.updateServiceBackends(node, serviceName)

        } else if len(nodePath) >= 4 && nodePath[2] == "backends" {
            backendName := nodePath[3]

            if len(nodePath) == 4 && !node.IsDir {
                var serviceBackend ServiceBackend

                if err := node.unmarshal(&serviceBackend); err != nil {
                    return fmt.Errorf("service %s backend %s: %s", serviceName, backendName, err)
                }

                return config.updateServiceBackend(node, serviceName, backendName, serviceBackend)

            } else {
                return fmt.Errorf("Ignore unknown service %s backends node", serviceName)
            }

        } else {
            return fmt.Errorf("Ignore unknown service %s node", serviceName)
        }

    } else if len(nodePath) == 1 && nodePath[0] == "routes" && node.IsDir {
        return config.updateRoutes(node)

    } else if len(nodePath) >= 2 && nodePath[0] == "routes" {
        routeName := nodePath[1]

        if len(nodePath) == 2 && !node.IsDir {
            var route Route

            if err := node.unmarshal(&route); err != nil {
                return fmt.Errorf("route %s: %s", routeName, err)
            }

            return config.updateRoute(node, routeName, route)

        } else {
            return fmt.Errorf("Ignore unknown route node")
        }
    } else {
        return fmt.Errorf("Ignore unknown node")
    }
}

/*
func (self ConfigService) Path() string {
    return makePath("services", self.ServiceName)
}

func (self ConfigServiceFrontend) Path() string {
    return makePath("services", self.ServiceName, "frontend")
}

func (self ConfigServiceBackend) Path() string {
    return makePath("services", self.ServiceName, "backends", self.BackendName)
}

func (self ConfigRoute) Path() string {
    return makePath("routes", self.RouteName)
}
*/
