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
	Listen	bool	`long:"listen" help:"Listen for updates"`
	JSON	bool	`long:"json" help:"Output JSON"`
}

var flagsParser = flags.NewParser(&Options,  flags.Default)

func outputConfig (config config.Config) {
	if Options.JSON {
		if err := json.NewEncoder(os.Stdout).Encode(config); err != nil {
			log.Fatalf("json.Encode: %v\n", err)
		}
	} else {
		fmt.Printf("\n")

		fmt.Printf("Routes:\n")
		for routeName, route := range config.Routes {
			fmt.Printf("\t%s: %v -> %v (%v) \n", routeName, route.Prefix4, route.Gateway4, route.IpvsMethod)
		}

		fmt.Printf("Services:\n")
		for serviceName, service := range config.Services {
			fmt.Printf("\t%s: ipv4=%v ipv6=%v tcp=%v udp=%v\n", serviceName, service.Frontend.IPv4, service.Frontend.IPv6, service.Frontend.TCP, service.Frontend.UDP)

			for backendName, backend := range service.Backends {
				fmt.Printf("\t\t%s: ipv4=%v ipv6=%v tcp=%v udp=%v\n", backendName, backend.IPv4, backend.IPv6, backend.TCP, backend.UDP)
			}
		}
	}
}

func main() {
	var configReader config.Reader

	if args, err := flagsParser.Parse(); err != nil {
		log.Fatalf("flags.Parser.Parse: %v\n", err)
	} else if err := configReader.Open(args...); err != nil {
		log.Fatalf("config.Reader: Open: %v\n", err)
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
