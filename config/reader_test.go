package config

import (
	"github.com/kylelemons/godebug/pretty"
	"testing"
	"math/rand"
	"sync"
	"time"
)

type testReaderSource struct{
	name		string
	scanNodes	[]Node
	syncNodes	[]Node
	config		Config	// final state

	syncGroup	*sync.WaitGroup
}

func (test testReaderSource) String() string {
	return test.name
}

func (test testReaderSource) Scan() ([]Node, error) {
	for _, node := range test.scanNodes {
		node.Source = test
	}

	return test.scanNodes, nil
}

func (test testReaderSource) Sync(syncChan chan Node) error {
	test.syncGroup.Add(1)

	go func(){
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

var testReaderSources = map[string]testReaderSource{
	"test1": {
		name: "test1",
        scanNodes: []Node{
            Node{Path:"", IsDir:true},
            Node{Path:"services", IsDir:true},
            Node{Path:"services/test", IsDir:true},
            Node{Path:"services/test/frontend", Value: "{\"ipv4\": \"192.0.2.0\", \"tcp\": 80}"},
            Node{Path:"services/test/backends", IsDir:true},
            Node{Path:"services/test/backends/test1", Value: "{\"ipv4\": \"192.168.1.1\", \"tcp\": 8080}"},
            Node{Path:"services/test/backends/test2", Value: "{\"ipv4\": \"192.168.1.2\", \"tcp\": 8080}"},
        },
        syncNodes: []Node{
            Node{Path:"services/test/backends/test3", Value: "{\"ipv4\": \"192.168.1.3\", \"tcp\": 8080}"},
            Node{Path:"services/test/backends/test1", Remove: true},
            Node{Path:"services/test/backends", IsDir:true, Remove: true},
            Node{Path:"services/test6/frontend", Value: "{\"ipv6\": \"2001:db8::1\", \"tcp\": 80}"},
			Node{Path:"services/test6/backends/test1", Value: "{\"ipv6\": \"2001:db8:1::1\", \"tcp\": 8080}"},
		},
	},
	"test2": {
		name: "test2",
        scanNodes: []Node{
            Node{Path:"", IsDir:true},
            Node{Path:"services", IsDir:true},
            Node{Path:"services/test2", IsDir:true},
            Node{Path:"services/test2/frontend", Value: "{\"ipv4\": \"192.0.2.2\", \"tcp\": 80}"},
            Node{Path:"services/test2/backends", IsDir:true},
        },
        syncNodes: []Node{
            Node{Path:"services/test2/backends/test1", Value: "{\"ipv4\": \"192.168.2.1\", \"tcp\": 8080}"},
            // Node{Path:"services", IsDir:true, Remove: true},
		},
	},
}

var testReaderConfig Config = Config{
    Services: map[string]Service{
        "test": Service{
            Frontend: ServiceFrontend{
                IPv4:   "192.0.2.0",
                TCP:    80,
            },
            Backends: map[string]ServiceBackend{},
        },
		"test2": Service{
            Frontend: ServiceFrontend{
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
		},
        "test6": Service{
            Frontend:   ServiceFrontend{
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
}


func TestReaderUpdate(t *testing.T) {
	var reader = Reader{

	}

	// setup
	var syncGroup sync.WaitGroup

	for name, testSource := range testReaderSources {
		testSource.syncGroup = &syncGroup

		if err := reader.open(testSource); err != nil {
			t.Fatalf("reader.open %v: %v\n", name, err)
		}
	}

	reader.start()

	// ensure the test terminates
	go func(){
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

	if diff := pretty.Compare(testReaderConfig, readerConfig); diff != "" {
		t.Errorf("reader config:\n%s", diff)
	}
}
