package server

import (
    "github.com/coreos/go-etcd/etcd"
    "log"
    "regexp"
    "testing"
)

func loadServer (t *testing.T, raw string) ServiceServer {
    var out ServiceServer

    node := etcd.Node{Key: "/test", Value: raw, Dir: false}

    if err := out.loadEtcd(&node); err != nil {
        t.Error("ServiceServer.loadEtcd(%v): %s", raw, err)
    }

    return out
}

func TestServerLoad (t *testing.T) {
    simple := loadServer(t, "{\"ipv4\": \"127.0.0.1\"}")

    if simple.IPv4 != "127.0.0.1" {
        t.Error("%v.IPv4 != 127.0.0.1", simple)
    }
}

var testSyncErrors = []struct {
    action  string
    key     string
    dir     bool
    value   string

    event   *Event
    error   string
}{
    {action:"set", key:"/clusterf", dir:false, value:"haha", error: "Ignore unknown node"},
    {action:"set", key:"/clusterf/services", dir:false, value:"haha", error: "Ignore unknown node"},
    {action:"set", key:"/clusterf/wtf", dir:false, value:"haha", error: "Ignore unknown node"},
    {action:"set", key:"/clusterf/wtf", dir:true, error: "Ignore unknown node"},
    {action:"create", key:"/clusterf/services/wtf/frontend", dir:true, error: "Ignore unknown service wtf node"},
    {action:"create", key:"/clusterf/services/wtf/servers/test", dir:true, error: "Ignore unknown service wtf servers node"},
    {action:"set", key:"/clusterf/services/wtf/servers/test/three", value: "3", error: "Ignore unknown service wtf servers node"},
    {action:"set", key:"/clusterf/services/wtf/asdf", value: "quux", error: "Ignore unknown service wtf node"},

    {action:"set",      key:"/clusterf/services/test/frontend", value:"not json", error: "service test frontend: invalid character 'o' in literal null"},

    {action:"create",   key:"/clusterf", dir:true},
    {action:"create",   key:"/clusterf/services", dir:true},
    {action:"create",   key:"/clusterf/services/test", dir:true},
    {action:"set",      key:"/clusterf/services/test/frontend", value:"{\"ipv4\": \"127.0.0.1\", \"tcp\": 8080}",
        event: &Event{Type: NewService, Service: &Service{Name: "test"}}},
    {action:"create",   key:"/clusterf/services/test/servers", dir:true},
    {action:"set",      key:"/clusterf/services/test/servers/test1", value:"{\"ipv4\": \"127.0.0.1\", \"tcp\": 8081}",
        event: &Event{Type: NewServer, Service: &Service{Name: "test"}, ServerName: "test1"}},
    {action:"set",      key:"/clusterf/services/test/servers/test2", value:"{\"ipv4\": \"127.0.0.1\", \"tcp\": 8082}",
        event: &Event{Type: NewServer, Service: &Service{Name: "test"}, ServerName: "test2"}},

    {action:"delete",   key:"/clusterf/services/test3/servers/test1"},
    {action:"delete",   key:"/clusterf/services/test3/servers", dir:true},
    {action:"delete",   key:"/clusterf/services/test3", dir:true},
    {action:"delete",   key:"/clusterf/services/test", dir:true,
        event: &Event{Type: DelService, Service: &Service{Name: "test"}}},
    {action:"delete",   key:"/clusterf/services", dir:true},
}

func TestSync(t *testing.T) {
    self := &Etcd{
        config:     EtcdConfig{Prefix: "/clusterf"},
        services:   make(map[string]Service),
    }

    for _, testCase := range testSyncErrors {
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

        if event != nil {
            if testCase.event == nil {
                t.Errorf("fail %+v: event %+v", testCase, event)
            } else {
                if event.Type != testCase.event.Type {
                    t.Errorf("fail %+v: event %+v type", testCase, event)
                }

                if event.Service == nil {
                    // XXX: srs?
                    if testCase.event.Service != nil {
                        t.Errorf("fail %+v: event %+v service", testCase, event)
                    }
                } else if event.Service.Name != testCase.event.Service.Name {
                    t.Errorf("fail %+v: event %+v service name", testCase, event)
                }
            }
        } else if testCase.event != nil {
            t.Errorf("fail %+v: event nil", testCase)
        }

        // t.Logf("ok %+v", testCase)
    }
}
