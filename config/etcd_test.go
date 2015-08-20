package config

import (
    "github.com/coreos/go-etcd/etcd"
    "fmt"
    "log"
    "regexp"
    "testing"
)

func loadBackend (t *testing.T, value string) ServiceBackend {
    node := etcd.Node{Key: "/clusterf/services/test/backends/test", Value: value, Dir: false}

    if backend, err := loadEtcdServiceBackend(&node); err != nil {
        t.Fatalf("ServiceBackend.loadEtcd(%v): %s", value, err)
        return ServiceBackend{ } // XXX
    } else {
        return backend
    }
}

func TestBackendLoad (t *testing.T) {
    simple := loadBackend(t, "{\"ipv4\": \"127.0.0.1\"}")

    if simple.IPv4 != "127.0.0.1" {
        t.Error("%v.IPv4 != 127.0.0.1", simple)
    }
}

var testSync = []struct {
    action  string
    key     string
    dir     bool
    value   string

    event   Event
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
    {action:"create",   key:"/clusterf/services", dir:true,
        event: Event{Action: NewConfig, Config: &ConfigService{ServiceName: ""}},
    },
    {action:"create",   key:"/clusterf/services/test", dir:true,
        event: Event{Action: NewConfig, Config: &ConfigService{ServiceName: "test"}},
    },
    {action:"set",      key:"/clusterf/services/test/frontend",
        value: "{\"ipv4\": \"127.0.0.1\", \"tcp\": 8080}",
        event: Event{Action: SetConfig, Config: &ConfigServiceFrontend{
            ServiceName: "test",
            Frontend:    ServiceFrontend{IPv4: "127.0.0.1", TCP: 8080},
        }},
    },
    {action:"create",   key:"/clusterf/services/test/backends", dir:true,
        event: Event{Action: NewConfig, Config: &ConfigServiceBackend{
            ServiceName: "test",
            BackendName: "",
        }},
    },
    {action:"set",      key:"/clusterf/services/test/backends/test1",
        value: "{\"ipv4\": \"127.0.0.1\", \"tcp\": 8081}",
        event: Event{Action: SetConfig, Config: &ConfigServiceBackend{
            ServiceName: "test",
            BackendName: "test1",
            Backend:     ServiceBackend{IPv4: "127.0.0.1", TCP: 8081},
        }},
    },
    {action:"set",      key:"/clusterf/services/test/backends/test2",
        value: "{\"ipv4\": \"127.0.0.1\", \"tcp\": 8082}",
        event: Event{Action: SetConfig, Config: &ConfigServiceBackend{
            ServiceName: "test",
            BackendName: "test2",
            Backend:     ServiceBackend{IPv4: "127.0.0.1", TCP: 8082},
        }},
    },
    {action:"set",      key:"/clusterf/services/test6/frontend",
        value: "{\"ipv6\": \"2001:db8::1\", \"tcp\": 8080}",
        event: Event{Action: SetConfig, Config: &ConfigServiceFrontend{
            ServiceName: "test6",
            Frontend:    ServiceFrontend{IPv6: "2001:db8::1", TCP: 8080},
        }},
    },

    {action:"delete",   key:"/clusterf/services/test3/backends/test1",
        event: Event{Action: DelConfig, Config: &ConfigServiceBackend{ServiceName: "test3", BackendName: "test1"}},
    },
    {action:"delete",   key:"/clusterf/services/test3/backends", dir:true,
        event: Event{Action: DelConfig, Config: &ConfigServiceBackend{ServiceName: "test3", BackendName: ""}},
    },
    {action:"delete",   key:"/clusterf/services/test3", dir:true,
        event: Event{Action: DelConfig, Config: &ConfigService{ServiceName: "test3"}},
    },
    {action:"delete",   key:"/clusterf/services/test", dir:true,
        event: Event{Action: DelConfig, Config: &ConfigService{ServiceName: "test"}},
    },
    {action:"delete",   key:"/clusterf/services", dir:true,
        event: Event{Action: DelConfig, Config: &ConfigService{ServiceName: ""}},
    },
}

func TestSync(t *testing.T) {
    self := &Etcd{
        config:     EtcdConfig{Prefix: "/clusterf"},
    }

    for _, testCase := range testSync {
        node := &etcd.Node{
            Key:    testCase.key,
            Dir:    testCase.dir,
            Value:  testCase.value,
        }

        log.Printf("--- %+v\n", testCase)
        event, err := self.sync(testCase.action, node)

        if err != nil {
            if testCase.error == "" {
                t.Errorf("error %+v: error %s", testCase, err)
            } else if !regexp.MustCompile(testCase.error).MatchString(err.Error()) {
                t.Errorf("fail %+v: error: %s", testCase, err)
            }
        } else if testCase.error != "" {
            t.Errorf("fail %+v: error nil", testCase)
        }

        if event == nil && testCase.event.Action == "" {

        } else if event == nil && testCase.event.Action != "" {
            t.Errorf("fail %+v: missing event %+v", testCase, testCase.event)
        } else if event != nil && testCase.event.Action == "" {
            t.Errorf("fail %+v: extra event %+v", testCase, event)
        } else {
            if event.Action != testCase.event.Action {
                t.Errorf("fail %+v: event %+v action", testCase, event)
            }

            // XXX: lawlz comparing interface{} structs for equality
            if fmt.Sprintf("%#v", event.Config) != fmt.Sprintf("%#v", testCase.event.Config) {
                t.Errorf("fail %+v: event %#v config", testCase, event.Config)
            }
        }
    }
}
