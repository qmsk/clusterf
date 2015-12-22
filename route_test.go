package clusterf

import (
    "github.com/qmsk/clusterf/config"
    "fmt"
    "testing"
)

// Test multiple NewConfig for Routes from multiple config sources
// https://github.com/qmsk/clusterf/issues/7
func TestNewConfigRoute(t *testing.T) {
    services := NewServices()

    // initial configs from one source
    services.NewConfig(&config.ConfigRoute{RouteName:""})
    services.NewConfig(&config.ConfigRoute{RouteName:"test", Route:config.Route{Prefix4:"10.0.0.0/24", IpvsMethod:"droute"}})

    route := services.routes["test"]

    if route == nil {
        t.Errorf("test route not configured")
        return
    }
    if fmt.Sprintf("%v", route.Prefix4) != "10.0.0.0/24" || route.ipvs_fwdMethod.String() != "droute" {
        t.Errorf("test route mis-configured: %#v", route)
        return
    }

    // second round of configs from a different sources
    services.NewConfig(&config.ConfigRoute{RouteName:""})
    route2 := services.routes["test"]

    if route2 == nil {
        t.Errorf("test route disappeared")
        return
    }
    if fmt.Sprintf("%v", route2.Prefix4) != "10.0.0.0/24" || route2.ipvs_fwdMethod.String() != "droute" {
        t.Errorf("test route mis-configured: %#v", route2)
        return
    }
}

// TODO: test Services.ConfigEvent(config.DelConfig, config.ConfigRoute{RouteName:""})
//       requires mock IPVSDriver to use for SyncIPVS()
