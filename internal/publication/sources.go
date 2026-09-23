// Package publication inspects an ML Project's sources and saves them into
// its database. The project directory is the artifact: there is no
// intermediate package file between what a developer edits and what the
// database holds.
package publication

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/k33alexey/MetaLab/internal/metadata"
	"github.com/k33alexey/MetaLab/internal/project"
	"github.com/k33alexey/MetaLab/internal/schemadiff"
	"github.com/k33alexey/MetaLab/internal/uuid"
	"go.yaml.in/yaml/v3"
)

const (
	CurrentPackageFormat    = 6
	PackageExtension        = ".mlpkg"
	maxSourceFileBytes      = 64 << 20
	maxPackageInputBytes    = 512 << 20
	maxPackageManifestBytes = 64 << 20
)

type SourceState struct {
	GitCommit string
	Dirty     bool
}

type Manifest struct {
	Format                  int                      `json:"format"`
	ProjectID               uuid.UUID                `json:"projectId"`
	ProjectName             string                   `json:"projectName"`
	ProjectFormat           int                      `json:"projectFormat"`
	GitCommit               string                   `json:"gitCommit,omitempty"`
	Dirty                   bool                     `json:"dirty"`
	ContentSHA256           string                   `json:"contentSha256"`
	Files                   []FileEntry              `json:"files"`
	ConstantIDs             []uuid.UUID              `json:"constantIds,omitempty"`
	CatalogIDs              []uuid.UUID              `json:"catalogIds,omitempty"`
	DocumentIDs             []uuid.UUID              `json:"documentIds,omitempty"`
	InformationRegisterIDs  []uuid.UUID              `json:"informationRegisterIds,omitempty"`
	AccumulationRegisterIDs []uuid.UUID              `json:"accumulationRegisterIds,omitempty"`
	SchemaSHA256            string                   `json:"schemaSha256"`
	Runtime                 metadata.RuntimeSnapshot `json:"runtime"`
}

type FileEntry struct {
	Path   string `json:"path"`
	Size   int64  `json:"size"`
	SHA256 string `json:"sha256"`
}

// BuildFile validates a project and atomically writes its deterministic package.
// Build writes a deterministic ZIP stream. Callers should discard the stream on error.
func validateGitCommit(commit string) error {
	if commit == "" {
		return nil
	}
	decoded, err := hex.DecodeString(commit)
	if err != nil || len(decoded) != 20 && len(decoded) != 32 {
		return fmt.Errorf("Git commit must be a full SHA-1 or SHA-256 identifier")
	}
	return nil
}

func inspect(ctx context.Context, root string, state SourceState) (Manifest, error) {
	if err := validateGitCommit(strings.TrimSpace(state.GitCommit)); err != nil {
		return Manifest{}, err
	}
	projectManifest, err := project.ValidateLayout(root)
	if err != nil {
		return Manifest{}, err
	}
	metadataCatalog, err := metadata.Load(root)
	if err != nil {
		return Manifest{}, fmt.Errorf("validate application metadata: %w", err)
	}
	applicationSchema, err := metadataCatalog.ApplicationSchema()
	if err != nil {
		return Manifest{}, fmt.Errorf("build application schema: %w", err)
	}
	schemaSHA256, err := schemadiff.SchemaSHA256(applicationSchema)
	if err != nil {
		return Manifest{}, err
	}
	root, err = filepath.Abs(root)
	if err != nil {
		return Manifest{}, fmt.Errorf("resolve ML Project path: %w", err)
	}
	paths, err := canonicalSourcePaths(root)
	if err != nil {
		return Manifest{}, err
	}
	entries := make([]FileEntry, 0, len(paths))
	forms := make([]metadata.ManagedForm, 0)
	var total int64
	contentHash := sha256.New()
	for _, relative := range paths {
		if err := ctx.Err(); err != nil {
			return Manifest{}, err
		}
		absolute := filepath.Join(root, filepath.FromSlash(relative))
		entry, err := inspectFile(absolute, relative)
		if err != nil {
			return Manifest{}, err
		}
		if isManagedFormSourcePath(relative) {
			file, openErr := os.Open(absolute)
			if openErr != nil {
				return Manifest{}, openErr
			}
			form, decodeErr := metadata.DecodeManagedForm(relative, file, projectManifest)
			closeErr := file.Close()
			if decodeErr != nil {
				return Manifest{}, decodeErr
			}
			filenameID, parseErr := uuid.Parse(strings.TrimSuffix(filepath.Base(relative), ".yaml"))
			if parseErr != nil || form.ID != filenameID {
				return Manifest{}, fmt.Errorf("form UUID does not match %q", relative)
			}
			forms = append(forms, form)
			if closeErr != nil {
				return Manifest{}, closeErr
			}
		}
		total += entry.Size
		if total > maxPackageInputBytes {
			return Manifest{}, fmt.Errorf("ML Project sources exceed %d bytes", maxPackageInputBytes)
		}
		_, _ = fmt.Fprintf(contentHash, "%s\x00%d\x00%s\n", entry.Path, entry.Size, entry.SHA256)
		entries = append(entries, entry)
	}
	runtimeSnapshot, err := metadata.NewRuntimeSnapshot(metadataCatalog, forms)
	if err != nil {
		return Manifest{}, fmt.Errorf("build runtime metadata: %w", err)
	}
	manifest := Manifest{
		Format: CurrentPackageFormat, ProjectID: projectManifest.ID, ProjectName: projectManifest.Name,
		ProjectFormat: projectManifest.Format, GitCommit: strings.TrimSpace(state.GitCommit), Dirty: state.Dirty,
		ContentSHA256: hex.EncodeToString(contentHash.Sum(nil)), Files: entries,
		ConstantIDs: metadataCatalog.ConstantIDs(),
		CatalogIDs:  metadataCatalog.CatalogIDs(), SchemaSHA256: schemaSHA256,
		DocumentIDs:             metadataCatalog.DocumentIDs(),
		InformationRegisterIDs:  metadataCatalog.InformationRegisterIDs(),
		AccumulationRegisterIDs: metadataCatalog.AccumulationRegisterIDs(),
		Runtime:                 runtimeSnapshot,
	}
	return manifest, nil
}

func canonicalSourcePaths(root string) ([]string, error) {
	paths := []string{project.ManifestFile}
	for _, directory := range project.RootDirectories() {
		err := filepath.WalkDir(filepath.Join(root, directory), func(current string, entry fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if current == filepath.Join(root, directory) {
				return nil
			}
			relative, err := filepath.Rel(root, current)
			if err != nil {
				return err
			}
			relative = filepath.ToSlash(relative)
			if entry.Type()&os.ModeSymlink != 0 {
				return fmt.Errorf("publication source %q must not be a symbolic link", relative)
			}
			if entry.IsDir() {
				return validateSourcePath(relative, true)
			}
			if !entry.Type().IsRegular() {
				return fmt.Errorf("publication source %q must be a regular file", relative)
			}
			if err := validateSourcePath(relative, false); err != nil {
				return err
			}
			if entry.Name() != ".gitkeep" {
				paths = append(paths, relative)
			}
			return nil
		})
		if err != nil {
			return nil, fmt.Errorf("scan publication sources: %w", err)
		}
	}
	sort.Strings(paths)
	return paths, nil
}

func validateSourcePath(relative string, directory bool) error {
	parts := strings.Split(relative, "/")
	if len(parts) < 2 {
		return nil
	}
	if parts[0] == "metadata" {
		if directory && len(parts) == 2 && contains(project.MetadataKinds(), parts[1]) {
			return nil
		}
		if !directory && len(parts) == 2 && parts[1] == ".gitkeep" {
			return nil
		}
		if !directory && len(parts) == 3 && parts[2] == ".gitkeep" && contains(project.MetadataKinds(), parts[1]) {
			return nil
		}
		if contains(project.ObjectFolderKinds(), parts[1]) {
			return validateObjectFolderSourcePath(parts, relative, directory)
		}
		if !directory && len(parts) == 3 {
			id, err := uuid.Parse(strings.TrimSuffix(parts[2], ".yaml"))
			expected, pathErr := project.MetadataPath(parts[1], id)
			if err == nil && pathErr == nil && expected == relative {
				return nil
			}
		}
		return fmt.Errorf("unexpected publication source path %q", relative)
	}
	if directory || len(parts) != 2 {
		return fmt.Errorf("unexpected publication source path %q", relative)
	}
	if parts[1] == ".gitkeep" {
		return nil
	}
	id, err := uuid.Parse(strings.TrimSuffix(parts[1], filepath.Ext(parts[1])))
	if err != nil {
		return fmt.Errorf("publication source %q must use a UUID name", relative)
	}
	var expected string
	switch parts[0] {
	case "modules":
		expected, err = project.ModulePath(id)
	default:
		err = fmt.Errorf("unknown source directory")
	}
	if err != nil || expected != relative {
		return fmt.Errorf("unexpected publication source path %q", relative)
	}
	return nil
}

func contains(items []string, value string) bool {
	for _, item := range items {
		if item == value {
			return true
		}
	}
	return false
}

// validateObjectFolderSourcePath validates the shapes physically grouped
// under one catalog/document/register's own folder: the object's
// description, its module(s) directly inside it, and its managed forms
// inside a forms/ subdirectory.
func validateObjectFolderSourcePath(parts []string, relative string, directory bool) error {
	if len(parts) < 3 {
		return fmt.Errorf("unexpected publication source path %q", relative)
	}
	objectID, err := uuid.Parse(parts[2])
	if err != nil {
		return fmt.Errorf("unexpected publication source path %q", relative)
	}
	if directory && len(parts) == 3 {
		return nil
	}
	if directory && len(parts) == 4 && parts[3] == "forms" {
		return nil
	}
	if !directory && len(parts) == 4 && parts[3] == "object.yaml" {
		if expected, err := project.ObjectMetadataPath(parts[1], objectID); err == nil && expected == relative {
			return nil
		}
	}
	if !directory {
		if moduleID, ok := objectFolderModuleID(relative); ok {
			if expected, err := project.ObjectModulePath(parts[1], objectID, moduleID); err == nil && expected == relative {
				return nil
			}
		}
		if formID, ok := objectFolderFormID(relative); ok {
			if expected, err := project.ObjectFormPath(parts[1], objectID, formID); err == nil && expected == relative {
				return nil
			}
		}
	}
	return fmt.Errorf("unexpected publication source path %q", relative)
}

// objectFolderModuleID reports the module UUID if relative is one of an
// object's own module files (metadata/<kind>/<id>/<module>.bsl).
func objectFolderModuleID(relative string) (uuid.UUID, bool) {
	parts := strings.Split(relative, "/")
	if len(parts) != 4 || parts[0] != "metadata" || !contains(project.ObjectFolderKinds(), parts[1]) {
		return uuid.UUID{}, false
	}
	if _, err := uuid.Parse(parts[2]); err != nil {
		return uuid.UUID{}, false
	}
	id, err := uuid.Parse(strings.TrimSuffix(parts[3], ".bsl"))
	if err != nil || parts[3] != id.String()+".bsl" {
		return uuid.UUID{}, false
	}
	return id, true
}

// objectFolderFormID reports the form UUID if relative is one of an
// object's own managed forms (metadata/<kind>/<id>/forms/<form>.yaml).
func objectFolderFormID(relative string) (uuid.UUID, bool) {
	parts := strings.Split(relative, "/")
	if len(parts) != 5 || parts[0] != "metadata" || parts[3] != "forms" || !contains(project.ObjectFolderKinds(), parts[1]) {
		return uuid.UUID{}, false
	}
	if _, err := uuid.Parse(parts[2]); err != nil {
		return uuid.UUID{}, false
	}
	id, err := uuid.Parse(strings.TrimSuffix(parts[4], ".yaml"))
	if err != nil || parts[4] != id.String()+".yaml" {
		return uuid.UUID{}, false
	}
	return id, true
}

// isManagedFormSourcePath reports whether relative is any managed form
// source: flat (forms/<id>.yaml) or nested under its owning object.
func isManagedFormSourcePath(relative string) bool {
	if strings.HasPrefix(relative, "metadata/common-forms/") {
		return true
	}
	_, ok := objectFolderFormID(relative)
	return ok
}

func inspectFile(absolute, relative string) (FileEntry, error) {
	info, err := os.Lstat(absolute)
	if err != nil {
		return FileEntry{}, fmt.Errorf("inspect publication source %q: %w", relative, err)
	}
	if !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 {
		return FileEntry{}, fmt.Errorf("publication source %q must be a regular file", relative)
	}
	if info.Size() > maxSourceFileBytes {
		return FileEntry{}, fmt.Errorf("publication source %q exceeds %d bytes", relative, maxSourceFileBytes)
	}
	file, err := os.Open(absolute)
	if err != nil {
		return FileEntry{}, fmt.Errorf("open publication source %q: %w", relative, err)
	}
	content, readErr := io.ReadAll(io.LimitReader(file, maxSourceFileBytes+1))
	closeErr := file.Close()
	if readErr != nil {
		return FileEntry{}, fmt.Errorf("read publication source %q: %w", relative, readErr)
	}
	if closeErr != nil {
		return FileEntry{}, fmt.Errorf("close publication source %q: %w", relative, closeErr)
	}
	if len(content) > maxSourceFileBytes {
		return FileEntry{}, fmt.Errorf("publication source %q exceeds %d bytes", relative, maxSourceFileBytes)
	}
	if strings.HasSuffix(relative, ".yaml") {
		if err := validateYAML(relative, content); err != nil {
			return FileEntry{}, err
		}
	} else if strings.HasSuffix(relative, ".bsl") && (!utf8.Valid(content) || bytes.IndexByte(content, 0) >= 0) {
		return FileEntry{}, fmt.Errorf("BSL source %q must be valid UTF-8 text", relative)
	}
	digest := sha256.Sum256(content)
	return FileEntry{Path: relative, Size: int64(len(content)), SHA256: hex.EncodeToString(digest[:])}, nil
}

func validateYAML(relative string, content []byte) error {
	if len(content) > project.MaxYAMLDocumentBytes {
		return fmt.Errorf("YAML source %q exceeds %d bytes", relative, project.MaxYAMLDocumentBytes)
	}
	decoder := yaml.NewDecoder(bytes.NewReader(content))
	var document yaml.Node
	if err := decoder.Decode(&document); err != nil {
		return fmt.Errorf("decode YAML source %q: %w", relative, err)
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		if err != nil {
			return fmt.Errorf("decode YAML source %q: %w", relative, err)
		}
		return fmt.Errorf("YAML source %q contains multiple documents", relative)
	}
	return nil
}
