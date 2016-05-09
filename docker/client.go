package docker

import (
    "fmt"
    "github.com/fsouza/go-dockerclient"
    "log"
)

type Options struct {
	Endpoint	string	`long:"docker-endpoint"`
}

type Client struct {
	options			Options
    dockerClient	*docker.Client
}

func (options Options) Open() (*Client, error) {
	var client Client

    if err := client.open(options); err != nil {
        return nil, err
    }

	return &client, nil
}

func (client *Client) open(options Options) error {
    var dockerClient *docker.Client
    var err error

    if options.Endpoint != "" {
        dockerClient, err = docker.NewClient(options.Endpoint)
    } else {
        dockerClient, err = docker.NewClientFromEnv()
    }

    if err != nil {
        return err
    } else {
		client.options = options
        client.dockerClient = dockerClient
    }

    return nil
}

func (client *Client) String() string {
	return fmt.Sprintf("docker.Client %v", client.dockerClient.Endpoint())
}

// Return Container, or nil if not exists
func (client *Client) getContainer(id string) (*docker.Container, error) {
	if container, err := client.dockerClient.InspectContainer(id); err == nil {
		return container, nil
	} else if _, ok := err.(*docker.NoSuchContainer); ok {
		return nil, nil
	} else {
		return nil, err
	}
}

func (client *Client) getContainers() (Containers, error) {
	var containers = make(Containers)
	var opts = docker.ListContainersOptions{
		All: true,
	}

    listContainers, err := client.dockerClient.ListContainers(opts)
	if err != nil {
		return nil, fmt.Errorf("docker:Client.ListContainers: %v", err)
    }

	for _, listContainer := range listContainers {
		if container, err := client.getContainer(listContainer.ID); err != nil {
			return containers, err
		} else if container != nil {
			containers.list(container)
		} else {
			// disappeared while listing
		}
	}

	return containers, nil
}

func (client *Client) getNetwork(id string) (*docker.Network, error) {
	if network, err := client.dockerClient.NetworkInfo(id); err != nil {
		return network, nil
	} else if _, ok := err.(*docker.NoSuchNetwork); ok {
		return nil, nil
	} else {
		return nil, err
	}
}

func (client *Client) getNetworks() (Networks, error) {
	var networks = make(Networks)

	listNetworks, err := client.dockerClient.ListNetworks()
	if err != nil {
		return nil, fmt.Errorf("docker:Client.ListNetworks: %v", err)
	}

	for _, network := range listNetworks {
		networks.list(network)
	}

	return networks, nil
}

func (client *Client) getState() (State, error) {
	var state State

    // Version
    if env, err := client.dockerClient.Version(); err != nil {
        return state, err
    } else {
        state.Version = env.Get("Version")
    }

    // Info
    if dockerInfo, err := client.dockerClient.Info(); err != nil {
        return state, err
    } else {
        state.Name = dockerInfo.Name
    }

	if containers, err := client.getContainers(); err != nil {
		return state, err
	} else {
		state.Containers = containers
	}

	if networks, err := client.getNetworks(); err != nil {
		return state, err
	} else {
		state.Networks = networks
	}

	return state, nil
}

func (client *Client) updateState(state *State, dockerEvent *docker.APIEvents) error {
	switch dockerEvent.Type {
	case "container":
		var containerEvent = containerEvent{
			ID:			dockerEvent.Actor.ID,
			Action:		dockerEvent.Action,
		}

		if container, err := client.getContainer(dockerEvent.Actor.ID); err != nil {
			return err
		} else if container != nil {
			containerEvent.Container = container
		} else {
			// disappeared
		}

		state.updateContainers(containerEvent)

		return nil

	case "network":
		var networkEvent = networkEvent{
			ID:			dockerEvent.Actor.ID,
			Action:		dockerEvent.Action,
		}

		if network, err := client.getNetwork(dockerEvent.Actor.ID); err != nil {
			return err
		} else if network != nil {
			networkEvent.Network = network
		} else {
			// gone
		}

		state.updateNetworks(networkEvent)

		return nil

	default:
		log.Printf("docker:Client.updateState: %s %s %s: skip", dockerEvent.Action, dockerEvent.Type, dockerEvent.ID)

		return nil
	}
}

// update State from events
func (client *Client) listen(state State, listener chan *docker.APIEvents, stateChan chan State) {
	defer client.dockerClient.RemoveEventListener(listener)
	defer close(stateChan)

	stateChan <- state

    for dockerEvent := range listener {
		if err := client.updateState(&state, dockerEvent); err != nil {
			log.Printf("%v: listen updateState %v: %v", client, dockerEvent, err)
			return
		} else {
			stateChan <- state
		}
    }
}

// Return State state
func (client *Client) Get() (State, error) {
	return client.getState()
}

// Return chan with State state and updates
func (client *Client) Listen() (chan State, error) {
    listener := make(chan *docker.APIEvents)

    if err := client.dockerClient.AddEventListener(listener); err != nil {
		return nil, fmt.Errorf("watch: AddEventListener: %v", err)
    }

	state, err := client.getState()
	if err != nil {
		return nil, err
	}

	// update state from events
	listenChan := make(chan State)

	go client.listen(state, listener, listenChan)

	return listenChan, nil
}
