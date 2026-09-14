package project

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"slices"
	"sort"
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
	// ErrProjectIdentityChanged prevents accidental replacement with another project manifest.
	ErrProjectIdentityChanged = errors.New("ML Project identity cannot be changed")

	rootDirectories = []string{"metadata", "modules", "forms", "reports", "tests", "assets"}
	// objectFolderKinds lists metadata kinds whose objects group their own
	// description, module(s) and managed forms under one folder named by
	// the object's stable UUID, instead of scattering them across the flat
	// modules/ and forms/ roots.
	objectFolderKinds = []string{"catalogs", "documents", "information-registers", "accumulation-registers"}
	metadataKinds     = []string{
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

// ObjectFolderKinds returns the metadata kinds whose objects use a
// per-object folder layout instead of a single flat YAML file.
func ObjectFolderKinds() []string { return slices.Clone(objectFolderKinds) }

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
	manifest, err := readManifest(manifestPath)
	if err != nil {
		return Project{}, fmt.Errorf("%w: %v", ErrInvalidLayout, err)
	}
	return manifest, nil
}

// SaveManifest atomically writes a validated manifest without allowing its stable UUID to change.
func SaveManifest(root string, manifest Project) error {
	if err := manifest.Validate(); err != nil {
		return err
	}
	root, err := cleanRoot(root)
	if err != nil {
		return err
	}
	current, err := ValidateLayout(root)
	if err != nil {
		return err
	}
	if current.ID != manifest.ID {
		return ErrProjectIdentityChanged
	}
	temporary, err := os.CreateTemp(root, ".mlproject-*.yaml")
	if err != nil {
		return fmt.Errorf("create temporary ML Project manifest: %w", err)
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)
	if err := temporary.Chmod(0o644); err != nil {
		_ = temporary.Close()
		return fmt.Errorf("set temporary manifest permissions: %w", err)
	}
	if err := Encode(temporary, manifest); err != nil {
		_ = temporary.Close()
		return err
	}
	if err := temporary.Sync(); err != nil {
		_ = temporary.Close()
		return fmt.Errorf("sync ML Project manifest: %w", err)
	}
	if err := temporary.Close(); err != nil {
		return fmt.Errorf("close ML Project manifest: %w", err)
	}
	if err := replaceProjectFile(temporaryPath, filepath.Join(root, ManifestFile)); err != nil {
		return fmt.Errorf("replace ML Project manifest: %w", err)
	}
	return nil
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

// ObjectDirectory returns the per-object folder for one of ObjectFolderKinds,
// named by the object's own stable UUID.
func ObjectDirectory(kind string, id uuid.UUID) (string, error) {
	if !slices.Contains(objectFolderKinds, kind) {
		return "", fmt.Errorf("kind %q does not use a per-object folder", kind)
	}
	if id.IsZero() {
		return "", fmt.Errorf("source UUID must not be zero")
	}
	return path.Join("metadata", kind, id.String()), nil
}

// ObjectMetadataPath returns the fixed-name description file inside an
// object's own folder.
func ObjectMetadataPath(kind string, id uuid.UUID) (string, error) {
	directory, err := ObjectDirectory(kind, id)
	if err != nil {
		return "", err
	}
	return path.Join(directory, "object.yaml"), nil
}

// ObjectModulePath returns one of an object's own BSL modules (object,
// manager or record-set module), named by its own stable module UUID.
func ObjectModulePath(kind string, objectID, moduleID uuid.UUID) (string, error) {
	directory, err := ObjectDirectory(kind, objectID)
	if err != nil {
		return "", err
	}
	if moduleID.IsZero() {
		return "", fmt.Errorf("source UUID must not be zero")
	}
	return path.Join(directory, moduleID.String()+".bsl"), nil
}

// ObjectFormPath returns one of an object's own managed forms (object, list
// or choice form), named by its own stable form UUID.
func ObjectFormPath(kind string, objectID, formID uuid.UUID) (string, error) {
	directory, err := ObjectDirectory(kind, objectID)
	if err != nil {
		return "", err
	}
	if formID.IsZero() {
		return "", fmt.Errorf("source UUID must not be zero")
	}
	return path.Join(directory, "forms", formID.String()+".yaml"), nil
}

// ObjectFolderSourcePaths walks every per-object folder (ObjectFolderKinds)
// and returns every real file it physically holds: the object's own
// description, its module(s) and its managed forms — sorted canonical
// relative paths. Missing kind directories are treated as empty, matching
// loadKind's tolerance for an ML Project that has not used a kind yet.
func ObjectFolderSourcePaths(root string) ([]string, error) {
	var paths []string
	for _, kind := range objectFolderKinds {
		directory := filepath.Join(root, "metadata", kind)
		objectEntries, err := os.ReadDir(directory)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, fmt.Errorf("read metadata %s: %w", kind, err)
		}
		for _, objectEntry := range objectEntries {
			if !objectEntry.IsDir() || objectEntry.Type()&fs.ModeSymlink != 0 {
				continue
			}
			objectDirectory := filepath.Join(directory, objectEntry.Name())
			fileEntries, err := os.ReadDir(objectDirectory)
			if err != nil {
				return nil, fmt.Errorf("read metadata %s object %s: %w", kind, objectEntry.Name(), err)
			}
			for _, fileEntry := range fileEntries {
				if fileEntry.Type()&fs.ModeSymlink != 0 {
					continue
				}
				if fileEntry.IsDir() {
					if fileEntry.Name() != "forms" {
						continue
					}
					formEntries, err := os.ReadDir(filepath.Join(objectDirectory, "forms"))
					if err != nil {
						return nil, fmt.Errorf("read metadata %s object %s forms: %w", kind, objectEntry.Name(), err)
					}
					for _, formEntry := range formEntries {
						if formEntry.IsDir() || formEntry.Type()&fs.ModeSymlink != 0 {
							continue
						}
						paths = append(paths, path.Join("metadata", kind, objectEntry.Name(), "forms", formEntry.Name()))
					}
					continue
				}
				paths = append(paths, path.Join("metadata", kind, objectEntry.Name(), fileEntry.Name()))
			}
		}
	}
	sort.Strings(paths)
	return paths, nil
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

func readManifest(path string) (Project, error) {
	file, err := os.Open(path)
	if err != nil {
		return Project{}, fmt.Errorf("open manifest %q: %w", path, err)
	}
	defer file.Close()
	return DecodeSource(path, file)
}
