package clusterf

import (
    "github.com/qmsk/clusterf/config"
    "github.com/qmsk/clusterf/ipvs"
	"net"
	"github.com/kylelemons/godebug/pretty"
    "syscall"
    "testing"
)

var testConfigServices = map[string]struct{
	options			IPVSOptions
	configRoutes	map[string]config.Route
	config			map[string]config.Service
	services		Services
}{
	"simple": {
		options:	IPVSOptions{
			SchedName:	"wlc",
			FwdMethod:	ipvs.IP_VS_CONN_F_MASQ,
		},
		configRoutes: map[string]config.Route{

		},
		config:		map[string]config.Service{
			"test": config.Service{
				Frontend:	config.ServiceFrontend{IPv4:"10.0.0.1", TCP:80, UDP:80},
				Backends:	map[string]config.ServiceBackend{
					"test1":	config.ServiceBackend{IPv4:"10.1.0.1", TCP:8080, UDP:8081, Weight:10},
					"test2":	config.ServiceBackend{IPv4:"10.1.0.2", TCP:8082, Weight:10},
				},
			},
		},
		services:	Services{
			"inet+tcp://10.0.0.1:80": Service{
				Service: ipvs.Service{
					Af:			syscall.AF_INET,
					Protocol:	syscall.IPPROTO_TCP,
					Addr:		net.IP{10,0,0,1},
					Port:		80,

					SchedName:	"wlc",
					Flags:		ipvs.Flags{0, 0xffffffff},
					Netmask:	0xffffffff,
				},
				dests: ServiceDests{
					"10.1.0.1:8080": Dest{
						Dest: ipvs.Dest{
							Addr:		net.IP{10,1,0,1},
							Port:		8080,
							FwdMethod:	ipvs.IP_VS_CONN_F_MASQ,
							Weight:		10,
						},
					},
					"10.1.0.2:8082": Dest{
						Dest: ipvs.Dest{
							Addr:		net.IP{10,1,0,2},
							Port:		8082,
							FwdMethod:	ipvs.IP_VS_CONN_F_MASQ,
							Weight:		10,
						},
					},
				},
			},
			"inet+udp://10.0.0.1:80": Service{
				Service: ipvs.Service{
					Af:			syscall.AF_INET,
					Protocol:	syscall.IPPROTO_UDP,
					Addr:		net.IP{10,0,0,1},
					Port:		80,

					SchedName:	"wlc",
					Flags:		ipvs.Flags{0, 0xffffffff},
					Netmask:	0xffffffff,
				},
				dests: ServiceDests{
					"10.1.0.1:8081": Dest{
						Dest: ipvs.Dest{
							Addr:		net.IP{10,1,0,1},
							Port:		8081,
							FwdMethod:	ipvs.IP_VS_CONN_F_MASQ,
							Weight:		10,
						},
					},
				},
			},
		},
	},
}

func TestConfigServices(t *testing.T) {
	for testName, test := range testConfigServices {
		routes, err := configRoutes(test.configRoutes)
		if err != nil {
			t.Fatalf("%v configRoutes: %v\n", testName, err)
		}

		services, err := configServices(test.config, routes, test.options)
		if err != nil {
			t.Fatalf("%v configServices error: %v\n", testName, err)
		}

		if diff := pretty.Compare(test.services, services); diff != "" {
			t.Errorf("%v configServices incorrect:\n%s", testName, diff)
		}
	}
}

// Test adding a new ConfigServiceFrontend after sync
// https://github.com/qmsk/clusterf/issues/4
/* func TestServiceAdd(t *testing.T) {
    serviceFrontend := config.ServiceFrontend{IPv4:"10.0.1.1", TCP:80}
    serviceBackend := config.ServiceBackend{IPv4:"10.1.0.1", TCP:80}

    services := NewServices()

    // sync
    ipvsDriver, err := services.SyncIPVS(IpvsConfig{FwdMethod: "masq", SchedName: "wlc", mock: true})
    if err != nil {
        t.Fatalf("services.SyncIPVS: %v", err)
    }

    // add
    services.ConfigEvent(config.Event{Action:config.SetConfig, Config:&config.ConfigServiceFrontend{ConfigSource:"test", ServiceName:"test", Frontend:serviceFrontend}})
    services.ConfigEvent(config.Event{Action:config.SetConfig, Config:&config.ConfigServiceBackend{ConfigSource:"test", ServiceName:"test", BackendName:"test1", Backend:serviceBackend}})

    // test ipvsDriver.dests
    ipvsKey := ipvsKey{"inet+tcp://10.0.1.1:80", "10.1.0.1:80"}

    if len(ipvsDriver.dests) != 1 {
        t.Errorf("incorrect sync dests: %v", ipvsDriver.dests)
    }
    if ipvsDriver.dests[ipvsKey] == nil {
        t.Errorf("missing sync dest: %v", ipvsKey)
    }
} */
