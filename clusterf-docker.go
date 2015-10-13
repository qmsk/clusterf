package main
import (
    "qmsk.net/clusterf/config"
    "qmsk.net/clusterf/docker"
    "flag"
    "fmt"
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
    containerConfig map[string]*config.ConfigServiceBackend
}

// Translate a docker container to a service config
func (self *self) configContainer (container *docker.Container) *config.ConfigServiceBackend {
    configBackend := config.ConfigServiceBackend{}

    if serviceLabel, set := container.Labels["net.qmsk.clusterf.service"]; !set {
        return nil
    } else {
        configBackend.ServiceName = serviceLabel
    }

    configBackend.BackendName = container.ID
    configBackend.Backend.IPv4 = container.IPv4.String()

    for _, port := range container.Ports {
        // limit by label?
        portLabel := container.Labels[fmt.Sprintf("net.qmsk.clusterf.backend.%s", port.Proto)]

        if portLabel != "" && portLabel != fmt.Sprintf("%v", port.Port) {
            continue
        }

        switch port.Proto {
        case "tcp":
            configBackend.Backend.TCP = port.Port
        case "udp":
            configBackend.Backend.UDP = port.Port
        }
    }

    return &configBackend
}

// Synchronize active container state to config
func (self *self) syncContainer(container *docker.Container) {
    containerConfig := self.configContainer(container)

    if self.containerConfig[container.ID] == containerConfig {
        // no-op
        log.Printf("syncContainer %s: no-op\n", container.ID)

        return
    }

    log.Printf("syncContainer %s: update: %#v\n", container.ID, containerConfig)

    self.containerConfig[container.ID] = containerConfig
}

// Teardown container state if active
func (self *self) teardownContainer(containerID string) {
    if containerConfig, exists := self.containerConfig[containerID]; !exists {
        log.Printf("teardownContainer %s: unknown\n", containerID)

    } else {
        log.Printf("teardownContainer %s: %#v\n", containerID, containerConfig)

        delete(self.containerConfig, containerID)
    }
}

// Update container state
func (self *self) containerEvent(containerEvent docker.ContainerEvent) {
    if !containerEvent.Running {
        log.Printf("containerEvent %s:%s: teardown\n", containerEvent.Status, containerEvent.ID)

        self.teardownContainer(containerEvent.ID)

    } else if containerEvent.State != nil {
        log.Printf("containerEvent %s:%s: sync\n", containerEvent.Status, containerEvent.ID)

        self.syncContainer(containerEvent.State)

    } else {
        log.Printf("containerEvent %s:%s: unknown\n", containerEvent.Status, containerEvent.ID)
    }
}

func main() {
    self := self{
        containerConfig:    make(map[string]*config.ConfigServiceBackend),
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
