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
	return workspace.saveSourceLocked(relative, content, expectedRevision)
}

func (workspace *Workspace) saveSourceLocked(relative, content, expectedRevision string) (SourceFile, error) {
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

func prepareStudioSource(directory string, content []byte) (string, error) {
	temporary, err := os.CreateTemp(directory, ".ml-refactor-*")
	if err != nil {
		return "", fmt.Errorf("create temporary refactoring source: %w", err)
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
		return "", fmt.Errorf("set temporary refactoring source permissions: %w", err)
	}
	if _, err := temporary.Write(content); err != nil {
		return "", fmt.Errorf("write temporary refactoring source: %w", err)
	}
	if err := temporary.Sync(); err != nil {
		return "", fmt.Errorf("sync temporary refactoring source: %w", err)
	}
	if err := temporary.Close(); err != nil {
		return "", fmt.Errorf("close temporary refactoring source: %w", err)
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

// yamlScalarField reads one top-level scalar out of a YAML document without
// decoding the whole of it into a type. It is used where the type is not known
// at the point of the check.
func yamlScalarField(content []byte, key string) string {
	var document yaml.Node
	if err := yaml.Unmarshal(content, &document); err != nil {
		return ""
	}
	if len(document.Content) != 1 || document.Content[0].Kind != yaml.MappingNode {
		return ""
	}
	mapping := document.Content[0]
	for index := 0; index+1 < len(mapping.Content); index += 2 {
		if mapping.Content[index].Value == key && mapping.Content[index+1].Kind == yaml.ScalarNode {
			return strings.TrimSpace(mapping.Content[index+1].Value)
		}
	}
	return ""
}

func validateEditablePath(relative string) (string, string, error) {
	if relative == "" || strings.Contains(relative, `\`) || path.IsAbs(relative) || path.Clean(relative) != relative {
		return "", "", ErrInvalidSourcePath
	}
	if relative == project.ManifestFile {
		return relative, "yaml", nil
	}
	if relative == project.SessionModuleFile {
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
	if len(parts) >= 4 && parts[0] == "metadata" && slices.Contains(project.ObjectFolderKinds(), parts[1]) {
		if project.ObjectName(parts[2]) == nil {
			if len(parts) == 4 && parts[3] == project.ObjectMetadataFile {
				return relative, "yaml", nil
			}
			// A module is named after the role it plays, not after an
			// identifier: the file is what says which module it is.
			if len(parts) == 4 && slices.Contains(project.ObjectModuleFiles(), parts[3]) {
				return relative, "bsl", nil
			}
			// A form is a folder named after itself, holding its description
			// and, under the name of its role, the module that runs it.
			if len(parts) == 6 && parts[3] == "forms" && project.SubordinateName(parts[4]) == nil {
				switch parts[5] {
				case project.FormMetadataFile:
					return relative, "yaml", nil
				case project.FormModuleFile:
					return relative, "bsl", nil
				}
			}
			if len(parts) == 6 && parts[3] == "commands" && project.SubordinateName(parts[4]) == nil &&
				parts[5] == project.CommandModuleFile {
				return relative, "bsl", nil
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
	parts := strings.Split(relative, "/")
	if len(parts) == 4 && parts[0] == "metadata" && parts[1] == "common-forms" && parts[3] == project.FormMetadataFile {
		manifest, err := project.ValidateLayout(workspace.root)
		if err != nil {
			return nil, err
		}
		value, err := metadata.DecodeManagedForm(relative, bytes.NewReader(content), manifest)
		if err != nil {
			return nil, err
		}
		if !strings.EqualFold(value.Name, parts[2]) {
			return nil, fmt.Errorf("form %s does not match the folder %s it lies in", value.Name, parts[2])
		}
		var canonical bytes.Buffer
		if err := metadata.Encode(&canonical, value); err != nil {
			return nil, err
		}
		return canonical.Bytes(), nil
	}
	// A form of an object lies in a folder named after the form, so it is the
	// name that has to agree with where the file is. The identifier inside is
	// the form's own and is not checked against anything here: it is what
	// roles, the portal and ML App refer to the form by, not what finds it.
	if len(parts) == 6 && parts[0] == "metadata" && parts[3] == "forms" &&
		parts[5] == project.FormMetadataFile && slices.Contains(project.ObjectFolderKinds(), parts[1]) {
		manifest, err := project.ValidateLayout(workspace.root)
		if err != nil {
			return nil, err
		}
		value, err := metadata.DecodeManagedForm(relative, bytes.NewReader(content), manifest)
		if err != nil {
			return nil, err
		}
		if !strings.EqualFold(value.Name, parts[4]) {
			return nil, fmt.Errorf("form %s does not match the folder %s it lies in", value.Name, parts[4])
		}
		var canonical bytes.Buffer
		if err := metadata.Encode(&canonical, value); err != nil {
			return nil, err
		}
		return canonical.Bytes(), nil
	}
	// A flat kind is addressed by the identifier in its file name; a kind that
	// keeps a folder is addressed by the name of that folder. So one is
	// checked against the identifier inside the file and the other against the
	// name inside it.
	kindPart, filenameIDPart, folderPart := "", "", ""
	if len(parts) == 3 && parts[0] == "metadata" {
		kindPart, filenameIDPart = parts[1], strings.TrimSuffix(parts[2], ".yaml")
	} else if len(parts) == 4 && parts[0] == "metadata" && parts[3] == "object.yaml" && slices.Contains(project.ObjectFolderKinds(), parts[1]) {
		kindPart, folderPart = parts[1], parts[2]
	}
	if kindPart != "" {
		manifest, err := project.ValidateLayout(workspace.root)
		if err != nil {
			return nil, err
		}
		var value any
		switch metadata.Kind(kindPart) {
		case metadata.RoleKind:
			value, err = metadata.DecodeRole(relative, bytes.NewReader(content), manifest)
		case metadata.SubsystemKind:
			value, err = metadata.DecodeSubsystem(relative, bytes.NewReader(content), manifest)
		case metadata.ConstantKind:
			value, err = metadata.DecodeConstant(relative, bytes.NewReader(content), manifest)
		case metadata.SessionParameterKind:
			value, err = metadata.DecodeSessionParameter(relative, bytes.NewReader(content), manifest)
		case metadata.CommonAttributeKind:
			value, err = metadata.DecodeCommonAttribute(relative, bytes.NewReader(content), manifest)
		case metadata.CommonModuleKind:
			value, err = metadata.DecodeCommonModule(relative, bytes.NewReader(content), manifest)
		case metadata.EventSubscriptionKind:
			value, err = metadata.DecodeEventSubscription(relative, bytes.NewReader(content), manifest)
		case metadata.EnumerationKind:
			value, err = metadata.DecodeEnumeration(relative, bytes.NewReader(content), manifest)
		case metadata.DefinedTypeKind:
			value, err = metadata.DecodeDefinedType(relative, bytes.NewReader(content), manifest)
		case metadata.CatalogKind:
			value, err = metadata.DecodeCatalog(relative, bytes.NewReader(content), manifest)
		case metadata.ChartOfCharacteristicTypesKind:
			value, err = metadata.DecodeChartOfCharacteristicTypes(relative, bytes.NewReader(content), manifest)
		case metadata.ChartOfAccountsKind:
			value, err = metadata.DecodeChartOfAccounts(relative, bytes.NewReader(content), manifest)
		case metadata.ChartOfCalculationTypesKind:
			value, err = metadata.DecodeChartOfCalculationTypes(relative, bytes.NewReader(content), manifest)
		case metadata.BusinessProcessKind:
			value, err = metadata.DecodeBusinessProcess(relative, bytes.NewReader(content), manifest)
		case metadata.ExchangePlanKind:
			value, err = metadata.DecodeExchangePlan(relative, bytes.NewReader(content), manifest)
		case metadata.AccountingRegisterKind:
			value, err = metadata.DecodeAccountingRegister(relative, bytes.NewReader(content), manifest)
		case metadata.ReportKind:
			value, err = metadata.DecodeReport(relative, bytes.NewReader(content), manifest)
		case metadata.DataProcessorKind:
			value, err = metadata.DecodeDataProcessor(relative, bytes.NewReader(content), manifest)
		case metadata.CalculationRegisterKind:
			value, err = metadata.DecodeCalculationRegister(relative, bytes.NewReader(content), manifest)
		case metadata.NumeratorKind:
			value, err = metadata.DecodeNumerator(relative, bytes.NewReader(content), manifest)
		case metadata.SequenceKind:
			value, err = metadata.DecodeSequence(relative, bytes.NewReader(content), manifest)
		case metadata.DocumentJournalKind:
			value, err = metadata.DecodeDocumentJournal(relative, bytes.NewReader(content), manifest)
		case metadata.TaskKind:
			value, err = metadata.DecodeTask(relative, bytes.NewReader(content), manifest)
		case metadata.DocumentKind:
			value, err = metadata.DecodeDocument(relative, bytes.NewReader(content), manifest)
		case metadata.InformationRegisterKind:
			value, err = metadata.DecodeInformationRegister(relative, bytes.NewReader(content), manifest)
		case metadata.AccumulationRegisterKind:
			value, err = metadata.DecodeAccumulationRegister(relative, bytes.NewReader(content), manifest)
		}
		if err != nil {
			return nil, err
		}
		if value != nil {
			filenameID, _ := uuid.Parse(filenameIDPart)
			var metadataID uuid.UUID
			switch item := value.(type) {
			case metadata.RoleDefinition:
				metadataID = item.ID
			case metadata.SubsystemDefinition:
				metadataID = item.ID
			case metadata.Constant:
				metadataID = item.ID
			case metadata.SessionParameter:
				metadataID = item.ID
			case metadata.CommonAttributeDefinition:
				metadataID = item.ID
			case metadata.CommonModuleDefinition:
				metadataID = item.ID
			case metadata.EventSubscriptionDefinition:
				metadataID = item.ID
			case metadata.Enumeration:
				metadataID = item.ID
			case metadata.DefinedTypeObject:
				metadataID = item.ID
			case metadata.CatalogDefinition:
				metadataID = item.ID
			case metadata.DocumentDefinition:
				metadataID = item.ID
			case metadata.InformationRegisterDefinition:
				metadataID = item.ID
			case metadata.AccumulationRegisterDefinition:
				metadataID = item.ID
			}
			if folderPart != "" {
				if name := yamlScalarField(content, "name"); name != folderPart {
					return nil, fmt.Errorf("object %s lies in folder %s", name, folderPart)
				}
			} else if metadataID != filenameID {
				return nil, fmt.Errorf("metadata UUID %s does not match filename UUID %s", metadataID, filenameID)
			}
			var canonical bytes.Buffer
			if err := metadata.Encode(&canonical, value); err != nil {
				return nil, err
			}
			return canonical.Bytes(), nil
		}
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
