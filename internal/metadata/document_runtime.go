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
	movements  *documentMovementsObject
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
	if object.movements != nil {
		current, ok := object.movements.RuntimeDynamicMemory(limit - size)
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
	if propertyName(name, "Движения", "Movements") {
		object.mu.Lock()
		defer object.mu.Unlock()
		if object.movements == nil {
			movements, err := runtime.newDocumentMovements(object.definition, object.record)
			if err != nil {
				return bytecode.Undefined(), err
			}
			object.movements = movements
		}
		return bytecode.Object(object.movements)
	}
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
		if object.movements != nil {
			object.movements.setDocument(object.record.Reference, object.record.Date)
		}
		return nil
	case propertyName(name, "Ссылка", "Ref"), propertyName(name, "Проведен", "Posted"),
		propertyName(name, "Проведён", "Posted"), propertyName(name, "Версия", "Version"),
		propertyName(name, "ПометкаУдаления", "DeletionMark"), propertyName(name, "Движения", "Movements"):
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
		writeMode, postingMode, err := documentWriteArguments(name, arguments)
		if err != nil {
			return bytecode.Undefined(), err
		}
		return bytecode.Undefined(), runtime.writeDocumentObject(ctx, object, writeMode, postingMode)
	case propertyName(name, "Провести", "Post"):
		if len(arguments) > 1 {
			return bytecode.Undefined(), fmt.Errorf("%s expects zero or one posting-mode argument", name)
		}
		postingMode := DocumentPostingRegular
		if len(arguments) == 1 {
			var err error
			postingMode, err = parseDocumentPostingMode(arguments[0])
			if err != nil {
				return bytecode.Undefined(), err
			}
		}
		return bytecode.Undefined(), runtime.writeDocumentObject(ctx, object, DocumentPost, postingMode)
	case propertyName(name, "ОтменитьПроведение", "UndoPosting"):
		if len(arguments) != 0 {
			return bytecode.Undefined(), fmt.Errorf("%s expects no arguments", name)
		}
		return bytecode.Undefined(), runtime.writeDocumentObject(ctx, object, DocumentUndoPosting, DocumentPostingRegular)
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

func (runtime *Runtime) writeDocumentObject(ctx context.Context, object *documentObject, writeMode DocumentWriteMode, postingMode DocumentPostingMode) error {
	if runtime.documentRepository == nil {
		return fmt.Errorf("document repository is not configured")
	}
	object.mu.Lock()
	defer object.mu.Unlock()
	previous := cloneDocumentRecord(object.record)
	if err := runtime.syncDocumentTables(object); err != nil {
		return err
	}
	handler := runtime.documentEventHandler(object.definition.ID)
	postingAction := func(operationContext context.Context, record *DocumentRecord, mode DocumentWriteMode, posting DocumentPostingMode) error {
		if err := runtime.clearDocumentMovements(operationContext, record.Reference); err != nil {
			return err
		}
		event := DocumentEventPosting
		if mode == DocumentUndoPosting {
			event = DocumentEventUndoPosting
		}
		return dispatchDocumentEvent(operationContext, handler, event, cloneDocumentRecord(record))
	}
	if err := runtime.documentRepository.Write(ctx, object.record, writeMode, postingMode, handler, postingAction); err != nil {
		object.record = previous
		return err
	}
	return nil
}

func (runtime *Runtime) clearDocumentMovements(ctx context.Context, recorder DocumentReference) error {
	if runtime.informationRegisterRepository != nil {
		for _, definition := range runtime.catalog.InformationRegisters {
			if definition.WriteMode != InformationRegisterRecorder || !allowedInformationRegisterRecorder(definition, recorder) {
				continue
			}
			set := &InformationRegisterRecordSet{
				RegisterID: definition.ID,
				Filter:     InformationRegisterFilter{Recorder: &recorder, Dimensions: map[uuid.UUID]Value{}},
				Records:    []*InformationRegisterRecord{},
			}
			if err := runtime.informationRegisterRepository.WriteWithHandler(ctx, set, true, runtime.informationRegisterEventHandler(definition.ID)); err != nil {
				return fmt.Errorf("clear document movements in information register %s: %w", definition.Name, err)
			}
		}
	}
	if runtime.accumulationRegisterRepository != nil {
		for _, definition := range runtime.catalog.AccumulationRegisters {
			if !allowedAccumulationRegisterRecorder(definition, recorder) {
				continue
			}
			set := &AccumulationRegisterRecordSet{
				RegisterID: definition.ID,
				Filter:     AccumulationRegisterFilter{Recorder: &recorder},
				Records:    []*AccumulationRegisterRecord{},
			}
			if err := runtime.accumulationRegisterRepository.WriteWithHandler(ctx, set, true, runtime.accumulationRegisterEventHandler(definition.ID)); err != nil {
				return fmt.Errorf("clear document movements in accumulation register %s: %w", definition.Name, err)
			}
		}
	}
	return nil
}

func documentWriteArguments(name string, arguments []bytecode.Value) (DocumentWriteMode, DocumentPostingMode, error) {
	if len(arguments) > 2 {
		return "", "", fmt.Errorf("%s expects zero, one or two arguments", name)
	}
	writeMode, postingMode := DocumentWrite, DocumentPostingRegular
	var err error
	if len(arguments) >= 1 {
		writeMode, err = parseDocumentWriteMode(arguments[0])
		if err != nil {
			return "", "", err
		}
	}
	if len(arguments) == 2 {
		if writeMode != DocumentPost {
			return "", "", fmt.Errorf("posting mode is only allowed when posting a document")
		}
		postingMode, err = parseDocumentPostingMode(arguments[1])
		if err != nil {
			return "", "", err
		}
	}
	return writeMode, postingMode, nil
}

func parseDocumentWriteMode(value bytecode.Value) (DocumentWriteMode, error) {
	text, ok := value.AsString()
	if !ok {
		return "", fmt.Errorf("document write mode must be a РежимЗаписиДокумента value")
	}
	switch strings.ToLower(strings.ReplaceAll(strings.TrimSpace(text), "_", "-")) {
	case "write", "запись":
		return DocumentWrite, nil
	case "post", "проведение":
		return DocumentPost, nil
	case "undo-posting", "undoposting", "отменапроведения", "отмена-проведения":
		return DocumentUndoPosting, nil
	default:
		return "", fmt.Errorf("document write mode %q is invalid", text)
	}
}

func parseDocumentPostingMode(value bytecode.Value) (DocumentPostingMode, error) {
	text, ok := value.AsString()
	if !ok {
		return "", fmt.Errorf("document posting mode must be a РежимПроведенияДокумента value")
	}
	switch strings.ToLower(strings.ReplaceAll(strings.TrimSpace(text), "_", "-")) {
	case "regular", "неоперативный", "неоперативное":
		return DocumentPostingRegular, nil
	case "real-time", "realtime", "оперативный", "оперативное":
		return DocumentPostingRealTime, nil
	default:
		return "", fmt.Errorf("document posting mode %q is invalid", text)
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
