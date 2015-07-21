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

type ServiceServer struct {
    IPv4    string  `json:"ipv4,omitempty"`
    IPv6    string  `json:"ipv6,omitempty"`
    TCP     uint16  `json:"tcp,omitempty"`
    UDP     uint16  `json:"udp,omitempty"`
}

type Service struct {
    Name        string

    Frontend    *ServiceFrontend
    Servers     map[string]ServiceServer
}

type EventType string

const (
    NewService     EventType   = "new-service"
    SetService     EventType   = "set-service"
    DelService     EventType   = "del-service"

    NewServer      EventType   = "new-server"
    SetServer      EventType   = "set-server"
    DelServer      EventType   = "del-server"
)

type Event struct {
    Type        EventType

    /*
     * The service in its updated state after the event.
     */
    Service     *Service

    /*
     * Existing frontend for {Set,Del}Service events.
     */
    PrevFrontend    *ServiceFrontend

    /*
     * Assoicated server details for {New,Set,Del}Server events.
     *
     * {New,Set,Del}Service events implicitly include all associated servers, they are not returned as separate events.
     */
    ServerName      string
    Server          ServiceServer
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
            return &Event{Service: self, Type: NewService, PrevFrontend: nil}
        }
    } else {
        if set == nil {
            return &Event{Service: self, Type: DelService, PrevFrontend: get}
        } else if *get != *set {
            return &Event{Service: self, Type: SetService, PrevFrontend: get}
        }
    }

    return nil
}

/*
 * One of the service's servers has been changed. The server will be given as nil if it not defined anymore.
 */
func (self *Service) syncServer(serverName string, action string, server *ServiceServer) *Event {
    get, isGet := self.Servers[serverName]
    set := server

    switch action {
    case "delete", "expire":
        set = nil

    case "create":
        isGet = false

    case "set":
    }


    log.Printf("server:Service %s: syncServer %s: %s %+v <- %+v\n", self.Name, serverName, action, set, get)

    // apply
    if set != nil {
        self.Servers[serverName] = *set
    } else {
        delete(self.Servers, serverName)
    }

    if !isGet {
        if set != nil {
            return &Event{Service: self, Type: NewServer, ServerName: serverName}
        }
    } else {
        if set == nil {
            return &Event{Service: self, Type: DelServer, ServerName: serverName, Server: get}
        } else if get != *set {
            return &Event{Service: self, Type: SetServer, ServerName: serverName, Server: get}
        }
    }

    return nil
}
