package server

import (
    "github.com/coreos/go-etcd/etcd"
    etcdError "github.com/coreos/etcd/error"
    "fmt"
    "encoding/json"
    "log"
    "path"
    "strings"
)

type EtcdConfig struct {
    Machines    string
    Prefix      string
}

type Etcd struct {
    config      EtcdConfig
    client      *etcd.Client
    syncIndex   uint64

    services    map[string]Service
}

func (self *Etcd) String() string {
    return fmt.Sprintf("%s%s", self.config.Machines, self.config.Prefix)
}

func (self *ServiceFrontend) loadEtcd (node *etcd.Node) error {
    if err := json.Unmarshal([]byte(node.Value), &self); err != nil {
        return err
    }

    return nil
}

func (self *ServiceServer) loadEtcd (node *etcd.Node) error {
    if err := json.Unmarshal([]byte(node.Value), &self); err != nil {
        return err
    }

    return nil
}

/*
 * Open etcd session
 */
func (self EtcdConfig) Open() (*Etcd, error) {
    e := &Etcd{config: self}

    machines := strings.Split(self.Machines, ",")

    e.client = etcd.NewClient(machines)

    e.services = make(map[string]Service)

    return e, nil
}

/*
 * Get current list of Services.
 */
func (self *Etcd) Services() []Service {
    services := make([]Service, 0, len(self.services))

    for _, service := range self.services {
        if service.Frontend == nil {
            continue
        }

        services = append(services, service)
    }

    return services
}

/*
 * Initialize state in etcd
 */
func (self *Etcd) Init() error {
    if response, err := self.client.CreateDir(self.config.Prefix, 0); err != nil {
        return err
    } else {
        self.syncIndex = response.Node.CreatedIndex
    }

    return nil
}

/*
 * Watch for changes in etcd
 *
 * Sends any changes on the returned channel.
 */
func (self *Etcd) Sync() chan Event {
    // kick off new goroutine to handle initial services and updates
    out := make(chan Event)

    go func() {
        defer close(out)

        for {
            response, err := self.client.Watch(self.config.Prefix, self.syncIndex + 1, true, nil, nil)
            if err != nil {
                log.Printf("etcd:Watch %s @ %d: %s\n", self.config.Prefix, self.syncIndex + 1, err)
                break
            } else {
                self.syncIndex = response.Node.ModifiedIndex
            }

            if response.PrevNode != nil {
                log.Printf("etcd.Watch: %s %+v <- %+v\n", response.Action, response.Node, response.PrevNode)
            } else {
                log.Printf("etcd.Watch: %s %+v\n", response.Action, response.Node)
            }

            if err := self.sync(response.Action, response.Node); err != nil {
                log.Printf("server:etcd.sync: %s\n", err)
            }
        }
    }()

    return out
}

/*
 * Parse a changed node and return any actions.
 */
func (self *Etcd) sync(action string, node *etcd.Node) error {
    path := node.Key
    path = strings.TrimPrefix(path, self.config.Prefix)
    path = strings.Trim(path, "/")

    log.Printf("etcd:sync: %s %v\n", action, path)

    // match against our tree
    nodePath := strings.Split(path, "/")

    if len(path) == 0 {
        // Split("", "/") would give [""]
        nodePath = nil
    }

    if len(nodePath) == 0 && node.Dir {
        // XXX: handle rmdir/mkdir of root?
        log.Printf("server:etcd.sync: %s\n", action)

    } else if len(nodePath) == 1 && nodePath[0] == "services" && node.Dir {
        log.Printf("server:etcd.sync: services: %s\n", action)

        // propagate
        for _, service := range self.services {
            service.sync(action)
        }

    } else if len(nodePath) >= 2 && nodePath[0] == "services" {
        serviceName := nodePath[1]
        service, serviceExists := self.services[serviceName]

        if !serviceExists {
            service = Service{Name: serviceName}
        }

        if len(nodePath) == 2 && node.Dir {
            service.sync(action)

        } else if len(nodePath) == 3 && nodePath[2] == "frontend" && !node.Dir {
            var frontend ServiceFrontend

            if node.Value == "" {
                service.syncFrontend(action, nil)
            } else if err := frontend.loadEtcd(node); err != nil {
                return fmt.Errorf("service %s frontend: %s\n", serviceName, err)
            } else {
                service.syncFrontend(action, &frontend)
            }

        } else if len(nodePath) == 3 && nodePath[2] == "servers" && node.Dir {
            log.Printf("server:etcd.sync: services %s servers: %s\n", serviceName, action)

            // propagate
            for serverName, server := range service.Servers {
                service.syncServer(serverName, action, &server) // XXX: server?
            }

        } else if len(nodePath) >= 4 && nodePath[2] == "servers" {
            serverName := nodePath[3]

            if len(nodePath) == 4 && !node.Dir {
                var server ServiceServer

                if node.Value == "" {
                    service.syncServer(serverName, action, nil)
                } else if err := server.loadEtcd(node); err != nil {
                    return fmt.Errorf("service %s server %s: %s\n", serviceName, serverName, err)
                } else {
                    service.syncServer(serverName, action, &server)
                }

            } else {
                return fmt.Errorf("Ignore unknown service %s servers node: %s\n", serviceName, path)
            }

        } else {
            return fmt.Errorf("Ignore unknown service %s node: %s\n", serviceName, path)
        }

        if !serviceExists {
            self.services[serviceName] = service
        }

    } else {
        return fmt.Errorf("Ignore unknown node: %s\n", path)
    }

    return nil
}

/*
 * Synchronize current state in etcd.
 *
 * Returns an atomic snapshot of the state in etcd, and sets the .syncIndex
 */
func (self *Etcd) Scan() ([]Service, error) {
    response, err := self.client.Get(self.config.Prefix, false, /* recursive */ true)

    if err != nil {
        if etcdErr, ok := err.(*etcd.EtcdError); ok {
            if etcdErr.ErrorCode == etcdError.EcodeKeyNotFound {
                // create directory instead
                return nil, self.Init()
            }
        }

        return nil, err
    }

    if response.Node.Dir != true {
        return nil, fmt.Errorf("--etcd-prefix=%s is not a directory", response.Node.Key)
    }

    // the tree root's ModifiedTime may be a long long time in the past, so we can't want to use that for waits
    // XXX: is this enough to ensure atomic sync with later .Watch() on the same tree?
    self.syncIndex = response.EtcdIndex

    return self.scan(response.Node), nil
}

func (self *Etcd) scan(rootNode *etcd.Node) []Service {
    for _, node := range rootNode.Nodes {
        name := path.Base(node.Key)

        if name == "services" && node.Dir {
            for _, serviceNode := range node.Nodes {
                service := self.scanService(serviceNode)

                log.Printf("server:etcd.Scan %s: Service %+v\n", serviceNode.Key, service)

                self.services[service.Name] = service
            }
        } else {
            log.Printf("server:etcd.Scan %s: Ignore unknown node\n", node.Key)
        }
    }

    return self.Services()
}

func (self *Etcd) scanService(serviceNode *etcd.Node) Service {
    service := Service{
        Name:       path.Base(serviceNode.Key),
        Servers:    make(map[string]ServiceServer),
    }

    for _, node := range serviceNode.Nodes {
        name := path.Base(node.Key)

        if name == "frontend" {
            service.Frontend = &ServiceFrontend{}

            if err := service.Frontend.loadEtcd(node); err != nil {
                log.Printf("server:etcd.scanService %s: ServiceFrontend.loadEtcd: %s\n", node.Key, err)
                continue
            } else {
                log.Printf("server:etcd.scanService %s: Frontend:%+v\n", node.Key, service.Frontend)
            }
        } else if name == "servers" && node.Dir {
            for _, serverNode := range node.Nodes {
                server := ServiceServer{}
                serverName := path.Base(serverNode.Key)

                if err := server.loadEtcd(serverNode); err != nil {
                    log.Printf("server:etcd.scanService %s: ServiceServer.loadEtcd %s: %s\n", serverNode.Key, serverName, err)
                    continue
                } else {
                    log.Printf("server:etcd.scanService %s: Server %s:%+v\n", serverNode.Key, serverName, server)

                    service.Servers[serverName] = server
                }
            }
        } else {
            log.Printf("server:etcd.scanService %s: Ignore unknown node\n", node.Key)
        }
    }

    return service
}
