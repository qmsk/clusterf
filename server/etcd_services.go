package server

import (
    "github.com/coreos/go-etcd/etcd"
    "fmt"
    "encoding/json"
    "log"
    "path"
    "strings"
)

func loadEtcdServiceFrontend (node *etcd.Node) (*ServiceFrontend, error) {
    frontend := &ServiceFrontend{}

    if err := json.Unmarshal([]byte(node.Value), &frontend); err != nil {
        return nil, err
    }

    return frontend, nil
}

func loadEtcdServiceBackend (node *etcd.Node) (*ServiceBackend, error) {
    backend := &ServiceBackend{}

    if err := json.Unmarshal([]byte(node.Value), &backend); err != nil {
        return nil, err
    }

    return backend, nil
}

/*
 * Scan through the recursive /clusterf/services/ node to build a new Services set.
 */
func (self *Etcd) scanServices(servicesNode *etcd.Node) *Services {
    services := newServices()

    for _, serviceNode := range servicesNode.Nodes {
        service := self.scanService(serviceNode)

        log.Printf("server:etcd.Scan %s: Service %+v\n", serviceNode.Key, service)

        services.add(service)
    }

    return services
}

/*
 * Scan through the recursive /clusterf/services/../ node to build a new Service.
 */
func (self *Etcd) scanService(serviceNode *etcd.Node) *Service {
    serviceName := path.Base(serviceNode.Key)
    service := newService(serviceName)

    for _, node := range serviceNode.Nodes {
        name := path.Base(node.Key)

        if name == "frontend" {
            if frontend, err := loadEtcdServiceFrontend(node); err != nil {
                log.Printf("server:etcd.scanService %s: loadEtcdServiceFrontend: %s\n", node.Key, err)
                continue
            } else {
                log.Printf("server:etcd.scanService %s: Frontend: %+v\n", serviceName, frontend)

                service.Frontend = frontend
            }
        } else if name == "backends" && node.Dir {
            /* Scan backends */
            for _, backendNode := range node.Nodes {
                backendName := path.Base(backendNode.Key)

                if backend, err := loadEtcdServiceBackend(backendNode); err != nil {
                    log.Printf("server:etcd.scanService %s: loadEtcdServiceBackend %s: %s\n", backendNode.Key, backendName, err)
                    continue
                } else {
                    log.Printf("server:etcd.scanService %s: Backend %s:%+v\n", serviceName, backendName, backend)

                    service.Backends[backendName] = backend
                }
            }
        } else {
            log.Printf("server:etcd.scanService %s: Ignore unknown node\n", node.Key)
        }
    }

    return service
}

/*
 * Figure out what changed, parse any new JSON value, and update the Services state.
 *
 * Changes to the Services state are passed to the given eventHandler func. Any nil events should be ignored.
 */
func (self *Etcd) sync(action string, node *etcd.Node, eventHandler func(event *Event)) error {
    path := node.Key

    if strings.HasPrefix(path, self.config.Prefix) {
        path = strings.TrimPrefix(path, self.config.Prefix)
    } else {
        return fmt.Errorf("path outside tree")
    }
    path = strings.Trim(path, "/")

    log.Printf("etcd:sync: %s %v\n", action, path)

    // match against our tree
    nodePath := strings.Split(path, "/")

    if len(path) == 0 {
        // Split("", "/") would give [""]
        nodePath = nil
    }

    if len(nodePath) == 0 && node.Dir {
        // XXX: just ignore? Undefined if it makes sense to do anything here
        log.Printf("server:etcd.sync: %s\n", action)

    } else if len(nodePath) == 1 && nodePath[0] == "services" && node.Dir {
        log.Printf("server:etcd.sync: services: %s\n", action)

        // propagate
        // XXX: should this be moved to inside services; we both iterate and mutate the map here
        for _, service := range self.services.services {
            eventHandler(self.services.syncService(service, action))
        }
    } else if len(nodePath) >= 2 && nodePath[0] == "services" {
        serviceName := nodePath[1]
        service := self.services.get(serviceName)

        if len(nodePath) == 2 && node.Dir {
            eventHandler(self.services.syncService(service, action))

        } else if len(nodePath) == 3 && nodePath[2] == "frontend" && !node.Dir {
            if node.Value == "" {
                // deleted node has empty value
                eventHandler(service.syncFrontend(action, nil))
            } else if frontend, err := loadEtcdServiceFrontend(node); err != nil {
                return fmt.Errorf("service %s frontend: %s", serviceName, err)
            } else {
                eventHandler(service.syncFrontend(action, frontend))
            }

        } else if len(nodePath) == 3 && nodePath[2] == "backends" && node.Dir {
            log.Printf("server:etcd.sync: services %s backends: %s\n", serviceName, action)

            // propagate
            for backendName, backend := range service.Backends {
                eventHandler(service.syncBackend(backendName, action, backend))
            }

        } else if len(nodePath) >= 4 && nodePath[2] == "backends" {
            backendName := nodePath[3]

            if len(nodePath) == 4 && !node.Dir {
                if node.Value == "" {
                    // deleted node has empty value
                    eventHandler(service.syncBackend(backendName, action, nil))
                } else if backend, err := loadEtcdServiceBackend(node); err != nil {
                    return fmt.Errorf("service %s backend %s: %s", serviceName, backendName, err)
                } else {
                    eventHandler(service.syncBackend(backendName, action, backend))
                }

            } else {
                return fmt.Errorf("Ignore unknown service %s backends node: %s", serviceName, path)
            }

        } else {
            return fmt.Errorf("Ignore unknown service %s node: %s", serviceName, path)
        }

    } else {
        return fmt.Errorf("Ignore unknown node: %s", path)
    }

    return nil
}
