package config

import (
	"fmt"
	"log"
	"strings"
)

type ReaderOptions struct {
	SourceOptions
	SourceURLs []string `long:"config-source" value-name:"(file|etcd|etcd+http|etcd+https)://[<host>]/<path>" description:"Read and merge config from sources"`

	FilterRoutes string `long:"filter-routes" value-name:"URL-PREFIX" description:"Only apply routes from matching --config-source"`
}

// Return a new Reader with the given config URLs opened
func (options ReaderOptions) Reader() (*Reader, error) {
	var reader = Reader{
		options:	options,
	}

	if err := reader.init(); err != nil {
		return nil, err
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

	return &reader, nil
}

// Per-source state
type readerSource struct {
	options ReaderOptions
	source	Source
	config	Config
}

func (rs *readerSource) String() string {
	return rs.source.String()
}

func (rs *readerSource) update(node Node) error {
	if rs.options.FilterRoutes != "" && strings.HasPrefix(node.Path, "routes/") {
		if !strings.HasPrefix(rs.source.String(), rs.options.FilterRoutes) {
			log.Printf("config:readerSource %v: Filter out route: %v", rs, node)
			return nil
		}
	}

	if err := rs.config.update(node); err != nil {
		return fmt.Errorf("config.readerSource %v: update %v: %v", rs, node, err)
	}

	return nil
}

func (rs *readerSource) scan(scanSource scanSource) error {
	if nodes, err := scanSource.Scan(); err != nil {
		return err
	} else {
		for _, node := range nodes {
			if err := rs.update(node); err != nil {
				return err
			}
		}
	}

	return nil
}

func (rs *readerSource) open(syncChan chan Node) error {
	if scanSource, ok := rs.source.(scanSource); !ok {

	} else if err := rs.scan(scanSource); err != nil {
		return err
	}

	// sync to the shared syncChan
	// XXX: the source will close() this chan on errors, all other sources panic?!
	if syncSource, ok := rs.source.(syncSource); !ok {

	} else if err := syncSource.Sync(syncChan); err != nil {
		return err
	}

	return nil
}

// Read and merge Configs from multiple Sources
type Reader struct {
	options ReaderOptions
	sources	map[string]*readerSource

	syncChan chan Node
	listenChan chan Config
}

func (reader *Reader) init() error {
	reader.sources = make(map[string]*readerSource)
	reader.syncChan = make(chan Node)

	return nil
}

// Add new config Source during setup. Does initial scan() and setup sync() if any
//
// Must be called before start()
func (reader *Reader) open(source Source) error {
	var readerSource = &readerSource{
		options:	reader.options,
		source:		source,
	}

	if err := readerSource.open(reader.syncChan); err != nil {
		return err
	}

	reader.sources[readerSource.String()] = readerSource

	return nil
}

func (reader *Reader) start() {
	reader.listenChan = make(chan Config)

	go reader.run()
}

func (reader *Reader) stop() {
	// XXX: not cool
	close(reader.syncChan)
}

// Return Config from merged source Configs.
//
// Each merged Config is a complete copy of any per-source Configs, and safe against later modifications.
func (reader *Reader) get() Config {
	// start from empty config
	var config Config

	for _, rs := range reader.sources {
		config.merge(rs.config)
	}

	return config
}

func (reader *Reader) run() {
	defer close(reader.listenChan)

	// output initial state
	reader.listenChan <- reader.get()

	// apply sync updates
	for node := range reader.syncChan {
		// modify the source's Config in-place
		reader.sources[node.Source.String()].update(node)

		// send a merged copy, safe for concurrent reading by chan receivers
		reader.listenChan <- reader.get()
	}
}

// Get current config
func (reader *Reader) Get() Config {
	if reader.listenChan != nil {
		panic("Get() from Listening Reader")
	}

	return reader.get()
}

// Follow config updates
// Closed if there are no sources to sync updates from, or on error.
// TODO: errors from chan close
func (reader *Reader) Listen() chan Config {
	if reader.listenChan != nil {
		panic("Listen() from Listening Reader")
	}

	reader.start()

	return reader.listenChan
}
