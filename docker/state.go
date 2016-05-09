package docker

type State struct {
    // Convenience info from docker
    Name		string
    Version		string

	Containers	Containers
	Networks	Networks
}

func (state *State) updateContainers(event containerEvent) {
	containers := state.Containers.clone()
	containers.update(event)

	state.Containers = containers
}

func (state *State) updateNetworks(event networkEvent) {
	networks := state.Networks.clone()
	networks.update(event)

	state.Networks = networks
}
