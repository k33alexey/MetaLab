package metadata

import (
	"maps"
	"slices"
	"sort"
	"strings"
	"unicode"

	"github.com/k33alexey/MetaLab/internal/uuid"
)

// PermissionSchema is editor data, not an authenticated runtime catalog.
type PermissionSchema struct {
	// Rights names every right a role can carry, in the order of the help,
	// as the configurator writes them: the one place the editors take their
	// captions from.
	Rights  []PermissionRight  `json:"rights"`
	Objects []PermissionObject `json:"objects"`
	// Parts is everything a role may give rights on - the configuration, by
	// the project's identifier, and every object of it - with the keys of
	// its parts. The editor shows fewer objects than this; what it does not
	// show it must neither lose nor take for something deleted.
	Parts map[uuid.UUID][]string `json:"parts"`
	Forms []PermissionForm       `json:"forms"`
	// SessionParameters lists the names an access policy may compare a field
	// against, so the editor offers a choice instead of a free-text field that
	// only fails later, at publication.
	SessionParameters []string `json:"sessionParameters"`
}

type PermissionRight struct {
	Key   PermissionOperation `json:"key"`
	Title string              `json:"title"`
}

// nameInWords turns an identifier into words, the way the configurator
// shows a name it has no synonym for: ПометкаУдаления as Пометка удаления.
func nameInWords(name string) string {
	words := []rune(strings.ReplaceAll(kebab(name), "-", " "))
	if len(words) > 0 {
		words[0] = unicode.ToUpper(words[0])
	}
	return string(words)
}

type PermissionObject struct {
	ID         uuid.UUID             `json:"id"`
	Kind       Kind                  `json:"kind"`
	Name       string                `json:"name"`
	Title      LocalizedText         `json:"title"`
	Operations []PermissionOperation `json:"operations"`
	Fields     []PermissionField     `json:"fields"`
	// PolicyFields are the fields a restriction of rows may compare, under
	// the keys the restriction is checked and run by. They are not the parts
	// rights are given on: a restriction reads the record's identifier and
	// its version too, which have no rights of their own.
	PolicyFields []PermissionField `json:"policyFields"`
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
	for _, right := range roleRights {
		result.Rights = append(result.Rights, PermissionRight{Key: right.operation, Title: nameInWords(right.russian)})
	}
	result.Parts = map[uuid.UUID][]string{}
	for _, id := range append(slices.Collect(maps.Keys(catalog.objectKindByID)), catalog.Project.ID) {
		if parts, ok := catalog.rightsParts(id); ok {
			keys := slices.Sorted(maps.Keys(parts))
			if keys == nil {
				keys = []string{}
			}
			result.Parts[id] = keys
		}
	}
	for _, item := range catalog.SessionParameters {
		result.SessionParameters = append(result.SessionParameters, item.Name)
	}
	sort.Strings(result.SessionParameters)
	appendObject := func(kind Kind, id uuid.UUID, name string, title LocalizedText, attributes []Attribute, parts []TablePart) {
		target, _ := catalog.permissionTarget(id)
		object := PermissionObject{ID: id, Kind: kind, Name: name, Title: cloneTitle(title), Fields: []PermissionField{}}
		for _, operation := range objectOperations {
			if target.operations[operation] {
				object.Operations = append(object.Operations, operation)
			}
		}
		// A part of an object has the prototype's two rights of its own, view
		// and edit, whatever the part is: the configurator offers both on a
		// reference as on an attribute, and what the runtime lets be written
		// is its own business.
		appendField := func(key, name, parent string, title LocalizedText) {
			field := PermissionField{Key: key, Name: name, Parent: parent, Title: title,
				Operations: []PermissionOperation{PermissionView, PermissionEdit}}
			object.Fields = append(object.Fields, field)
			// A declared field is one key for both: its identifier.
			if _, err := uuid.Parse(key); err == nil && target.fields != nil {
				if _, ok := target.fields[key]; ok {
					object.PolicyFields = append(object.PolicyFields, PermissionField{Key: key, Name: name, Parent: parent, Title: title})
				}
			}
		}
		for _, field := range catalog.standardAttributeFields(kind, id) {
			appendField(strings.ToLower(field.en), field.ru, "", LocalizedText{catalog.Project.DefaultLanguage: nameInWords(field.ru)})
		}
		var standard []string
		for key := range target.fields {
			if _, err := uuid.Parse(key); err != nil {
				standard = append(standard, key)
			}
		}
		slices.Sort(standard)
		object.PolicyFields = []PermissionField{}
		captions := map[string]string{}
		for _, field := range catalog.standardAttributeFields(kind, id) {
			captions[strings.ToLower(field.en)] = nameInWords(field.ru)
		}
		for _, key := range standard {
			field := PermissionField{Key: key, Name: key}
			if caption, ok := captions[key]; ok {
				field.Title = LocalizedText{catalog.Project.DefaultLanguage: caption}
			}
			object.PolicyFields = append(object.PolicyFields, field)
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
	for _, item := range catalog.ChartsOfCharacteristicTypes {
		appendObject(ChartOfCharacteristicTypesKind, item.ID, item.Name, item.Title, item.Attributes, item.TableParts)
	}
	for _, item := range catalog.ChartsOfAccounts {
		// Accounting flags are fields of the account, so the editor has to show
		// them by name next to the attributes - a right on "Количественный" is
		// meaningless if the editor calls it by its UUID.
		fields := slices.Clone(item.Attributes)
		for _, flag := range append(slices.Clone(item.AccountingFlags), item.ExtDimensionAccountingFlags...) {
			fields = append(fields, Attribute{ID: flag.ID, Name: flag.Name, Title: flag.Title, Types: []Type{{Kind: BooleanType}}})
		}
		appendObject(ChartOfAccountsKind, item.ID, item.Name, item.Title, fields, item.TableParts)
	}
	for _, item := range catalog.ChartsOfCalculationTypes {
		appendObject(ChartOfCalculationTypesKind, item.ID, item.Name, item.Title, item.Attributes, item.TableParts)
	}
	for _, item := range catalog.BusinessProcesses {
		appendObject(BusinessProcessKind, item.ID, item.Name, item.Title, item.Attributes, item.TableParts)
	}
	for _, item := range catalog.ExchangePlans {
		appendObject(ExchangePlanKind, item.ID, item.Name, item.Title, item.Attributes, item.TableParts)
	}
	for _, item := range catalog.Sequences {
		appendObject(SequenceKind, item.ID, item.Name, item.Title, nil, nil)
	}
	for _, item := range catalog.DocumentJournals {
		appendObject(DocumentJournalKind, item.ID, item.Name, item.Title, nil, nil)
	}
	for _, item := range catalog.Tasks {
		// Addressing attributes are shown beside the rest: a right on
		// "Исполнитель" is exactly the kind of right an application restricts.
		appendObject(TaskKind, item.ID, item.Name, item.Title,
			append(addressingAsAttributes(item), item.Attributes...), item.TableParts)
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
	for _, item := range catalog.AccountingRegisters {
		appendObject(AccountingRegisterKind, item.ID, item.Name, item.Title,
			append(accountingRegisterFields(item), item.Attributes...), nil)
	}
	for _, item := range catalog.Reports {
		appendObject(ReportKind, item.ID, item.Name, item.Title, item.Attributes, item.TableParts)
	}
	for _, item := range catalog.DataProcessors {
		appendObject(DataProcessorKind, item.ID, item.Name, item.Title, item.Attributes, item.TableParts)
	}
	for _, item := range catalog.CalculationRegisters {
		appendObject(CalculationRegisterKind, item.ID, item.Name, item.Title,
			append(calculationRegisterFields(item), item.attributeFields()...), nil)
	}
	sort.Slice(result.Objects, func(i, j int) bool {
		a, b := result.Objects[i], result.Objects[j]
		if a.Kind != b.Kind {
			return a.Kind < b.Kind
		}
		return strings.ToLower(a.Name) < strings.ToLower(b.Name)
	})
	// A configuration without common forms has no directory for them, the
	// same as any other metadata kind nobody has used yet.
	forms, err := ReadCommonForms(root, catalog.Project)
	if err != nil {
		return PermissionSchema{}, err
	}
	for _, form := range forms {
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
