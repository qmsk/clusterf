package config

import (
	"github.com/kylelemons/godebug/pretty"
	"testing"
	"math/rand"
	"time"
)

type testReaderSource struct{
	name		string
	scanNodes	[]Node
	syncNodes	[]Node
	config		Config	// final state
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
	go func(){
		// XXX: not correct for multiple sync sources
		defer close(syncChan)

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
            Node{Path:"services/test6/frontend", Value: "{\"ipv6\": \"2001:db8::1\", \"tcp\": 8080}"},
			Node{Path:"services/test6/backends/test1", Value: "{\"ipv6\": \"2001:db8:1::1\", \"tcp\": 8080}"},
		},
	},
}

func TestReaderUpdate(t *testing.T) {
	var reader Reader

	if err := reader.open(testReaderSources["test1"]); err != nil {
		t.Fatalf("reader.open test1: %v\n", err)
	}

	reader.start()

	for config := range reader.Listen() {
		t.Logf("reader.Listen: tick\n")

		// touch it
		_ = pretty.Sprint(config)
	}
}
