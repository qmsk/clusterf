package main

import (
    "github.com/qmsk/clusterf/config"
    "github.com/qmsk/clusterf"
	"github.com/jessevdk/go-flags"
    "log"
)

var Options struct {
	ConfigReader	config.ReaderOptions	`group:"Config Reader"`
	IPVS			clusterf.IPVSOptions	`group:"IPVS"`

	Flush			bool		`long:"flush" help:"Flush all IPVS services before applying configuration"`
	Print			bool		`long:"print" help:"Output all IPVS rules after applying configuration"`
}

var flagsParser = flags.NewParser(&Options,  flags.Default)

func main() {
	if args, err := flagsParser.Parse(); err != nil {
		log.Fatalf("flags.Parser.Parse: %v\n", err)
	} else if len(args) > 0 {
		log.Fatalf("Extra arguments: %v\n", args)
	}

	configReader, err := Options.ConfigReader.Reader()
	if err != nil {
		log.Fatalf("config.Reader: %v\n", err)
	}

    // setup
	ipvsDriver, err := Options.IPVS.Open()
	if err != nil {
		log.Fatalf("IPVSOptions.Open: %v\n", err)
	}

    // sync
	if Options.Flush {
		if err := ipvsDriver.Flush(); err != nil {
			log.Fatalf("IPVSDriver.Flush: %v\n", err)
		}
	} else {
		if err := ipvsDriver.Sync(); err != nil {
			log.Fatalf("IPVSDriver.Sync: %v\n", err)
		}
	}

	if Options.Print {
		ipvsDriver.Print()
	}

	// configure
	log.Printf("Configure...\n")

	for config := range configReader.Listen() {
		if err := ipvsDriver.Config(config); err != nil {
			log.Fatalf("IPVSDriver.Config: %v\n\tconfig=%#v\n", err, config)
		}

		if Options.Print {
			ipvsDriver.Print()
		}
	}

    log.Printf("Exit\n")
}
