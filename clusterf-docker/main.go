package main

import (
    "github.com/qmsk/clusterf/config"
    "github.com/qmsk/clusterf/docker"
    "flag"
    "log"
    "os"
)

var (
    dockerConfig docker.DockerConfig
    etcdConfig  config.EtcdConfig
)

func init() {
    flag.StringVar(&dockerConfig.Endpoint, "docker-endpoint", "",
        "Docker client endpoint for dockerd")

    flag.StringVar(&etcdConfig.Machines, "etcd-machines", "http://127.0.0.1:2379",
        "Client endpoint for etcd")
    flag.StringVar(&etcdConfig.Prefix, "etcd-prefix", "/clusterf",
        "Etcd tree prefix")
}

type self struct {
    configEtcd  *config.Etcd
    docker      *docker.Docker

    // registered state
    containers  map[string]*containerState
}

// Update container state
func (self *self) containerEvent(containerEvent docker.ContainerEvent) {
    containerState := self.containers[containerEvent.ID]

    if containerState != nil && !containerEvent.Running {
        log.Printf("containerEvent %s:%s: teardown\n", containerEvent.Status, containerEvent.ID)

        self.teardownContainer(containerState)

        delete(self.containers, containerEvent.ID)

    } else if containerEvent.Running && containerEvent.State != nil {
        if containerState == nil {
            log.Printf("containerEvent %v: new\n", containerEvent)

            self.containers[containerEvent.ID] = self.newContainer(containerEvent.State)
        } else {
            log.Printf("containerEvent %v sync\n", containerEvent)

            self.syncContainer(containerState, containerEvent.State)
        }
    } else {
        log.Printf("containerEvent %v: unknown\n", containerEvent)
    }
}

func main() {
    self := self{
        containers:    make(map[string]*containerState),
    }

    flag.Parse()

    if len(flag.Args()) > 0 {
        flag.Usage()
        os.Exit(1)
    }

    if configEtcd, err := etcdConfig.Open(); err != nil {
        log.Fatalf("config:etcd.Open: %v\n", err)
    } else {
        log.Printf("config:etcd.Open: %v\n", configEtcd)

        self.configEtcd = configEtcd
    }

    if docker, err := dockerConfig.Open(); err != nil {
        log.Fatalf("docker:Docker.Open: %v\n", err)
    } else {
        log.Printf("docker:Docker.Open: %v\n", docker)

        self.docker = docker
    }

    // scan
    if containers, err := self.docker.List(); err != nil {
        log.Fatalf("docker:Docker.List: %v\n", err)
    } else {
        log.Printf("docker:Docker.List...\n")

        for _, container := range containers {

            if container.Running {
                self.containers[container.ID] = self.newContainer(container)
            }
        }
    }

    // sync
    if containerEvents, err := self.docker.Subscribe(); err != nil {
        log.Fatalf("docker:Docker.Subscribe: %v\n", err)
    } else {
        log.Printf("docker:Docker.Subscribe...\n")

        for containerEvent := range containerEvents {
            self.containerEvent(containerEvent)
        }
    }
}
