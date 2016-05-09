package config

import (
	"encoding/json"
	"strings"
)

// Low-level config model, used to load files from local and etcd
type Node struct {
	/* Identity */
	Source Source

	// clusterf-relative path, so with any prefix and leading / stripped
	Path string

	/* Type/Value */
	IsDir bool

	// json-encoded; empty if removed
	Value  string
	Remove bool
}

// XXX: include source?
func (node Node) String() string {
	return node.Path
}

func (node Node) Matches(other Node) bool {
	if node.Path != other.Path {
		return false
	}

	return true
}

func (node Node) Equals(other Node) bool {
	if !node.Matches(other) {
		return false
	}

	if node.IsDir != other.IsDir {
		return false
	}

	if node.Value != other.Value {
		return false
	}

	return true
}

func (node Node) unmarshal(object interface{}) error {
	if node.Value == "" {
		return nil
	}

	return json.Unmarshal([]byte(node.Value), object)
}

func makePath(path ...string) string {
	return strings.Join(path, "/")
}

func makeDirNode(path ...string) Node {
	return Node{Path: makePath(path...), IsDir: true}
}

// Return Node from path and value
//
// Panics if json marshal fails
func makeNode(value interface{}, path ...string) Node {
	if jsonValue, err := json.Marshal(value); err != nil {
		panic(err)
	} else {
		return Node{Path: makePath(path...), Value: string(jsonValue)}
	}
}
