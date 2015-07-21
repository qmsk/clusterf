package server

import (
    "log"
    "net"
)

type ServiceFrontend struct {
    IPv4    net.IP  `json:"ipv4,omitempty"`
    TCP     uint16  `json:"tcp,omitempty"`
}

type ServiceServer struct {
    IPv4    net.IP  `json:"ipv4,omitempty"`
    TCP     uint16  `json:"tcp,omitempty"`
}

type Service struct {
    Name        string

    Frontend    *ServiceFrontend
    Servers     map[string]ServiceServer
}

type EventType string

const (
    New     EventType   = "new"
    Set     EventType   = "set"
    Del     EventType   = "del"
)

type Event struct {
    Service     *Service
    Type        EventType

    Frontend    *ServiceFrontend
    Server      *ServiceServer
}

/*
 * The service as a whole has been changed (e.g. removed).
 */
func (self *Service) sync(action string) *Event {
    log.Printf("server:Service %s: sync %s\n", self.Name, action)

    switch action {
    case "delete", "expire":
        return &Event{Service: self, Type: Del}
    }

    return nil
}

/*
 * The service's frontend has been changed. The frontend will be given as nil if it is not defined anymore.
 */
func (self *Service) syncFrontend(action string, frontend *ServiceFrontend) *Event {
    log.Printf("server:Service %s: syncFrontend %s: %+v\n", self.Name, action, frontend)

    return nil
}

/*
 * One of the service's servers has been changed. The server will be given as nil if it not defined anymore.
 */
func (self *Service) syncServer(serverName string, action string, server *ServiceServer) *Event {
    log.Printf("server:Service %s: syncServer %s %s: %+v\n", self.Name, serverName, action, server)

    return nil
}
