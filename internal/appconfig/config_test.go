package appconfig

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/k33alexey/MetaLab/internal/postgresconn"
)

func TestLoadStrictConfiguration(t *testing.T) {
	directory := t.TempDir()
	path := filepath.Join(directory, "config.yaml")
	writeFile(t, path, "version: 1\nlanguage: uk\nservice:\n  listen: 0.0.0.0:9000\n")
	settings, loadedPath, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if loadedPath != path || settings.Language != "uk" || settings.Service.Listen != "0.0.0.0:9000" {
		t.Fatalf("settings = %+v, path = %q", settings, loadedPath)
	}
	if got := settings.LocalServiceURL(); got != "http://127.0.0.1:9000" {
		t.Fatalf("LocalServiceURL() = %q", got)
	}
}

func TestLoadAppliesEnvironmentOverrides(t *testing.T) {
	t.Setenv("ML_LANGUAGE", "en")
	t.Setenv("ML_SERVICE_LISTEN", "127.0.0.1:9100")
	path := filepath.Join(t.TempDir(), "config.yaml")
	writeFile(t, path, "version: 1\nlanguage: ru\nservice:\n  listen: 127.0.0.1:8090\n")
	settings, _, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if settings.Language != "en" || settings.Service.Listen != "127.0.0.1:9100" {
		t.Fatalf("settings = %+v", settings)
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
				t.Fatal("Load() accepted invalid settings")
			}
		})
	}
}

func TestLoadRequiresExplicitPath(t *testing.T) {
	path := filepath.Join(t.TempDir(), "missing.yaml")
	if _, _, err := Load(path); err == nil || !strings.Contains(err.Error(), "open settings") {
		t.Fatalf("Load() error = %v", err)
	}
}

func TestDefaultConfigurationIsValid(t *testing.T) {
	t.Parallel()

	settings := Default()
	if err := settings.Validate(); err != nil {
		t.Fatal(err)
	}
	if settings.LocalServiceURL() != "http://127.0.0.1:8090" {
		t.Fatalf("LocalServiceURL() = %q", settings.LocalServiceURL())
	}
}

func TestBackupDirectoryRequiresAbsolutePathAndSupportsEnvironment(t *testing.T) {
	directory := t.TempDir()
	t.Setenv("ML_BACKUP_DIRECTORY", directory)
	path := filepath.Join(t.TempDir(), "config.yaml")
	writeFile(t, path, "version: 1\nlanguage: ru\nservice: {listen: '127.0.0.1:8090'}\n")
	settings, _, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	resolved, err := settings.BackupDirectory()
	if err != nil || resolved != directory {
		t.Fatalf("backup directory=%q error=%v", resolved, err)
	}
	settings.Backups.Directory = "relative"
	if err := settings.Validate(); err == nil {
		t.Fatal("relative backup directory was accepted")
	}
}

func TestSaveAndLoadSystemDatabaseWithoutPassword(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "nested", "config.yaml")
	settings := Default()
	settings.SystemDatabase = &postgresconn.Descriptor{
		Host: "db.example.test", Port: 5432, Database: "metalab_system", User: "metalab_service",
		SSLMode: "verify-full", SecretKey: postgresconn.DefaultSystemSecretKey,
	}
	if err := Save(path, settings); err != nil {
		t.Fatal(err)
	}
	loaded, _, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.SystemDatabase == nil || *loaded.SystemDatabase != *settings.SystemDatabase {
		t.Fatalf("loaded settings = %+v", loaded)
	}
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(content), "password:") {
		t.Fatalf("settings contains password field: %s", content)
	}
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

// The portal's address: the configured public one when there is one, and the
// listen address otherwise, with "every interface" turned into one anybody can
// actually open.
func TestPortalURLPrefersThePublicAddress(t *testing.T) {
	t.Parallel()
	for name, want := range map[string]struct {
		listen, public, url string
	}{
		"локальный адрес прослушивания":        {"127.0.0.1:8090", "", "http://127.0.0.1:8090"},
		"все интерфейсы — не адрес входа":      {"0.0.0.0:8090", "", "http://127.0.0.1:8090"},
		"за обратным прокси":                   {"127.0.0.1:8090", "https://ml.example.test", "https://ml.example.test"},
		"хвостовая косая черта не едет дальше": {"127.0.0.1:8090", "https://ml.example.test/", "https://ml.example.test"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			settings := Default()
			settings.Service.Listen, settings.Service.PublicURL = want.listen, want.public
			if address := settings.PortalURL(); address != want.url {
				t.Fatalf("%s: %q", name, address)
			}
		})
	}
}

// A public address that is not an address is refused at the door: the button
// would send the browser nowhere and say nothing.
func TestAnInvalidPublicAddressIsRefused(t *testing.T) {
	t.Parallel()
	for name, public := range map[string]string{
		"без схемы":      "ml.example.test",
		"чужая схема":    "ftp://ml.example.test",
		"только схема":   "https://",
		"не адрес вовсе": "://",
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			settings := Default()
			settings.Service.PublicURL = public
			if err := settings.Validate(); err == nil {
				t.Fatalf("%s: accepted %q", name, public)
			}
		})
	}
}
