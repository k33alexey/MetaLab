// Package publication builds deterministic, verifiable ML publication artifacts.
package publication

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/k33alexey/MetaLab/internal/project"
	"github.com/k33alexey/MetaLab/internal/uuid"
	"go.yaml.in/yaml/v3"
)

const (
	CurrentPackageFormat = 1
	PackageExtension     = ".mlpkg"
	maxSourceFileBytes   = 64 << 20
	maxPackageInputBytes = 512 << 20
)

var ErrSourceChanged = errors.New("ML Project changed while its publication package was being built")

type SourceState struct {
	GitCommit string
	Dirty     bool
}

type Manifest struct {
	Format        int         `json:"format"`
	ProjectID     uuid.UUID   `json:"projectId"`
	ProjectName   string      `json:"projectName"`
	ProjectFormat int         `json:"projectFormat"`
	GitCommit     string      `json:"gitCommit,omitempty"`
	Dirty         bool        `json:"dirty"`
	ContentSHA256 string      `json:"contentSha256"`
	Files         []FileEntry `json:"files"`
}

type FileEntry struct {
	Path   string `json:"path"`
	Size   int64  `json:"size"`
	SHA256 string `json:"sha256"`
}

type sourceFile struct {
	entry    FileEntry
	absolute string
	info     fs.FileInfo
}

// BuildFile validates a project and atomically writes its deterministic package.
func BuildFile(ctx context.Context, root, destination string, state SourceState) (Manifest, error) {
	if strings.TrimSpace(destination) == "" {
		return Manifest{}, fmt.Errorf("publication package destination is required")
	}
	destination, err := filepath.Abs(destination)
	if err != nil {
		return Manifest{}, fmt.Errorf("resolve publication package destination: %w", err)
	}
	if filepath.Ext(destination) != PackageExtension {
		return Manifest{}, fmt.Errorf("publication package must use %s extension", PackageExtension)
	}
	if err := os.MkdirAll(filepath.Dir(destination), 0o755); err != nil {
		return Manifest{}, fmt.Errorf("create publication package directory: %w", err)
	}
	temporary, err := os.CreateTemp(filepath.Dir(destination), ".ml-package-*")
	if err != nil {
		return Manifest{}, fmt.Errorf("create temporary publication package: %w", err)
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)
	manifest, buildErr := Build(ctx, root, temporary, state)
	if buildErr == nil {
		buildErr = temporary.Sync()
	}
	if closeErr := temporary.Close(); buildErr == nil {
		buildErr = closeErr
	}
	if buildErr != nil {
		return Manifest{}, buildErr
	}
	if err := os.Chmod(temporaryPath, 0o644); err != nil {
		return Manifest{}, fmt.Errorf("set publication package permissions: %w", err)
	}
	if err := replacePackageFile(temporaryPath, destination); err != nil {
		return Manifest{}, fmt.Errorf("publish publication package: %w", err)
	}
	return manifest, nil
}

// Build writes a deterministic ZIP stream. Callers should discard the stream on error.
func Build(ctx context.Context, root string, writer io.Writer, state SourceState) (Manifest, error) {
	state.GitCommit = strings.TrimSpace(state.GitCommit)
	if err := validateGitCommit(state.GitCommit); err != nil {
		return Manifest{}, err
	}
	manifest, sources, err := inspect(ctx, root, state)
	if err != nil {
		return Manifest{}, err
	}
	archive := zip.NewWriter(writer)
	if err := writeManifest(archive, manifest); err != nil {
		_ = archive.Close()
		return Manifest{}, err
	}
	for _, source := range sources {
		if err := writeSource(ctx, archive, source); err != nil {
			_ = archive.Close()
			return Manifest{}, err
		}
	}
	if err := verifySources(root, sources); err != nil {
		_ = archive.Close()
		return Manifest{}, err
	}
	if err := archive.Close(); err != nil {
		return Manifest{}, fmt.Errorf("close publication package: %w", err)
	}
	return manifest, nil
}

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

func inspect(ctx context.Context, root string, state SourceState) (Manifest, []sourceFile, error) {
	projectManifest, err := project.ValidateLayout(root)
	if err != nil {
		return Manifest{}, nil, err
	}
	root, err = filepath.Abs(root)
	if err != nil {
		return Manifest{}, nil, fmt.Errorf("resolve ML Project path: %w", err)
	}
	paths, err := canonicalSourcePaths(root)
	if err != nil {
		return Manifest{}, nil, err
	}
	sources := make([]sourceFile, 0, len(paths))
	var total int64
	contentHash := sha256.New()
	for _, relative := range paths {
		if err := ctx.Err(); err != nil {
			return Manifest{}, nil, err
		}
		absolute := filepath.Join(root, filepath.FromSlash(relative))
		entry, info, err := inspectFile(absolute, relative)
		if err != nil {
			return Manifest{}, nil, err
		}
		total += entry.Size
		if total > maxPackageInputBytes {
			return Manifest{}, nil, fmt.Errorf("ML Project sources exceed %d bytes", maxPackageInputBytes)
		}
		_, _ = fmt.Fprintf(contentHash, "%s\x00%d\x00%s\n", entry.Path, entry.Size, entry.SHA256)
		sources = append(sources, sourceFile{entry: entry, absolute: absolute, info: info})
	}
	manifest := Manifest{
		Format: CurrentPackageFormat, ProjectID: projectManifest.ID, ProjectName: projectManifest.Name,
		ProjectFormat: projectManifest.Format, GitCommit: strings.TrimSpace(state.GitCommit), Dirty: state.Dirty,
		ContentSHA256: hex.EncodeToString(contentHash.Sum(nil)), Files: make([]FileEntry, len(sources)),
	}
	for index := range sources {
		manifest.Files[index] = sources[index].entry
	}
	return manifest, sources, nil
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
	case "forms":
		expected, err = project.FormPath(id)
	case "reports":
		expected, err = project.ReportPath(id)
	case "tests":
		expected, err = project.TestPath(id)
	case "assets":
		expected, err = project.AssetPath(id, filepath.Ext(parts[1]))
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

func inspectFile(absolute, relative string) (FileEntry, fs.FileInfo, error) {
	info, err := os.Lstat(absolute)
	if err != nil {
		return FileEntry{}, nil, fmt.Errorf("inspect publication source %q: %w", relative, err)
	}
	if !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 {
		return FileEntry{}, nil, fmt.Errorf("publication source %q must be a regular file", relative)
	}
	if info.Size() > maxSourceFileBytes {
		return FileEntry{}, nil, fmt.Errorf("publication source %q exceeds %d bytes", relative, maxSourceFileBytes)
	}
	file, err := os.Open(absolute)
	if err != nil {
		return FileEntry{}, nil, fmt.Errorf("open publication source %q: %w", relative, err)
	}
	content, readErr := io.ReadAll(io.LimitReader(file, maxSourceFileBytes+1))
	closeErr := file.Close()
	if readErr != nil {
		return FileEntry{}, nil, fmt.Errorf("read publication source %q: %w", relative, readErr)
	}
	if closeErr != nil {
		return FileEntry{}, nil, fmt.Errorf("close publication source %q: %w", relative, closeErr)
	}
	if len(content) > maxSourceFileBytes {
		return FileEntry{}, nil, fmt.Errorf("publication source %q exceeds %d bytes", relative, maxSourceFileBytes)
	}
	if strings.HasSuffix(relative, ".yaml") {
		if err := validateYAML(relative, content); err != nil {
			return FileEntry{}, nil, err
		}
	} else if strings.HasSuffix(relative, ".bsl") && (!utf8.Valid(content) || bytes.IndexByte(content, 0) >= 0) {
		return FileEntry{}, nil, fmt.Errorf("BSL source %q must be valid UTF-8 text", relative)
	}
	digest := sha256.Sum256(content)
	return FileEntry{Path: relative, Size: int64(len(content)), SHA256: hex.EncodeToString(digest[:])}, info, nil
}

func verifySources(root string, sources []sourceFile) error {
	paths, err := canonicalSourcePaths(root)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrSourceChanged, err)
	}
	if len(paths) != len(sources) {
		return ErrSourceChanged
	}
	for index, source := range sources {
		if paths[index] != source.entry.Path {
			return ErrSourceChanged
		}
		current, err := os.Lstat(source.absolute)
		if err != nil || !os.SameFile(source.info, current) || current.Size() != source.entry.Size || !current.ModTime().Equal(source.info.ModTime()) {
			return fmt.Errorf("%w: %s", ErrSourceChanged, source.entry.Path)
		}
	}
	return nil
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

func writeManifest(archive *zip.Writer, manifest Manifest) error {
	content, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return fmt.Errorf("encode publication manifest: %w", err)
	}
	content = append(content, '\n')
	entry, err := archive.CreateHeader(zipHeader("package.json"))
	if err != nil {
		return fmt.Errorf("create publication manifest entry: %w", err)
	}
	if _, err := entry.Write(content); err != nil {
		return fmt.Errorf("write publication manifest entry: %w", err)
	}
	return nil
}

func writeSource(ctx context.Context, archive *zip.Writer, source sourceFile) error {
	file, err := os.Open(source.absolute)
	if err != nil {
		return fmt.Errorf("open publication source %q: %w", source.entry.Path, err)
	}
	defer file.Close()
	entry, err := archive.CreateHeader(zipHeader(source.entry.Path))
	if err != nil {
		return fmt.Errorf("create publication entry %q: %w", source.entry.Path, err)
	}
	hash := sha256.New()
	written, err := copyContext(ctx, io.MultiWriter(entry, hash), io.LimitReader(file, maxSourceFileBytes+1))
	if err != nil {
		return fmt.Errorf("write publication source %q: %w", source.entry.Path, err)
	}
	actualDigest := hex.EncodeToString(hash.Sum(nil))
	if written != source.entry.Size || actualDigest != source.entry.SHA256 {
		return fmt.Errorf("%w: %s", ErrSourceChanged, source.entry.Path)
	}
	return nil
}

func copyContext(ctx context.Context, destination io.Writer, source io.Reader) (int64, error) {
	buffer := make([]byte, 64<<10)
	var total int64
	for {
		if err := ctx.Err(); err != nil {
			return total, err
		}
		read, readErr := source.Read(buffer)
		if read > 0 {
			written, writeErr := destination.Write(buffer[:read])
			total += int64(written)
			if writeErr != nil {
				return total, writeErr
			}
			if written != read {
				return total, io.ErrShortWrite
			}
		}
		if errors.Is(readErr, io.EOF) {
			return total, nil
		}
		if readErr != nil {
			return total, readErr
		}
	}
}

func zipHeader(name string) *zip.FileHeader {
	header := &zip.FileHeader{Name: filepath.ToSlash(name), Method: zip.Deflate}
	header.SetMode(0o644)
	header.SetModTime(time.Date(1980, 1, 1, 0, 0, 0, 0, time.UTC))
	return header
}
