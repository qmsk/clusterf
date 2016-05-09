package docker

type State struct {
    // Convenience info from docker
    Name		string
    Version		string

	// Running containers
	Containers	Containers
}

func (state *State) updateContainers(event containerEvent) {
	containers := state.Containers.clone()
	containers.update(event)

	state.Containers = containers
}
