package main
import (
    "qmsk.net/clusterf/config"
    "qmsk.net/clusterf/docker"
    "flag"
    "log"
    "os"
)

var (
    dockerConfig docker.DockerConfig
    etcdConfig  config.EtcdConfig
)

func init() {
    flag.StringVar(&dockerConfig.Endpoint, "docker-endpoint", "unix:///var/run/docker.sock",
        "Docker client endpoint for dockerd")

    flag.StringVar(&etcdConfig.Machines, "etcd-machines", "http://127.0.0.1:2379",
        "Client endpoint for etcd")
    flag.StringVar(&etcdConfig.Prefix, "etcd-prefix", "/clusterf",
        "Etcd tree prefix")
}

type self struct {
    configEtcd *config.Etcd
    docker *docker.Docker

    // registered state
    containerState map[string]*docker.Container
}

// Synchronize active container state to config
func (self *self) syncContainer(container *docker.Container) {
    if self.containerState[container.ID] == container {
        // no-op
        log.Printf("syncContainer %s: no-op\n", container.ID)

        return
    }

    log.Printf("syncContainer %s: update\n", container.ID)

    self.containerState[container.ID] = container
}

// Teardown inactive container state
func (self *self) teardownContainer(container *docker.Container) {
    log.Printf("teardownContainer %s\n", container.ID)

    delete(self.containerState, container.ID)
}

// Update container state
func (self *self) containerEvent(containerEvent docker.ContainerEvent) {
    container := self.containerState[containerEvent.ID]

    if containerEvent.State != nil {
        container = containerEvent.State
    }

    if container == nil {
        log.Printf("containerEvent %s:%s: unknown\n", containerEvent.Status, containerEvent.ID)

    } else if !containerEvent.Running {
        log.Printf("containerEvent %s:%s: teardown\n", containerEvent.Status, containerEvent.ID)

        self.teardownContainer(container)

    } else {
        log.Printf("containerEvent %s:%s: sync\n", containerEvent.Status, containerEvent.ID)

        self.syncContainer(container)
    }
}

func main() {
    self := self{
        containerState:  make(map[string]*docker.Container),
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
        for _, container := range containers {
            log.Printf("docker:Docker.List: %#v\n", container)

            self.syncContainer(container)
        }
    }

    // sync
    if containerEvents, err := self.docker.Subscribe(); err != nil {
        log.Fatalf("docker:Docker.Subscribe: %v\n", err)
    } else {
        for containerEvent := range containerEvents {
            log.Printf("Docker:Docker.Subscribe: %#v\n", containerEvent)

            self.containerEvent(containerEvent)
        }
    }
}
