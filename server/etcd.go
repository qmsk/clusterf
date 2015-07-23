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

    services    *Services
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

func (self *ServiceBackend) loadEtcd (node *etcd.Node) error {
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

    return e, nil
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

            event, err := self.sync(response.Action, response.Node)
            if err != nil {
                log.Printf("server:etcd.sync: %s\n", err)
            } else if event != nil {
                log.Printf("server:etcd.sync: %s\n", event)

                out <- *event
            }
        }
    }()

    return out
}

/*
 * Parse a changed node and return any actions.
 */
func (self *Etcd) sync(action string, node *etcd.Node) (*Event, error) {
    var event *Event
    path := node.Key

    if strings.HasPrefix(path, self.config.Prefix) {
        path = strings.TrimPrefix(path, self.config.Prefix)
    } else {
        return nil, fmt.Errorf("path outside tree")
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
        // XXX: just ignore?
        log.Printf("server:etcd.sync: %s\n", action)

    } else if len(nodePath) == 1 && nodePath[0] == "services" && node.Dir {
        log.Printf("server:etcd.sync: services: %s\n", action)

        // propagate; XXX: range what
        for _, service := range self.services.services {
            // XXX: multiple events
            event = self.services.syncService(service, action)
        }
    } else if len(nodePath) >= 2 && nodePath[0] == "services" {
        serviceName := nodePath[1]
        service := self.services.get(serviceName)

        if len(nodePath) == 2 && node.Dir {
            event = self.services.syncService(service, action)

        } else if len(nodePath) == 3 && nodePath[2] == "frontend" && !node.Dir {
            var frontend ServiceFrontend

            if node.Value == "" {
                event = service.syncFrontend(action, nil)
            } else if err := frontend.loadEtcd(node); err != nil {
                return nil, fmt.Errorf("service %s frontend: %s", serviceName, err)
            } else {
                event = service.syncFrontend(action, &frontend)
            }

        } else if len(nodePath) == 3 && nodePath[2] == "backends" && node.Dir {
            log.Printf("server:etcd.sync: services %s backends: %s\n", serviceName, action)

            // propagate
            for backendName, backend := range service.Backends {
                // XXX: multiple events
                event = service.syncBackend(backendName, action, backend)
            }

        } else if len(nodePath) >= 4 && nodePath[2] == "backends" {
            backendName := nodePath[3]

            if len(nodePath) == 4 && !node.Dir {
                var backend ServiceBackend

                if node.Value == "" {
                    event = service.syncBackend(backendName, action, nil)
                } else if err := backend.loadEtcd(node); err != nil {
                    return nil, fmt.Errorf("service %s backend %s: %s", serviceName, backendName, err)
                } else {
                    event = service.syncBackend(backendName, action, &backend) // XXX: pointer-to-value?
                }

            } else {
                return nil, fmt.Errorf("Ignore unknown service %s backends node: %s", serviceName, path)
            }

        } else {
            return nil, fmt.Errorf("Ignore unknown service %s node: %s", serviceName, path)
        }

    } else {
        return nil, fmt.Errorf("Ignore unknown node: %s", path)
    }

    return event, nil
}

/*
 * Synchronize current state in etcd.
 *
 * Returns an atomic snapshot of the state in etcd, and sets the .syncIndex
 */
func (self *Etcd) Scan() ([]*Service, error) {
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

    for _, node := range response.Node.Nodes {
        name := path.Base(node.Key)

        if name == "services" && node.Dir {
            self.services = self.scanServices(response.Node)
        } else {
            log.Printf("server:etcd.Scan %s: Ignore unknown node\n", node.Key)
        }
    }

    if self.services != nil {
        return self.services.Services(), nil
    } else {
        return nil, nil
    }
}

func (self *Etcd) scanServices(servicesNode *etcd.Node) *Services {
    services := newServices()

    for _, serviceNode := range servicesNode.Nodes {
        service := self.scanService(serviceNode)

        log.Printf("server:etcd.Scan %s: Service %+v\n", serviceNode.Key, service)

        self.services.add(service)
    }

    return services
}

func (self *Etcd) scanService(serviceNode *etcd.Node) *Service {
    serviceName := path.Base(serviceNode.Key)
    service := newService(serviceName)

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
        } else if name == "backends" && node.Dir {
            for _, backendNode := range node.Nodes {
                backendName := path.Base(backendNode.Key)
                backend := ServiceBackend{}

                if err := backend.loadEtcd(backendNode); err != nil {
                    log.Printf("server:etcd.scanService %s: ServiceBackend.loadEtcd %s: %s\n", backendNode.Key, backendName, err)
                    continue
                } else {
                    log.Printf("server:etcd.scanService %s: Backend %s:%+v\n", serviceName, backendName, backend)

                    service.Backends[backendName] = &backend // XXX: plant possible pointer-to-for-loop-struct-value-variable bug to discover via later testing
                }
            }
        } else {
            log.Printf("server:etcd.scanService %s: Ignore unknown node\n", node.Key)
        }
    }

    return service
}
