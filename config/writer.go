package config

import (
	"fmt"
)

type WriterOptions struct {
	SourceOptions
	SourceURL string `long:"config-source" value-name:"(file|etcd|etcd+http|etcd+https)://[<host>]/<path>" description:"Write to given source"`
}

func (options WriterOptions) Writer() (*Writer, error) {
	writer := Writer{
		options: options,
	}

	if err := writer.open(options.SourceURL); err != nil {
		return nil, err
	}

	return &writer, nil
}

type Writer struct {
	options WriterOptions
	source  writeSource
}

func (writer *Writer) open(sourceURL string) error {
	if source, err := writer.options.SourceOptions.openURL(sourceURL); err != nil {
		return err
	} else if writeSource, ok := source.(writeSource); !ok {
		return fmt.Errorf("Config source is not writeable: %v", source)
	} else {
		writer.source = writeSource
	}

	return nil
}

// Publish config as nodes to our source
func (writer *Writer) Write(config Config) error {
	if nodes, err := config.compile(); err != nil {
		return err
	} else {
		return writer.source.Write(nodes)
	}
}

// Stop publishing
func (writer *Writer) Flush() error {
	return writer.source.Flush()
}
