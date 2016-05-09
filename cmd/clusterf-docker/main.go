package main

import (
    "github.com/qmsk/clusterf/config"
    "github.com/qmsk/clusterf/docker"
	"github.com/jessevdk/go-flags"
    "log"
)

var Options struct {
	ConfigWriter	config.WriterOptions	`group:"Config Writer"`
	Docker			docker.Options
}

var flagsParser = flags.NewParser(&Options,  flags.Default)

func main() {
	if _, err := flagsParser.Parse(); err != nil {
		log.Fatalf("flags.Parser.Parse: %v\n", err)
	}

	configWriter, err := Options.ConfigWriter.Writer()
	if err != nil {
		log.Fatalf("config.Writer: %v\n", err)
	}

    docker, err := Options.Docker.Open()
	if err != nil {
        log.Fatalf("docker:Docker.Open: %v\n", err)
    } else {
        log.Printf("docker:Docker.Open: %v\n", docker)
    }

    if dockerChan, err := docker.Listen(); err != nil {
        log.Fatalf("docker:Docker.Listen: %v\n", err)
    } else {
        log.Printf("docker:Docker.Listen...\n")

        for dockerState := range dockerChan {
			if config, err := makeConfig(dockerState); err != nil {
				log.Fatalf("configContainers: %v\n", err)
			} else if err := configWriter.Write(config); err != nil {
				log.Fatalf("config:Writer.Write: %v\n", err)
			} else {
				log.Printf("Update config\n")
			}
        }
    }

	// cleanup
	log.Printf("Stop..")

	if err := configWriter.Close(); err != nil {
		log.Fatalf("config:Writer.Close: %v\n", err)
	}

	log.Printf("Exit")
}
