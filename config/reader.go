package config

import (
	"fmt"
	"log"
	"strings"
)

type ReaderOptions struct {
	SourceOptions
	SourceURLs		[]string	`long:"config-source" value-name:"(file|etcd|etcd+http|etcd+https)://[<host>]/<path>"`

	FilterRoutes	string		`long:"filter-routes" value-name:"URL-PREFIX" help:"Only apply routes from matching --config-source"`
}

// Return a new Reader with the given config URLs opened
func (options ReaderOptions) Reader() (*Reader, error) {
	reader := Reader{
		options:		options,
	}

	// Open all sources, and start running in preparation for Get or Listen()
	for _, urlString := range options.SourceURLs {
		if source, err := options.SourceOptions.openURL(urlString); err != nil {
			return nil, err
		} else if err := reader.open(source); err != nil {
			return nil, err
		} else {

		}
	}

	reader.start()

	return &reader, nil
}

// Read and combine a Config from multiple Sources
type Reader struct {
	options		ReaderOptions
	config		Config

	syncChan	chan Node
	listenChan	chan Config
}

func (reader *Reader) update(node Node) error {
	if reader.options.FilterRoutes != "" && strings.HasPrefix(node.Path, "routes/") && !strings.HasPrefix(node.Source.String(), reader.options.FilterRoutes) {
		log.Printf("Filter out route: %v", node.Path)
		return nil
	}

	return reader.config.update(node)
}

func (reader *Reader) open(source Source) error {
	if scanSource, ok := source.(scanSource); !ok {

	} else if nodes, err := scanSource.Scan(); err != nil {
		return err
	} else {
		for _, node := range nodes {
			if err := reader.update(node); err != nil {
				return err
			}
		}
	}

	if syncSource, ok := source.(syncSource); !ok {

	} else {
		// Only set sync chan if we have a source to sync from
		if reader.syncChan == nil {
			reader.syncChan = make(chan Node)
		}

		if err := syncSource.Sync(reader.syncChan); err != nil {
			return err
		}
	}

	return nil
}

// Start after initial open() sync of all sources to our config
func (reader *Reader) start() {
	if reader.listenChan != nil {
		panic(fmt.Errorf("Already running"))
	}

	reader.listenChan = make(chan Config)

	go reader.run()
}

func (reader *Reader) stop() {
	// XXX: not cool
	close(reader.syncChan)
}

// Get current config
func (reader *Reader) Get() Config {
	// XXX: unsafe
	return reader.config
}

// Follow config updates
// Closed if there are no sources to sync updates from, or on error.
// TODO: errors from chan close
func (reader *Reader) Listen() chan Config {
	return reader.listenChan
}

func (reader *Reader) run() {
	defer close(reader.listenChan)

	// output initial state
	reader.listenChan <- reader.config

	if reader.syncChan == nil {
		// did not open() any Sources to Sync() from, so no config updates to apply
		return
	}

	// apply sync updates
	for node := range reader.syncChan {
		if err := reader.update(node); err != nil {
			log.Printf("config:Reader.listener: Config.update %#v: %v\n", node, err)
			return
		}

		reader.listenChan <- reader.config
	}
}
