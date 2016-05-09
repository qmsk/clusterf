package config

import (
	"fmt"
	"net/url"
)

type SourceOptions struct {
	Etcd EtcdOptions `group:"Config etcd://"`
}

// A single Config may contain Nodes from different Sources
type Source interface {
	// uniquely identifying
	String() string
}

func (options SourceOptions) openURL(sourceURL string) (Source, error) {
	url, err := url.Parse(sourceURL)
	if err != nil {
		return nil, err
	}

	switch url.Scheme {
	case "etcd", "etcd+http", "etcd+https":
		return options.Etcd.OpenURL(url)

	case "file":
		return openFileSource(url)

	default:
		return nil, fmt.Errorf("Invalid config URL Scheme=%v: %v\n", url.Scheme, url)
	}
}

type scanSource interface {
	Source

	Scan() ([]Node, error)
}

type syncSource interface {
	Source

	Sync(chan Node) error
}

type writeSource interface {
	Source

	// Update the set of written nodes, adding any new nodes, setting any changed nodes, and removing any missing nodes
	//
	// If source has a TTL mechanism, also refreshes the nodes.
	//
	// The map must be safe for reading from a different goroutine until the next Write() completes
	Write(nodes map[string]Node) error

	// Remove any written nodes
	Flush() error
}
