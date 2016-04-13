package config

// Local configuration from the filesystem

import (
    "path/filepath"
    "fmt"
    "io/ioutil"
    "os"
    "strings"
)

type FilesOptions struct {
    Path        string
}

func (options FilesOptions) Files() (*Files, error) {
    files := Files{
        options:   options,
    }

    return &files, nil
}

type Files struct {
    options FilesOptions
}

func (self *Files) String() string {
    return fmt.Sprintf("file://%s", self.options.Path)
}

// Recursively any Config's under given path
func (self *Files) Scan(config *Config) error {
    return filepath.Walk(self.options.Path, func(path string, info os.FileInfo, err error) error {
        if err != nil {
            return err
        }

        if strings.HasPrefix(info.Name(), ".") {
            // skip
            return nil
        }

        node := Node{
            Path:   strings.Trim(strings.TrimPrefix(path, self.options.Path), "/"),
            IsDir:  info.IsDir(),
            Source: self,
        }

        if info.Mode().IsRegular() {
            if value, err := ioutil.ReadFile(path); err != nil {
                return err
            } else {
                node.Value = string(value)
            }
        }

        return config.update(node)
    })
}
