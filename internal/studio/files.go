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
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/k33alexey/MetaLab/internal/metadata"
	"github.com/k33alexey/MetaLab/internal/project"
	"github.com/k33alexey/MetaLab/internal/uuid"
	"go.yaml.in/yaml/v3"
)

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
	return workspace.saveSourceLocked(relative, content, expectedRevision)
}

func (workspace *Workspace) saveSourceLocked(relative, content, expectedRevision string) (SourceFile, error) {
	relative, language, err := validateEditablePath(relative)
	if err != nil {
		return SourceFile{}, err
	}
	if !utf8.ValidString(content) || strings.IndexByte(content, 0) >= 0 {
		return SourceFile{}, fmt.Errorf("editable source must be valid UTF-8")
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
	temporaryPath, err := prepareStudioSource(filepath.Dir(target), bytesToWrite)
	if err != nil {
		return SourceFile{}, err
	}
	defer os.Remove(temporaryPath)
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
	workspace.invalidateStudioIndexesLocked()
	return sourceFile(relative, language, bytesToWrite), nil
}

type studioSourceReplacement struct {
	relative string
	target   string
	next     string
	rollback string
}

// replaceSourcesLocked applies prevalidated refactoring edits and rolls back all
// replaced files when one replacement or last-moment revision check fails.
func (workspace *Workspace) replaceSourcesLocked(changes, originals map[string][]byte) error {
	paths := make([]string, 0, len(changes))
	for relative := range changes {
		if _, ok := originals[relative]; !ok {
			return fmt.Errorf("missing refactoring source snapshot for %s", relative)
		}
		paths = append(paths, relative)
	}
	sort.Strings(paths)
	prepared := make([]studioSourceReplacement, 0, len(paths))
	defer func() {
		for _, item := range prepared {
			_ = os.Remove(item.next)
			_ = os.Remove(item.rollback)
		}
	}()
	for _, relative := range paths {
		target, err := workspace.resolveExistingSource(relative)
		if err != nil {
			return err
		}
		next, err := prepareStudioSource(filepath.Dir(target), changes[relative])
		if err != nil {
			return err
		}
		rollback, err := prepareStudioSource(filepath.Dir(target), originals[relative])
		if err != nil {
			_ = os.Remove(next)
			return err
		}
		prepared = append(prepared, studioSourceReplacement{relative: relative, target: target, next: next, rollback: rollback})
	}

	applied := -1
	for index, item := range prepared {
		latest, err := workspace.readSource(item.relative)
		if err != nil || !bytes.Equal([]byte(latest.Content), originals[item.relative]) {
			if err == nil {
				err = ErrSourceChanged
			}
			return rollbackStudioSources(prepared, applied, err)
		}
		if err := replaceStudioFile(item.next, item.target); err != nil {
			return rollbackStudioSources(prepared, index, fmt.Errorf("replace refactored source %s: %w", item.relative, err))
		}
		applied = index
	}
	return nil
}

// prepareStudioSource writes content to a new file in directory, synced and
// closed, and answers its path: the file that replaceStudioFile then puts in
// place of a source, of the grid of a role or of the settings of the user.
func prepareStudioSource(directory string, content []byte) (string, error) {
	temporary, err := os.CreateTemp(directory, ".ml-write-*")
	if err != nil {
		return "", fmt.Errorf("create temporary file: %w", err)
	}
	path := temporary.Name()
	failed := true
	defer func() {
		_ = temporary.Close()
		if failed {
			_ = os.Remove(path)
		}
	}()
	if err := temporary.Chmod(0o644); err != nil {
		return "", fmt.Errorf("set temporary file permissions: %w", err)
	}
	if _, err := temporary.Write(content); err != nil {
		return "", fmt.Errorf("write temporary file: %w", err)
	}
	if err := temporary.Sync(); err != nil {
		return "", fmt.Errorf("sync temporary file: %w", err)
	}
	if err := temporary.Close(); err != nil {
		return "", fmt.Errorf("close temporary file: %w", err)
	}
	failed = false
	return path, nil
}

func rollbackStudioSources(items []studioSourceReplacement, through int, cause error) error {
	var rollbackError error
	for index := min(through, len(items)-1); index >= 0; index-- {
		if err := replaceStudioFile(items[index].rollback, items[index].target); err != nil && rollbackError == nil {
			rollbackError = err
		}
	}
	if rollbackError != nil {
		return fmt.Errorf("%w; rollback failed: %v", cause, rollbackError)
	}
	return cause
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
	// A file is read whole, whatever its size: the loader takes a file of any
	// size since 2.187, and a file the project holds but the editor refuses
	// would take down every screen that reads all files of its kind.
	content, err := io.ReadAll(file)
	if err != nil {
		return SourceFile{}, fmt.Errorf("read source: %w", err)
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
	if relative == project.ConfigurationFile {
		return relative, "yaml", nil
	}
	if project.IsRootModuleFile(relative) {
		return relative, "bsl", nil
	}
	parts := strings.Split(relative, "/")
	if len(parts) == 2 {
		extension := path.Ext(parts[1])
		language := ""
		switch parts[0] {
		case "modules":
			if extension == ".bsl" {
				language = "bsl"
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
	// Some kinds keep a folder named after the object, holding a fixed set of
	// files: a description and the modules that run it.
	if len(parts) == 4 && parts[0] == "metadata" && project.ObjectName(parts[2]) == nil {
		if named, ok := project.NamedFolderFiles(parts[1]); ok && slices.Contains(named, parts[3]) {
			if path.Ext(parts[3]) == ".bsl" {
				return relative, "bsl", nil
			}
			return relative, "yaml", nil
		}
	}
	// An object folder is read from its address, wherever it lies: a table
	// of an external data source keeps the same files a catalog does, one
	// folder deeper, and the project is what knows how much deeper.
	if kind, name, rest, ok := project.SplitObjectPath(relative); ok && project.IsObjectFolderKind(kind) {
		if len(rest) == 1 && rest[0] == project.ObjectMetadataFile {
			return relative, "yaml", nil
		}
		// A module is named after the role it plays, not after an
		// identifier: the file is what says which module it is.
		if len(rest) == 1 && project.KeepsModule(kind, rest[0]) {
			return relative, "bsl", nil
		}
		// A form is a folder named after itself, holding its description
		// and, under the name of its role, the module that runs it.
		if len(rest) == 3 && rest[0] == "forms" && project.SubordinateName(rest[1]) == nil {
			switch rest[2] {
			case project.FormMetadataFile:
				return relative, "yaml", nil
			case project.FormModuleFile:
				return relative, "bsl", nil
			}
		}
		if len(rest) == 3 && rest[0] == "commands" && project.SubordinateName(rest[1]) == nil &&
			rest[2] == project.CommandModuleFile {
			return relative, "bsl", nil
		}
		// What the object describes inside itself keeps its modules a folder
		// each: the recalculations of a calculation register.
		if len(rest) == 3 {
			if inner, ok := project.InnerKind(kind, rest[0]); ok {
				if expected, err := project.InnerModulePath(inner, name, rest[1], rest[2]); err == nil && expected == relative {
					return relative, "bsl", nil
				}
			}
		}
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
	if relative == project.ConfigurationFile {
		configuration, err := project.DecodeSource(relative, bytes.NewReader(content))
		if err != nil {
			return nil, err
		}
		current, err := project.ValidateLayout(workspace.root)
		if err != nil {
			return nil, err
		}
		if configuration.ID != current.ID {
			return nil, project.ErrProjectIdentityChanged
		}
		var canonical bytes.Buffer
		if err := project.Encode(&canonical, configuration); err != nil {
			return nil, err
		}
		return canonical.Bytes(), nil
	}
	configuration, err := project.ValidateLayout(workspace.root)
	if err != nil {
		return nil, err
	}
	// A role's description is read with the grid it stands beside, as the
	// loader reads it.
	var rights io.Reader
	if strings.HasPrefix(relative, "metadata/"+string(metadata.RoleKind)+"/") {
		files, err := workspace.readRoleFilesLocked(relative)
		if err != nil && !errors.Is(err, ErrSourceNotFound) {
			return nil, err
		}
		if files.grid != nil {
			rights = bytes.NewReader(files.grid)
		}
	}
	value, described, err := metadata.DecodeSource(relative, bytes.NewReader(content), rights, configuration)
	if err != nil {
		return nil, err
	}
	if described {
		// A role's description is half of the role: written back from the
		// whole, it keeps its restrictions and leaves the grid to the grid's
		// own file.
		if role, ok := value.(metadata.RoleDefinition); ok {
			description, _, err := metadata.EncodeRole(role)
			return description, err
		}
		var canonical bytes.Buffer
		if err := metadata.Encode(&canonical, value); err != nil {
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
