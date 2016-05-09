package main

import (
    "github.com/qmsk/clusterf/config"
	docker "github.com/qmsk/clusterf/docker" // XXX: mixing two APIs sucks
    dockerclient "github.com/fsouza/go-dockerclient"
    "fmt"
	"log"
    "strings"
)

// Translate a docker container into service configs
func configContainer(updateConfig *config.Config, container *dockerclient.Container) error {
	labels := container.Config.Labels
    exposedPorts := container.Config.ExposedPorts

    // service backends
	backendName := container.ID

    for _, serviceName := range strings.Fields(labels["net.qmsk.clusterf.service"]) {
        var backend config.ServiceBackend

        backend.IPv4 = container.NetworkSettings.IPAddress
        backend.IPv6 = container.NetworkSettings.GlobalIPv6Address

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
            portName, labelFound := labels[portLabel.label]
            if !labelFound {
                continue
            }

			dockerPort := dockerclient.Port(fmt.Sprintf("%s/%s", portName, portLabel.proto))

			// check that the port is exposed
            _, portFound := exposedPorts[dockerPort]
            if !portFound {
				// ignore
                log.Printf("configContainer %v: service %v %s port %s is not exposed\n", container.ID, serviceName, portLabel.proto, dockerPort)
				continue
            }

			var port uint16

			if _, err := fmt.Sscanf(dockerPort.Port(), "%d", &port); err != nil {
				log.Printf("configContainer %v: service %v %v port invalid: %#v", container.ID, serviceName, portLabel.proto, dockerPort.Port())
				continue
			}

            // configure
            switch dockerPort.Proto() {
            case "tcp":
                backend.TCP = port
            case "udp":
                backend.UDP = port
            }

			// state
			if container.State.Running {
				backend.Weight = config.ServiceBackendWeight
			} else {
				backend.Weight = 0
			}
        }

		if backend.IPv4 == "" && backend.IPv6 == "" {
			continue
		}

        if backend.TCP == 0 && backend.UDP == 0 {
			continue
		}

		if service, serviceExists := updateConfig.Services[serviceName]; !serviceExists {
			updateConfig.Services[serviceName] = config.Service{
				Backends: map[string]config.ServiceBackend{
					backendName: backend,
				},
			}
		} else if serviceBackend, backendExists := service.Backends[backendName]; !backendExists {
			service.Backends[backendName] = backend
		} else {
			log.Printf("configContainer %v: service %v backend %v collision: %v", serviceName, backendName, serviceBackend)
		}
    }

    return nil
}

func makeConfig (dockerState docker.State) (config.Config, error) {
	var config = config.Config{
		Services:	make(map[string]config.Service),
	}

	for _, container := range dockerState.Containers {
		if err := configContainer(&config, container); err != nil {
			return config, err
		}
	}

	return config, nil
}
