package metadata

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"unicode/utf8"

	"github.com/k33alexey/MetaLab/internal/bsl/bytecode"
	"github.com/k33alexey/MetaLab/internal/uuid"
)

type catalogObject struct {
	mu         sync.RWMutex
	definition CatalogDefinition
	record     *CatalogRecord
	tables     map[uuid.UUID]bytecode.Value
	runtime    *Runtime
}

type catalogReferenceObject struct {
	definition CatalogDefinition
	reference  CatalogReference
	runtime    *Runtime
}

func (object *catalogObject) RuntimeTypeName() string {
	return "CatalogObject." + object.definition.Name
}
func (object *catalogObject) RuntimeEqual(other bytecode.RuntimeObject) bool {
	candidate, ok := other.(*catalogObject)
	return ok && candidate == object
}
func (object *catalogObject) RuntimeDynamicMemory(limit uint64) (uint64, bool) {
	object.mu.RLock()
	defer object.mu.RUnlock()
	size := uint64(1024 + len(object.record.Code) + len(object.record.Description))
	for _, value := range object.record.Attributes {
		if size > limit {
			return limit, false
		}
		size += uint64(len(value.Data) + 96)
	}
	for _, table := range object.tables {
		if size > limit {
			return limit, false
		}
		current, ok := table.DynamicMemory(limit - size)
		if !ok {
			return limit, false
		}
		size += current
	}
	return size, size <= limit
}

func (reference *catalogReferenceObject) RuntimeTypeName() string {
	return "CatalogRef." + reference.definition.Name
}
func (reference *catalogReferenceObject) RuntimeEqual(other bytecode.RuntimeObject) bool {
	candidate, ok := other.(*catalogReferenceObject)
	return ok && candidate.reference == reference.reference
}
func (*catalogReferenceObject) RuntimeDynamicMemory(limit uint64) (uint64, bool) {
	return 192, limit >= 192
}

func (runtime *Runtime) CreateCatalogObject(ctx context.Context, name string) (bytecode.Value, error) {
	if runtime.catalogRepository == nil {
		return bytecode.Undefined(), fmt.Errorf("catalog repository is not configured")
	}
	definition, ok := runtime.catalog.CatalogDefinition(name)
	if !ok {
		return bytecode.Undefined(), fmt.Errorf("unknown catalog %q", name)
	}
	record, err := runtime.catalogRepository.New(ctx, name, runtime.catalogEventHandler(definition.ID))
	if err != nil {
		return bytecode.Undefined(), err
	}
	return runtime.wrapCatalogRecord(definition, record)
}

func (runtime *Runtime) GetCatalogObject(ctx context.Context, name string, value bytecode.Value) (bytecode.Value, error) {
	if runtime.catalogRepository == nil {
		return bytecode.Undefined(), fmt.Errorf("catalog repository is not configured")
	}
	definition, ok := runtime.catalog.CatalogDefinition(name)
	if !ok {
		return bytecode.Undefined(), fmt.Errorf("unknown catalog %q", name)
	}
	reference, err := runtime.catalogReference(definition, value)
	if err != nil {
		return bytecode.Undefined(), err
	}
	record, err := runtime.catalogRepository.Get(ctx, reference)
	if err != nil {
		return bytecode.Undefined(), err
	}
	return runtime.wrapCatalogRecord(definition, record)
}

func (runtime *Runtime) FindCatalogByCode(ctx context.Context, name string, value bytecode.Value) (bytecode.Value, error) {
	if runtime.catalogRepository == nil {
		return bytecode.Undefined(), fmt.Errorf("catalog repository is not configured")
	}
	code, err := bslScalarText(value)
	if err != nil {
		return bytecode.Undefined(), fmt.Errorf("catalog code: %w", err)
	}
	reference, found, err := runtime.catalogRepository.FindByCode(ctx, name, code)
	if err != nil {
		return bytecode.Undefined(), err
	}
	definition, ok := runtime.catalog.CatalogDefinition(name)
	if !ok {
		return bytecode.Undefined(), fmt.Errorf("unknown catalog %q", name)
	}
	if !found {
		reference = CatalogReference{CatalogID: definition.ID}
	}
	return runtime.wrapCatalogReference(definition, reference)
}

func (runtime *Runtime) GetCatalogReference(_ context.Context, name string, value bytecode.Value) (bytecode.Value, error) {
	definition, ok := runtime.catalog.CatalogDefinition(name)
	if !ok {
		return bytecode.Undefined(), fmt.Errorf("unknown catalog %q", name)
	}
	text, ok := value.AsString()
	if !ok {
		return bytecode.Undefined(), fmt.Errorf("catalog reference UUID must be a string")
	}
	reference := CatalogReference{CatalogID: definition.ID}
	if strings.TrimSpace(text) != "" {
		id, err := uuid.Parse(text)
		if err != nil {
			return bytecode.Undefined(), fmt.Errorf("invalid catalog reference UUID: %w", err)
		}
		reference.ObjectID = id
	}
	return runtime.wrapCatalogReference(definition, reference)
}

func (runtime *Runtime) GetObjectProperty(_ context.Context, value bytecode.RuntimeObject, name string) (bytecode.Value, error) {
	switch object := value.(type) {
	case *catalogObject:
		if object.runtime != runtime {
			return bytecode.Undefined(), fmt.Errorf("catalog object belongs to another metadata runtime")
		}
		object.mu.RLock()
		defer object.mu.RUnlock()
		switch {
		case propertyName(name, "Ссылка", "Ref"):
			return runtime.wrapCatalogReference(object.definition, object.record.Reference)
		case propertyName(name, "Код", "Code"):
			if object.definition.Code.Type == NumberType {
				return bytecode.ParseNumber(object.record.Code)
			}
			return bytecode.String(object.record.Code), nil
		case propertyName(name, "Наименование", "Description"):
			return bytecode.String(object.record.Description), nil
		case propertyName(name, "Версия", "Version"):
			return bytecode.ParseNumber(fmt.Sprint(object.record.Version))
		}
		if attribute, ok := findCatalogAttribute(object.definition.Attributes, name); ok {
			stored, present := object.record.Attributes[attribute.ID]
			if !present {
				return bytecode.Undefined(), nil
			}
			return runtime.applicationValueToBSL(attribute.Types, stored)
		}
		if part, ok := findCatalogTablePart(object.definition.TableParts, name); ok {
			return object.tables[part.ID], nil
		}
		return bytecode.Undefined(), fmt.Errorf("%s has no property %s", object.RuntimeTypeName(), name)
	case *catalogReferenceObject:
		if object.runtime != runtime {
			return bytecode.Undefined(), fmt.Errorf("catalog reference belongs to another metadata runtime")
		}
		switch {
		case propertyName(name, "UUID", "UUID"):
			if object.reference.ObjectID.IsZero() {
				return bytecode.String(""), nil
			}
			return bytecode.String(object.reference.ObjectID.String()), nil
		}
		return bytecode.Undefined(), fmt.Errorf("%s has no property %s", object.RuntimeTypeName(), name)
	case *documentObject:
		if object.runtime != runtime {
			return bytecode.Undefined(), fmt.Errorf("document object belongs to another metadata runtime")
		}
		return runtime.getDocumentProperty(object, name)
	case *documentReferenceObject:
		if object.runtime != runtime {
			return bytecode.Undefined(), fmt.Errorf("document reference belongs to another metadata runtime")
		}
		if propertyName(name, "UUID", "UUID") {
			if object.reference.ObjectID.IsZero() {
				return bytecode.String(""), nil
			}
			return bytecode.String(object.reference.ObjectID.String()), nil
		}
		return bytecode.Undefined(), fmt.Errorf("%s has no property %s", object.RuntimeTypeName(), name)
	default:
		return bytecode.Undefined(), fmt.Errorf("unsupported application object %T", value)
	}
}

func (runtime *Runtime) SetObjectProperty(_ context.Context, value bytecode.RuntimeObject, name string, assigned bytecode.Value) error {
	if object, ok := value.(*documentObject); ok {
		if object.runtime != runtime {
			return fmt.Errorf("document object belongs to another metadata runtime")
		}
		return runtime.setDocumentProperty(object, name, assigned)
	}
	object, ok := value.(*catalogObject)
	if !ok || object.runtime != runtime {
		return fmt.Errorf("%s property %s is not writable", value.RuntimeTypeName(), name)
	}
	object.mu.Lock()
	defer object.mu.Unlock()
	switch {
	case propertyName(name, "Код", "Code"):
		text, err := bslScalarText(assigned)
		if err != nil {
			return err
		}
		text, err = normalizeCatalogCode(object.definition.Code, text)
		if err != nil {
			return err
		}
		object.record.Code = text
		return nil
	case propertyName(name, "Наименование", "Description"):
		text, ok := assigned.AsString()
		if !ok || !utf8.ValidString(text) || utf8.RuneCountInString(text) > object.definition.DescriptionLength {
			return fmt.Errorf("description must be a string of at most %d characters", object.definition.DescriptionLength)
		}
		object.record.Description = text
		return nil
	case propertyName(name, "Ссылка", "Ref"), propertyName(name, "Версия", "Version"):
		return fmt.Errorf("catalog property %s is read-only", name)
	}
	attribute, ok := findCatalogAttribute(object.definition.Attributes, name)
	if !ok {
		if _, tablePart := findCatalogTablePart(object.definition.TableParts, name); tablePart {
			return fmt.Errorf("catalog table part %s is read-only", name)
		}
		return fmt.Errorf("%s has no property %s", object.RuntimeTypeName(), name)
	}
	if assigned.Kind() == bytecode.UndefinedKind {
		if attribute.Required {
			return fmt.Errorf("catalog attribute %s is required", attribute.Name)
		}
		delete(object.record.Attributes, attribute.ID)
		return nil
	}
	converted, err := runtime.applicationValueFromBSL(attribute.Types, assigned, "attribute "+attribute.Name)
	if err != nil {
		return err
	}
	object.record.Attributes[attribute.ID] = converted
	return nil
}

func (runtime *Runtime) CallObjectMethod(ctx context.Context, value bytecode.RuntimeObject, name string, arguments []bytecode.Value) (bytecode.Value, error) {
	switch object := value.(type) {
	case *catalogObject:
		if object.runtime != runtime {
			return bytecode.Undefined(), fmt.Errorf("catalog object belongs to another metadata runtime")
		}
		switch {
		case propertyName(name, "Записать", "Write"):
			if len(arguments) != 0 {
				return bytecode.Undefined(), fmt.Errorf("%s expects no arguments", name)
			}
			object.mu.Lock()
			defer object.mu.Unlock()
			if err := runtime.syncCatalogTables(object); err != nil {
				return bytecode.Undefined(), err
			}
			if err := runtime.catalogRepository.Save(ctx, object.record, runtime.catalogEventHandler(object.definition.ID)); err != nil {
				return bytecode.Undefined(), err
			}
			return bytecode.Undefined(), nil
		case propertyName(name, "ПолучитьСсылку", "GetRef"), propertyName(name, "ПолучитьСсылку", "GetReference"):
			if len(arguments) != 0 {
				return bytecode.Undefined(), fmt.Errorf("%s expects no arguments", name)
			}
			object.mu.RLock()
			defer object.mu.RUnlock()
			return runtime.wrapCatalogReference(object.definition, object.record.Reference)
		}
	case *catalogReferenceObject:
		if object.runtime != runtime {
			return bytecode.Undefined(), fmt.Errorf("catalog reference belongs to another metadata runtime")
		}
		switch {
		case propertyName(name, "Пустая", "IsEmpty"):
			if len(arguments) != 0 {
				return bytecode.Undefined(), fmt.Errorf("%s expects no arguments", name)
			}
			return bytecode.Boolean(object.reference.ObjectID.IsZero()), nil
		case propertyName(name, "ПолучитьОбъект", "GetObject"):
			if len(arguments) != 0 {
				return bytecode.Undefined(), fmt.Errorf("%s expects no arguments", name)
			}
			if object.reference.ObjectID.IsZero() {
				return bytecode.Undefined(), ErrCatalogRecordNotFound
			}
			record, err := runtime.catalogRepository.Get(ctx, object.reference)
			if err != nil {
				return bytecode.Undefined(), err
			}
			return runtime.wrapCatalogRecord(object.definition, record)
		}
	case *documentObject:
		if object.runtime != runtime {
			return bytecode.Undefined(), fmt.Errorf("document object belongs to another metadata runtime")
		}
		return runtime.callDocumentMethod(ctx, object, name, arguments)
	case *documentReferenceObject:
		if object.runtime != runtime {
			return bytecode.Undefined(), fmt.Errorf("document reference belongs to another metadata runtime")
		}
		return runtime.callDocumentReferenceMethod(ctx, object, name, arguments)
	}
	return bytecode.Undefined(), fmt.Errorf("%s has no method %s", value.RuntimeTypeName(), name)
}

func (runtime *Runtime) wrapCatalogRecord(definition CatalogDefinition, record *CatalogRecord) (bytecode.Value, error) {
	object := &catalogObject{definition: definition, record: cloneCatalogRecord(record), tables: make(map[uuid.UUID]bytecode.Value), runtime: runtime}
	for _, part := range definition.TableParts {
		table, err := runtime.catalogTableToBSL(part, object.record.TableParts[part.ID])
		if err != nil {
			return bytecode.Undefined(), err
		}
		object.tables[part.ID] = table
	}
	return bytecode.Object(object)
}

func (runtime *Runtime) wrapCatalogReference(definition CatalogDefinition, reference CatalogReference) (bytecode.Value, error) {
	return bytecode.Object(&catalogReferenceObject{definition: definition, reference: reference, runtime: runtime})
}

func (runtime *Runtime) catalogReference(definition CatalogDefinition, value bytecode.Value) (CatalogReference, error) {
	if opaque, ok := value.AsRuntimeObject(); ok {
		if reference, ok := opaque.(*catalogReferenceObject); ok && reference.runtime == runtime && reference.reference.CatalogID == definition.ID {
			return reference.reference, nil
		}
	}
	text, ok := value.AsString()
	if !ok {
		return CatalogReference{}, fmt.Errorf("catalog %s reference is invalid", definition.Name)
	}
	id, err := uuid.Parse(text)
	if err != nil {
		return CatalogReference{}, fmt.Errorf("catalog %s reference is invalid: %w", definition.Name, err)
	}
	return CatalogReference{CatalogID: definition.ID, ObjectID: id}, nil
}

func (runtime *Runtime) catalogTableToBSL(part TablePart, rows []CatalogRow) (bytecode.Value, error) {
	table, err := bytecode.ConstructCollection("ТаблицаЗначений", nil)
	if err != nil {
		return bytecode.Undefined(), err
	}
	columns, err := bytecode.CollectionProperty(table, "Колонки")
	if err != nil {
		return bytecode.Undefined(), err
	}
	for _, attribute := range part.Attributes {
		if _, err := bytecode.CollectionMethod(columns, "Добавить", []bytecode.Value{bytecode.String(attribute.Name)}); err != nil {
			return bytecode.Undefined(), err
		}
	}
	for _, source := range rows {
		row, err := bytecode.CollectionMethod(table, "Добавить", nil)
		if err != nil {
			return bytecode.Undefined(), err
		}
		for _, attribute := range part.Attributes {
			stored, present := source.Values[attribute.ID]
			if !present {
				continue
			}
			value, err := runtime.applicationValueToBSL(attribute.Types, stored)
			if err != nil {
				return bytecode.Undefined(), err
			}
			if err := bytecode.SetCollectionProperty(row, attribute.Name, value); err != nil {
				return bytecode.Undefined(), err
			}
		}
	}
	return table, nil
}

func (runtime *Runtime) syncCatalogTables(object *catalogObject) error {
	for _, part := range object.definition.TableParts {
		table := object.tables[part.ID]
		length, ok := bytecode.CollectionLength(table)
		if !ok {
			return fmt.Errorf("catalog table part %s is invalid", part.Name)
		}
		rows := make([]CatalogRow, length)
		for index := range length {
			row, ok := bytecode.CollectionElement(table, index)
			if !ok {
				return fmt.Errorf("catalog table part %s row %d is invalid", part.Name, index+1)
			}
			rows[index].Values = make(map[uuid.UUID]Value)
			for _, attribute := range part.Attributes {
				value, err := bytecode.CollectionProperty(row, attribute.Name)
				if err != nil {
					return err
				}
				if value.Kind() == bytecode.UndefinedKind {
					if attribute.Required {
						return fmt.Errorf("catalog table part %s attribute %s is required", part.Name, attribute.Name)
					}
					continue
				}
				stored, err := runtime.applicationValueFromBSL(attribute.Types, value, "attribute "+part.Name+"."+attribute.Name)
				if err != nil {
					return err
				}
				rows[index].Values[attribute.ID] = stored
			}
		}
		object.record.TableParts[part.ID] = rows
	}
	return nil
}

func (runtime *Runtime) applicationValueFromBSL(types []Type, value bytecode.Value, owner string) (Value, error) {
	if opaque, ok := value.AsRuntimeObject(); ok {
		kind, metadataID, objectID, runtimeType := TypeKind(""), uuid.UUID{}, uuid.UUID{}, ""
		switch reference := opaque.(type) {
		case *catalogReferenceObject:
			if reference.runtime != runtime {
				return Value{}, fmt.Errorf("%s received an object reference from another metadata runtime", owner)
			}
			kind, metadataID, objectID, runtimeType = CatalogType, reference.reference.CatalogID, reference.reference.ObjectID, reference.RuntimeTypeName()
		case *documentReferenceObject:
			if reference.runtime != runtime {
				return Value{}, fmt.Errorf("%s received an object reference from another metadata runtime", owner)
			}
			kind, metadataID, objectID, runtimeType = DocumentType, reference.reference.DocumentID, reference.reference.ObjectID, reference.RuntimeTypeName()
		default:
			return Value{}, fmt.Errorf("%s cannot store %s", owner, value.String())
		}
		if objectID.IsZero() {
			return Value{}, fmt.Errorf("%s cannot store an empty object reference", owner)
		}
		allowed, err := runtime.catalog.allowsObjectReference(types, kind, metadataID)
		if err != nil {
			return Value{}, err
		}
		if !allowed {
			return Value{}, fmt.Errorf("%s does not allow %s", owner, runtimeType)
		}
		return runtime.catalog.normalizeTypes(owner, types, Value{Kind: kind, Data: objectID.String()})
	}
	constant := Constant{Name: owner, Types: types}
	return runtime.valueFromBSL(constant, value)
}

func (runtime *Runtime) applicationValueToBSL(types []Type, value Value) (bytecode.Value, error) {
	if value.Kind != CatalogType && value.Kind != DocumentType {
		return valueToBSL(value)
	}
	resolved, err := runtime.catalog.expandTypes(types, nil)
	if err != nil {
		return bytecode.Undefined(), err
	}
	for _, item := range resolved {
		if item.Kind == value.Kind && item.Reference != nil {
			id, err := uuid.Parse(value.Data)
			if err != nil {
				return bytecode.Undefined(), err
			}
			if item.Kind == DocumentType {
				definition, ok := runtime.catalog.DocumentByID(*item.Reference)
				if !ok {
					return bytecode.Undefined(), fmt.Errorf("unknown document %s", item.Reference)
				}
				return runtime.wrapDocumentReference(definition, DocumentReference{DocumentID: definition.ID, ObjectID: id})
			}
			definition, ok := runtime.catalog.CatalogByID(*item.Reference)
			if !ok {
				return bytecode.Undefined(), fmt.Errorf("unknown catalog %s", item.Reference)
			}
			return runtime.wrapCatalogReference(definition, CatalogReference{CatalogID: definition.ID, ObjectID: id})
		}
	}
	return bytecode.Undefined(), fmt.Errorf("object reference type is missing")
}

func findCatalogAttribute(attributes []Attribute, name string) (Attribute, bool) {
	for _, attribute := range attributes {
		if strings.EqualFold(attribute.Name, name) {
			return attribute, true
		}
	}
	return Attribute{}, false
}

func findCatalogTablePart(parts []TablePart, name string) (TablePart, bool) {
	for _, part := range parts {
		if strings.EqualFold(part.Name, name) {
			return part, true
		}
	}
	return TablePart{}, false
}

func propertyName(value, russian, english string) bool {
	return strings.EqualFold(value, russian) || strings.EqualFold(value, english)
}

func bslScalarText(value bytecode.Value) (string, error) {
	if text, ok := value.AsString(); ok {
		return text, nil
	}
	if text, ok := value.NumberText(); ok {
		return text, nil
	}
	return "", fmt.Errorf("expected a string or number")
}
