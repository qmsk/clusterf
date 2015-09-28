package config

import (
    "github.com/coreos/go-etcd/etcd"
    "log"
    "path"
)

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
                compatNode := Node{Value: node.Value}

                if frontend, err := compatNode.loadServiceFrontend(); err != nil {
                    log.Printf("config:etcd.scan %s: loadServiceFrontend: %s\n", node.Key, err)
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
                    compatNode := Node{Value: backendNode.Value}

                    if backend, err := compatNode.loadServiceBackend(); err != nil {
                        log.Printf("config:etcd.scan %s: loadServiceBackend: %s\n", backendNode.Key, err)
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

