package clusterf

import (
    "github.com/qmsk/clusterf/config"
    "github.com/qmsk/clusterf/ipvs"
	"net"
	"reflect"
    "syscall"
    "testing"
)

// trivial testcase with a single service with a single backend on startup
func TestConfigServices(t *testing.T) {
	routes := make(Routes)
	options := IPVSOptions{
		SchedName:	"wlc",
		FwdMethod:	ipvs.IP_VS_CONN_F_MASQ,
	}
	servicesConfig := map[string]config.Service{
		"test": config.Service{
			Frontend:	config.ServiceFrontend{IPv4:"10.0.1.1", TCP:80},
			Backends:	map[string]config.ServiceBackend{
				"test1":	config.ServiceBackend{IPv4:"10.1.0.1", TCP:80, Weight: 10},
			},
		},
	}

	testServices := Services{
		"inet+tcp://10.0.1.1:80": Service{
			Service: ipvs.Service{
				Af:			syscall.AF_INET,
				Protocol:	syscall.IPPROTO_TCP,
				Addr:		net.IP{10,0,1,1},
				Port:		80,

				SchedName:	"wlc",
				Flags:		ipvs.Flags{0, 0xffffffff},
				Netmask:	0xffffffff,
			},
			dests: ServiceDests{
				"10.1.0.1:80":	Dest{
					Dest: ipvs.Dest{
						Addr:		net.IP{10,1,0,1},
						Port:		80,
						FwdMethod:	ipvs.IP_VS_CONN_F_MASQ,
						Weight:		10,
					},
				},
			},
		},
	}

    services, err := configServices(servicesConfig, routes, options)
	if err != nil {
		t.Fatalf("configServices error: %v\n", err)
	}

	if !reflect.DeepEqual(services, testServices) {
		t.Errorf("configServices mismatch:\n\t+ %#v\n\t- %#v\n", services, testServices)
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
