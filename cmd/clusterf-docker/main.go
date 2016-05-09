package main

import (
    "github.com/qmsk/clusterf/config"
    "github.com/qmsk/clusterf/docker"
	"github.com/jessevdk/go-flags"
    "log"
	"os"
	"os/signal"
	"syscall"
)

var Options struct {
	ConfigWriter	config.WriterOptions	`group:"Config Writer"`
	Docker			docker.Options

	ExitFlush		bool	`long:"exit-flush" help:"Flush backends on exit signal" default:"true"`

	RouteNetwork	string  `long:"route-network" help:"Advertise docker network by name"`
	RouteGateway4	string	`long:"route-gateway4" help:"Advertise docker network routes with IPv4 gateway"`
	RouteGateway6	string	`long:"route-gateway6" help:"Advertise docker network routes with IPv6 gateway"`
	RouteIPVSMethod string	`long:"route-ipvs-method" help:"Advertise docker network routes with ipvs-method"`
}

var flagsParser = flags.NewParser(&Options,  flags.Default)

// Flush service backends when stopping
func stop(configWriter *config.Writer) {
	log.Printf("Flush.....")

	if err := configWriter.Flush(); err != nil {
		log.Fatalf("config:Writer.Flush: %v", err)
	} else {
		log.Printf("Flushed")
	}
}

// Listen for updated docker.State, compile to config.Config and update config.Writer.
//
// Stops on os.Signal
func run(configWriter *config.Writer, dockerListen chan docker.State, stopChan chan os.Signal) {
	defer stop(configWriter)

	for {
		select {
		case dockerState, ok := <-dockerListen:
			if !ok {
				// docker quit? exit and restart
				return
			}

			if config, err := makeConfig(dockerState); err != nil {
				log.Fatalf("configContainers: %v", err)
			} else if err := configWriter.Write(config); err != nil {
				log.Fatalf("config:Writer.Write: %v", err)
			} else {
				log.Printf("Update config...")
			}

		case s := <-stopChan:
			log.Printf("Stopping on %v...", s)

			// reset signal in case stopping gets stuck
			signal.Stop(stopChan)

			return
		}
	}

	log.Printf("Stop...")
}

func main() {
	if _, err := flagsParser.Parse(); err != nil {
		log.Fatalf("flags.Parser.Parse: %v", err)
	}

	configWriter, err := Options.ConfigWriter.Writer()
	if err != nil {
		log.Fatalf("config.Writer: %v", err)
	}

    docker, err := Options.Docker.Open()
	if err != nil {
        log.Fatalf("docker:Docker.Open: %v", err)
    } else {
        log.Printf("docker:Docker.Open: %v", docker)
    }

    dockerChan, err := docker.Listen()
	if err != nil {
        log.Fatalf("docker:Docker.Listen: %v", err)
    } else {
        log.Printf("docker:Docker.Listen...")
	}

	// optionally arrange to stop on signal
	var stopChan chan os.Signal

	if Options.ExitFlush {
		stopChan = make(chan os.Signal)

		signal.Notify(stopChan, syscall.SIGINT, syscall.SIGTERM)
	}

	// mainloop
	run(configWriter, dockerChan, stopChan)
}
