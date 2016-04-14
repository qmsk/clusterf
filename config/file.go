package config

// Local configuration from the filesystem

import (
    "path/filepath"
    "fmt"
    "io/ioutil"
    "os"
    "strings"
	"net/url"
)

func openFileSource(url *url.URL) (*FileSource, error) {
	fileOptions := FileOptions{
		Path:	url.Path,
	}

	return fileOptions.Open()
}

type FileOptions struct {
    Path        string
}

func (options FileOptions) Open() (*FileSource, error) {
    fileSource := FileSource{
        options:   options,
    }

    return &fileSource, nil
}

type FileSource struct {
    options FileOptions
}

func (fs *FileSource) String() string {
    return fmt.Sprintf("file://%s", fs.options.Path)
}

// Recursively any Config's under given path
func (fs *FileSource) Scan() (nodes []Node, err error) {
    err = filepath.Walk(fs.options.Path, func(path string, info os.FileInfo, err error) error {
        if err != nil {
            return err
        }

        if strings.HasPrefix(info.Name(), ".") {
            // skip
            return nil
        }

        node := Node{
            Path:   strings.Trim(strings.TrimPrefix(path, fs.options.Path), "/"),
            IsDir:  info.IsDir(),
            Source: fs,
        }

        if info.Mode().IsRegular() {
            if value, err := ioutil.ReadFile(path); err != nil {
                return err
            } else {
                node.Value = string(value)
            }
        }

		nodes = append(nodes, node)

		return nil
    })

	return
}