package appconfig

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadStrictConfiguration(t *testing.T) {
	directory := t.TempDir()
	path := filepath.Join(directory, "config.yaml")
	writeFile(t, path, "version: 1\nlanguage: uk\nservice:\n  listen: 0.0.0.0:9000\n")
	configuration, loadedPath, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if loadedPath != path || configuration.Language != "uk" || configuration.Service.Listen != "0.0.0.0:9000" {
		t.Fatalf("configuration = %+v, path = %q", configuration, loadedPath)
	}
	if got := configuration.LocalServiceURL(); got != "http://127.0.0.1:9000" {
		t.Fatalf("LocalServiceURL() = %q", got)
	}
}

func TestLoadAppliesEnvironmentOverrides(t *testing.T) {
	t.Setenv("ML_LANGUAGE", "en")
	t.Setenv("ML_SERVICE_LISTEN", "127.0.0.1:9100")
	path := filepath.Join(t.TempDir(), "config.yaml")
	writeFile(t, path, "version: 1\nlanguage: ru\nservice:\n  listen: 127.0.0.1:8090\n")
	configuration, _, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if configuration.Language != "en" || configuration.Service.Listen != "127.0.0.1:9100" {
		t.Fatalf("configuration = %+v", configuration)
	}
}

func TestLoadRejectsInvalidConfiguration(t *testing.T) {
	tests := map[string]string{
		"unknown field":      "version: 1\nlanguage: ru\nunknown: true\n",
		"version":            "version: 2\nlanguage: ru\nservice: {listen: '127.0.0.1:8090'}\n",
		"language":           "version: 1\nlanguage: de\nservice: {listen: '127.0.0.1:8090'}\n",
		"address":            "version: 1\nlanguage: ru\nservice: {listen: invalid}\n",
		"multiple documents": "version: 1\nlanguage: ru\nservice: {listen: '127.0.0.1:8090'}\n---\n{}\n",
	}
	for name, content := range tests {
		t.Run(name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "config.yaml")
			writeFile(t, path, content)
			if _, _, err := Load(path); err == nil {
				t.Fatal("Load() accepted invalid configuration")
			}
		})
	}
}

func TestLoadRequiresExplicitPath(t *testing.T) {
	path := filepath.Join(t.TempDir(), "missing.yaml")
	if _, _, err := Load(path); err == nil || !strings.Contains(err.Error(), "open configuration") {
		t.Fatalf("Load() error = %v", err)
	}
}

func TestDefaultConfigurationIsValid(t *testing.T) {
	t.Parallel()

	configuration := Default()
	if err := configuration.Validate(); err != nil {
		t.Fatal(err)
	}
	if configuration.LocalServiceURL() != "http://127.0.0.1:8090" {
		t.Fatalf("LocalServiceURL() = %q", configuration.LocalServiceURL())
	}
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}
