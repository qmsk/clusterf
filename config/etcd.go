package config

import (
	"github.com/coreos/etcd/client"
	"golang.org/x/net/context"
    "fmt"
    "log"
    "strings"
	"time"
	"net/url"
)

type EtcdOptions struct {
	Scheme		string			`long:"etcd-scheme" value-name:"http|https" default:"http"`
	Hosts		[]string		`long:"etcd-host" value-name:"HOST:PORT"`
	Prefix      string			`long:"etcd-prefix" value-name:"/PATH" default:"/clusterf"`
	TTL			time.Duration	`long:"etcd-ttl" default:"10s"`
}

func (options EtcdOptions) OpenURL(url *url.URL) (*EtcdSource, error) {
	switch url.Scheme {
	case "etcd":

	case "etcd+http":
		options.Scheme = "http"
	case "etcd+https":
		options.Scheme = "https"
	}

	for _, host := range strings.Split(url.Host, ",") {
		options.Hosts = append(options.Hosts, host)
	}

	if url.Path != "" {
		options.Prefix = url.Path
	}

	return options.Open()
}


func (options EtcdOptions) String() string {
	return fmt.Sprintf("etcd+%s://%s%s", options.Scheme, strings.Join(options.Hosts, ","), options.Prefix)
}

func (options EtcdOptions) clientConfig() (clientConfig client.Config, err error) {
	for _, host := range options.Hosts {
		endpointURL := url.URL{Scheme: options.Scheme, Host: host}

		clientConfig.Endpoints = append(clientConfig.Endpoints, endpointURL.String())
	}

	return
}

func (options EtcdOptions) Open() (*EtcdSource, error) {
    etcdSource := EtcdSource{
		options:	options,
	}

	if clientConfig, err := options.clientConfig(); err != nil {
		return nil, err
	} else if client, err := client.New(clientConfig); err != nil {
		return nil, err
	} else {
		etcdSource.client = client
	}

	etcdSource.keysAPI = client.NewKeysAPI(etcdSource.client)

    return &etcdSource, nil
}

// Undo etcd/client:ClusterError fuckery
// https://github.com/coreos/etcd/pull/4503
func fixupClusterError(err error) error {
	if clusterError, ok := err.(*client.ClusterError); ok {
		var errs []string

		for _, clusterErr := range clusterError.Errors {
			errs = append(errs, clusterErr.Error())
		}

		return fmt.Errorf("%s: %s", clusterError.Error(), strings.Join(errs, "; "))
	} else {
		return err
	}
}

type EtcdSource struct {
    options		EtcdOptions

    client      client.Client
	keysAPI		client.KeysAPI

	// state to track changes from Scan() to Sync()
    syncIndex   uint64

	// refresh nodes
	writeChan	chan map[string]Node
	flushChan	chan error
}

func (etcd *EtcdSource) String() string {
	return etcd.options.String()
}

func (etcd *EtcdSource) path(parts ...string) string {
    return strings.Join(append([]string{etcd.options.Prefix}, parts...), "/")
}

/*
 * Initialize state in etcd.
 *
 * Creates the top-level config directory if it does not exist, and initialize to follow it
 */
func (etcd *EtcdSource) Init() error {
	if response, err := etcd.keysAPI.Set(context.Background(), etcd.path(), "", &client.SetOptions{Dir: true}); err != nil {
        return fixupClusterError(err)
    } else {
        etcd.syncIndex = response.Node.CreatedIndex
    }

    return nil
}

/*
 * Synchronize current state in etcd.
 *
 * Does a recursive get on the complete /clusterf tree in etcd, and builds the services state from it.
 *
 * Stores the current etcd-index from the snapshot in .syncIndex, so that .Sync() can be used to continue updating any changes.
 */
func (etcd *EtcdSource) Scan() ([]Node, error) {
	response, err := etcd.keysAPI.Get(context.Background(), etcd.path(), &client.GetOptions{Recursive: true})

    if err == nil {

	} else if clientError, ok := err.(client.Error); ok && clientError.Code == client.ErrorCodeKeyNotFound {
		// create directory instead
		return nil, etcd.Init()
	} else {
        return nil, fixupClusterError(err)
    }

    if response.Node.Dir != true {
        return nil, fmt.Errorf("etcd prefix=%s is not a directory", response.Node.Key)
    }

    // the tree root's ModifiedTime may be a long long time in the past, so we can't want to use that for waits
    // we assume this enough to ensure atomic sync with .Watch() on the same tree..
    etcd.syncIndex = response.Index

    // scan, collect and return
	var nodes []Node

	err = etcd.scanNode(response.Node, func(node Node) { nodes = append(nodes, node) })

	return nodes, err
}

func (etcd *EtcdSource) parseNode(etcdNode *client.Node) (node Node, err error) {
    // decode etcd path into config tree path
    path := etcdNode.Key

    if !strings.HasPrefix(path, etcd.options.Prefix) {
        return node, fmt.Errorf("node outside tree: %s", path)
    }

    path = strings.TrimPrefix(path, etcd.options.Prefix)
    path = strings.Trim(path, "/")

	node.Source = etcd
	node.Path = path
	node.IsDir = etcdNode.Dir
	node.Value = etcdNode.Value

	return
}

// Scan through the recursive /clusterf node to return Nodes in pre-order (top-level nodes before their children)
func (etcd *EtcdSource) scanNode(etcdNode *client.Node, handler func(node Node)) error {
	if node, err := etcd.parseNode(etcdNode); err != nil {
		return err
	} else {
		// pre-order
		handler(node)
	}

    // recurse
    for _, childNode := range etcdNode.Nodes {
        if err := etcd.scanNode(childNode, handler); err != nil {
            return err
        }
    }

    return nil
}

/*
 * Watch for changed Nodes in etcd.
 *
 * Sends any changes on the returned channel. Shared amongst all listeners.
 */
func (etcd *EtcdSource) Sync(syncChan chan Node) error {
	// kick off new goroutine to handle initial services and updates
	go etcd.watch(syncChan)

	return nil
}

// Watch etcd for changes, and sync them over the chan
func (etcd *EtcdSource) watch(watchChan chan Node) {
    defer close(watchChan)

	watcher := etcd.keysAPI.Watcher(etcd.path(), &client.WatcherOptions{AfterIndex: etcd.syncIndex, Recursive: true})

    for {
		if response, err := watcher.Next(context.Background()); err != nil {
			err = fixupClusterError(err)
            log.Printf("config:EtcdSource.watch: %s\n", err)
			return
		} else if node, err := etcd.syncNode(response.Action, response.Node); err != nil {
			log.Printf("config:EtcdSource.watch %#v: syncNode: %s\n", response, err)
			return
		} else {
            log.Printf("config:EtcdSource.watch: %v\n", node)
            watchChan <- node
        }
    }
}

// Handle changed node
func (etcd *EtcdSource) syncNode(etcdAction string, etcdNode *client.Node) (Node, error) {
	node, err := etcd.parseNode(etcdNode)
	if err != nil {
		return node, err
	}

    // decode action
    switch etcdAction {
    case "create", "set", "update", "compareAndSwap":

    case "delete", "expire", "compareAndDelete":
		node.Remove = true

    default:
		return node, fmt.Errorf("Unknown etcd action: %s", etcdAction)
	}

	return node, nil
}

func (etcd *EtcdSource) refresh(node Node) error {
	var opts = client.SetOptions{
		TTL:     etcd.options.TTL,
		Refresh: true,
	}

	if _, err := etcd.keysAPI.Set(context.Background(), etcd.path(node.Path), node.Value, &opts); err != nil {
		return fixupClusterError(err)
	} else {
		return nil
	}
}

func (etcd *EtcdSource) set(node Node) error {
	var opts = client.SetOptions{
		TTL: etcd.options.TTL,
		Dir: node.IsDir,
	}

	if _, err := etcd.keysAPI.Set(context.Background(), etcd.path(node.Path), node.Value, &opts); err != nil {
		return fixupClusterError(err)
	} else {
		return nil
	}
}

func (etcd *EtcdSource) remove(node Node) error {
	var opts = client.DeleteOptions{
		Dir: node.IsDir,
	}

	if node.IsDir {
		opts.Recursive = true
	}

	if _, err := etcd.keysAPI.Delete(context.Background(), etcd.path(node.Path), &opts); err != nil {
		return fixupClusterError(err)
	} else {
		return nil
	}
}

func (etcd *EtcdSource) writer() {
	defer close(etcd.flushChan)

	var nodes map[string]Node
	var timer = time.Tick(etcd.options.TTL / 2)

	for {
		select {
		case <-timer:
			// XXX: how much of our TTL does this refresh-loop consume...?
			for _, node := range nodes {
				if err := etcd.refresh(node); err != nil {
					log.Printf("config:EtcdSource %v: writer: refresh %v: %v\n", etcd, node, err)
				}
			}
		case writeNodes, open := <-etcd.writeChan:
			// if the chan is closed from Flush(), this will get an empty map - and we remove all nodes

			// update to new dict
			for key, node := range nodes {
				if _, exists := writeNodes[key]; !exists {
					// removed
					if err := etcd.remove(node); err != nil {
						log.Printf("config:EtcdSource %v: writer: remove %v: %v\n", etcd, node, err)
					}
				}
			}
			for key, node := range writeNodes {
				if oldNode, exists := nodes[key]; !exists {
					// new node
					if err := etcd.set(node); err != nil {
						log.Printf("config:EtcdSource %v: writer: set %v: %v\n", etcd, node, err)
					}
				} else if !node.Equals(oldNode) {
					// changed
					if err := etcd.set(node); err != nil {
						log.Printf("config:EtcdSource %v: writer: set %v: %v\n", etcd, node, err)
					}
				}
			}

			if open {
				nodes = writeNodes
			} else {
				// exit
				return
			}
		}
	}
}

// Publish a config into etcd. The node will be refreshed per our TTL
func (etcd *EtcdSource) Write(nodes map[string]Node) error {
	if etcd.writeChan == nil {
		etcd.writeChan = make(chan map[string]Node)
		etcd.flushChan = make(chan error)

		go etcd.writer()
	}

	etcd.writeChan <- nodes

	// XXX: errors?
	return nil
}

// Remove all published nodes
func (etcd *EtcdSource) Flush() (err error) {
	if etcd.writeChan != nil {
		close(etcd.writeChan)
	}

	// wait for flush to complete
	for err = range etcd.flushChan {
		log.Printf("config:EtcdSource %v: Flush: %v\n", etcd, err)
	}

	return
}

