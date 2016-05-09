package docker

import (
    "github.com/fsouza/go-dockerclient"
	"fmt"
	"log"
)

type containerEvent struct {
	ID			string
	Action		string
	Container	*docker.Container
}

func (event containerEvent) String() string {
	return fmt.Sprintf("%s container %v", event.Action, event.ID)
}

type containerState struct {
	Exists		bool
	Running		bool
	Status		string
}

func (state containerState) String() string {
	return state.Status
}

// Expected state for container from Action.
//
// This does not necessarily reflect the State of the inspected container, since events and inspect are racy
//
// https://docs.docker.com/engine/reference/api/images/event_state.png
func (event containerEvent) State() containerState {
	var state containerState

	if event.Container != nil {
		state.Exists = true
		state.Running = event.Container.State.Running
		state.Status = event.Container.State.Status
	}

	switch event.Action {
	case "create":
		state.Status = "created"
	case "start", "restart":
		state.Status = "running"
	case "stop":
		state.Status = "stopped"
		state.Running = false
	case "die", "kill", "oom":
		state.Status = "stopping"
		state.Running = false
	case "pause":
		state.Status = "paused"
		state.Running = false
	case "unpause":
		state.Status = "running"
	case "destroy":
		state.Status = "deleted"
		state.Exists = false
	}

	return state
}

type Containers	map[string]*docker.Container

func (containers Containers) clone() Containers {
	copy := make(Containers)

	for containerID, container := range containers {
		copy[containerID] = container
	}

	return copy
}

func (containers Containers) list(container *docker.Container) {
	if container.State.Running {
		log.Printf("docker:Containers.list %v: up", container.ID)
	} else {
		log.Printf("docker:Containers.list %v: down", container.ID)
	}

	containers[container.ID] = container
}

func (containers Containers) update(event containerEvent) {
	state := event.State()

	if !state.Exists {
		log.Printf("docker:Containers.update %v -> %v: remove", event, state)

		delete(containers, event.ID)

	} else {
		if !state.Running {
			// override
			event.Container.State.Running = false
		}

		if event.Container.State.Running {
			log.Printf("docker:Containers.update %v -> %v: down", event, state)
		} else {
			log.Printf("docker:Containers.update %v -> %v: up", event, state)
		}

		containers[event.ID] = event.Container
	}
}
