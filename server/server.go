package server

import (
    "log"
)

type ServiceFrontend struct {
    IPv4    string  `json:"ipv4,omitempty"`
    IPv6    string  `json:"ipv6,omitempty"`
    TCP     uint16  `json:"tcp,omitempty"`
    UDP     uint16  `json:"udp,omitempty"`
}

type ServiceBackend struct {
    IPv4    string  `json:"ipv4,omitempty"`
    IPv6    string  `json:"ipv6,omitempty"`
    TCP     uint16  `json:"tcp,omitempty"`
    UDP     uint16  `json:"udp,omitempty"`
}

type Service struct {
    Name        string

    Frontend    *ServiceFrontend
    Backends    map[string]*ServiceBackend
}

/*
 * Events when services change
 */
type EventType string

const (
    NewService     EventType   = "new-service"
    SetService     EventType   = "set-service"
    DelService     EventType   = "del-service"

    NewBackend     EventType   = "new-backend"
    SetBackend     EventType   = "set-backend"
    DelBackend     EventType   = "del-backend"
)

type Event struct {
    Type            EventType

    /*
     * The service in its updated state after the event.
     */
    Service         *Service

    /*
     * Existing frontend for {Set,Del}Service events.
     */
    Frontend        *ServiceFrontend

    /*
     * Assoicated server details for {New,Set,Del}Server events.
     *
     * {New,Set,Del}Service events implicitly include all associated servers, they are not returned as separate events.
     */
    BackendName     string
    Backend         *ServiceBackend
}

/*
 * Manage services state
 */
type Services struct {
    services    map[string]*Service
}

func newServices() *Services {
    return &Services{
        services:   make(map[string]*Service),
    }
}

func newService(name string) *Service {
    return &Service{
        Name:       name,
        Backends:   make(map[string]*ServiceBackend),
    }
}

func (self *Services) get(name string) *Service {
    service, serviceExists := self.services[name]

    if !serviceExists {
        service = newService(name)
        self.services[name] = service
    }

    return service
}

func (self *Services) add(service *Service) {
    self.services[service.Name] = service
}

/* Get all currently valid Services */
func (self *Services) Services() []*Service {
    services := make([]*Service, 0, len(self.services))

    for _, service := range self.services {
        if service.Frontend == nil {
            continue
        }

        services = append(services, service)
    }

    return services
}

/*
 * The service as a whole has been changed (e.g. removed).
 */
func (self *Services) syncService(service *Service, action string) *Event {
    log.Printf("server:Services.syncService %s: sync %s\n", service.Name, action)

    switch action {
    case "delete", "expire":
        delete(self.services, service.Name)

        if service.Frontend != nil {
            return &Event{Service: service, Type: DelService, Frontend: service.Frontend}
        }
    }

    return nil
}

/*
 * The service's frontend has been changed. The frontend will be given as nil if it is not defined anymore.
 */
func (self *Service) syncFrontend(action string, frontend *ServiceFrontend) *Event {
    get := self.Frontend
    set := frontend

    switch action {
    case "delete", "expire":
        set = nil

    case "create":
        get = nil

    case "set":
    }

    log.Printf("server:Service %s: syncFrontend: %s %+v <- %+v\n", self.Name, action, set, get)

    // apply
    self.Frontend = set

    if get == nil {
        if set != nil {
            return &Event{Service: self, Type: NewService}
        }
    } else {
        if set == nil {
            return &Event{Service: self, Type: DelService, Frontend: get}
        } else if *get != *set {
            return &Event{Service: self, Type: SetService, Frontend: get}
        }
    }

    return nil
}

/*
 * One of the service's servers has been changed. The server will be given as nil if it not defined anymore.
 */
func (self *Service) syncBackend(name string, action string, backend *ServiceBackend) *Event {
    get := self.Backends[name]
    set := backend

    switch action {
    case "delete", "expire":
        set = nil

    case "create":
        get = nil

    case "set":
    }


    log.Printf("server:Service %s: syncBackend %s: %s %+v <- %+v\n", self.Name, name, action, set, get)

    // apply
    if set != nil {
        self.Backends[name] = set
    } else {
        delete(self.Backends, name)
    }

    if get == nil {
        if set != nil {
            return &Event{Service: self, Type: NewBackend, BackendName: name}
        }
    } else {
        if set == nil {
            return &Event{Service: self, Type: DelBackend, BackendName: name, Backend: get}
        } else if *get != *set {
            return &Event{Service: self, Type: SetBackend, BackendName: name, Backend: get}
        }
    }

    return nil
}
