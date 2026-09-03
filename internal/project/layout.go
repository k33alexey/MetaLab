package project

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"

	"github.com/k33alexey/MetaLab/internal/uuid"
)

const (
	// ManifestFile is the single root manifest of an ML Project.
	ManifestFile = "mlproject.yaml"
	keepFile     = ".gitkeep"
)

var (
	// ErrProjectExists prevents initialization over any existing file or directory.
	ErrProjectExists = errors.New("ML Project path already exists")
	// ErrInvalidLayout indicates a missing, unsafe, or malformed project path.
	ErrInvalidLayout = errors.New("invalid ML Project layout")

	rootDirectories = []string{"metadata", "modules", "forms", "reports", "tests", "assets"}
	metadataKinds   = []string{
		"subsystems",
		"common-modules",
		"session-parameters",
		"roles",
		"common-attributes",
		"event-subscriptions",
		"scheduled-jobs",
		"defined-types",
		"common-commands",
		"common-forms",
		"common-templates",
		"common-pictures",
		"http-services",
		"styles",
		"languages",
		"constants",
		"settings-storages",
		"catalogs",
		"documents",
		"document-journals",
		"enumerations",
		"reports",
		"data-processors",
		"charts-of-characteristic-types",
		"charts-of-accounts",
		"information-registers",
		"accumulation-registers",
		"accounting-registers",
		"folders",
	}
)

// RootDirectories returns the canonical Git-tracked source directories.
func RootDirectories() []string { return slices.Clone(rootDirectories) }

// MetadataKinds returns the supported physical metadata directory names.
func MetadataKinds() []string { return slices.Clone(metadataKinds) }

// Initialize atomically creates a new canonical ML Project at a previously unused path.
func Initialize(root string, manifest Project) error {
	if err := manifest.Validate(); err != nil {
		return err
	}
	root, err := cleanRoot(root)
	if err != nil {
		return err
	}
	if _, err := os.Lstat(root); err == nil {
		return ErrProjectExists
	} else if !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("inspect ML Project target: %w", err)
	}

	parent := filepath.Dir(root)
	if err := os.MkdirAll(parent, 0o755); err != nil {
		return fmt.Errorf("create ML Project parent: %w", err)
	}
	staging, err := os.MkdirTemp(parent, "."+filepath.Base(root)+"-*")
	if err != nil {
		return fmt.Errorf("create ML Project staging directory: %w", err)
	}
	defer os.RemoveAll(staging)
	if err := os.Chmod(staging, 0o755); err != nil {
		return fmt.Errorf("set ML Project directory permissions: %w", err)
	}

	manifestPath := filepath.Join(staging, ManifestFile)
	file, err := os.OpenFile(manifestPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return fmt.Errorf("create ML Project manifest: %w", err)
	}
	if err := Encode(file, manifest); err != nil {
		_ = file.Close()
		return err
	}
	if err := file.Sync(); err != nil {
		_ = file.Close()
		return fmt.Errorf("sync ML Project manifest: %w", err)
	}
	if err := file.Close(); err != nil {
		return fmt.Errorf("close ML Project manifest: %w", err)
	}
	for _, directory := range rootDirectories {
		path := filepath.Join(staging, directory)
		if err := os.Mkdir(path, 0o755); err != nil {
			return fmt.Errorf("create ML Project directory %q: %w", directory, err)
		}
		if err := os.WriteFile(filepath.Join(path, keepFile), nil, 0o644); err != nil {
			return fmt.Errorf("create Git placeholder for %q: %w", directory, err)
		}
	}
	if err := os.Rename(staging, root); err != nil {
		if _, statErr := os.Lstat(root); statErr == nil {
			return ErrProjectExists
		}
		return fmt.Errorf("publish ML Project directory: %w", err)
	}
	return nil
}

// ValidateLayout verifies the safe root structure and the current manifest.
func ValidateLayout(root string) (Project, error) {
	root, err := cleanRoot(root)
	if err != nil {
		return Project{}, err
	}
	if err := requirePath(root, true); err != nil {
		return Project{}, err
	}
	manifestPath := filepath.Join(root, ManifestFile)
	if err := requirePath(manifestPath, false); err != nil {
		return Project{}, err
	}
	for _, directory := range rootDirectories {
		if err := requirePath(filepath.Join(root, directory), true); err != nil {
			return Project{}, err
		}
	}
	file, err := os.Open(manifestPath)
	if err != nil {
		return Project{}, fmt.Errorf("%w: open manifest: %v", ErrInvalidLayout, err)
	}
	defer file.Close()
	manifest, err := Decode(file)
	if err != nil {
		return Project{}, fmt.Errorf("%w: %v", ErrInvalidLayout, err)
	}
	return manifest, nil
}

// MetadataPath returns the canonical relative YAML path for a metadata object.
func MetadataPath(kind string, id uuid.UUID) (string, error) {
	if !slices.Contains(metadataKinds, kind) {
		return "", fmt.Errorf("unknown metadata kind %q", kind)
	}
	return sourcePath(path.Join("metadata", kind), id, ".yaml")
}

// ModulePath returns the canonical relative BSL path for a module.
func ModulePath(id uuid.UUID) (string, error) { return sourcePath("modules", id, ".bsl") }

// FormPath returns the canonical relative YAML path for a managed form.
func FormPath(id uuid.UUID) (string, error) { return sourcePath("forms", id, ".yaml") }

// ReportPath returns the canonical relative YAML path for a data composition schema.
func ReportPath(id uuid.UUID) (string, error) { return sourcePath("reports", id, ".yaml") }

// TestPath returns the canonical relative BSL path for a test module.
func TestPath(id uuid.UUID) (string, error) { return sourcePath("tests", id, ".bsl") }

// AssetPath returns a stable relative resource path while preserving its format extension.
func AssetPath(id uuid.UUID, extension string) (string, error) {
	if len(extension) < 2 || len(extension) > 17 || extension[0] != '.' || extension != strings.ToLower(extension) {
		return "", fmt.Errorf("asset extension must be lowercase and start with a dot")
	}
	for _, symbol := range extension[1:] {
		if (symbol < 'a' || symbol > 'z') && (symbol < '0' || symbol > '9') {
			return "", fmt.Errorf("asset extension contains an unsupported character")
		}
	}
	return sourcePath("assets", id, extension)
}

func sourcePath(directory string, id uuid.UUID, extension string) (string, error) {
	if id.IsZero() {
		return "", fmt.Errorf("source UUID must not be zero")
	}
	return path.Join(directory, id.String()+extension), nil
}

func cleanRoot(root string) (string, error) {
	if strings.TrimSpace(root) == "" {
		return "", fmt.Errorf("%w: project path is empty", ErrInvalidLayout)
	}
	absolute, err := filepath.Abs(root)
	if err != nil {
		return "", fmt.Errorf("%w: resolve project path: %v", ErrInvalidLayout, err)
	}
	return filepath.Clean(absolute), nil
}

func requirePath(path string, directory bool) error {
	info, err := os.Lstat(path)
	if err != nil {
		return fmt.Errorf("%w: inspect %q: %v", ErrInvalidLayout, path, err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("%w: %q must not be a symbolic link", ErrInvalidLayout, path)
	}
	if directory && !info.IsDir() {
		return fmt.Errorf("%w: %q must be a directory", ErrInvalidLayout, path)
	}
	if !directory && !info.Mode().IsRegular() {
		return fmt.Errorf("%w: %q must be a regular file", ErrInvalidLayout, path)
	}
	return nil
}
