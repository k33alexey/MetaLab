// Package appconfig loads the shared configuration used by all MetaLab modes.
package appconfig

import (
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"

	"go.yaml.in/yaml/v3"
)

const Version = 1

// Config is the versioned configuration shared by Manager, Service and CLI.
type Config struct {
	Version  int           `yaml:"version"`
	Language string        `yaml:"language"`
	Service  ServiceConfig `yaml:"service"`
}

// ServiceConfig contains non-secret ML Service settings.
type ServiceConfig struct {
	Listen string `yaml:"listen"`
}

// Default returns safe local settings for a first run.
func Default() Config {
	return Config{
		Version: Version, Language: "ru",
		Service: ServiceConfig{Listen: "127.0.0.1:8090"},
	}
}

// DefaultPath returns the platform-specific MetaLab configuration path.
func DefaultPath() (string, error) {
	directory, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("resolve user configuration directory: %w", err)
	}
	return filepath.Join(directory, "MetaLab", "config.yaml"), nil
}

// Load reads a strict YAML file and applies supported environment overrides.
// An empty path uses an optional platform default; an explicit path is required.
func Load(path string) (Config, string, error) {
	configuration := Default()
	explicit := path != ""
	if !explicit {
		var err error
		path, err = DefaultPath()
		if err != nil {
			return Config{}, "", err
		}
	}
	file, err := os.Open(path)
	if err != nil {
		if !explicit && errors.Is(err, os.ErrNotExist) {
			applyEnvironment(&configuration)
			return configuration, path, configuration.Validate()
		}
		return Config{}, path, fmt.Errorf("open configuration %q: %w", path, err)
	}
	defer file.Close()

	decoder := yaml.NewDecoder(file)
	decoder.KnownFields(true)
	if err := decoder.Decode(&configuration); err != nil {
		return Config{}, path, fmt.Errorf("decode configuration %q: %w", path, err)
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		if err == nil {
			return Config{}, path, fmt.Errorf("configuration %q contains multiple YAML documents", path)
		}
		return Config{}, path, fmt.Errorf("decode configuration %q: %w", path, err)
	}
	applyEnvironment(&configuration)
	if err := configuration.Validate(); err != nil {
		return Config{}, path, fmt.Errorf("validate configuration %q: %w", path, err)
	}
	return configuration, path, nil
}

// Validate checks the shared configuration contract.
func (configuration Config) Validate() error {
	if configuration.Version != Version {
		return fmt.Errorf("unsupported configuration version %d", configuration.Version)
	}
	switch configuration.Language {
	case "ru", "uk", "en":
	default:
		return fmt.Errorf("unsupported language %q", configuration.Language)
	}
	if _, _, err := net.SplitHostPort(configuration.Service.Listen); err != nil {
		return fmt.Errorf("invalid service listen address %q: %w", configuration.Service.Listen, err)
	}
	return nil
}

// LocalServiceURL returns the URL that a local Manager can open.
func (configuration Config) LocalServiceURL() string {
	host, port, _ := net.SplitHostPort(configuration.Service.Listen)
	host = strings.Trim(host, "[]")
	if host == "" || host == "0.0.0.0" || host == "::" {
		host = "127.0.0.1"
	}
	return "http://" + net.JoinHostPort(host, port)
}

func applyEnvironment(configuration *Config) {
	if language := os.Getenv("ML_LANGUAGE"); language != "" {
		configuration.Language = language
	}
	if listen := os.Getenv("ML_SERVICE_LISTEN"); listen != "" {
		configuration.Service.Listen = listen
	}
}
