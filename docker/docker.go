package docker

import (
    "fmt"
    "github.com/fsouza/go-dockerclient"
    "log"
    "net"
    "path"
)

type DockerConfig struct {
    Endpoint string
}

type Docker struct {
    config DockerConfig
    client *docker.Client

    // convenience info from docker
    Version string

    // XXX: not supported on docker 1.3.3
    Name string
}

type ContainerState struct {
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
}

type ContainerEvent struct {
    ID          string
    Event       string

    // Interpretation of running state after this event
    Running     bool

    // Current state of container; may be inconsistent or missing
    State       *ContainerState
}

func (self DockerConfig) Open() (*Docker, error) {
    d := &Docker{config: self}

    if err := d.open(); err != nil {
        return nil, err
    } else {
        return d, err
    }
}

func (self *Docker) open() error {
    if dockerClient, err := docker.NewClient(self.config.Endpoint); err != nil {
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
    return fmt.Sprintf("Docker<%v>", self.config)
}

/*
 * Return the state of the given container, based on the given event.
 *
 *  event       - /event status, or "" when listing
 */
func (self *Docker) inspectContainerState(id string) (*ContainerState, error) {
    container, err := self.client.InspectContainer(id)
    if err != nil {
        log.Printf("%v.inspectContainerState(%v): %v\n", self, id, err)
        return nil, err
    }

    return &ContainerState{
        ID:         id,
        Name:       path.Base(container.Name),
        Running:    container.State.Running,
        IPv4:       net.ParseIP(container.NetworkSettings.IPAddress),
        Hostname:   container.Config.Hostname,
        Image:      path.Base(container.Config.Image),
    }, nil
}

/*
 * Full list of (running) containers.
 *
 * TODO: somehow synchronize this with Subscribe() events to ensure consistency during listings?
 */
func (self *Docker) List() ([]ContainerState, error) {
    containers, err := self.client.ListContainers(docker.ListContainersOptions{All: true})
    if err != nil {
        log.Printf("%v.ListContainers: %v\n", self, err)
        return nil, err
    }

    var out []ContainerState

    for _, listContainer := range containers {
        if containerState, err := self.inspectContainerState(listContainer.ID); err != nil {
            break
        } else {
            out = append(out, *containerState)
        }
    }

    return out, nil
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

    go func() {
        defer close(out)

        for dockerEvent := range listener {
            if dockerEvent == docker.EOFEvent {
                // XXX: how is this different to close()'ing the chan?
                log.Printf("%v.Subscribe: EOF\n", self)
                break
            }

            event := ContainerEvent{ID: dockerEvent.ID, Event: dockerEvent.Status}

            if dockerEvent.Status == "destroy" {
                // skip lookup for cases where we don't have the container state anymore

            } else if containerState, err := self.inspectContainerState(dockerEvent.ID); err != nil {
                break

            } else {
                event.State = containerState
                event.Running = containerState.Running
            }

            if dockerEvent.Status == "die" {
                // XXX: docker seems to be inconsistent about the inspected container State.Running=true/false immediately after a die?
                event.Running = false
            }

            log.Printf("%v.Subscribe: %v %v: %#v\n", self, event.Event, event.ID, event.State)

            out <- event
        }
    }()

    return out, nil
}
