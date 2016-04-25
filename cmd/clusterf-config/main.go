package main

import (
	"github.com/qmsk/clusterf/config"
	"github.com/jessevdk/go-flags"
	"fmt"
	"encoding/json"
	"log"
	"os"
)

var Options struct {
	Listen			bool	`long:"listen" help:"Listen for updates"`
	JSON			bool	`long:"json" help:"Output JSON"`

	ConfigReader	config.ReaderOptions	`group:"Config Reader"`
}

var flagsParser = flags.NewParser(&Options,  flags.Default)

func printFrontend (frontend config.ServiceFrontend) {
	if frontend.IPv4 != "" {
		fmt.Printf(" ipv4=%v", frontend.IPv4)
	}
	if frontend.IPv6 != "" {
		fmt.Printf(" ipv6=%v", frontend.IPv6)
	}
	if frontend.TCP != 0 {
		fmt.Printf(" tcp=%v", frontend.TCP)
	}
	if frontend.UDP != 0 {
		fmt.Printf(" udp=%v", frontend.UDP)
	}
}
func printBackend (backend config.ServiceBackend) {
	if backend.IPv4 != "" {
		fmt.Printf(" ipv4=%v", backend.IPv4)
	}
	if backend.IPv6 != "" {
		fmt.Printf(" ipv6=%v", backend.IPv6)
	}
	if backend.TCP != 0 {
		fmt.Printf(" tcp=%v", backend.TCP)
	}
	if backend.UDP != 0 {
		fmt.Printf(" udp=%v", backend.UDP)
	}
}

func outputConfig (config config.Config) {
	if Options.JSON {
		if err := json.NewEncoder(os.Stdout).Encode(config); err != nil {
			log.Fatalf("json.Encode: %v\n", err)
		}
	} else {
		fmt.Printf("Routes:\n")
		for routeName, route := range config.Routes {
			fmt.Printf("\t%s: %v %v", routeName, route.IpvsMethod, route.Prefix4)
			if route.Gateway4 != "" {
				fmt.Printf(" gateway %v", route.Gateway4)
			}
			fmt.Printf("\n")
		}

		fmt.Printf("Services:\n")
		for serviceName, service := range config.Services {
			fmt.Printf("\t%s:", serviceName)
			if service.Frontend != nil {
				printFrontend(*service.Frontend)
			}
			fmt.Printf("\n")

			for backendName, backend := range service.Backends {
				fmt.Printf("\t\t%s:", backendName)
				printBackend(backend)
				fmt.Printf("\n")
			}
		}
	}
}

func main() {
	if _, err := flagsParser.Parse(); err != nil {
		log.Fatalf("flags.Parser.Parse: %v\n", err)
	}

	configReader, err := Options.ConfigReader.Reader()
	if err != nil {
		log.Fatalf("config.Reader: %v\n", err)
	}

	if Options.Listen {
		for config := range configReader.Listen() {
			outputConfig(config)
		}
	} else {
		config := configReader.Get()

		outputConfig(config)
	}
}
