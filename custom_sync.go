package main

import (
	"bytes"
	"fmt"
	"gopkg.in/yaml.v3"
	"io"
	"log"
	"os"
	"path"
	"strings"
)

type customSync struct {
	config *Configurations
}

func (c customSync) Sync() {

	// var paths []string

	f, err := os.Open(path.Join(c.config.DotfilePath, "dotfile-config.yaml"))
	defer f.Close()

	if err != nil {
		Error(err.Error())
		return
	}

	var buf bytes.Buffer
	_, err = io.Copy(&buf, f)

	paths, err := parseYAMLToPaths(buf.String())
	if err != nil {
		log.Fatalf("Error parsing YAML: %v", err)
	}

	home, err := os.UserHomeDir()
	fmt.Println(home)

	for _, p := range paths {
		p = strings.ReplaceAll(p, "home", home)
		file, err := os.Stat(p)
		if err != nil {
			Error(err.Error())
			return
		}

		if file.IsDir() {
			Infoln(p, "is a directory")
		} else {
			Infoln(p, "is a file")
		}
	}
}

func (c customSync) Consume(consumers ...Consumer) {

}

func NewCustomSync(config *Configurations) Syncer {
	return customSync{config: config}
}

// Recursively parses YAML and generates file paths.
func parseYAMLToPaths(data string) ([]string, error) {
	var rawData map[string]interface{}
	if err := yaml.Unmarshal([]byte(data), &rawData); err != nil {
		return nil, err
	}

	var paths []string
	generatePaths(rawData, "", &paths)
	return paths, nil
}

// Recursive helper to process YAML nodes and form paths.
func generatePaths(node map[string]interface{}, currentPath string, paths *[]string) {
	for key, value := range node {
		// Update the current path with the map key
		newBasePath := path.Join(currentPath, key)

		switch val := value.(type) {
		case []interface{}:
			// Process list of files or directories
			for _, v := range val {
				switch item := v.(type) {
				case string:
					// Append file paths directly
					*paths = append(*paths, path.Join(newBasePath, item))
				case map[string]interface{}:
					// Recurse into nested maps
					generatePaths(item, newBasePath, paths)
				}
			}
		}
	}
}
