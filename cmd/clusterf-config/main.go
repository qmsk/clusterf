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
	JSON	bool	`long:"json" help:"Output JSON"`
}

var flagsParser = flags.NewParser(&Options,  flags.Default)

func main() {
	var configReader config.Reader

	if args, err := flagsParser.Parse(); err != nil {
		log.Fatalf("flags.Parser.Parse: %v\n", err)
	} else if err := configReader.Open(args...); err != nil {
		log.Fatalf("config.Reader: Open: %v\n", err)
	}

	// out
	config := configReader.Get()

	if Options.JSON {
		if err := json.NewEncoder(os.Stdout).Encode(config); err != nil {
			log.Fatalf("json.Encode: %v\n", err)
		}
	} else {
		fmt.Printf("%#v\n", config)
	}
}
