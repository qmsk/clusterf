package config

import (
	"github.com/kylelemons/godebug/pretty"
	"math/rand"
	"sync"
	"testing"
	"time"
)

// from config/test-files
var testFilesConfig Config = Config{
	Services: map[string]Service{
		"test": Service{
			Frontend: &ServiceFrontend{
				IPv4: "192.0.2.0",
				TCP:  80,
			},
			Backends: map[string]ServiceBackend{
				"test1": ServiceBackend{
					IPv4:   "192.168.1.1",
					TCP:    8080,
					Weight: 10,
				},
				"test2": ServiceBackend{
					IPv4:   "192.168.1.2",
					TCP:    8080,
					Weight: 10,
				},
			},
		},
		"test6": Service{
			Frontend: &ServiceFrontend{
				IPv6: "2001:db8::1",
				TCP:  80,
			},
			Backends: map[string]ServiceBackend{
				"test1": ServiceBackend{
					IPv6:   "2001:db8:1::1",
					TCP:    8080,
					Weight: 10,
				},
			},
		},
	},
	Routes: map[string]Route{
		"default": Route{},
		"test1": Route{
			Prefix:     "192.168.1.0/24",
			IPVSMethod: "droute",
		},
		"test2": Route{
			Prefix:     "192.168.2.0/24",
			IPVSMethod: "droute",
		},
	},
}

func TestReaderFiles(t *testing.T) {
	var readerOptions = ReaderOptions{
		SourceURLs: []string{
			"file://./test-files",
		},
	}

	reader, err := readerOptions.Reader()
	if err != nil {
		t.Fatalf("Reader: %v", err)
	}

	config := reader.Get()

	// diff
	prettyConfig := pretty.Config{
		// omit Meta node
		IncludeUnexported: false,
	}

	if diff := prettyConfig.Compare(testFilesConfig, config); diff != "" {
		t.Errorf("reader config:\n%s", diff)
	}
}

type testReaderSource struct {
	name      string
	scanNodes []Node
	config    Config // final state
}

func (test *testReaderSource) String() string {
	return test.name
}

func (test *testReaderSource) Scan() ([]Node, error) {
	for i, node := range test.scanNodes {
		node.Source = test

		test.scanNodes[i] = node
	}

	return test.scanNodes, nil
}

type testReaderSyncSource struct {
	testReaderSource

	syncNodes []Node
	syncGroup *sync.WaitGroup
}

func (test *testReaderSyncSource) Sync(syncChan chan Node) error {
	test.syncGroup.Add(1)

	go func() {
		defer test.syncGroup.Done()

		for _, node := range test.syncNodes {
			node.Source = test

			// millisleep to tempt concurrency
			time.Sleep(time.Duration(rand.Float32() * float32(time.Millisecond)))

			syncChan <- node
		}
	}()

	return nil
}

var testReaderSourceScanErr = &testReaderSource{
	name: "test-scan-err",
	scanNodes: []Node{
		Node{Path: "", IsDir: true},
		Node{Path: "routes", IsDir: true},
		Node{Path: "routes/test1", Value: `asdf`},
	},
}

func TestReaderScanError(t *testing.T) {
	var reader Reader

	if err := reader.init(); err != nil {
		panic(err)
	}

	err := reader.open(testReaderSourceScanErr)

	if err == nil {
		t.Errorf("reader.open %v: %v\n", testReaderSourceScanErr, err)
	} else {
		t.Logf("reader.open %v: %v\n", testReaderSourceScanErr, err)
	}
}

var testReaderSources = map[string]*testReaderSyncSource{
	"test-1": {
		testReaderSource: testReaderSource{
			name: "test-1",
			scanNodes: []Node{
				Node{Path: "", IsDir: true},
				Node{Path: "routes", IsDir: true},
				Node{Path: "routes/test1", Value: `{"Prefix": "192.168.1.0/24", "IPVSMethod": "droute"}`},
				Node{Path: "services", IsDir: true},
				Node{Path: "services/test", IsDir: true},
				Node{Path: "services/test/frontend", Value: `{"ipv4": "192.0.2.0", "tcp": 80}`},
				Node{Path: "services/test/backends", IsDir: true},
				Node{Path: "services/test/backends/test1", Value: `{"ipv4": "192.168.1.1", "tcp": 8080}`},
				Node{Path: "services/test/backends/test2", Value: `{"ipv4": "192.168.1.2", "tcp": 8080}`},
			},
		},
		syncNodes: []Node{
			Node{Path: "services/test/backends/test3", Value: `{"ipv4": "192.168.1.3", "tcp": 8080}`},
			Node{Path: "services/test/backends/test1", Remove: true},
			Node{Path: "services/test/backends", IsDir: true, Remove: true},
			Node{Path: "services/test6/frontend", Value: `{"ipv6": "2001:db8::1", "tcp": 80}`},
			Node{Path: "services/test6/backends/test1", Value: `{"ipv6": "2001:db8:1::1", "tcp": 8080}`},
		},
	},
	"test-2": {
		testReaderSource: testReaderSource{
			name: "test-2",
			scanNodes: []Node{
				Node{Path: "", IsDir: true},
				Node{Path: "services", IsDir: true},
				Node{Path: "services/test2", IsDir: true},
				Node{Path: "services/test2/frontend", Value: `{"ipv4": "192.0.2.2", "tcp": 80}`},
				Node{Path: "services/test2/backends", IsDir: true},
			},
		},
		syncNodes: []Node{
			Node{Path: "services/test2/backends/test1", Value: `{"ipv4": "192.168.2.1", "tcp": 8080}`},
			Node{Path: "services", IsDir: true, Remove: true},
			Node{Path: "routes", IsDir: true},
			Node{Path: "routes/test2", Value: `{"Prefix": "192.168.2.0/24", "IpvsMethod": "droute"}`},
		},
	},

	// these routes get filtered out
	"skip-test3": {
		testReaderSource: testReaderSource{
			name: "skip-test3",
			scanNodes: []Node{
				Node{Path: "routes", IsDir: true},
				Node{Path: "routes/test3", Value: `{"Prefix": "192.168.2.0/24", "IpvsMethod": "masq"}`},
			},
		},
		syncNodes: []Node{
			Node{Path: "routes/test3", Value: `{"Prefix": "192.168.2.0/24", "IpvsMethod": "droute"}`},
		},
	},
}

var testReaderConfig Config = Config{
	Services: map[string]Service{
		"test": Service{
			Frontend: &ServiceFrontend{
				IPv4: "192.0.2.0",
				TCP:  80,
			},
			Backends: map[string]ServiceBackend{},
		},
		/*"test2": Service{
		            Frontend: &ServiceFrontend{
		                IPv4:   "192.0.2.2",
		                TCP:    80,
		            },
		            Backends: map[string]ServiceBackend{
						"test1": ServiceBackend{
							IPv4:   "192.168.2.1",
							TCP:    8080,
							Weight: 10,
						},
					},
				},*/
		"test6": Service{
			Frontend: &ServiceFrontend{
				IPv6: "2001:db8::1",
				TCP:  80,
			},
			Backends: map[string]ServiceBackend{
				"test1": ServiceBackend{
					IPv6:   "2001:db8:1::1",
					TCP:    8080,
					Weight: 10,
				},
			},
		},
	},
	Routes: map[string]Route{
		"test1": Route{
			Prefix:     "192.168.1.0/24",
			IPVSMethod: "droute",
		},
		"test2": Route{
			Prefix:     "192.168.2.0/24",
			IPVSMethod: "droute",
		},
	},
}

func TestReaderUpdate(t *testing.T) {
	var reader Reader

	reader.options.FilterRoutes = "test-"

	if err := reader.init(); err != nil {
		panic(err)
	}

	// setup
	var syncGroup sync.WaitGroup

	for name, testSource := range testReaderSources {
		testSource.syncGroup = &syncGroup

		if err := reader.open(testSource); err != nil {
			t.Fatalf("reader.open %v: %v\n", name, err)
		}
	}

	// ensure the test terminates
	go func() {
		syncGroup.Wait()
		reader.stop()
	}()

	// read
	var readerConfig Config

	for config := range reader.Listen() {
		t.Logf("reader.Listen: tick\n")

		// touch it
		_ = pretty.Sprint(config)

		readerConfig = config
	}

	prettyConfig := pretty.Config{
		// omit Meta node
		IncludeUnexported: false,
	}

	if diff := prettyConfig.Compare(testReaderConfig, readerConfig); diff != "" {
		t.Errorf("reader config:\n%s", diff)
	}
}
