package server

import (
    "github.com/coreos/go-etcd/etcd"
    "log"
    "regexp"
    "testing"
)

func loadBackend (t *testing.T, value string) *ServiceBackend {
    node := etcd.Node{Key: "/clusterf/services/test/backends/test", Value: value, Dir: false}

    if backend, err := loadEtcdServiceBackend(&node); err != nil {
        t.Error("ServiceBackend.loadEtcd(%v): %s", value, err)
        return nil
    } else {
        return backend
    }
}

func TestBackendLoad (t *testing.T) {
    simple := loadBackend(t, "{\"ipv4\": \"127.0.0.1\"}")

    if simple == nil {

    } else if simple.IPv4 != "127.0.0.1" {
        t.Error("%v.IPv4 != 127.0.0.1", simple)
    }
}

var testSyncErrors = []struct {
    action  string
    key     string
    dir     bool
    value   string

    events  []Event
    error   string
}{
    {action:"set", key:"/clusterf", dir:false, value:"haha", error: "Ignore unknown node"},
    {action:"set", key:"/clusterf/services", dir:false, value:"haha", error: "Ignore unknown node"},
    {action:"set", key:"/clusterf/wtf", dir:false, value:"haha", error: "Ignore unknown node"},
    {action:"set", key:"/clusterf/wtf", dir:true, error: "Ignore unknown node"},
    {action:"create", key:"/clusterf/services/wtf/frontend", dir:true, error: "Ignore unknown service wtf node"},
    {action:"create", key:"/clusterf/services/wtf/backends/test", dir:true, error: "Ignore unknown service wtf backends node"},
    {action:"set", key:"/clusterf/services/wtf/backends/test/three", value: "3", error: "Ignore unknown service wtf backends node"},
    {action:"set", key:"/clusterf/services/wtf/asdf", value: "quux", error: "Ignore unknown service wtf node"},

    {action:"set",      key:"/clusterf/services/test/frontend", value:"not json", error: "service test frontend: invalid character 'o' in literal null"},

    {action:"create",   key:"/clusterf", dir:true},
    {action:"create",   key:"/clusterf/services", dir:true},
    {action:"create",   key:"/clusterf/services/test", dir:true},
    {action:"set",      key:"/clusterf/services/test/frontend",
        value: "{\"ipv4\": \"127.0.0.1\", \"tcp\": 8080}",
        events: []Event{{Type: NewService, Service: &Service{Name: "test"}}},
    },
    {action:"create",   key:"/clusterf/services/test/backends", dir:true},
    {action:"set",      key:"/clusterf/services/test/backends/test1",
        value: "{\"ipv4\": \"127.0.0.1\", \"tcp\": 8081}",
        events: []Event{{Type: NewBackend, Service: &Service{Name: "test"}, BackendName: "test1"}},
    },
    {action:"set",      key:"/clusterf/services/test/backends/test2",
        value: "{\"ipv4\": \"127.0.0.1\", \"tcp\": 8082}",
        events: []Event{{Type: NewBackend, Service: &Service{Name: "test"}, BackendName: "test2"}},
    },
    {action:"set",      key:"/clusterf/services/test6/frontend",
        value: "{\"ipv6\": \"2001:db8::1\", \"tcp\": 8080}",
        events: []Event{{Type: NewService, Service: &Service{Name: "test6"}}},
    },

    {action:"delete",   key:"/clusterf/services/test3/backends/test1"},
    {action:"delete",   key:"/clusterf/services/test3/backends", dir:true},
    {action:"delete",   key:"/clusterf/services/test3", dir:true},
    {action:"delete",   key:"/clusterf/services/test", dir:true,
        events: []Event{{Type: DelService, Service: &Service{Name: "test"}}},
    },
    {action:"delete",   key:"/clusterf/services", dir:true,
        events: []Event{{Type: DelService, Service: &Service{Name: "test6"}}},
    },
}

func TestSync(t *testing.T) {
    self := &Etcd{
        config:     EtcdConfig{Prefix: "/clusterf"},
        services:   newServices(),
    }

    for _, testCase := range testSyncErrors {
        node := &etcd.Node{
            Key:    testCase.key,
            Dir:    testCase.dir,
            Value:  testCase.value,
        }
        var events []*Event

        log.Printf("--- %+v\n", testCase)
        err := self.sync(testCase.action, node, func (event *Event) {
            events = append(events, event)
        })

        if err != nil {
            if testCase.error == "" {
                t.Errorf("error %+v: error %s", testCase, err)
            } else if !regexp.MustCompile(testCase.error).MatchString(err.Error()) {
                t.Errorf("fail %+v: error: %s", testCase, err)
            }
        } else if testCase.error != "" {
            t.Errorf("fail %+v: error nil", testCase)
        }

        for i := 0; i < len(events) && i < len(testCase.events); i++ {
            if i >= len(events) {
                t.Errorf("fail %+v: missing event %+v", testCase, testCase.events[i])
            } else if i >= len(testCase.events) {
                t.Errorf("fail %+v: extra event %+v", testCase, events[i])
            } else {
                if events[i].Type != testCase.events[i].Type {
                    t.Errorf("fail %+v: event %+v type", testCase, events[i])
                }

                if events[i].Service == nil {
                    // XXX: srs?
                    if testCase.events[i].Service != nil {
                        t.Errorf("fail %+v: event %+v service", testCase, events[i])
                    }
                } else if events[i].Service.Name != testCase.events[i].Service.Name {
                    t.Errorf("fail %+v: event %+v service name", testCase, events[i])
                }
            }
        }

        // t.Logf("ok %+v", testCase)
    }
}
