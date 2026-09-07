package metadata

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/k33alexey/MetaLab/internal/bsl/bytecode"
	"github.com/k33alexey/MetaLab/internal/uuid"
)

type documentObject struct {
	mu         sync.RWMutex
	definition DocumentDefinition
	record     *DocumentRecord
	tables     map[uuid.UUID]bytecode.Value
	runtime    *Runtime
}

type documentReferenceObject struct {
	definition DocumentDefinition
	reference  DocumentReference
	runtime    *Runtime
}

func (object *documentObject) RuntimeTypeName() string {
	return "DocumentObject." + object.definition.Name
}
func (object *documentObject) RuntimeEqual(other bytecode.RuntimeObject) bool {
	candidate, ok := other.(*documentObject)
	return ok && candidate == object
}
func (object *documentObject) RuntimeDynamicMemory(limit uint64) (uint64, bool) {
	object.mu.RLock()
	defer object.mu.RUnlock()
	size := uint64(1024 + len(object.record.Number))
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

func (reference *documentReferenceObject) RuntimeTypeName() string {
	return "DocumentRef." + reference.definition.Name
}
func (reference *documentReferenceObject) RuntimeEqual(other bytecode.RuntimeObject) bool {
	candidate, ok := other.(*documentReferenceObject)
	return ok && candidate.reference == reference.reference
}
func (*documentReferenceObject) RuntimeDynamicMemory(limit uint64) (uint64, bool) {
	return 192, limit >= 192
}

func (runtime *Runtime) CreateDocumentObject(ctx context.Context, name string) (bytecode.Value, error) {
	if runtime.documentRepository == nil {
		return bytecode.Undefined(), fmt.Errorf("document repository is not configured")
	}
	definition, ok := runtime.catalog.DocumentDefinition(name)
	if !ok {
		return bytecode.Undefined(), fmt.Errorf("unknown document %q", name)
	}
	record, err := runtime.documentRepository.New(ctx, name, runtime.documentEventHandler(definition.ID))
	if err != nil {
		return bytecode.Undefined(), err
	}
	return runtime.wrapDocumentRecord(definition, record)
}

func (runtime *Runtime) GetDocumentObject(ctx context.Context, name string, value bytecode.Value) (bytecode.Value, error) {
	if runtime.documentRepository == nil {
		return bytecode.Undefined(), fmt.Errorf("document repository is not configured")
	}
	definition, ok := runtime.catalog.DocumentDefinition(name)
	if !ok {
		return bytecode.Undefined(), fmt.Errorf("unknown document %q", name)
	}
	reference, err := runtime.documentReference(definition, value)
	if err != nil {
		return bytecode.Undefined(), err
	}
	record, err := runtime.documentRepository.Get(ctx, reference)
	if err != nil {
		return bytecode.Undefined(), err
	}
	return runtime.wrapDocumentRecord(definition, record)
}

func (runtime *Runtime) FindDocumentByNumber(ctx context.Context, name string, value, dateValue bytecode.Value) (bytecode.Value, error) {
	if runtime.documentRepository == nil {
		return bytecode.Undefined(), fmt.Errorf("document repository is not configured")
	}
	number, err := bslScalarText(value)
	if err != nil {
		return bytecode.Undefined(), fmt.Errorf("document number: %w", err)
	}
	var date *time.Time
	if dateValue.Kind() != bytecode.UndefinedKind {
		parsed, ok := dateValue.AsDate()
		if !ok {
			return bytecode.Undefined(), fmt.Errorf("document number period must be a date")
		}
		date = &parsed
	}
	reference, found, err := runtime.documentRepository.FindByNumber(ctx, name, number, date)
	if err != nil {
		return bytecode.Undefined(), err
	}
	definition, ok := runtime.catalog.DocumentDefinition(name)
	if !ok {
		return bytecode.Undefined(), fmt.Errorf("unknown document %q", name)
	}
	if !found {
		reference = DocumentReference{DocumentID: definition.ID}
	}
	return runtime.wrapDocumentReference(definition, reference)
}

func (runtime *Runtime) GetDocumentReference(_ context.Context, name string, value bytecode.Value) (bytecode.Value, error) {
	definition, ok := runtime.catalog.DocumentDefinition(name)
	if !ok {
		return bytecode.Undefined(), fmt.Errorf("unknown document %q", name)
	}
	text, ok := value.AsString()
	if !ok {
		return bytecode.Undefined(), fmt.Errorf("document reference UUID must be a string")
	}
	reference := DocumentReference{DocumentID: definition.ID}
	if strings.TrimSpace(text) != "" {
		id, err := uuid.Parse(text)
		if err != nil {
			return bytecode.Undefined(), fmt.Errorf("invalid document reference UUID: %w", err)
		}
		reference.ObjectID = id
	}
	return runtime.wrapDocumentReference(definition, reference)
}

func (runtime *Runtime) getDocumentProperty(object *documentObject, name string) (bytecode.Value, error) {
	object.mu.RLock()
	defer object.mu.RUnlock()
	switch {
	case propertyName(name, "Ссылка", "Ref"):
		return runtime.wrapDocumentReference(object.definition, object.record.Reference)
	case propertyName(name, "Номер", "Number"):
		if object.definition.Number.Type == NumberType {
			return bytecode.ParseNumber(object.record.Number)
		}
		return bytecode.String(object.record.Number), nil
	case propertyName(name, "Дата", "Date"):
		return bytecode.Date(object.record.Date)
	case propertyName(name, "Проведен", "Posted"), propertyName(name, "Проведён", "Posted"):
		return bytecode.Boolean(object.record.Posted), nil
	case propertyName(name, "Версия", "Version"):
		return bytecode.ParseNumber(fmt.Sprint(object.record.Version))
	case propertyName(name, "ПометкаУдаления", "DeletionMark"):
		return bytecode.Boolean(object.record.DeletionMark), nil
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
}

func (runtime *Runtime) setDocumentProperty(object *documentObject, name string, assigned bytecode.Value) error {
	object.mu.Lock()
	defer object.mu.Unlock()
	switch {
	case propertyName(name, "Номер", "Number"):
		text, err := bslScalarText(assigned)
		if err != nil {
			return err
		}
		text, err = normalizeDocumentNumber(object.definition.Number, text)
		if err != nil {
			return err
		}
		object.record.Number = text
		return nil
	case propertyName(name, "Дата", "Date"):
		date, ok := assigned.AsDate()
		if !ok {
			return fmt.Errorf("document date must be a date")
		}
		object.record.Date = normalizeDocumentDate(date)
		return nil
	case propertyName(name, "Ссылка", "Ref"), propertyName(name, "Проведен", "Posted"),
		propertyName(name, "Проведён", "Posted"), propertyName(name, "Версия", "Version"),
		propertyName(name, "ПометкаУдаления", "DeletionMark"):
		return fmt.Errorf("document property %s is read-only", name)
	}
	attribute, ok := findCatalogAttribute(object.definition.Attributes, name)
	if !ok {
		if _, tablePart := findCatalogTablePart(object.definition.TableParts, name); tablePart {
			return fmt.Errorf("document table part %s is read-only", name)
		}
		return fmt.Errorf("%s has no property %s", object.RuntimeTypeName(), name)
	}
	if assigned.Kind() == bytecode.UndefinedKind {
		if attribute.Required {
			return fmt.Errorf("document attribute %s is required", attribute.Name)
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

func (runtime *Runtime) callDocumentMethod(ctx context.Context, object *documentObject, name string, arguments []bytecode.Value) (bytecode.Value, error) {
	switch {
	case propertyName(name, "Записать", "Write"):
		if len(arguments) != 0 {
			return bytecode.Undefined(), fmt.Errorf("%s expects no arguments", name)
		}
		if runtime.documentRepository == nil {
			return bytecode.Undefined(), fmt.Errorf("document repository is not configured")
		}
		object.mu.Lock()
		defer object.mu.Unlock()
		if err := runtime.syncDocumentTables(object); err != nil {
			return bytecode.Undefined(), err
		}
		if err := runtime.documentRepository.Save(ctx, object.record, runtime.documentEventHandler(object.definition.ID)); err != nil {
			return bytecode.Undefined(), err
		}
		return bytecode.Undefined(), nil
	case propertyName(name, "ПолучитьСсылку", "GetRef"), propertyName(name, "ПолучитьСсылку", "GetReference"):
		if len(arguments) != 0 {
			return bytecode.Undefined(), fmt.Errorf("%s expects no arguments", name)
		}
		object.mu.RLock()
		defer object.mu.RUnlock()
		return runtime.wrapDocumentReference(object.definition, object.record.Reference)
	case propertyName(name, "УстановитьПометкуУдаления", "SetDeletionMark"):
		if len(arguments) != 1 {
			return bytecode.Undefined(), fmt.Errorf("%s expects one boolean argument", name)
		}
		mark, ok := arguments[0].AsBoolean()
		if !ok {
			return bytecode.Undefined(), fmt.Errorf("%s expects one boolean argument", name)
		}
		object.mu.Lock()
		defer object.mu.Unlock()
		previous := cloneDocumentRecord(object.record)
		object.record.DeletionMark = mark
		if err := runtime.syncDocumentTables(object); err != nil {
			object.record = previous
			return bytecode.Undefined(), err
		}
		if err := runtime.documentRepository.Save(ctx, object.record, runtime.documentEventHandler(object.definition.ID)); err != nil {
			object.record = previous
			return bytecode.Undefined(), err
		}
		return bytecode.Undefined(), nil
	case propertyName(name, "Удалить", "Delete"):
		if len(arguments) != 0 {
			return bytecode.Undefined(), fmt.Errorf("%s expects no arguments", name)
		}
		object.mu.Lock()
		defer object.mu.Unlock()
		if _, err := runtime.documentRepository.Delete(ctx, object.record, runtime.documentEventHandler(object.definition.ID), runtime.actor); err != nil {
			return bytecode.Undefined(), err
		}
		return bytecode.Undefined(), nil
	default:
		return bytecode.Undefined(), fmt.Errorf("%s has no method %s", object.RuntimeTypeName(), name)
	}
}

func (runtime *Runtime) callDocumentReferenceMethod(ctx context.Context, object *documentReferenceObject, name string, arguments []bytecode.Value) (bytecode.Value, error) {
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
		if runtime.documentRepository == nil {
			return bytecode.Undefined(), fmt.Errorf("document repository is not configured")
		}
		if object.reference.ObjectID.IsZero() {
			return bytecode.Undefined(), ErrDocumentRecordNotFound
		}
		record, err := runtime.documentRepository.Get(ctx, object.reference)
		if err != nil {
			return bytecode.Undefined(), err
		}
		return runtime.wrapDocumentRecord(object.definition, record)
	default:
		return bytecode.Undefined(), fmt.Errorf("%s has no method %s", object.RuntimeTypeName(), name)
	}
}

func (runtime *Runtime) wrapDocumentRecord(definition DocumentDefinition, record *DocumentRecord) (bytecode.Value, error) {
	object := &documentObject{
		definition: definition, record: cloneDocumentRecord(record), tables: make(map[uuid.UUID]bytecode.Value), runtime: runtime,
	}
	for _, part := range definition.TableParts {
		table, err := runtime.catalogTableToBSL(part, object.record.TableParts[part.ID])
		if err != nil {
			return bytecode.Undefined(), err
		}
		object.tables[part.ID] = table
	}
	return bytecode.Object(object)
}

func (runtime *Runtime) wrapDocumentReference(definition DocumentDefinition, reference DocumentReference) (bytecode.Value, error) {
	return bytecode.Object(&documentReferenceObject{definition: definition, reference: reference, runtime: runtime})
}

func (runtime *Runtime) documentReference(definition DocumentDefinition, value bytecode.Value) (DocumentReference, error) {
	if opaque, ok := value.AsRuntimeObject(); ok {
		if reference, ok := opaque.(*documentReferenceObject); ok && reference.runtime == runtime && reference.reference.DocumentID == definition.ID {
			return reference.reference, nil
		}
	}
	text, ok := value.AsString()
	if !ok {
		return DocumentReference{}, fmt.Errorf("document %s reference is invalid", definition.Name)
	}
	id, err := uuid.Parse(text)
	if err != nil {
		return DocumentReference{}, fmt.Errorf("document %s reference is invalid: %w", definition.Name, err)
	}
	return DocumentReference{DocumentID: definition.ID, ObjectID: id}, nil
}

func (runtime *Runtime) syncDocumentTables(object *documentObject) error {
	for _, part := range object.definition.TableParts {
		table := object.tables[part.ID]
		length, ok := bytecode.CollectionLength(table)
		if !ok {
			return fmt.Errorf("document table part %s is invalid", part.Name)
		}
		rows := make([]DocumentRow, length)
		for index := range length {
			row, ok := bytecode.CollectionElement(table, index)
			if !ok {
				return fmt.Errorf("document table part %s row %d is invalid", part.Name, index+1)
			}
			rows[index].Values = make(map[uuid.UUID]Value)
			for _, attribute := range part.Attributes {
				value, err := bytecode.CollectionProperty(row, attribute.Name)
				if err != nil {
					return err
				}
				if value.Kind() == bytecode.UndefinedKind {
					if attribute.Required {
						return fmt.Errorf("document table part %s attribute %s is required", part.Name, attribute.Name)
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
