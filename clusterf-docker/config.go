package main

import (
    "github.com/qmsk/clusterf/config"
    "github.com/qmsk/clusterf/docker"
    "fmt"
)

// Translate a docker container to a service config
func configContainer (container *docker.Container) (configs []config.Config) {
    configBackend := config.ConfigServiceBackend{}

    if serviceLabel, set := container.Labels["net.qmsk.clusterf.service"]; !set {
        return
    } else {
        configBackend.ServiceName = serviceLabel
    }

    configBackend.BackendName = container.ID
    configBackend.Backend.IPv4 = container.IPv4.String()

    for _, port := range container.Ports {
        // select correct port by label
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

    configs = append(configs, configBackend)

    return
}
