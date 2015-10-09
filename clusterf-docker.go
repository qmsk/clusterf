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

func main() {
    flag.Parse()

    if len(flag.Args()) > 0 {
        flag.Usage()
        os.Exit(1)
    }

    var self struct {
        configEtcd *config.Etcd
        docker *docker.Docker
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
        }
    }

    // sync
    if containerEvents, err := self.docker.Subscribe(); err != nil {
        log.Fatalf("docker:Docker.Subscribe: %v\n", err)
    } else {
        for containerEvent := range containerEvents {
            log.Printf("Docker:Docker.Subscribe: %#v\n", containerEvent)
        }
    }
}
