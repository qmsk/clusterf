package docker

import (
	"fmt"
	"github.com/fsouza/go-dockerclient"
	"log"
)

type networkEvent struct {
	ID      string
	Action  string
	Network *docker.Network
}

func (event networkEvent) String() string {
	return fmt.Sprintf("%s network %v", event.Action, event.ID)
}

func (event networkEvent) State() networkState {
	var state networkState

	if event.Network != nil {
		state.Exists = true
	}

	switch event.Action {
	case "create":

	case "connect", "disconnect":

	case "destroy":
		state.Exists = false
	}

	return state
}

type networkState struct {
	Exists bool
}

func (state networkState) String() string {
	return fmt.Sprintf("exists=%v", state.Exists)
}

type Networks map[string]*docker.Network

func (networks Networks) clone() Networks {
	copy := make(Networks)

	for id, network := range networks {
		copy[id] = network
	}

	return copy
}

func (networks Networks) list(network docker.Network) {
	log.Printf("docker:Networks.list %v: up", network.ID)

	networks[network.ID] = &network
}

func (networks Networks) update(event networkEvent) {
	state := event.State()

	if !state.Exists {
		log.Printf("docker:Networks.update %v -> %v: remove", event, state)

		delete(networks, event.ID)

	} else {
		log.Printf("docker:Networks.update %v -> %v: up", event, state)

		networks[event.ID] = event.Network
	}
}
