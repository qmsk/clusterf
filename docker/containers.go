package docker

import (
	"log"
)

type Containers	map[string]Container

func (containers Containers) clone() Containers {
	copy := make(Containers)

	for containerID, container := range containers {
		copy[containerID] = container
	}

	return copy
}

func (containers Containers) update(containerEvent ContainerEvent) {
    _, exists := containers[containerEvent.ID]

    if containerEvent.Running {
        if containerEvent.State == nil {
            log.Printf("containerEvent %v: unknown\n", containerEvent)

			return
		}

        if !exists {
            log.Printf("containerEvent %v: new\n", containerEvent)
        } else {
            log.Printf("containerEvent %v sync\n", containerEvent)
        }

		containers[containerEvent.ID] = *containerEvent.State
    } else {
        if !exists {
            log.Printf("containerEvent %v: skip\n", containerEvent)
        } else {
            log.Printf("containerEvent %v: teardown\n", containerEvent)

            delete(containers, containerEvent.ID)
        }
    }
}
