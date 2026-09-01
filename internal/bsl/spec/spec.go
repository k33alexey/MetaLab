// Package spec exposes the versioned BSL syntax contract used by MetaLab.
package spec

import (
	"embed"
	"fmt"

	"go.yaml.in/yaml/v3"
)

//go:embed syntax.yaml corpus/*.bsl
var files embed.FS

// Catalog is the machine-readable BSL syntax contract.
type Catalog struct {
	Version  int       `yaml:"version"`
	Keywords []Keyword `yaml:"keywords"`
	Features []Feature `yaml:"features"`
}

// Keyword is one canonical Russian/English BSL keyword pair.
type Keyword struct {
	ID       string `yaml:"id"`
	Russian  string `yaml:"ru"`
	English  string `yaml:"en"`
	Category string `yaml:"category"`
}

// Feature describes one independently testable part of BSL syntax.
type Feature struct {
	ID       string   `yaml:"id"`
	Category string   `yaml:"category"`
	Title    string   `yaml:"title"`
	Corpus   []string `yaml:"corpus"`
}

// Load returns the embedded syntax catalog.
func Load() (Catalog, error) {
	data, err := files.ReadFile("syntax.yaml")
	if err != nil {
		return Catalog{}, fmt.Errorf("read syntax catalog: %w", err)
	}

	var catalog Catalog
	if err := yaml.Unmarshal(data, &catalog); err != nil {
		return Catalog{}, fmt.Errorf("decode syntax catalog: %w", err)
	}
	return catalog, nil
}

// Corpus returns one embedded BSL conformance source.
func Corpus(name string) ([]byte, error) {
	data, err := files.ReadFile("corpus/" + name)
	if err != nil {
		return nil, fmt.Errorf("read BSL corpus %q: %w", name, err)
	}
	return data, nil
}
