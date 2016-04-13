package config

import (
    "encoding/json"
    "strings"
)

// Low-level config model, used to load files from local and etcd
type Node struct {
    /* Identity */
    Source  ConfigSource

    // clusterf-relative path, so with any prefix stripped
    Path    string

    /* Type/Value */
    IsDir   bool

    // json-encoded; empty if removed
    Value   string
    Remove  bool

}

func (self *Node) unmarshal(object interface{}) error {
    if self.Value == "" {
        return nil
    }

    return json.Unmarshal([]byte(self.Value), object)
}

func makePath(pathParts ...string) string {
    return strings.Join(pathParts, "/")
}

func makeDirNode(path string) (Node, error) {
    return Node{Path: path, IsDir: true}, nil
}

func makeNode(path string, value interface{}) (Node, error) {
    jsonValue, err := json.Marshal(value)

    return Node{Path: path, Value: string(jsonValue)}, err
}
