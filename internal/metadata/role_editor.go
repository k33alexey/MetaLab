package metadata

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"

	"github.com/k33alexey/MetaLab/internal/uuid"
)

// PermissionSchema is editor data, not an authenticated runtime catalog.
type PermissionSchema struct {
	Objects []PermissionObject `json:"objects"`
	Forms   []PermissionForm   `json:"forms"`
	// SessionParameters lists the names an access policy may compare a field
	// against, so the editor offers a choice instead of a free-text field that
	// only fails later, at publication.
	SessionParameters []string `json:"sessionParameters"`
}

type PermissionObject struct {
	ID         uuid.UUID             `json:"id"`
	Kind       Kind                  `json:"kind"`
	Name       string                `json:"name"`
	Title      LocalizedText         `json:"title"`
	Operations []PermissionOperation `json:"operations"`
	Fields     []PermissionField     `json:"fields"`
}

type PermissionField struct {
	Key        string                `json:"key"`
	Name       string                `json:"name"`
	Title      LocalizedText         `json:"title,omitempty"`
	Parent     string                `json:"parent,omitempty"`
	Operations []PermissionOperation `json:"operations"`
}

type PermissionForm struct {
	ID       uuid.UUID           `json:"id"`
	Name     string              `json:"name"`
	Title    LocalizedText       `json:"title"`
	Commands []PermissionCommand `json:"commands"`
}

type PermissionCommand struct {
	ID    uuid.UUID     `json:"id"`
	Name  string        `json:"name"`
	Title LocalizedText `json:"title"`
}

// LoadPermissionSchema deliberately ignores existing role references so a
// developer can repair them visually. It never returns executable metadata.
func LoadPermissionSchema(root string) (PermissionSchema, error) {
	catalog, err := load(root, false)
	if err != nil {
		return PermissionSchema{}, err
	}
	result := PermissionSchema{Objects: []PermissionObject{}, Forms: []PermissionForm{}, SessionParameters: []string{}}
	for _, item := range catalog.SessionParameters {
		result.SessionParameters = append(result.SessionParameters, item.Name)
	}
	sort.Strings(result.SessionParameters)
	// The platform's own current-user parameter is never declared by a project,
	// yet "restrict to the rows of the current user" is the restriction most
	// policies are written for. Listing it first makes it reachable in the
	// editor instead of being a name only a hand-edited YAML could use.
	result.SessionParameters = append([]string{CurrentUserParameter}, result.SessionParameters...)
	appendObject := func(kind Kind, id uuid.UUID, name string, title LocalizedText, attributes []Attribute, parts []TablePart) {
		target, _ := catalog.permissionTarget(id)
		object := PermissionObject{ID: id, Kind: kind, Name: name, Title: cloneTitle(title), Fields: []PermissionField{}}
		for _, operation := range objectOperations {
			if target.operations[operation] {
				object.Operations = append(object.Operations, operation)
			}
		}
		appendField := func(key, name, parent string, title LocalizedText) {
			field := PermissionField{Key: key, Name: name, Parent: parent, Title: title, Operations: []PermissionOperation{PermissionRead}}
			if target.fields[key] {
				field.Operations = append(field.Operations, PermissionUpdate)
			}
			object.Fields = append(object.Fields, field)
		}
		var standard []string
		for key := range target.fields {
			if _, err := uuid.Parse(key); err != nil {
				standard = append(standard, key)
			}
		}
		slices.Sort(standard)
		for _, key := range standard {
			appendField(key, key, "", nil)
		}
		for _, field := range attributes {
			appendField(field.ID.String(), field.Name, "", cloneTitle(field.Title))
		}
		for _, part := range parts {
			appendField(part.ID.String(), part.Name, "", cloneTitle(part.Title))
			for _, field := range part.Attributes {
				appendField(field.ID.String(), field.Name, part.ID.String(), cloneTitle(field.Title))
			}
		}
		result.Objects = append(result.Objects, object)
	}
	for _, item := range catalog.Constants {
		appendObject(ConstantKind, item.ID, item.Name, item.Title, nil, nil)
	}
	for _, item := range catalog.Enumerations {
		appendObject(EnumerationKind, item.ID, item.Name, item.Title, nil, nil)
	}
	for _, item := range catalog.Catalogs {
		appendObject(CatalogKind, item.ID, item.Name, item.Title, item.Attributes, item.TableParts)
	}
	for _, item := range catalog.Documents {
		appendObject(DocumentKind, item.ID, item.Name, item.Title, item.Attributes, item.TableParts)
	}
	for _, item := range catalog.InformationRegisters {
		appendObject(InformationRegisterKind, item.ID, item.Name, item.Title, informationRegisterFields(item), nil)
	}
	for _, item := range catalog.AccumulationRegisters {
		appendObject(AccumulationRegisterKind, item.ID, item.Name, item.Title, accumulationRegisterFields(item), nil)
	}
	sort.Slice(result.Objects, func(i, j int) bool {
		a, b := result.Objects[i], result.Objects[j]
		if a.Kind != b.Kind {
			return a.Kind < b.Kind
		}
		return strings.ToLower(a.Name) < strings.ToLower(b.Name)
	})
	entries, err := os.ReadDir(filepath.Join(root, "forms"))
	if err != nil {
		return PermissionSchema{}, err
	}
	if len(entries) > maxObjectsPerKind+1 {
		return PermissionSchema{}, fmt.Errorf("too many form sources")
	}
	for _, entry := range entries {
		if entry.Name() == ".gitkeep" {
			continue
		}
		id, err := uuid.Parse(strings.TrimSuffix(entry.Name(), ".yaml"))
		if err != nil || filepath.Ext(entry.Name()) != ".yaml" || !entry.Type().IsRegular() {
			return PermissionSchema{}, fmt.Errorf("invalid form source %q", entry.Name())
		}
		form, err := catalog.readRoleForm(root, id)
		if err != nil {
			return PermissionSchema{}, err
		}
		if len(form.Commands) == 0 {
			continue
		}
		item := PermissionForm{ID: form.ID, Name: form.Name, Title: cloneTitle(form.Title)}
		for _, command := range form.Commands {
			item.Commands = append(item.Commands, PermissionCommand{ID: command.ID, Name: command.Name, Title: cloneTitle(command.Title)})
		}
		result.Forms = append(result.Forms, item)
	}
	return result, nil
}

// ValidateProjectRole checks the edited role against current objects and forms,
// without requiring other, possibly unfinished roles to have valid references.
func ValidateProjectRole(root string, role RoleDefinition) error {
	catalog, err := load(root, false)
	if err != nil {
		return err
	}
	catalog.Roles = []RoleDefinition{cloneRole(role)}
	return catalog.indexAndValidate(root)
}
