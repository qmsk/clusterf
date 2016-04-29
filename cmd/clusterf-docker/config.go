package main

import (
    "github.com/qmsk/clusterf/config"
    "github.com/qmsk/clusterf/docker"
    "fmt"
    "strings"
)

// Translate a docker container into service configs
func configContainer(updateConfig *config.Config, container docker.Container) error {
	backendName := container.ID

    // map ports
    containerPorts := make(map[string]docker.Port)

    for _, port := range container.Ports {
        containerPorts[fmt.Sprintf("%s:%d", port.Proto, port.Port)] = port
    }

    // services
    for _, serviceName := range strings.Fields(container.Labels["net.qmsk.clusterf.service"]) {
        var backend config.ServiceBackend

        if container.IPv4 != nil {
            backend.IPv4 = container.IPv4.String()
        }

        // find potential ports for service by label
        portLabels := []struct{
            proto string
            label string
        }{
            {"tcp", "net.qmsk.clusterf.backend.tcp"},
            {"udp", "net.qmsk.clusterf.backend.udp"},
            {"tcp", fmt.Sprintf("net.qmsk.clusterf.backend:%s.tcp", serviceName)},
            {"udp", fmt.Sprintf("net.qmsk.clusterf.backend:%s.udp", serviceName)},
        }

        for _, portLabel := range portLabels {
            // lookup exposed docker.Port
            portName, labelFound := container.Labels[portLabel.label]
            if !labelFound {
                continue
            }

            port, portFound := containerPorts[fmt.Sprintf("%s:%s", portLabel.proto, portName)]
            if !portFound {
				// ignore
                fmt.Printf("configContainer %v: service %v port %v is not exposed\n", container, serviceName, portName)
				continue
            }

            // configure
            switch port.Proto {
            case "tcp":
                backend.TCP = port.Port
            case "udp":
                backend.UDP = port.Port
            }
        }

        if backend.TCP == 0 && backend.UDP == 0 {
			continue
		}

		if service, exists := updateConfig.Services[serviceName]; exists {
			service.Backends[backendName] = backend
		} else {
			updateConfig.Services[serviceName] = config.Service{
				Backends: map[string]config.ServiceBackend{
					backendName: backend,
				},
			}
		}
    }

    return nil
}

func configContainers (containers docker.Containers) (config.Config, error) {
	var config = config.Config{
		Services:	make(map[string]config.Service),
	}

	for _, container := range containers {
		if err := configContainer(&config, container); err != nil {
			return config, err
		}
	}

	return config, nil
}
