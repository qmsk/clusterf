package docker

import (
    "fmt"
    "github.com/fsouza/go-dockerclient"
    "log"
    "net"
    "path"
    "strconv"
)

type Options struct {
	Endpoint string	`long:"docker-endpoint"`
}

type Docker struct {
	options	Options
    client *docker.Client

    // convenience info from docker
    Version string

    // XXX: not supported on docker 1.3.3
    Name string
}

type Container struct {
    // local unique ID for continer
    ID          string

    // optional human-readble name for container, or ID
    Name        string

    // Current running state
    Running     bool

    // internal IPv4 address assigned to container
    IPv4        net.IP

    // internal hostname for container, or short ID
    Hostname    string

    // basename of image used to run container
    Image       string

    // exposed, published ports
    Ports       []Port

    // XXX: configured image, run labels?
    Labels      map[string]string
}

func (self Container) String() string {
    return self.ID
}

type Port struct {
    Proto       string
    Port        uint16

    // exposed
    HostIP      string
    HostPort    uint16
}

type ContainerEvent struct {
    ID          string
    Status      string

    // Interpretation of State.Running *after* this event; depends on Status
    Running     bool

    // Current state of container; may be inconsistent or missing
    State       *Container
}

func (self ContainerEvent) String() string {
    return fmt.Sprintf("%s:%s", self.Status, self.ID)
}

func (options Options) Open() (*Docker, error) {
	d := &Docker{options: options}

    if err := d.open(); err != nil {
        return nil, err
    } else {
        return d, err
    }
}

func (self *Docker) open() error {
    var dockerClient *docker.Client
    var err error

    if self.options.Endpoint != "" {
        dockerClient, err = docker.NewClient(self.options.Endpoint)
    } else {
        dockerClient, err = docker.NewClientFromEnv()
    }

    if err != nil {
        return err
    } else {
        self.client = dockerClient
    }

    // Version
    if env, err := self.client.Version(); err != nil {
        return err
    } else {
        self.Version = env.Get("Version")
    }

    // Info
    if env, err := self.client.Info(); err != nil {
        return err
    } else {
        self.Name = env.Get("Name")
    }

    return nil
}

func (self *Docker) String() string {
    return fmt.Sprintf("Docker<%v>", self.options)
}

func parsePort(portString string) (uint16, error) {
    if portValue, err := strconv.Atoi(portString); err != nil {
        return 0, fmt.Errorf("port invalid: %v", portString)
    } else if portValue <= 0 || portValue >= (1 << 16) {
        return 0, fmt.Errorf("port out of range: %v", portString)
    } else {
        return uint16(portValue), nil
    }
}

/*
 * Return the state of the given container, based on the given event.
 *
 *  event       - /event status, or "" when listing
 */
func (self *Docker) inspectContainer(id string) (*Container, error) {
    dockerContainer, err := self.client.InspectContainer(id)
    if err != nil {
        log.Printf("%v.inspectContainer(%v): %v\n", self, id, err)
        return nil, err
    }

    state := Container{
        ID:         id,
        Name:       path.Base(dockerContainer.Name),
        Running:    dockerContainer.State.Running,
        IPv4:       net.ParseIP(dockerContainer.NetworkSettings.IPAddress),
        Hostname:   dockerContainer.Config.Hostname,
        Image:      path.Base(dockerContainer.Config.Image),
        Labels:     dockerContainer.Config.Labels,
    }

    for dockerPort, portBindings := range dockerContainer.NetworkSettings.Ports {
        port := Port{
            Proto:  dockerPort.Proto(),
        }

        if portValue, err := parsePort(dockerPort.Port()); err != nil {
            return nil, err
        } else {
            port.Port = portValue
        }

        for _, portBinding := range portBindings {
            // XXX: choose one
            port.HostIP = portBinding.HostIP

            if hostPort, err := parsePort(portBinding.HostPort); err != nil {
                return nil, err
            } else {
                port.HostPort = hostPort
            }
        }

        state.Ports = append(state.Ports, port)
    }

    return &state, nil
}

/*
 * Full list of (running) containers.
 *
 * TODO: somehow synchronize this with Subscribe() events to ensure consistency during listings?
 */
func (self *Docker) List() (out []Container, err error) {
    containers, err := self.client.ListContainers(docker.ListContainersOptions{All: true})
    if err != nil {
        log.Printf("%v.ListContainers: %v\n", self, err)
        return nil, err
    }

    for _, listContainer := range containers {
        if containerState, err := self.inspectContainer(listContainer.ID); err != nil {
            break
        } else {
            out = append(out, *containerState)
        }
    }

    return out, nil
}

// Handle a container event
func (self *Docker) containerEvent(dockerEvent *docker.APIEvents) (event ContainerEvent, err error) {
    event.ID = dockerEvent.ID
    event.Status = dockerEvent.Status

    if containerState, err := self.inspectContainer(dockerEvent.ID); err != nil {
        // skip lookup for cases where we don't have the container state anymore
        // this is normal for "destroy", but other events could also race
        event.State = nil

        // XXX: Running is indeterminite, but we can assume it is not?

    } else {
        event.State = containerState
        event.Running = containerState.Running
    }

    switch dockerEvent.Status {
        case "start":
            event.Running = true

        case "die", "kill", "stop":
            // State.Running may still be true, while the container is stopping
            event.Running = false
    }

    return
}

func (self *Docker) dockerEvent(dockerEvent *docker.APIEvents, out *ContainerEvent) error {
	switch dockerEvent.Status {
	case "EOF":
		// XXX: how is this different to close()'ing the chan?
		log.Printf("%v.Subscribe: EOF\n", self)
		break

	// container events
	case "attach", "commit", "copy", "create", "destroy", "die", "exec_create", "exec_start", "export", "kill", "oom", "pause", "rename", "resize", "restart", "start", "stop", "top", "unpause":
		if containerEvent, err := self.containerEvent(dockerEvent); err != nil {
			log.Printf("%v.Subscribe %v:%v: containerEvent: %v\n", self, dockerEvent.Status, dockerEvent.ID, err)

		} else {
			// log.Printf("%v.Subscribe %v:%v: %#v\n", self, dockerEvent.Status, dockerEvent.ID, containerEvent)

			*out = containerEvent
		}

	// image events
	case "delete", "import", "pull", "push", "tag", "untag":
		log.Printf("%v.Subscribe %v:%v: image event: ignore\n", self, dockerEvent.Status, dockerEvent.ID)

	default:
		log.Printf("%v.Subscribe %v:%v: unknown event: ignore\n", self, dockerEvent.Status, dockerEvent.ID)
	}

	return nil
}

/*
 * Subscribe to container events.
 */
func (self *Docker) Subscribe() (chan ContainerEvent, error) {
    listener := make(chan *docker.APIEvents)
    out := make(chan ContainerEvent)

    if err := self.client.AddEventListener(listener); err != nil {
        log.Printf("%v.Subscribe: AddEventListener\n", self, err)
        return nil, err
    }

    go self.subscribe(listener, out)

    return out, nil
}

func (self *Docker) subscribe(listener chan *docker.APIEvents, out chan ContainerEvent) {
    defer close(out)

    for dockerEvent := range listener {
		var containerEvent ContainerEvent

		if err := self.dockerEvent(dockerEvent, &containerEvent); err != nil {
			return
		} else {
			out <- containerEvent
		}
    }
}

/*
 * Combine List() and Subscribe() to synchronize docker container states.
 *
 * Sends updated Containers maps on every container event.
 */
func (self *Docker) Watch() (chan Containers, error) {
	containers := make(Containers)
    out := make(chan Containers)

    // initial state
    containerList, err := self.List()
    if err != nil {
        return nil, err
    }

	for _, container := range containerList {
		containers[container.ID] = container
	}

    // subscribe events
    listener := make(chan *docker.APIEvents)

    if err := self.client.AddEventListener(listener); err != nil {
        return nil, fmt.Errorf("docker:Client.AddEventListener: %v", err)
    }

    go self.watch(containers, listener, out)

    return out, nil
}

func (self *Docker) watch(containers Containers, listener chan *docker.APIEvents, out chan Containers) {
	defer close(out)

	out <- containers

    for dockerEvent := range listener {
		var containerEvent ContainerEvent

		if err := self.dockerEvent(dockerEvent, &containerEvent); err != nil {
			return
		} else {
			containers = containers.clone()
			containers.update(containerEvent)

			out <- containers
		}
	}
}
