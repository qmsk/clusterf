package config

import (
    "testing"
)

func TestNodeUnmarshalBackend (t *testing.T) {
    var testBackendLoads = []struct{
        node Node
        serviceBackend ServiceBackend
    }{
        {
            Node{
                Source: nil,
                Path:   "services/test/backends/test",
                Value:  "{\"ipv4\": \"127.0.0.1\"}",
            },
            ServiceBackend{
                IPv4:   "127.0.0.1",
            },
        },
    }

    for _, test := range testBackendLoads {
        var serviceBackend ServiceBackend

        if err := test.node.unmarshal(&serviceBackend); err != nil {
            t.Fatalf("Node.unmarshal: %v\n", err)
        }

        if serviceBackend != test.serviceBackend {
            t.Errorf("Node.unmarshal %v: %#v\n", test.node, serviceBackend)
        }
    }
}
