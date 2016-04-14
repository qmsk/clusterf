package clusterf

import (
    "github.com/qmsk/clusterf/config"
    "github.com/qmsk/clusterf/ipvs"
	"net"
	"reflect"
    "testing"
)

// Test basic route configuration
// Test multiple NewConfig for Routes from multiple config sources
func TestConfigRoute(t *testing.T) {
	routeConfig := map[string]config.Route{
		"test": config.Route{Prefix4:"10.0.0.0/24", IpvsMethod:"droute"},
	}
	ipvs_fwdMethod := ipvs.FwdMethod(ipvs.IP_VS_CONN_F_DROUTE)
	testRoutes := Routes{
		"test": Route{
			Prefix4:		&net.IPNet{net.IP{10,0,0,0}, net.IPMask{255,255,255,0}},
			Gateway4:		nil,
			ipvs_fwdMethod:	&ipvs_fwdMethod,
		},
	}

	routes, err := configRoutes(routeConfig)
	if err != nil {
		t.Fatalf("configRoutes: %v\n", err)
	}

	if !reflect.DeepEqual(routes, testRoutes) {
		t.Errorf("configServices mismatch: %#v\n\texpected: %#v\n", routes, testRoutes)
	}
}
