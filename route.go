package clusterf

import (
	"fmt"
	"github.com/qmsk/clusterf/config"
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
	Prefix *net.IPNet

	// attributes
	Gateway    net.IP
	IPVSMethod *ipvs.FwdMethod // or nil
}

// Build new route state from config
func (route *Route) config(configRoute config.Route) error {
	if configRoute.Prefix == "" {
		route.Prefix = nil // default
	} else if _, ipnet, err := net.ParseCIDR(configRoute.Prefix); err != nil {
		return fmt.Errorf("Invalid Prefix: %s", configRoute.Prefix)
	} else {
		route.Prefix = ipnet
	}

	if configRoute.Gateway == "" {
		route.Gateway = nil
	} else if ip := net.ParseIP(configRoute.Gateway); ip == nil {
		return fmt.Errorf("Invalid Gateway: %s", configRoute.Gateway)
	} else {
		route.Gateway = ip
	}

	if configRoute.IPVSMethod == "" {
		route.IPVSMethod = nil
	} else if fwdMethod, err := ipvs.ParseFwdMethod(configRoute.IPVSMethod); err != nil {
		return err
	} else {
		route.IPVSMethod = &fwdMethod
	}

	return nil
}

// Match given ip within our prefix
// Returns true if matches, with the length of the matching prefix
// Returns false otherwise
func (route Route) match(ip net.IP) (match bool, length int) {
	if route.Prefix == nil {
		// default match
		return true, 0

	} else if !route.Prefix.Contains(ip) {

	} else {
		prefixLength, _ := route.Prefix.Mask.Size()

		return true, prefixLength
	}

	return false, 0
}
