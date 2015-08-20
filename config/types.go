package config
/*
 * Externally exposed types.
 */

type ServiceFrontend struct {
    IPv4    string  `json:"ipv4,omitempty"`
    IPv6    string  `json:"ipv6,omitempty"`
    TCP     uint16  `json:"tcp,omitempty"`
    UDP     uint16  `json:"udp,omitempty"`
}

type ServiceBackend struct {
    IPv4    string  `json:"ipv4,omitempty"`
    IPv6    string  `json:"ipv6,omitempty"`
    TCP     uint16  `json:"tcp,omitempty"`
    UDP     uint16  `json:"udp,omitempty"`
}

/*
 * Events when config changes
 */
type Action string

const (
    // NOTE: all etcd.Sync() updates are SetConfig
    //       NewConfig can be used together with Scan()...?
    NewConfig     Action   = "new"
    SetConfig     Action   = "set"
    DelConfig     Action   = "del"
)

// A Config has changed
type Event struct {
    Action      Action
    Config      Config
}

type Config interface {

}

/* Different config objects */

// Used when a new service directory is created or destroyed.
// May not necessarily be delivered when a new service is created; you can expect to directly get a ConfigService* event for a new service
// May be delievered with an empty ServiceName:"" if *all* services are to be deleted
type ConfigService struct {
    ServiceName     string
}

type ConfigServiceFrontend struct {
    ServiceName     string

    Frontend        ServiceFrontend
}

// May be delivered with an empty BackendName:"" if *all* service backends are to be deleted
type ConfigServiceBackend struct {
    ServiceName     string
    BackendName     string

    Backend         ServiceBackend
}
