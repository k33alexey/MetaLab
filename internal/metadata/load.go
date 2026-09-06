package metadata

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"

	"github.com/k33alexey/MetaLab/internal/project"
	"github.com/k33alexey/MetaLab/internal/uuid"
)

// Load reads and cross-validates constants, enumerations and defined types.
func Load(root string) (*Catalog, error) {
	manifest, err := project.ValidateLayout(root)
	if err != nil {
		return nil, err
	}
	catalog := &Catalog{Project: manifest}
	if err := loadKind(root, ConstantKind, func(source string, file *os.File, id uuid.UUID) error {
		value, err := DecodeConstant(source, file, manifest)
		if err == nil && value.ID != id {
			err = fmt.Errorf("metadata UUID %s does not match filename UUID %s", value.ID, id)
		}
		if err == nil {
			catalog.Constants = append(catalog.Constants, value)
		}
		return err
	}); err != nil {
		return nil, err
	}
	if err := loadKind(root, EnumerationKind, func(source string, file *os.File, id uuid.UUID) error {
		value, err := DecodeEnumeration(source, file, manifest)
		if err == nil && value.ID != id {
			err = fmt.Errorf("metadata UUID %s does not match filename UUID %s", value.ID, id)
		}
		if err == nil {
			catalog.Enumerations = append(catalog.Enumerations, value)
		}
		return err
	}); err != nil {
		return nil, err
	}
	if err := loadKind(root, DefinedTypeKind, func(source string, file *os.File, id uuid.UUID) error {
		value, err := DecodeDefinedType(source, file, manifest)
		if err == nil && value.ID != id {
			err = fmt.Errorf("metadata UUID %s does not match filename UUID %s", value.ID, id)
		}
		if err == nil {
			catalog.DefinedTypes = append(catalog.DefinedTypes, value)
		}
		return err
	}); err != nil {
		return nil, err
	}
	if err := loadKind(root, CatalogKind, func(source string, file *os.File, id uuid.UUID) error {
		value, err := DecodeCatalog(source, file, manifest)
		if err == nil && value.ID != id {
			err = fmt.Errorf("metadata UUID %s does not match filename UUID %s", value.ID, id)
		}
		if err == nil {
			catalog.Catalogs = append(catalog.Catalogs, value)
		}
		return err
	}); err != nil {
		return nil, err
	}
	if err := catalog.indexAndValidate(root); err != nil {
		return nil, err
	}
	return catalog, nil
}

// NewCatalogSnapshot validates already decoded metadata, for example from a publication package.
func NewCatalogSnapshot(manifest project.Project, constants []Constant, enumerations []Enumeration, definedTypes []DefinedTypeObject, catalogs []CatalogDefinition) (*Catalog, error) {
	result := &Catalog{
		Project: manifest, Constants: slices.Clone(constants), Enumerations: slices.Clone(enumerations),
		DefinedTypes: slices.Clone(definedTypes), Catalogs: slices.Clone(catalogs),
	}
	for index := range result.Constants {
		result.Constants[index] = cloneConstant(result.Constants[index])
	}
	for index := range result.Enumerations {
		result.Enumerations[index] = cloneEnumeration(result.Enumerations[index])
	}
	for index := range result.DefinedTypes {
		result.DefinedTypes[index] = cloneDefinedType(result.DefinedTypes[index])
	}
	for index := range result.Catalogs {
		result.Catalogs[index] = cloneCatalogDefinition(result.Catalogs[index])
	}
	if err := result.indexAndValidate(""); err != nil {
		return nil, err
	}
	return result, nil
}

func loadKind(root string, kind Kind, decode func(string, *os.File, uuid.UUID) error) error {
	directory := filepath.Join(root, "metadata", string(kind))
	info, err := os.Lstat(directory)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("inspect metadata %s: %w", kind, err)
	}
	if !info.IsDir() || info.Mode()&fs.ModeSymlink != 0 {
		return fmt.Errorf("metadata path %q must be a directory without symbolic links", filepath.Join("metadata", string(kind)))
	}
	entries, err := os.ReadDir(directory)
	if err != nil {
		return fmt.Errorf("read metadata %s: %w", kind, err)
	}
	count := 0
	for _, entry := range entries {
		if entry.Name() == ".gitkeep" {
			continue
		}
		count++
		if count > maxObjectsPerKind {
			return fmt.Errorf("metadata %s exceeds %d objects", kind, maxObjectsPerKind)
		}
		if entry.IsDir() || entry.Type()&fs.ModeSymlink != 0 || filepath.Ext(entry.Name()) != ".yaml" {
			return fmt.Errorf("unexpected metadata source %q", filepath.Join("metadata", string(kind), entry.Name()))
		}
		id, err := uuid.Parse(strings.TrimSuffix(entry.Name(), ".yaml"))
		if err != nil {
			return fmt.Errorf("metadata source %q must use a UUID filename: %w", entry.Name(), err)
		}
		relative := filepath.ToSlash(filepath.Join("metadata", string(kind), entry.Name()))
		file, err := os.Open(filepath.Join(directory, entry.Name()))
		if err != nil {
			return fmt.Errorf("open %s: %w", relative, err)
		}
		decodeErr := decode(relative, file, id)
		closeErr := file.Close()
		if decodeErr != nil {
			return fmt.Errorf("load %s: %w", relative, decodeErr)
		}
		if closeErr != nil {
			return fmt.Errorf("close %s: %w", relative, closeErr)
		}
	}
	return nil
}

func (catalog *Catalog) indexAndValidate(root string) error {
	sort.Slice(catalog.Constants, func(i, j int) bool { return catalog.Constants[i].ID.String() < catalog.Constants[j].ID.String() })
	sort.Slice(catalog.Enumerations, func(i, j int) bool { return catalog.Enumerations[i].ID.String() < catalog.Enumerations[j].ID.String() })
	sort.Slice(catalog.DefinedTypes, func(i, j int) bool { return catalog.DefinedTypes[i].ID.String() < catalog.DefinedTypes[j].ID.String() })
	sort.Slice(catalog.Catalogs, func(i, j int) bool { return catalog.Catalogs[i].ID.String() < catalog.Catalogs[j].ID.String() })
	catalog.constantByName, catalog.constantByID = make(map[string]int, len(catalog.Constants)), make(map[uuid.UUID]int, len(catalog.Constants))
	catalog.enumerationByName, catalog.enumerationByID = make(map[string]int, len(catalog.Enumerations)), make(map[uuid.UUID]int, len(catalog.Enumerations))
	catalog.definedTypeByName, catalog.definedTypeByID = make(map[string]int, len(catalog.DefinedTypes)), make(map[uuid.UUID]int, len(catalog.DefinedTypes))
	catalog.catalogByName, catalog.catalogByID = make(map[string]int, len(catalog.Catalogs)), make(map[uuid.UUID]int, len(catalog.Catalogs))
	allIDs := map[uuid.UUID]string{}
	add := func(kind string, id uuid.UUID, name string, index int, names map[string]int, ids map[uuid.UUID]int) error {
		folded := strings.ToLower(name)
		if _, exists := names[folded]; exists {
			return fmt.Errorf("%w: %s %q", ErrDuplicateName, kind, name)
		}
		if previous, exists := allIDs[id]; exists {
			return fmt.Errorf("%w: %s and %s use %s", ErrDuplicateID, previous, kind+" "+name, id)
		}
		names[folded], allIDs[id] = index, kind+" "+name
		if ids != nil {
			ids[id] = index
		}
		return nil
	}
	for index, item := range catalog.Constants {
		if err := add("constant", item.ID, item.Name, index, catalog.constantByName, catalog.constantByID); err != nil {
			return err
		}
	}
	for index, item := range catalog.Enumerations {
		if err := add("enumeration", item.ID, item.Name, index, catalog.enumerationByName, catalog.enumerationByID); err != nil {
			return err
		}
		for _, value := range item.Values {
			if previous, ok := allIDs[value.ID]; ok {
				return fmt.Errorf("%w: %s and enumeration value %s.%s use %s", ErrDuplicateID, previous, item.Name, value.Name, value.ID)
			}
			allIDs[value.ID] = "enumeration value " + item.Name + "." + value.Name
		}
	}
	for index, item := range catalog.DefinedTypes {
		if err := add("defined type", item.ID, item.Name, index, catalog.definedTypeByName, catalog.definedTypeByID); err != nil {
			return err
		}
	}
	for index, item := range catalog.Catalogs {
		if err := add("catalog", item.ID, item.Name, index, catalog.catalogByName, catalog.catalogByID); err != nil {
			return err
		}
		for _, attribute := range item.Attributes {
			if previous, ok := allIDs[attribute.ID]; ok {
				return fmt.Errorf("%w: %s and catalog attribute %s.%s use %s", ErrDuplicateID, previous, item.Name, attribute.Name, attribute.ID)
			}
			allIDs[attribute.ID] = "catalog attribute " + item.Name + "." + attribute.Name
		}
		for _, part := range item.TableParts {
			if previous, ok := allIDs[part.ID]; ok {
				return fmt.Errorf("%w: %s and catalog table part %s.%s use %s", ErrDuplicateID, previous, item.Name, part.Name, part.ID)
			}
			allIDs[part.ID] = "catalog table part " + item.Name + "." + part.Name
			for _, attribute := range part.Attributes {
				if previous, ok := allIDs[attribute.ID]; ok {
					return fmt.Errorf("%w: %s and catalog table part attribute %s.%s.%s use %s", ErrDuplicateID, previous, item.Name, part.Name, attribute.Name, attribute.ID)
				}
				allIDs[attribute.ID] = "catalog table part attribute " + item.Name + "." + part.Name + "." + attribute.Name
			}
		}
	}
	for _, item := range catalog.Constants {
		if err := catalog.validateReferences("constant "+item.Name, item.Types); err != nil {
			return err
		}
		if item.Default != nil {
			if _, err := catalog.NormalizeValue(item, *item.Default); err != nil {
				return fmt.Errorf("constant %s default: %w", item.Name, err)
			}
		}
	}
	for _, item := range catalog.DefinedTypes {
		if err := catalog.validateReferences("defined type "+item.Name, item.Types); err != nil {
			return err
		}
	}
	for _, item := range catalog.Catalogs {
		for _, attribute := range item.Attributes {
			if err := catalog.validateReferences("catalog "+item.Name+" attribute "+attribute.Name, attribute.Types); err != nil {
				return err
			}
		}
		for _, part := range item.TableParts {
			for _, attribute := range part.Attributes {
				if err := catalog.validateReferences("catalog "+item.Name+" table part "+part.Name+" attribute "+attribute.Name, attribute.Types); err != nil {
					return err
				}
			}
		}
		for role, module := range map[string]*uuid.UUID{"object": item.ObjectModule, "manager": item.ManagerModule} {
			if module == nil {
				continue
			}
			if root == "" {
				continue
			}
			path := filepath.Join(root, "modules", module.String()+".bsl")
			info, err := os.Lstat(path)
			if err != nil || !info.Mode().IsRegular() || info.Mode()&fs.ModeSymlink != 0 {
				return fmt.Errorf("catalog %s %s module %s is missing or unsafe", item.Name, role, module)
			}
		}
	}
	return catalog.validateDefinedTypeCycles()
}

func (catalog *Catalog) validateReferences(owner string, types []Type) error {
	for _, item := range types {
		if item.Reference == nil {
			continue
		}
		switch item.Kind {
		case EnumerationType:
			if _, ok := catalog.enumerationByID[*item.Reference]; !ok {
				return fmt.Errorf("%s references unknown enumeration %s", owner, item.Reference)
			}
		case DefinedType:
			if _, ok := catalog.definedTypeByID[*item.Reference]; !ok {
				return fmt.Errorf("%s references unknown defined type %s", owner, item.Reference)
			}
		case CatalogType:
			if _, ok := catalog.catalogByID[*item.Reference]; !ok {
				return fmt.Errorf("%s references unknown catalog %s", owner, item.Reference)
			}
		}
	}
	return nil
}

func (catalog *Catalog) validateDefinedTypeCycles() error {
	state := make(map[uuid.UUID]uint8, len(catalog.DefinedTypes))
	var visit func(uuid.UUID) error
	visit = func(id uuid.UUID) error {
		if state[id] == 1 {
			return fmt.Errorf("defined type reference cycle contains %s", id)
		}
		if state[id] == 2 {
			return nil
		}
		state[id] = 1
		item := catalog.DefinedTypes[catalog.definedTypeByID[id]]
		for _, allowed := range item.Types {
			if allowed.Kind == DefinedType && allowed.Reference != nil {
				if err := visit(*allowed.Reference); err != nil {
					return err
				}
			}
		}
		state[id] = 2
		return nil
	}
	for _, item := range catalog.DefinedTypes {
		if err := visit(item.ID); err != nil {
			return err
		}
	}
	return nil
}
