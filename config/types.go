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

type ConfigService struct {
    ServiceName     string
}

type ConfigServiceFrontend struct {
    ServiceName     string

    Frontend        ServiceFrontend
}

type ConfigServiceBackend struct {
    ServiceName     string
    BackendName     string

    Backend         ServiceBackend
}
