package clusterf
/*
 * Internal service state, maintained from config changes.
 */

import (
    "qmsk.net/clusterf/config"
    "log"
)

type Service struct {
    Name        string

    Frontend    *config.ServiceFrontend
    Backends    map[string]config.ServiceBackend

    driverFrontend  *ipvsFrontend
    driverBackends  map[string]*ipvsBackend
}

func newService(name string, driver *IPVSDriver) *Service {
    return &Service{
        Name:           name,
        Backends:       make(map[string]config.ServiceBackend),

        driverFrontend: driver.newFrontend(),
        driverBackends: make(map[string]*ipvsBackend),
    }
}

func (self *Service) driverError(err error) {
    log.Printf("cluster:Service %s: Error: %s\n", self.Name, err)
}

/* Configuration actions */
func (self *Service) configFrontend(action config.Action, frontendConfig *config.ConfigServiceFrontend) {
    frontend := frontendConfig.Frontend

    log.Printf("clusterf:Service %s: Frontend: %s %+v <- %+v\n", self.Name, action, frontend, self.Frontend)

    switch action {
    case config.NewConfig:
        self.newFrontend(frontend)

        self.Frontend = &frontend // XXX: copy?

    case config.SetConfig:
        if *self.Frontend != frontend {
            self.setFrontend(frontend)
        }

        self.Frontend = &frontend // XXX: copy?

    case config.DelConfig:
        self.delFrontend()

        self.Frontend = nil
    }
}

func (self *Service) configBackend(backendName string, action config.Action, backendConfig *config.ConfigServiceBackend) {
    log.Printf("clusterf:Service %s: Backend %s: %s %+v <- %+v\n", self.Name, backendName, action, backendConfig.Backend, self.Backends[backendName])

    switch action {
    case config.NewConfig:
        self.newBackend(backendName, backendConfig.Backend)

        self.Backends[backendName] = backendConfig.Backend

    case config.SetConfig:
        if self.Backends[backendName] != backendConfig.Backend {
            self.setBackend(backendName, backendConfig.Backend)

            self.Backends[backendName] = backendConfig.Backend
        }

    case config.DelConfig:
        self.delBackend(backendName)

        delete(self.Backends, backendName)
    }
}

/* Frontend actions */
func (self *Service) newFrontend(frontend config.ServiceFrontend) {
    log.Printf("clusterf:Service %s: new Frontend: %+v\n", self.Name, frontend)

    if err := self.driverFrontend.add(frontend); err != nil {
        self.driverError(err)
    } else {
        for backendName, backend := range self.Backends {
            self.newBackend(backendName, backend)
        }
    }
}

func (self *Service) setFrontend(frontend config.ServiceFrontend) {
    log.Printf("clusterf:Service %s: set Frontend: %+v\n", self.Name, frontend)

    // TODO: something more smooth...
    self.delFrontend()
    self.newFrontend(frontend)
}

func (self *Service) delFrontend() {
    log.Printf("clusterf:Service %s: del Frontend: %+v\n", self.Name, self.Frontend)

    // del'ing the frontend will also remove all backend state
    if err := self.driverFrontend.del(); err != nil {
        self.driverError(err)
    }
}

/* Backend actions */
func (self *Service) newBackend(backendName string, backend config.ServiceBackend) {
    log.Printf("clusterf:Service %s: new Backend %s: %+v\n", self.Name, backendName, backend)

    self.driverBackends[backendName] = self.driverFrontend.newBackend()

    if err := self.driverBackends[backendName].add(backend); err != nil {
        self.driverError(err)
    }
}

func (self *Service) setBackend(backendName string, backend config.ServiceBackend) {
    log.Printf("clusterf:Service %s: set Backend %s: %+v\n", self.Name, backendName, backend)

    if err := self.driverBackends[backendName].set(backend); err != nil {
        self.driverError(err)
    }
}

func (self *Service) delBackend(backendName string) {
    log.Printf("clusterf:Service %s: del Backend %s: %+v\n", self.Name, backendName, self.Backends[backendName])

    if err := self.driverBackends[backendName].del(); err != nil {
        self.driverError(err)
    }

    delete(self.driverBackends, backendName)
}
