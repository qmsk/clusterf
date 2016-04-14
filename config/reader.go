package config

import (
	"fmt"
	"net/url"
)

/*
 * Configuration sources: where the config is coming from
 */
type Source interface {
    // uniquely identifying
    String()    string
}

type readerSource interface {
	Source

	Scan(config *Config) error
}

// Read and combine a Config from multiple Sources
type Reader struct {
	config Config
}

func (reader *Reader) open(readerSource readerSource) error {
	return readerSource.Scan(&reader.config)
}

func (reader *Reader) openURL(url *url.URL) error {
	switch url.Scheme {
	case "file":
		fileOptions := FileOptions{
			Path:	url.Path,
		}

		if readerSource, err := fileOptions.Open(); err != nil {
			return err
		} else {
			return reader.open(readerSource)
		}
	default:
		return fmt.Errorf("Invalid config URL Scheme=%v: %v\n", url.Scheme, url)
	}
}

func (reader *Reader) Open(urls ...string) error {
	for _, urlString := range urls {
		if url, err := url.Parse(urlString); err != nil {
			return err
		} else if err := reader.openURL(url); err != nil {
			return err
		}
	}

	return nil
}

func (reader *Reader) Get() Config {
	return reader.config
}
