package clusterf

import (
    "github.com/qmsk/clusterf/config"
    "fmt"
    "github.com/qmsk/clusterf/ipvs"
    "log"
    "syscall"
)

type IPVSOptions struct {
	Debug			bool		`long:"ipvs-debug"`

	FwdMethod		ipvs.FwdMethod	`long:"ipvs-fwd-method" default:"masq"`
	SchedName		string			`long:"ipvs-sched-name" default:"wlc"`

	Elide			bool			`long:"ipvs-elide" help:"Omit services with no backends"`

	Mock			bool			`long:"ipvs-mock" default:"false" help:"Do not connect to the kernel IPVS state"`
	Noop			bool			`long:"ipvs-noop" default:"false" help:"Do not write to the kernel IPVS state"`
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
    readClient	*ipvs.Client
	writeClient	*ipvs.Client

	// running state
	routes		Routes
	services	Services
}

func (driver *IPVSDriver) init(options IPVSOptions) error {
	driver.options = options

    if options.Mock {

    } else if ipvsClient, err := ipvs.Open(); err != nil {
        return err
    } else {
		if options.Debug {
			ipvsClient.SetDebug()
		}

        driver.readClient = ipvsClient
    }

	if options.Noop {

	} else {
		driver.writeClient = driver.readClient
	}


    if driver.readClient == nil {
        // mock'd
    } else if info, err := driver.readClient.GetInfo(); err != nil {
        return err
    } else {
        log.Printf("ipvs.GetInfo: version=%s, conn_tab_size=%d\n", info.Version, info.ConnTabSize)
    }

    return nil
}

// Reset running state in kernel
func (driver *IPVSDriver) Flush() error {
	if driver.writeClient == nil {

	} else if err := driver.writeClient.Flush(); err != nil {
		return err
	}

	driver.services = nil

	return nil
}

// Sync running state from kernel
func (driver *IPVSDriver) Sync() error {
	services := make(Services)

	if driver.readClient == nil {
		return fmt.Errorf("Cannot sync against a mock'd ipvs.Client")
	} else if ipvsServices, err := driver.readClient.ListServices(); err != nil {
		return fmt.Errorf("ipvs.ListServices: %v\n", err)
	} else {
		for _, ipvsService := range ipvsServices {
			if dests, err := driver.readClient.ListDests(ipvsService); err != nil {
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
	log.Printf("IPVS: New service %v\n", service)

	if driver.writeClient == nil {
		return nil
	} else {
		return driver.writeClient.NewService(service.Service)
	}
}

func (driver *IPVSDriver) setService(service Service) error {
	log.Printf("IPVS: Set service %v\n", service)

	if driver.writeClient == nil {
		return nil
	} else {
		return driver.writeClient.SetService(service.Service)
	}
}

func (driver *IPVSDriver) delService(service Service) error {
	log.Printf("IPVS: Delete service %v\n", service)

	if driver.writeClient == nil {
		return nil
	} else {
		return driver.writeClient.DelService(service.Service)
	}
}

func (driver *IPVSDriver) newServiceDest(service Service, dest Dest) error {
	log.Printf("IPVS: New service %v dest %v\n", service, dest)

	if driver.writeClient == nil {
		return nil
	} else {
		return driver.writeClient.NewDest(service.Service, dest.Dest)
	}
}

func (driver *IPVSDriver) setServiceDest(service Service, dest Dest) error {
	log.Printf("IPVS: Set service %v dest %v\n", service, dest)

	if driver.writeClient == nil {
		return nil
	} else {
		return driver.writeClient.SetDest(service.Service, dest.Dest)
	}
}

func (driver *IPVSDriver) delServiceDest(service Service, dest Dest) error {
	log.Printf("IPVS: Delete service %v dest %v\n", service, dest)

	if driver.writeClient == nil {
		return nil
	} else {
		return driver.writeClient.DelDest(service.Service, dest.Dest)
	}
}

// Apply new state
func (driver *IPVSDriver) update(routes Routes, services Services) error {
	for serviceName, service := range services {
		oldService, exists := driver.services[serviceName]

		if !exists {
			driver.newService(service)
		} else if !service.Equals(oldService.Service) {
			driver.setService(service)
		}

		for destName, dest := range service.dests {
			oldDest, exists := oldService.dests[destName]

			if !exists {
				driver.newServiceDest(service, dest)
			} else if !dest.Equals(oldDest.Dest) {
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
func (driver *IPVSDriver) Config(config config.Config) error {
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
