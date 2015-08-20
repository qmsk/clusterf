package config

import (
    "github.com/coreos/go-etcd/etcd"
    "fmt"
    "encoding/json"
    "log"
    "path"
)

func loadEtcdServiceFrontend (node *etcd.Node) (frontend ServiceFrontend, err error) {
    err = json.Unmarshal([]byte(node.Value), &frontend)

    return
}

func loadEtcdServiceBackend (node *etcd.Node) (backend ServiceBackend, err error) {
    err = json.Unmarshal([]byte(node.Value), &backend)

    return
}

/*
 * Scan through the recursive /clusterf/services/ node to build a new Services set.
 */
func (self *Etcd) scanServices(servicesNode *etcd.Node, configHandler func(config Config)) {
    for _, serviceNode := range servicesNode.Nodes {
        serviceName := path.Base(serviceNode.Key)

        log.Printf("config:etcd.scan %s: Service %s\n", serviceNode.Key, serviceName)

        configHandler(&ConfigService{
            ServiceName: serviceName,
        })

        for _, node := range serviceNode.Nodes {
            name := path.Base(node.Key)

            if name == "frontend" {
                if frontend, err := loadEtcdServiceFrontend(node); err != nil {
                    log.Printf("config:etcd.scan %s: loadEtcdServiceFrontend: %s\n", node.Key, err)
                    continue
                } else {
                    log.Printf("config:etcd.scan %s: Service %s Frontend: %+v\n", node.Key, serviceName, frontend)

                    configHandler(&ConfigServiceFrontend{
                        ServiceName:    serviceName,
                        Frontend:       frontend,
                    })
                }
            } else if name == "backends" && node.Dir {
                for _, backendNode := range node.Nodes {
                    backendName := path.Base(backendNode.Key)

                    if backend, err := loadEtcdServiceBackend(backendNode); err != nil {
                        log.Printf("config:etcd.scan %s: loadEtcdServiceBackend: %s\n", backendNode.Key, err)
                        continue
                    } else {
                        log.Printf("config:etcd.scan %s: Service %s Backend %s: %+v\n", node.Key, serviceName, backendName, backend)

                        configHandler(&ConfigServiceBackend{
                            ServiceName:    serviceName,
                            BackendName:    backendName,
                            Backend:        backend,
                        })
                    }
                }
            } else {
                log.Printf("config:etcd.scan %s: Ignore unknown node\n", node.Key)
            }
        }
    }
}

// map config node path and value to Config
func (self *Etcd) syncPath(nodePath []string, node *etcd.Node) (Config, error) {
    // match config tree path
    if len(nodePath) == 0 && node.Dir {
        // XXX: just ignore? Undefined if it makes sense to do anything here
        return nil, nil

    } else if len(nodePath) == 1 && nodePath[0] == "services" && node.Dir {
        // recursive on all services
        return &ConfigService{ }, nil

    } else if len(nodePath) >= 2 && nodePath[0] == "services" {
        serviceName := nodePath[1]

        if len(nodePath) == 2 && node.Dir {
            return &ConfigService{ServiceName: serviceName}, nil

        } else if len(nodePath) == 3 && nodePath[2] == "frontend" && !node.Dir {
            if node.Value == "" {
                // deleted node has empty value
                return &ConfigServiceFrontend{ServiceName: serviceName}, nil
            } else if frontend, err := loadEtcdServiceFrontend(node); err != nil {
                return nil, fmt.Errorf("service %s frontend: %s", serviceName, err)
            } else {
                return &ConfigServiceFrontend{ServiceName: serviceName, Frontend: frontend}, nil
            }

        } else if len(nodePath) == 3 && nodePath[2] == "backends" && node.Dir {
            // recursive on all backends
            return &ConfigServiceBackend{ServiceName: serviceName}, nil

        } else if len(nodePath) >= 4 && nodePath[2] == "backends" {
            backendName := nodePath[3]

            if len(nodePath) == 4 && !node.Dir {
                if node.Value == "" {
                    // deleted node has empty value
                    return &ConfigServiceBackend{ServiceName: serviceName, BackendName: backendName}, nil
                } else if backend, err := loadEtcdServiceBackend(node); err != nil {
                    return nil, fmt.Errorf("service %s backend %s: %s", serviceName, backendName, err)
                } else {
                    return &ConfigServiceBackend{ServiceName: serviceName, BackendName: backendName, Backend: backend}, nil
                }

            } else {
                return nil, fmt.Errorf("Ignore unknown service %s backends node", serviceName)
            }

        } else {
            return nil, fmt.Errorf("Ignore unknown service %s node", serviceName)
        }

    } else {
        return nil, fmt.Errorf("Ignore unknown node")
    }

    return nil, nil
}
