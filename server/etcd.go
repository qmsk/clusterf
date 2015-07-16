package server

import (
    "github.com/coreos/go-etcd/etcd"
    etcdError "github.com/coreos/etcd/error"
    "fmt"
    "encoding/json"
    "log"
    "net"
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
}

type ServiceFrontend struct {
    IPv4    net.IP  `json:"ipv4,omitempty"`
    TCP     uint16  `json:"tcp,omitempty"`
}

type ServiceServer struct {
    IPv4    net.IP  `json:"ipv4,omitempty"`
    TCP     uint16  `json:"tcp,omitempty"`
}

type Service struct {
    Name        string

    Frontend    ServiceFrontend
    Servers     map[string]ServiceServer
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
func (self *Etcd) InitState() error {
    if response, err := self.client.CreateDir(self.config.Prefix, 0); err != nil {
        return err
    } else {
        self.syncIndex = response.EtcdIndex
    }

    return nil
}

/*
 * Synchronize current state in etcd.
 */
func (self *Etcd) Sync(handle func(service Service)) error {
    response, err := self.client.Get(self.config.Prefix, false, true)

    if err != nil {
        if etcdErr, ok := err.(*etcd.EtcdError); ok {
            if etcdErr.ErrorCode == etcdError.EcodeKeyNotFound {
                // create directory instead
                return self.InitState()
            }
        }

        return err
    }

    if response.Node.Dir != true {
        return fmt.Errorf("--etcd-prefix=%s is not a directory", response.Node.Key)
    }

    self.syncIndex = response.EtcdIndex

    for _, node := range response.Node.Nodes {
        name := path.Base(node.Key)

        if name == "services" && node.Dir {
            for _, serviceNode := range node.Nodes {
                service := self.SyncService(serviceNode)

                log.Printf("server:etcd.Sync: Service %+v\n", service)

                handle(service)
            }
        } else {
            log.Printf("server:etcd.Sync: Ignore unknown node: %s\n", node.Key)
        }
    }

    return nil
}

func (self *Etcd) SyncService(serviceNode *etcd.Node) Service {
    service := Service{
        Name:       path.Base(serviceNode.Key),
        Servers:    make(map[string]ServiceServer),
    }

    for _, node := range serviceNode.Nodes {
        name := path.Base(node.Key)

        if name == "frontend" {
            if err := service.Frontend.Load(node); err != nil {
                log.Printf("server:etcd.SyncService %s: Frontend.Load: %s\n", service.Name, err)
                continue
            } else {
                log.Printf("server:etcd.SyncService %s: Frontend:%+v\n", service.Name, service.Frontend)
            }
        } else if name == "servers" && node.Dir {
            for _, serverNode := range node.Nodes {
                server := ServiceServer{}
                serverName := path.Base(serverNode.Key)

                if err := server.Load(serverNode); err != nil {
                    log.Printf("server:etcd.SyncService %s: Server.Load %s: %s\n", service.Name, serverName, err)
                    continue
                } else {
                    log.Printf("server:etcd.SyncService %s: Server %s:%+v\n", service.Name, serverName, server)

                    service.Servers[serverName] = server
                }
            }
        } else {
            log.Printf("server:etcd.SyncService %s: Ignore unknown node: %s\n", service.Name, node.Key)
        }
    }

    return service
}

func (self *ServiceFrontend) Load (node *etcd.Node) error {
    if err := json.Unmarshal([]byte(node.Value), &self); err != nil {
        return err
    }

    return nil
}

func (self *ServiceServer) Load (node *etcd.Node) error {
    if err := json.Unmarshal([]byte(node.Value), &self); err != nil {
        return err
    }

    return nil
}
