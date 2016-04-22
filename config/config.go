package config

import (
    "fmt"
    "strings"
)

// A single Config may contain Nodes from different Sources
// TODO: tag each Config object with a Node/Source, such that
//		 sources override, and recursive updates are scoped to that source only
type Source interface {
    // uniquely identifying
    String()    string
}

/* Config objects */
type Meta struct {
	node		Node	// origin node
}


func (meta Meta) Source() string {
	if meta.node.Source != nil {
		return meta.node.Source.String()
	} else {
		// some nodes are implicitly created, and thus not directly applicable..
		return ""
	}
}

func (meta Meta) Path() string {
	return meta.node.Path
}

type ServiceFrontend struct {
	Meta			`json:"-"`

    IPv4    string  `json:"ipv4,omitempty"`
    IPv6    string  `json:"ipv6,omitempty"`
    TCP     uint16  `json:"tcp,omitempty"`
    UDP     uint16  `json:"udp,omitempty"`
}

type ServiceBackend struct {
	Meta			`json:"-"`

    IPv4    string  `json:"ipv4,omitempty"`
    IPv6    string  `json:"ipv6,omitempty"`
    TCP     uint16  `json:"tcp,omitempty"`
    UDP     uint16  `json:"udp,omitempty"`

    Weight  uint    `json:"weight,omitempty"`   // default: 10
}

const ServiceBackendWeight uint = 10

type Service struct {
	Meta			`json:"-"`

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

// Apply a recursive remove from source
func (service *Service) removeBackends(source Source) {
    serviceBackends := make(map[string]ServiceBackend)

    for backendName, backend := range service.Backends {
		if backend.Source() == source.String() {
			//log.Printf("service %s drop source=%s backends: %s source=%s\n", service.Path(), source.String(), backendName, backend.Source())

			// remove all from this source
			continue
		} else {
			//log.Printf("service %s keep source=%s backends: %s source=%s\n", service.Path(), source.String(), backendName, backend.Source())
		}

        serviceBackends[backendName] = backend
    }

    service.Backends = serviceBackends
}

type Route struct {
	Meta			`json:"-"`

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
		services := make(map[string]Service)

		for serviceName, service := range config.Services {
			if service.Source() == node.Source.String() {
				// remove service from this source
				continue
			} else {
				// remove any backends from this source
				service.removeBackends(node.Source)
			}

			services[serviceName] = service
		}

		config.Services = services
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

	// service is owned by whatever source has its Frontend
	service.node = node
    service.Frontend = serviceFrontend

    config.setService(serviceName, service, node.Remove)

    return nil
}

func (config *Config) updateServiceBackends(node Node, serviceName string) error {
    service := config.Services[serviceName]

	if node.Remove {
		service.removeBackends(node.Source)
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
		routes := make(map[string]Route)

		for routeName, route := range config.Routes {
			if route.Source() == node.Source.String() {
				// remove from this source
				continue
			}

			routes[routeName] = route
		}

		config.Routes = routes
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
            var serviceFrontend = ServiceFrontend{
				Meta:       Meta{node:node},
			}

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
                var serviceBackend = ServiceBackend{
					Meta:       Meta{node:node},
                    Weight:     ServiceBackendWeight,
                }

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
            var route = Route{
				Meta:       Meta{node:node},
			}

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
