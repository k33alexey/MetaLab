// Package appconfig loads the shared settings used by all MetaLab modes.
package appconfig

import (
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"

	"github.com/k33alexey/MetaLab/internal/postgresconn"
	"go.yaml.in/yaml/v3"
)

const Version = 1

// Config is the versioned settings shared by Manager, Service and CLI.
type Config struct {
	Version        int                      `yaml:"version"`
	Language       string                   `yaml:"language"`
	Service        ServiceConfig            `yaml:"service"`
	Backups        BackupConfig             `yaml:"backups,omitempty"`
	SystemDatabase *postgresconn.Descriptor `yaml:"system_database,omitempty"`
	SourcePath     string                   `yaml:"-"`
}

// ServiceConfig contains non-secret ML Service settings.
type ServiceConfig struct {
	Listen string `yaml:"listen"`
}

// BackupConfig controls the local archive directory. Scheduling is added with the job engine.
type BackupConfig struct {
	Directory string `yaml:"directory,omitempty"`
}

// Default returns safe local settings for a first run.
func Default() Config {
	return Config{
		Version: Version, Language: "ru",
		Service: ServiceConfig{Listen: "127.0.0.1:8090"},
	}
}

// DefaultPath returns the platform-specific MetaLab settings path.
func DefaultPath() (string, error) {
	directory, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("resolve user settings directory: %w", err)
	}
	return filepath.Join(directory, "MetaLab", "config.yaml"), nil
}

// Load reads a strict YAML file and applies supported environment overrides.
// An empty path uses an optional platform default; an explicit path is required.
func Load(path string) (Config, string, error) {
	settings := Default()
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
			applyEnvironment(&settings)
			settings.SourcePath = path
			return settings, path, settings.Validate()
		}
		return Config{}, path, fmt.Errorf("open settings %q: %w", path, err)
	}
	defer file.Close()

	decoder := yaml.NewDecoder(file)
	decoder.KnownFields(true)
	if err := decoder.Decode(&settings); err != nil {
		return Config{}, path, fmt.Errorf("decode settings %q: %w", path, err)
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		if err == nil {
			return Config{}, path, fmt.Errorf("settings %q contains multiple YAML documents", path)
		}
		return Config{}, path, fmt.Errorf("decode settings %q: %w", path, err)
	}
	applyEnvironment(&settings)
	settings.SourcePath = path
	if err := settings.Validate(); err != nil {
		return Config{}, path, fmt.Errorf("validate settings %q: %w", path, err)
	}
	return settings, path, nil
}

// Validate checks the shared settings contract.
func (settings Config) Validate() error {
	if settings.Version != Version {
		return fmt.Errorf("unsupported settings version %d", settings.Version)
	}
	switch settings.Language {
	case "ru", "uk", "en":
	default:
		return fmt.Errorf("unsupported language %q", settings.Language)
	}
	if _, _, err := net.SplitHostPort(settings.Service.Listen); err != nil {
		return fmt.Errorf("invalid service listen address %q: %w", settings.Service.Listen, err)
	}
	if settings.SystemDatabase != nil {
		if err := settings.SystemDatabase.Validate(); err != nil {
			return fmt.Errorf("invalid ML System database: %w", err)
		}
	}
	if settings.Backups.Directory != "" && !filepath.IsAbs(settings.Backups.Directory) {
		return fmt.Errorf("backup directory must be absolute")
	}
	return nil
}

// BackupDirectory resolves the configured or platform-default local archive directory.
func (settings Config) BackupDirectory() (string, error) {
	if settings.Backups.Directory != "" {
		return settings.Backups.Directory, nil
	}
	directory, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("resolve backup directory: %w", err)
	}
	return filepath.Join(directory, "MetaLab", "backups"), nil
}

// Save writes a validated settings atomically with owner-only permissions.
func Save(path string, settings Config) error {
	if err := settings.Validate(); err != nil {
		return fmt.Errorf("validate settings before save: %w", err)
	}
	directory := filepath.Dir(path)
	if err := os.MkdirAll(directory, 0o700); err != nil {
		return fmt.Errorf("create settings directory: %w", err)
	}
	temporary, err := os.CreateTemp(directory, ".config-*.yaml")
	if err != nil {
		return fmt.Errorf("create temporary settings: %w", err)
	}
	temporaryPath := temporary.Name()
	defer func() { _ = os.Remove(temporaryPath) }()
	if err := temporary.Chmod(0o600); err != nil {
		_ = temporary.Close()
		return fmt.Errorf("protect temporary settings: %w", err)
	}
	encoder := yaml.NewEncoder(temporary)
	encoder.SetIndent(2)
	if err := encoder.Encode(settings); err != nil {
		_ = temporary.Close()
		return fmt.Errorf("encode settings: %w", err)
	}
	if err := temporary.Sync(); err != nil {
		_ = temporary.Close()
		return fmt.Errorf("sync settings: %w", err)
	}
	if err := temporary.Close(); err != nil {
		return fmt.Errorf("close settings: %w", err)
	}
	if err := replaceFile(temporaryPath, path); err != nil {
		return fmt.Errorf("replace settings: %w", err)
	}
	return nil
}

// LocalServiceURL returns the URL that a local Manager can open.
func (settings Config) LocalServiceURL() string {
	host, port, _ := net.SplitHostPort(settings.Service.Listen)
	host = strings.Trim(host, "[]")
	if host == "" || host == "0.0.0.0" || host == "::" {
		host = "127.0.0.1"
	}
	return "http://" + net.JoinHostPort(host, port)
}

func applyEnvironment(settings *Config) {
	if language := os.Getenv("ML_LANGUAGE"); language != "" {
		settings.Language = language
	}
	if listen := os.Getenv("ML_SERVICE_LISTEN"); listen != "" {
		settings.Service.Listen = listen
	}
	if directory := os.Getenv("ML_BACKUP_DIRECTORY"); directory != "" {
		settings.Backups.Directory = directory
	}
}
