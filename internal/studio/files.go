package studio

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/k33alexey/MetaLab/internal/project"
	"github.com/k33alexey/MetaLab/internal/uuid"
	"go.yaml.in/yaml/v3"
)

const MaxEditableFileBytes = 8 << 20

var (
	ErrInvalidSourcePath = errors.New("invalid ML Project source path")
	ErrSourceChanged     = errors.New("ML Project source changed outside this editor")
	ErrSourceNotFound    = errors.New("ML Project source not found")
)

// SourceFile is a bounded UTF-8 file with a content revision used for safe saves.
type SourceFile struct {
	Path     string `json:"path"`
	Language string `json:"language"`
	Content  string `json:"content"`
	Revision string `json:"revision"`
}

// ReadSource reads a canonical editable source without following symbolic links.
func (workspace *Workspace) ReadSource(relative string) (SourceFile, error) {
	workspace.mu.Lock()
	defer workspace.mu.Unlock()
	return workspace.readSource(relative)
}

// SaveSource atomically replaces a source only if its expected content revision is current.
func (workspace *Workspace) SaveSource(relative, content, expectedRevision string) (SourceFile, error) {
	workspace.mu.Lock()
	defer workspace.mu.Unlock()
	relative, language, err := validateEditablePath(relative)
	if err != nil {
		return SourceFile{}, err
	}
	if len(content) > MaxEditableFileBytes || !utf8.ValidString(content) || strings.IndexByte(content, 0) >= 0 {
		return SourceFile{}, fmt.Errorf("editable source must be valid UTF-8 and at most %d bytes", MaxEditableFileBytes)
	}
	if len(expectedRevision) != sha256.Size*2 {
		return SourceFile{}, fmt.Errorf("expected source revision is invalid")
	}
	current, err := workspace.readSource(relative)
	if err != nil {
		return SourceFile{}, err
	}
	if !strings.EqualFold(current.Revision, expectedRevision) {
		return SourceFile{}, ErrSourceChanged
	}

	bytesToWrite := []byte(content)
	if language == "yaml" {
		bytesToWrite, err = workspace.validateYAMLSource(relative, bytesToWrite)
		if err != nil {
			return SourceFile{}, err
		}
	}
	target, err := workspace.resolveExistingSource(relative)
	if err != nil {
		return SourceFile{}, err
	}
	temporary, err := os.CreateTemp(filepath.Dir(target), ".ml-save-*")
	if err != nil {
		return SourceFile{}, fmt.Errorf("create temporary source: %w", err)
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)
	if err := temporary.Chmod(0o644); err != nil {
		_ = temporary.Close()
		return SourceFile{}, fmt.Errorf("set temporary source permissions: %w", err)
	}
	if _, err := temporary.Write(bytesToWrite); err != nil {
		_ = temporary.Close()
		return SourceFile{}, fmt.Errorf("write temporary source: %w", err)
	}
	if err := temporary.Sync(); err != nil {
		_ = temporary.Close()
		return SourceFile{}, fmt.Errorf("sync temporary source: %w", err)
	}
	if err := temporary.Close(); err != nil {
		return SourceFile{}, fmt.Errorf("close temporary source: %w", err)
	}
	latest, err := workspace.readSource(relative)
	if err != nil {
		return SourceFile{}, err
	}
	if latest.Revision != current.Revision {
		return SourceFile{}, ErrSourceChanged
	}
	if err := replaceStudioFile(temporaryPath, target); err != nil {
		return SourceFile{}, fmt.Errorf("replace source: %w", err)
	}
	return sourceFile(relative, language, bytesToWrite), nil
}

func (workspace *Workspace) readSource(relative string) (SourceFile, error) {
	relative, language, err := validateEditablePath(relative)
	if err != nil {
		return SourceFile{}, err
	}
	filePath, err := workspace.resolveExistingSource(relative)
	if err != nil {
		return SourceFile{}, err
	}
	file, err := os.Open(filePath)
	if errors.Is(err, fs.ErrNotExist) {
		return SourceFile{}, ErrSourceNotFound
	}
	if err != nil {
		return SourceFile{}, fmt.Errorf("open source: %w", err)
	}
	defer file.Close()
	content, err := io.ReadAll(io.LimitReader(file, MaxEditableFileBytes+1))
	if err != nil {
		return SourceFile{}, fmt.Errorf("read source: %w", err)
	}
	if len(content) > MaxEditableFileBytes {
		return SourceFile{}, fmt.Errorf("source exceeds %d bytes", MaxEditableFileBytes)
	}
	if !utf8.Valid(content) || bytes.IndexByte(content, 0) >= 0 {
		return SourceFile{}, fmt.Errorf("source is not valid UTF-8 text")
	}
	return sourceFile(relative, language, content), nil
}

func sourceFile(relative, language string, content []byte) SourceFile {
	digest := sha256.Sum256(content)
	return SourceFile{Path: relative, Language: language, Content: string(content), Revision: hex.EncodeToString(digest[:])}
}

func validateEditablePath(relative string) (string, string, error) {
	if relative == "" || strings.Contains(relative, `\`) || path.IsAbs(relative) || path.Clean(relative) != relative {
		return "", "", ErrInvalidSourcePath
	}
	if relative == project.ManifestFile {
		return relative, "yaml", nil
	}
	parts := strings.Split(relative, "/")
	if len(parts) == 2 {
		extension := path.Ext(parts[1])
		language := ""
		switch parts[0] {
		case "modules", "tests":
			if extension == ".bsl" {
				language = "bsl"
			}
		case "forms", "reports":
			if extension == ".yaml" {
				language = "yaml"
			}
		}
		if language != "" && validUUIDFile(parts[1], extension) {
			return relative, language, nil
		}
	}
	if len(parts) == 3 && parts[0] == "metadata" && slices.Contains(project.MetadataKinds(), parts[1]) &&
		path.Ext(parts[2]) == ".yaml" && validUUIDFile(parts[2], ".yaml") {
		return relative, "yaml", nil
	}
	return "", "", ErrInvalidSourcePath
}

func validUUIDFile(name, extension string) bool {
	id, err := uuid.Parse(strings.TrimSuffix(name, extension))
	return err == nil && name == id.String()+extension
}

func (workspace *Workspace) resolveExistingSource(relative string) (string, error) {
	current := workspace.root
	rootInfo, err := os.Lstat(current)
	if errors.Is(err, fs.ErrNotExist) {
		return "", ErrSourceNotFound
	}
	if err != nil {
		return "", fmt.Errorf("inspect project root: %w", err)
	}
	if !rootInfo.IsDir() || rootInfo.Mode()&os.ModeSymlink != 0 {
		return "", fmt.Errorf("%w: project root is no longer a safe directory", ErrInvalidSourcePath)
	}
	for _, component := range strings.Split(relative, "/") {
		current = filepath.Join(current, component)
		info, err := os.Lstat(current)
		if errors.Is(err, fs.ErrNotExist) {
			return "", ErrSourceNotFound
		}
		if err != nil {
			return "", fmt.Errorf("inspect source path: %w", err)
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return "", fmt.Errorf("%w: symbolic links are not editable", ErrInvalidSourcePath)
		}
	}
	info, err := os.Stat(current)
	if err != nil {
		return "", fmt.Errorf("inspect source: %w", err)
	}
	if !info.Mode().IsRegular() {
		return "", ErrInvalidSourcePath
	}
	return current, nil
}

func (workspace *Workspace) validateYAMLSource(relative string, content []byte) ([]byte, error) {
	if len(content) > project.MaxYAMLDocumentBytes {
		return nil, project.ErrYAMLDocumentTooLarge
	}
	if relative == project.ManifestFile {
		manifest, err := project.DecodeSource(relative, bytes.NewReader(content))
		if err != nil {
			return nil, err
		}
		current, err := project.ValidateLayout(workspace.root)
		if err != nil {
			return nil, err
		}
		if manifest.ID != current.ID {
			return nil, project.ErrProjectIdentityChanged
		}
		var canonical bytes.Buffer
		if err := project.Encode(&canonical, manifest); err != nil {
			return nil, err
		}
		return canonical.Bytes(), nil
	}
	decoder := yaml.NewDecoder(bytes.NewReader(content))
	var document yaml.Node
	if err := decoder.Decode(&document); err != nil {
		return nil, fmt.Errorf("decode %s: %w", relative, err)
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		if err != nil {
			return nil, fmt.Errorf("decode trailing YAML in %s: %w", relative, err)
		}
		return nil, fmt.Errorf("decode %s: multiple YAML documents are not allowed", relative)
	}
	return content, nil
}
