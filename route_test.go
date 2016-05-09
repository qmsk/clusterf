package clusterf

import (
	"github.com/kylelemons/godebug/pretty"
	"github.com/qmsk/clusterf/config"
	"github.com/qmsk/clusterf/ipvs"
	"net"
	"testing"
)

var testIpvsFwdMethodMasq = ipvs.FwdMethod(ipvs.IP_VS_CONN_F_MASQ)
var testIpvsFwdMethodDroute = ipvs.FwdMethod(ipvs.IP_VS_CONN_F_DROUTE)
var testRoutes = Routes{
	"test2": Route{
		Prefix:     &net.IPNet{net.IP{10, 2, 0, 0}, net.IPMask{255, 255, 255, 0}},
		Gateway:    net.IP{10, 255, 0, 2},
		IPVSMethod: &testIpvsFwdMethodMasq,
	},

	"test1": Route{
		Prefix:     &net.IPNet{net.IP{10, 1, 0, 0}, net.IPMask{255, 255, 255, 0}},
		Gateway:    nil,
		IPVSMethod: &testIpvsFwdMethodMasq,
	},
	"internal": Route{
		Prefix: &net.IPNet{net.IP{10, 0, 0, 0}, net.IPMask{255, 0, 0, 0}},
	},
}

// Test basic route configuration
// Test multiple NewConfig for Routes from multiple config sources
func TestConfigRoute(t *testing.T) {
	routeConfig := map[string]config.Route{
		"test2":    config.Route{Prefix: "10.2.0.0/24", Gateway: "10.255.0.2", IPVSMethod: "masq"},
		"test1":    config.Route{Prefix: "10.1.0.0/24", IPVSMethod: "masq"},
		"internal": config.Route{Prefix: "10.0.0.0/8", IPVSMethod: ""},
	}

	routes, err := configRoutes(routeConfig)
	if err != nil {
		t.Fatalf("configRoutes: %v\n", err)
	}

	if diff := pretty.Compare(testRoutes, routes); diff != "" {
		t.Errorf("configRoutes incorrect:\n%s", diff)
	}
}

func TestRouteLookup(t *testing.T) {
	if diff := pretty.Compare(testRoutes["test1"], testRoutes.Lookup(net.IP{10, 1, 0, 1})); diff != "" {
		t.Errorf("routes.Lookup 10.1.0.1:\n%s", diff)
	}
	if diff := pretty.Compare(testRoutes["internal"], testRoutes.Lookup(net.IP{10, 99, 0, 1})); diff != "" {
		t.Errorf("routes.Lookup 10.99.0.1:\n%s", diff)
	}
	if diff := pretty.Compare(nil, testRoutes.Lookup(net.IP{192, 0, 2, 1})); diff != "" {
		t.Errorf("routes.Lookup 192.0.2.1:\n%s", diff)
	}
}
