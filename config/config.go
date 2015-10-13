package config

import (
    "encoding/json"
    "strings"
)

func makeDirNode(pathParts...string) (Node, error) {
    path := strings.Join(pathParts, "/")

    return Node{Path: path, IsDir: true}, nil
}

func makeNode(jsonValue interface{}, pathParts...string) (Node, error) {
    path := strings.Join(pathParts, "/")
    value, err := json.Marshal(jsonValue)

    return Node{Path: path, Value: string(value)}, err
}

func (self ConfigService) publish() (node Node, err error) {
    return makeDirNode("services", self.ServiceName)
}

func (self ConfigServiceFrontend) publish() (node Node, err error) {
    return makeNode(self.Frontend, "services", self.ServiceName, "frontend")
}

func (self ConfigServiceBackend) publish() (node Node, err error) {
    return makeNode(self.Backend, "services", self.ServiceName, "backends", self.BackendName)
}

func (self ConfigRoute) publish() (node Node, err error) {
    return makeNode(self.Route, "routes", self.RouteName)
}

