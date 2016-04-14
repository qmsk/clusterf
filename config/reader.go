package config

import (
	"fmt"
	"log"
	"net/url"
)

type ReaderOptions struct {
	SourceURL	[]string	`long:"config-source" value-name:"(file|etcd|etcd+http|etcd+https)://[<host>]/<path>"`
}

// Return a new Reader with the given config URLs opened
func (options ReaderOptions) Reader() (*Reader, error) {
	reader := Reader{

	}

	if err := reader.Open(options.SourceURL...); err != nil {
		return nil, err
	}

	return &reader, nil
}

// Read and combine a Config from multiple Sources
type Reader struct {
	config		Config

	syncChan	chan Node
	listenChan	chan Config
}

type scanSource interface {
	Source

	Scan() ([]Node, error)
}

type syncSource interface {
	Sync(chan Node) error
}

func (reader *Reader) open(source Source) error {
	if scanSource, ok := source.(scanSource); !ok {

	} else if nodes, err := scanSource.Scan(); err != nil {
		return err
	} else {
		for _, node := range nodes {
			if err := reader.config.update(node); err != nil {
				return err
			}
		}
	}

	if syncSource, ok := source.(syncSource); !ok {

	} else {
		if reader.syncChan == nil {
			reader.syncChan = make(chan Node)
		}

		if err := syncSource.Sync(reader.syncChan); err != nil {
			return err
		}
	}

	return nil
}

func (reader *Reader) openURL(url *url.URL) error {
	switch url.Scheme {
	case "etcd", "etcd+http", "etcd+https":
		if source, err := openEtcdSource(url); err != nil {
			return err
		} else {
			return reader.open(source)
		}

	case "file":
		if source, err := openFileSource(url); err != nil {
			return err
		} else {
			return reader.open(source)
		}

	default:
		return fmt.Errorf("Invalid config URL Scheme=%v: %v\n", url.Scheme, url)
	}
}

func (reader *Reader) Open(urls ...string) error {
	for _, urlString := range urls {
		if url, err := url.Parse(urlString); err != nil {
			return err
		} else if err := reader.openURL(url); err != nil {
			return err
		}
	}

	return nil
}

// Get current config
func (reader *Reader) Get() Config {
	return reader.config
}

// Follow config updates
func (reader *Reader) Listen() chan Config {
	if reader.listenChan == nil {
		reader.listenChan = make(chan Config)

		go reader.listen()
	}

	return reader.listenChan
}

func (reader *Reader) listen() {
	defer close(reader.listenChan)

	// output initial state
	reader.listenChan <- reader.config

	if reader.syncChan == nil {
		return
	}

	// apply sync updates
	for node := range reader.syncChan {
		if err := reader.config.update(node); err != nil {
			log.Printf("config:Reader.listener: Config.update %#v: %v\n", node, err)
			return
		}

		reader.listenChan <- reader.config
	}
}
