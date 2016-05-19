package config

import (
	"fmt"
	"strings"
)

/* Config objects */
type Meta struct {
	node Node // origin node
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
	Meta `json:"-"`

	IPv4 string `json:"ipv4,omitempty"`
	IPv6 string `json:"ipv6,omitempty"`
	TCP  uint16 `json:"tcp,omitempty"`
	UDP  uint16 `json:"udp,omitempty"`
}

type ServiceBackend struct {
	Meta `json:"-"`

	IPv4 string `json:"ipv4,omitempty"`
	IPv6 string `json:"ipv6,omitempty"`
	TCP  uint16 `json:"tcp,omitempty"`
	UDP  uint16 `json:"udp,omitempty"`

	Weight uint `json:"weight"` // default: 10
}

const ServiceBackendWeight uint = 10

type Service struct {
	Meta `json:"-"`

	Frontend *ServiceFrontend
	Backends map[string]ServiceBackend
}

func (service *Service) setBackend(backendName string, serviceBackend ServiceBackend) {
	if service.Backends == nil {
		service.Backends = map[string]ServiceBackend{backendName: serviceBackend}
	} else {
		service.Backends[backendName] = serviceBackend
	}
}

// Modify this Service in-place, by merging in a copy of the given Service.
func (service *Service) merge(other Service) {
	if other.Frontend != nil {
		service.Frontend = other.Frontend
	}

	// backends
	if service.Backends == nil {
		service.Backends = make(map[string]ServiceBackend)
	}

	for backendName, backend := range other.Backends {
		service.Backends[backendName] = backend
	}
}

type Route struct {
	Meta `json:"-"`

	// IPv4/IPv6 prefix to match
	// empty for default match
	Prefix string `json:",omitempty"`

	// Override backend IPv4/IPv6 address for ipvs
	Gateway string `json:",omitempty"`

	// Configure IPVS fwd-method for destination:
	//  droute tunnel masq
	// Filter out backend if set to empty string.
	IPVSMethod string `json:",omitempty"`
}

// Top-level config object
type Config struct {
	Routes   map[string]Route
	Services map[string]Service
}

func (config *Config) setService(serviceName string, service Service) {
	if config.Services == nil {
		config.Services = map[string]Service{serviceName: service}
	} else {
		config.Services[serviceName] = service
	}
}

func (config *Config) updateServices(node Node) error {
	if node.Remove {
		config.Services = nil
	}

	return nil
}

func (config *Config) updateService(node Node, serviceName string) error {
	if node.Remove {
		delete(config.Services, serviceName)
	} else {
		service := config.Services[serviceName]

		config.setService(serviceName, service)
	}

	return nil
}

func (config *Config) updateServiceFrontend(node Node, serviceName string, serviceFrontend ServiceFrontend) error {
	service := config.Services[serviceName]

	if node.Remove {
		service.Frontend = nil
	} else {
		service.Frontend = &serviceFrontend
	}

	config.setService(serviceName, service)

	return nil
}

func (config *Config) updateServiceBackends(node Node, serviceName string) error {
	service := config.Services[serviceName]

	if node.Remove {
		service.Backends = nil
	}

	config.setService(serviceName, service)

	return nil
}

func (config *Config) updateServiceBackend(node Node, serviceName string, backendName string, serviceBackend ServiceBackend) error {
	service := config.Services[serviceName]

	if node.Remove {
		delete(service.Backends, backendName)
	} else {
		service.setBackend(backendName, serviceBackend)
	}

	config.setService(serviceName, service)

	return nil
}

func (config *Config) updateRoutes(node Node) error {
	if node.Remove {
		config.Routes = nil
	}

	return nil
}

func (config *Config) updateRoute(node Node, routeName string, route Route) error {
	if node.Remove {
		// clear
		delete(config.Routes, routeName)

	} else if config.Routes == nil {
		config.Routes = map[string]Route{routeName: route}

	} else {
		config.Routes[routeName] = route
	}

	return nil
}

// Update config from Node.
//
// This modifieds the Config in-place, and is not safe against any concurrent usage.
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
				Meta: Meta{node: node},
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
					Meta:   Meta{node: node},
					Weight: ServiceBackendWeight,
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
				Meta: Meta{node: node},
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

// visit() each Node in the tree.
//
// Returns error on panic.
func (config Config) walk(visit func(node Node)) (err error) {
	defer func() {
		if panicValue := recover(); panicValue == nil {

		} else if panicError, ok := panicValue.(error); ok {
			err = panicError
		} else {
			panic(panicValue)
		}
	}()

	if config.Routes != nil {
		for routeName, route := range config.Routes {
			visit(makeNode(route, "routes", routeName))
		}
	}

	if config.Services != nil {
		for serviceName, service := range config.Services {
			if service.Frontend != nil {
				visit(makeNode(service.Frontend, "services", serviceName, "frontend"))
			}

			for backendName, backend := range service.Backends {
				visit(makeNode(backend, "services", serviceName, "backends", backendName))
			}
		}
	}

	return
}

// Walk the config, and return a map of Nodes by path.
//
// Suitable for use with writeSource.Write()
func (config Config) compile() (map[string]Node, error) {
	var nodes = map[string]Node{}

	err := config.walk(func(node Node) {
		nodes[node.Path] = node
	})
	return nodes, err
}

// Modify this Config in-place, by merging in a copy of the given Config 
func (config *Config) merge(mergeConfig Config) {
	for serviceName, mergeService := range mergeConfig.Services {
		service := config.Services[serviceName]

		service.merge(mergeService)

		if config.Services == nil {
			config.Services = map[string]Service{serviceName: service}
		} else {
			config.Services[serviceName] = service
		}
	}

	for routeName, route := range mergeConfig.Routes {
		if config.Routes == nil {
			config.Routes = map[string]Route{routeName: route}
		} else {
			config.Routes[routeName] = route
		}
	}
}
