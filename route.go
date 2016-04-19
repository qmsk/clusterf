package clusterf

import (
    "github.com/qmsk/clusterf/config"
    "fmt"
    "github.com/qmsk/clusterf/ipvs"
    "net"
)

type Routes map[string]Route

// Return most-specific matching route for given IPv4/IPv6 IP
func (routes Routes) Lookup(ip net.IP) *Route {
    var matchRoute Route
    var matchLength int = -1

    for _, route := range routes {
        if match, routeLength := route.match(ip); !match {

        } else if routeLength > matchLength {
            matchRoute = route
            matchLength = routeLength
        }
    }

	if matchLength < 0 {
		return nil
	} else {
		return &matchRoute
	}
}

// Update state from config
func configRoutes(configRoutes map[string]config.Route) (Routes, error) {
	newRoutes := make(Routes)

	for routeName, configRoute := range configRoutes {
		var route Route

		if err := route.config(configRoute); err != nil {
			return nil, fmt.Errorf("Config route %v: %v", routeName, err)
		} else {
			newRoutes[routeName] = route
		}
	}

	return newRoutes, nil
}

type Route struct {
    // default -> nil
    Prefix4			*net.IPNet

    // attributes
    Gateway4        net.IP
    ipvs_fwdMethod  *ipvs.FwdMethod
    ipvs_filter     bool
}

// Build new route state from config
func (route *Route) config(configRoute config.Route) error {
    if configRoute.Prefix4 == "" {
        route.Prefix4 = nil // default
    } else if _, prefix4, err := net.ParseCIDR(configRoute.Prefix4); err != nil {
        return fmt.Errorf("Invalid Prefix4: %s", configRoute.Prefix4)
    } else {
        route.Prefix4 = prefix4
    }

    if configRoute.Gateway4 == "" {
        route.Gateway4 = nil
    } else if gateway4 := net.ParseIP(configRoute.Gateway4).To4(); gateway4 == nil {
        return fmt.Errorf("Invalid Gateway4: %s", configRoute.Gateway4)
    } else {
        route.Gateway4 = gateway4
    }

    if configRoute.IpvsMethod == "" {
        route.ipvs_filter = false
        route.ipvs_fwdMethod = nil
    } else if configRoute.IpvsMethod == "filter" {
        route.ipvs_filter = true
        route.ipvs_fwdMethod = nil
    } else if fwdMethod, err := ipvs.ParseFwdMethod(configRoute.IpvsMethod); err != nil {
        return err
    } else {
        route.ipvs_filter = false
        route.ipvs_fwdMethod = &fwdMethod
    }

    return nil
}


// Match given ip within our prefix
// Returns true if matches, with the length of the matching prefix
// Returns false otherwise
func (route Route) match(ip net.IP) (match bool, length int) {
    if ip4 := ip.To4(); ip4 == nil {

    } else if route.Prefix4 == nil {
        // default match
        return true, 0

    } else if !route.Prefix4.Contains(ip4) {

    } else {
        prefixLength, _:= route.Prefix4.Mask.Size()

        return true, prefixLength
    }

    return false, 0
}
