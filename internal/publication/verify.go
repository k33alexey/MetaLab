package publication

import (
	"archive/zip"
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"path"
	"slices"
	"strings"

	"github.com/k33alexey/MetaLab/internal/metadata"
	"github.com/k33alexey/MetaLab/internal/project"
	"github.com/k33alexey/MetaLab/internal/schemadiff"
	"github.com/k33alexey/MetaLab/internal/uuid"
)

const maxPackageManifestBytes = 16 << 20

// VerifyFile validates the package format, archive paths and every source digest.
func VerifyFile(ctx context.Context, packagePath string) (Manifest, error) {
	archive, err := zip.OpenReader(packagePath)
	if err != nil {
		return Manifest{}, fmt.Errorf("open publication package: %w", err)
	}
	defer archive.Close()
	if len(archive.File) == 0 || archive.File[0].Name != "package.json" {
		return Manifest{}, fmt.Errorf("publication package manifest is missing")
	}
	manifest, err := readPackageManifest(archive.File[0])
	if err != nil {
		return Manifest{}, err
	}
	if len(archive.File) != len(manifest.Files)+1 {
		return Manifest{}, fmt.Errorf("publication package entries do not match its manifest")
	}
	contentHash := sha256.New()
	var total int64
	previous := ""
	manifestSourceFound := false
	for index, expected := range manifest.Files {
		if err := ctx.Err(); err != nil {
			return Manifest{}, err
		}
		if err := validatePackagePath(expected.Path); err != nil {
			return Manifest{}, err
		}
		if expected.Path == project.ManifestFile {
			manifestSourceFound = true
		} else if err := validateSourcePath(expected.Path, false); err != nil {
			return Manifest{}, err
		}
		if previous != "" && expected.Path <= previous {
			return Manifest{}, fmt.Errorf("publication package files must be unique and sorted")
		}
		previous = expected.Path
		entry := archive.File[index+1]
		if entry.Name != expected.Path || !entry.Mode().IsRegular() || int64(entry.UncompressedSize64) != expected.Size || expected.Size < 0 || expected.Size > maxSourceFileBytes {
			return Manifest{}, fmt.Errorf("publication entry %q does not match its manifest", expected.Path)
		}
		total += expected.Size
		if total > maxPackageInputBytes {
			return Manifest{}, fmt.Errorf("publication package sources exceed %d bytes", maxPackageInputBytes)
		}
		digest, err := digestZipEntry(ctx, entry, expected.Size)
		if err != nil {
			return Manifest{}, err
		}
		if digest != expected.SHA256 {
			return Manifest{}, fmt.Errorf("publication entry %q checksum mismatch", expected.Path)
		}
		_, _ = fmt.Fprintf(contentHash, "%s\x00%d\x00%s\n", expected.Path, expected.Size, expected.SHA256)
	}
	if !manifestSourceFound {
		return Manifest{}, fmt.Errorf("publication package does not contain %s", project.ManifestFile)
	}
	if hex.EncodeToString(contentHash.Sum(nil)) != manifest.ContentSHA256 {
		return Manifest{}, fmt.Errorf("publication package content checksum mismatch")
	}
	if err := verifyMetadataManifest(&archive.Reader, manifest); err != nil {
		return Manifest{}, err
	}
	return manifest, nil
}

func verifyMetadataManifest(archive *zip.Reader, manifest Manifest) error {
	var projectManifest project.Project
	var constants []metadata.Constant
	var enumerations []metadata.Enumeration
	var definedTypes []metadata.DefinedTypeObject
	var catalogs []metadata.CatalogDefinition
	var documents []metadata.DocumentDefinition
	moduleIDs := make(map[uuid.UUID]bool)
	formIDs := make(map[uuid.UUID]bool)
	for index, entry := range manifest.Files {
		if entry.Path == project.ManifestFile {
			content, err := readMetadataEntry(archive.File[index+1], entry.Size)
			if err != nil {
				return err
			}
			projectManifest, err = project.DecodeSource(entry.Path, bytes.NewReader(content))
			if err != nil {
				return err
			}
		}
		if strings.HasPrefix(entry.Path, "modules/") {
			id, err := uuid.Parse(strings.TrimSuffix(path.Base(entry.Path), ".bsl"))
			if err != nil {
				return fmt.Errorf("invalid module source %q", entry.Path)
			}
			moduleIDs[id] = true
		}
		if strings.HasPrefix(entry.Path, "forms/") {
			id, err := uuid.Parse(strings.TrimSuffix(path.Base(entry.Path), ".yaml"))
			if err != nil {
				return fmt.Errorf("invalid form source %q", entry.Path)
			}
			formIDs[id] = true
		}
	}
	if projectManifest.ID.IsZero() {
		return fmt.Errorf("publication project manifest is invalid")
	}
	for index, entry := range manifest.Files {
		kind := metadata.Kind("")
		switch {
		case strings.HasPrefix(entry.Path, "metadata/constants/"):
			kind = metadata.ConstantKind
		case strings.HasPrefix(entry.Path, "metadata/enumerations/"):
			kind = metadata.EnumerationKind
		case strings.HasPrefix(entry.Path, "metadata/defined-types/"):
			kind = metadata.DefinedTypeKind
		case strings.HasPrefix(entry.Path, "metadata/catalogs/"):
			kind = metadata.CatalogKind
		case strings.HasPrefix(entry.Path, "metadata/documents/"):
			kind = metadata.DocumentKind
		}
		if kind == "" {
			continue
		}
		content, err := readMetadataEntry(archive.File[index+1], entry.Size)
		if err != nil {
			return err
		}
		var id uuid.UUID
		switch kind {
		case metadata.ConstantKind:
			value, err := metadata.DecodeConstant(entry.Path, bytes.NewReader(content), projectManifest)
			if err != nil {
				return err
			}
			id, constants = value.ID, append(constants, value)
		case metadata.EnumerationKind:
			value, err := metadata.DecodeEnumeration(entry.Path, bytes.NewReader(content), projectManifest)
			if err != nil {
				return err
			}
			id, enumerations = value.ID, append(enumerations, value)
		case metadata.DefinedTypeKind:
			value, err := metadata.DecodeDefinedType(entry.Path, bytes.NewReader(content), projectManifest)
			if err != nil {
				return err
			}
			id, definedTypes = value.ID, append(definedTypes, value)
		case metadata.CatalogKind:
			value, err := metadata.DecodeCatalog(entry.Path, bytes.NewReader(content), projectManifest)
			if err != nil {
				return err
			}
			id, catalogs = value.ID, append(catalogs, value)
		case metadata.DocumentKind:
			value, err := metadata.DecodeDocument(entry.Path, bytes.NewReader(content), projectManifest)
			if err != nil {
				return err
			}
			id, documents = value.ID, append(documents, value)
		}
		filenameID, err := uuid.Parse(strings.TrimSuffix(path.Base(entry.Path), ".yaml"))
		if err != nil || filenameID != id {
			return fmt.Errorf("metadata UUID does not match %q", entry.Path)
		}
	}
	catalog, err := metadata.NewCatalogSnapshot(projectManifest, constants, enumerations, definedTypes, catalogs, documents)
	if err != nil {
		return fmt.Errorf("validate packaged metadata: %w", err)
	}
	for _, definition := range catalog.Catalogs {
		if err := verifyPackagedObjectSources("catalog", definition.Name, definition.ObjectModule, definition.ManagerModule, definition.Forms, moduleIDs, formIDs); err != nil {
			return err
		}
	}
	for _, definition := range catalog.Documents {
		if err := verifyPackagedObjectSources("document", definition.Name, definition.ObjectModule, definition.ManagerModule, definition.Forms, moduleIDs, formIDs); err != nil {
			return err
		}
	}
	constantIDs, catalogIDs, documentIDs := catalog.ConstantIDs(), catalog.CatalogIDs(), catalog.DocumentIDs()
	if !slices.Equal(constantIDs, manifest.ConstantIDs) {
		return fmt.Errorf("publication constant UUIDs do not match packaged metadata")
	}
	if !slices.Equal(catalogIDs, manifest.CatalogIDs) {
		return fmt.Errorf("publication catalog UUIDs do not match packaged metadata")
	}
	if !slices.Equal(documentIDs, manifest.DocumentIDs) {
		return fmt.Errorf("publication document UUIDs do not match packaged metadata")
	}
	applicationSchema, err := catalog.ApplicationSchema()
	if err != nil {
		return err
	}
	digest, err := schemadiff.SchemaSHA256(applicationSchema)
	if err != nil {
		return err
	}
	if digest != manifest.SchemaSHA256 {
		return fmt.Errorf("publication schema does not match packaged metadata")
	}
	return nil
}

func readMetadataEntry(entry *zip.File, expected int64) ([]byte, error) {
	if expected < 0 || expected > project.MaxYAMLDocumentBytes {
		return nil, fmt.Errorf("metadata entry %q is too large", entry.Name)
	}
	reader, err := entry.Open()
	if err != nil {
		return nil, fmt.Errorf("open metadata entry %q: %w", entry.Name, err)
	}
	content, readErr := io.ReadAll(io.LimitReader(reader, expected+1))
	closeErr := reader.Close()
	if readErr != nil {
		return nil, fmt.Errorf("read metadata entry %q: %w", entry.Name, readErr)
	}
	if closeErr != nil {
		return nil, fmt.Errorf("close metadata entry %q: %w", entry.Name, closeErr)
	}
	if int64(len(content)) != expected {
		return nil, fmt.Errorf("metadata entry %q size mismatch", entry.Name)
	}
	return content, nil
}

func readPackageManifest(entry *zip.File) (Manifest, error) {
	if entry.UncompressedSize64 > maxPackageManifestBytes {
		return Manifest{}, fmt.Errorf("publication package manifest is too large")
	}
	file, err := entry.Open()
	if err != nil {
		return Manifest{}, fmt.Errorf("open publication package manifest: %w", err)
	}
	defer file.Close()
	decoder := json.NewDecoder(bufio.NewReader(io.LimitReader(file, maxPackageManifestBytes+1)))
	decoder.DisallowUnknownFields()
	var manifest Manifest
	if err := decoder.Decode(&manifest); err != nil {
		return Manifest{}, fmt.Errorf("decode publication package manifest: %w", err)
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		return Manifest{}, fmt.Errorf("publication package manifest must contain one JSON document")
	}
	if manifest.Format != CurrentPackageFormat || manifest.ProjectFormat != project.CurrentFormat || manifest.ProjectID.IsZero() || strings.TrimSpace(manifest.ProjectName) == "" {
		return Manifest{}, fmt.Errorf("unsupported or invalid publication package manifest")
	}
	if err := validateGitCommit(manifest.GitCommit); err != nil {
		return Manifest{}, err
	}
	if len(manifest.ContentSHA256) != sha256.Size*2 {
		return Manifest{}, fmt.Errorf("invalid publication package content checksum")
	}
	if !validSHA256(manifest.SchemaSHA256) {
		return Manifest{}, fmt.Errorf("invalid publication application schema checksum")
	}
	for label, identifiers := range map[string][]uuid.UUID{"constant": manifest.ConstantIDs, "catalog": manifest.CatalogIDs, "document": manifest.DocumentIDs} {
		for index, id := range identifiers {
			if id.IsZero() {
				return Manifest{}, fmt.Errorf("publication %s UUID must not be zero", label)
			}
			if index > 0 && identifiers[index-1].String() >= id.String() {
				return Manifest{}, fmt.Errorf("publication %s UUIDs must be unique and sorted", label)
			}
		}
	}
	return manifest, nil
}

func verifyPackagedObjectSources(kind, name string, objectModule, managerModule *uuid.UUID, forms metadata.ObjectForms, modules, formFiles map[uuid.UUID]bool) error {
	type source struct {
		role string
		id   *uuid.UUID
		set  map[uuid.UUID]bool
	}
	for _, item := range []source{
		{role: "object module", id: objectModule, set: modules},
		{role: "manager module", id: managerModule, set: modules},
		{role: "object form", id: forms.Object, set: formFiles},
		{role: "list form", id: forms.List, set: formFiles},
		{role: "choice form", id: forms.Choice, set: formFiles},
	} {
		if item.id != nil && !item.set[*item.id] {
			return fmt.Errorf("%s %s %s %s is absent from publication package", kind, name, item.role, item.id)
		}
	}
	return nil
}

func digestZipEntry(ctx context.Context, entry *zip.File, expectedSize int64) (string, error) {
	file, err := entry.Open()
	if err != nil {
		return "", fmt.Errorf("open publication entry %q: %w", entry.Name, err)
	}
	defer file.Close()
	hash := sha256.New()
	written, err := copyContext(ctx, hash, io.LimitReader(file, expectedSize+1))
	if err != nil {
		return "", fmt.Errorf("read publication entry %q: %w", entry.Name, err)
	}
	if written != expectedSize {
		return "", fmt.Errorf("publication entry %q size mismatch", entry.Name)
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

func validatePackagePath(value string) error {
	if value == "" || strings.Contains(value, "\\") || path.IsAbs(value) || path.Clean(value) != value || strings.HasPrefix(value, "../") {
		return fmt.Errorf("unsafe publication package path %q", value)
	}
	return nil
}
