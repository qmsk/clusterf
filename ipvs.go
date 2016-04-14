package clusterf

import (
    "github.com/qmsk/clusterf/config"
    "fmt"
    "github.com/qmsk/clusterf/ipvs"
    "log"
    "syscall"
)

type IPVSOptions struct {
	FilterRoutes	string					`long:"filter-routes" value-name:"URL-PREFIX" help:"Only apply routes from matching --config-source"`

	Debug			bool		`long:"ipvs-debug"`
	Reset			bool		`long:"ipvs-reset" help:"Flush all IPVS services before applying configuration"`
	Print			bool		`long:"ipvs-print"`

	FwdMethod		ipvs.FwdMethod	`long:"ipvs-fwd-method" default:"masq"`
	SchedName		string			`long:"ipvs-sched-name" default:"wlc"`

    mock			bool        // used for testing; do not actually setup the ipvsClient
}

func (options IPVSOptions) Open() (*IPVSDriver, error) {
	var driver IPVSDriver

	if err := driver.init(options); err != nil {
		return nil, err
	}

	return &driver, nil
}

// Used to expand ServiceFrontend/Backend -> multiple ipvs.service/Dest
type ipvsType struct {
    Af          ipvs.Af
    Protocol    ipvs.Protocol
}

var ipvsTypes = []ipvsType {
    { syscall.AF_INET,      syscall.IPPROTO_TCP },
    { syscall.AF_INET6,     syscall.IPPROTO_TCP },
    { syscall.AF_INET,      syscall.IPPROTO_UDP },
    { syscall.AF_INET6,     syscall.IPPROTO_UDP },
}

// Running state
type IPVSDriver struct {
	options		IPVSOptions
    ipvsClient *ipvs.Client

	// running state
	routes		Routes
	services	Services
}

func (driver *IPVSDriver) init(options IPVSOptions) error {
	driver.options = options

    if options.mock {

    } else if ipvsClient, err := ipvs.Open(); err != nil {
        return err
    } else {
        driver.ipvsClient = ipvsClient

		if options.Debug {
			driver.ipvsClient.SetDebug()
		}
    }

    if driver.ipvsClient == nil {
        // mock'd
    } else if info, err := driver.ipvsClient.GetInfo(); err != nil {
        return err
    } else {
        log.Printf("ipvs.GetInfo: version=%s, conn_tab_size=%d\n", info.Version, info.ConnTabSize)
    }

	if options.Reset {
		if err := driver.reset(); err != nil {
			return err
		}
	} else {
		if err := driver.sync(); err != nil {
			return err
		}
	}

	if options.Print {
		driver.Print()
	}

    return nil
}

// Reset running state in kernel
func (driver *IPVSDriver) reset() error {
	if err := driver.ipvsClient.Flush(); err != nil {
		return err
	}

	return nil
}

// Sync running state from kernel
func (driver *IPVSDriver) sync() error {
	services := make(Services)

	if driver.ipvsClient == nil {
		return fmt.Errorf("Cannot sync against a mock'd ipvs.Client")
	} else if ipvsServices, err := driver.ipvsClient.ListServices(); err != nil {
		return fmt.Errorf("ipvs.ListServices: %v\n", err)
	} else {
		for _, ipvsService := range ipvsServices {
			if dests, err := driver.ipvsClient.ListDests(ipvsService); err != nil {
				return fmt.Errorf("ipvs.ListDests %v: %v\n", ipvsService, err)
			} else {
				services.sync(ipvsService, dests)
			}
		}
	}

	driver.services = services

	return nil
}

func (driver *IPVSDriver) newService(service Service) error {
	if driver.ipvsClient == nil {
		return nil
	} else {
		return driver.ipvsClient.NewService(service.Service)
	}
}

func (driver *IPVSDriver) setService(service Service) error {
	if driver.ipvsClient == nil {
		return nil
	} else {
		return driver.ipvsClient.SetService(service.Service)
	}
}

func (driver *IPVSDriver) delService(service Service) error {
	if driver.ipvsClient == nil {
		return nil
	} else {
		return driver.ipvsClient.DelService(service.Service)
	}
}

func (driver *IPVSDriver) newServiceDest(service Service, dest Dest) error {
	if driver.ipvsClient == nil {
		return nil
	} else {
		return driver.ipvsClient.NewDest(service.Service, dest.Dest)
	}
}

func (driver *IPVSDriver) setServiceDest(service Service, dest Dest) error {
	if driver.ipvsClient == nil {
		return nil
	} else {
		return driver.ipvsClient.SetDest(service.Service, dest.Dest)
	}
}

func (driver *IPVSDriver) delServiceDest(service Service, dest Dest) error {
	if driver.ipvsClient == nil {
		return nil
	} else {
		return driver.ipvsClient.DelDest(service.Service, dest.Dest)
	}
}

// Apply new state
func (driver *IPVSDriver) update(routes Routes, services Services) error {
	for serviceName, service := range services {
		oldService, exists := driver.services[serviceName]

		if !exists {
			driver.newService(service)
		} else {
			driver.setService(service)
		}

		for destName, dest := range service.dests {
			if _, exists := oldService.dests[destName]; !exists {
				driver.newServiceDest(service, dest)
			} else {
				driver.setServiceDest(service, dest)
			}
		}

		for destName, oldDest := range oldService.dests {
			if _, exists := service.dests[destName]; !exists {
				driver.delServiceDest(service, oldDest)
			}
		}
	}

	for serviceName, oldService := range driver.services {
		if _, exists := services[serviceName]; !exists {
			// removing a service also removes all service.dests
			driver.delService(oldService)
		}
	}

	driver.routes = routes
	driver.services = services

	return nil
}

// Update state from config
func (driver *IPVSDriver) config(config config.Config) error {
	// routes
	routes, err := configRoutes(config.Routes)
	if err != nil {
		return err
	}

	// services
	services, err := configServices(config.Services, routes, driver.options)
	if err != nil {
		return err
	}

	return driver.update(routes, services)
}

func (driver *IPVSDriver) Print() {
	fmt.Printf("Proto                           Addr:Port\n")
	for _, service := range driver.services {
		fmt.Printf("%-5v %30s:%-5d %s\n",
			service.Protocol,
			service.Addr, service.Port,
			service.SchedName,
		)

		for _, dest := range service.dests {
			fmt.Printf("%5s %30s:%-5d %v\n",
				"",
				dest.Addr, dest.Port,
				dest.FwdMethod,
			)
		}
	}
}
