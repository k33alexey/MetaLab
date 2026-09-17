package platform

import (
	"context"
	"fmt"

	"github.com/k33alexey/MetaLab/internal/bsl/bytecode"
	"github.com/k33alexey/MetaLab/internal/metadata"
	"github.com/k33alexey/MetaLab/internal/uuid"
)

// ApplicationObjectState is what ML App reads to populate or refresh an
// object edit form: every standard field, declared attribute and table part
// row, keyed by name exactly as metadata.Attribute.Name/TablePart.Name spell
// it - the same names ApplicationForm's field descriptors already use.
type ApplicationObjectState struct {
	Reference    string                                 `json:"reference"`
	IsNew        bool                                   `json:"isNew"`
	Posted       bool                                   `json:"posted,omitempty"`
	DeletionMark bool                                   `json:"deletionMark"`
	Fields       map[string]metadata.Value              `json:"fields"`
	Tables       map[string][]map[string]metadata.Value `json:"tables,omitempty"`
}

// ApplicationObjectWrite is what ML App sends to save an object: an empty or
// "new" Reference creates one, anything else updates the existing object.
type ApplicationObjectWrite struct {
	Reference string                                 `json:"reference"`
	Fields    map[string]metadata.Value              `json:"fields"`
	Tables    map[string][]map[string]metadata.Value `json:"tables"`
}

// GetApplicationObject reads one catalog or document object's current state,
// or the defaults for a new one when reference is empty or "new".
func (runtime *Runtime) GetApplicationObject(ctx context.Context, token string, databaseID uuid.UUID, objectKind metadata.Kind, name, reference string) (ApplicationObjectState, error) {
	session, err := runtime.ResumePortalDatabase(ctx, token, databaseID)
	if err != nil {
		return ApplicationObjectState{}, err
	}
	ctx, applicationRuntime, catalog, closePool, err := runtime.openApplicationRuntime(ctx, databaseID, session.UserID)
	if err != nil {
		return ApplicationObjectState{}, err
	}
	defer closePool()
	value, isNew, err := openApplicationObjectValue(ctx, applicationRuntime, catalog, objectKind, name, reference)
	if err != nil {
		return ApplicationObjectState{}, err
	}
	return readApplicationObjectState(ctx, applicationRuntime, catalog, objectKind, name, value, isNew)
}

// SaveApplicationObject creates or updates one catalog or document object,
// running any ПередЗаписью/ПриЗаписи BSL hooks the object module declares.
func (runtime *Runtime) SaveApplicationObject(ctx context.Context, token string, databaseID uuid.UUID, objectKind metadata.Kind, name string, input ApplicationObjectWrite) (ApplicationObjectState, error) {
	session, err := runtime.ResumePortalDatabase(ctx, token, databaseID)
	if err != nil {
		return ApplicationObjectState{}, err
	}
	ctx, applicationRuntime, catalog, closePool, err := runtime.openApplicationRuntime(ctx, databaseID, session.UserID)
	if err != nil {
		return ApplicationObjectState{}, err
	}
	defer closePool()
	value, isNew, err := openApplicationObjectValue(ctx, applicationRuntime, catalog, objectKind, name, input.Reference)
	if err != nil {
		return ApplicationObjectState{}, err
	}
	objectValue, ok := value.AsRuntimeObject()
	if !ok {
		return ApplicationObjectState{}, fmt.Errorf("application object is not addressable")
	}
	if err := applyApplicationObjectWrite(ctx, applicationRuntime, catalog, objectKind, name, objectValue, input); err != nil {
		return ApplicationObjectState{}, err
	}
	if _, err := applicationRuntime.CallObjectMethod(ctx, objectValue, "Записать", nil); err != nil {
		return ApplicationObjectState{}, err
	}
	return readApplicationObjectState(ctx, applicationRuntime, catalog, objectKind, name, value, isNew)
}

// PostApplicationDocument runs Провести (regular posting mode), which
// dispatches ОбработкаПроведения and creates any accumulation register
// movements the document's object module declares.
func (runtime *Runtime) PostApplicationDocument(ctx context.Context, token string, databaseID uuid.UUID, name, reference string) (ApplicationObjectState, error) {
	return runtime.callApplicationDocumentMethod(ctx, token, databaseID, name, reference, "Провести", nil)
}

// UndoApplicationDocumentPosting runs ОтменитьПроведение.
func (runtime *Runtime) UndoApplicationDocumentPosting(ctx context.Context, token string, databaseID uuid.UUID, name, reference string) (ApplicationObjectState, error) {
	return runtime.callApplicationDocumentMethod(ctx, token, databaseID, name, reference, "ОтменитьПроведение", nil)
}

// SetApplicationDeletionMark runs УстановитьПометкуУдаления on a catalog or document object.
func (runtime *Runtime) SetApplicationDeletionMark(ctx context.Context, token string, databaseID uuid.UUID, objectKind metadata.Kind, name, reference string, mark bool) (ApplicationObjectState, error) {
	session, err := runtime.ResumePortalDatabase(ctx, token, databaseID)
	if err != nil {
		return ApplicationObjectState{}, err
	}
	ctx, applicationRuntime, catalog, closePool, err := runtime.openApplicationRuntime(ctx, databaseID, session.UserID)
	if err != nil {
		return ApplicationObjectState{}, err
	}
	defer closePool()
	value, _, err := openApplicationObjectValue(ctx, applicationRuntime, catalog, objectKind, name, reference)
	if err != nil {
		return ApplicationObjectState{}, err
	}
	objectValue, ok := value.AsRuntimeObject()
	if !ok {
		return ApplicationObjectState{}, fmt.Errorf("application object is not addressable")
	}
	if _, err := applicationRuntime.CallObjectMethod(ctx, objectValue, "УстановитьПометкуУдаления", []bytecode.Value{bytecode.Boolean(mark)}); err != nil {
		return ApplicationObjectState{}, err
	}
	return readApplicationObjectState(ctx, applicationRuntime, catalog, objectKind, name, value, false)
}

func (runtime *Runtime) callApplicationDocumentMethod(ctx context.Context, token string, databaseID uuid.UUID, name, reference, method string, arguments []bytecode.Value) (ApplicationObjectState, error) {
	session, err := runtime.ResumePortalDatabase(ctx, token, databaseID)
	if err != nil {
		return ApplicationObjectState{}, err
	}
	ctx, applicationRuntime, catalog, closePool, err := runtime.openApplicationRuntime(ctx, databaseID, session.UserID)
	if err != nil {
		return ApplicationObjectState{}, err
	}
	defer closePool()
	value, _, err := openApplicationObjectValue(ctx, applicationRuntime, catalog, metadata.DocumentKind, name, reference)
	if err != nil {
		return ApplicationObjectState{}, err
	}
	objectValue, ok := value.AsRuntimeObject()
	if !ok {
		return ApplicationObjectState{}, fmt.Errorf("application object is not addressable")
	}
	if _, err := applicationRuntime.CallObjectMethod(ctx, objectValue, method, arguments); err != nil {
		return ApplicationObjectState{}, err
	}
	return readApplicationObjectState(ctx, applicationRuntime, catalog, metadata.DocumentKind, name, value, false)
}

// openApplicationRuntime opens a fresh, BSL-wired application runtime for
// one request. There is no caching: every existing production read path
// (openApplicationPool) already opens a brand-new pool per call, and BSL
// compilation cost can be revisited later if it proves to matter in
// practice - building it in now would be speculative.
// openApplicationRuntime returns the context to use for the whole operation:
// it carries the caller's compiled permissions, which is what makes the
// requireObject/requireFields checks inside the metadata runtime - and every
// BSL call that inherits this context - actually enforce anything. Callers must
// use the returned context rather than the one they passed in.
func (runtime *Runtime) openApplicationRuntime(ctx context.Context, databaseID, actor uuid.UUID) (context.Context, *metadata.Runtime, *metadata.Catalog, func(), error) {
	snapshot, pool, err := runtime.loadPublishedMetadata(ctx, databaseID)
	if err != nil {
		return nil, nil, nil, nil, err
	}
	catalog, err := snapshot.Catalog()
	if err != nil {
		pool.Close()
		return nil, nil, nil, nil, err
	}
	permissions, err := runtime.applicationPermissions(ctx, databaseID, actor, catalog)
	if err != nil {
		pool.Close()
		return nil, nil, nil, nil, err
	}
	applicationRuntime, err := metadata.NewApplicationRuntime(pool, catalog, &actor)
	if err != nil {
		pool.Close()
		return nil, nil, nil, nil, err
	}
	if len(snapshot.Modules) > 0 {
		program, diagnostics, err := snapshot.CompileModules()
		if err != nil {
			pool.Close()
			return nil, nil, nil, nil, fmt.Errorf("compile BSL: %w (%v)", err, diagnostics)
		}
		if err := metadata.WireBSLEvents(applicationRuntime, program, catalog); err != nil {
			pool.Close()
			return nil, nil, nil, nil, err
		}
	}
	return metadata.WithPermissions(ctx, permissions), applicationRuntime, catalog, pool.Close, nil
}

func openApplicationObjectValue(ctx context.Context, applicationRuntime *metadata.Runtime, catalog *metadata.Catalog, objectKind metadata.Kind, name, reference string) (bytecode.Value, bool, error) {
	isNew := reference == "" || reference == "new"
	switch objectKind {
	case metadata.CatalogKind:
		if _, ok := catalog.CatalogDefinition(name); !ok {
			return bytecode.Undefined(), false, fmt.Errorf("unknown catalog %q", name)
		}
		if isNew {
			value, err := applicationRuntime.CreateCatalogObject(ctx, name)
			return value, true, err
		}
		value, err := applicationRuntime.GetCatalogObject(ctx, name, bytecode.String(reference))
		return value, false, err
	case metadata.DocumentKind:
		if _, ok := catalog.DocumentDefinition(name); !ok {
			return bytecode.Undefined(), false, fmt.Errorf("unknown document %q", name)
		}
		if isNew {
			value, err := applicationRuntime.CreateDocumentObject(ctx, name)
			return value, true, err
		}
		value, err := applicationRuntime.GetDocumentObject(ctx, name, bytecode.String(reference))
		return value, false, err
	default:
		return bytecode.Undefined(), false, fmt.Errorf("unsupported application object kind %q", objectKind)
	}
}

func applyApplicationObjectWrite(ctx context.Context, applicationRuntime *metadata.Runtime, catalog *metadata.Catalog, objectKind metadata.Kind, name string, objectValue bytecode.RuntimeObject, input ApplicationObjectWrite) error {
	var attributes []metadata.Attribute
	var tableParts []metadata.TablePart
	switch objectKind {
	case metadata.CatalogKind:
		definition, _ := catalog.CatalogDefinition(name)
		attributes, tableParts = definition.Attributes, definition.TableParts
	case metadata.DocumentKind:
		definition, _ := catalog.DocumentDefinition(name)
		attributes, tableParts = definition.Attributes, definition.TableParts
	}
	for _, attribute := range attributes {
		field, present := input.Fields[attribute.Name]
		if !present {
			continue
		}
		converted, err := applicationRuntime.ValueToBSL(attribute.Types, field)
		if err != nil {
			return fmt.Errorf("field %s: %w", attribute.Name, err)
		}
		if err := applicationRuntime.SetObjectProperty(ctx, objectValue, attribute.Name, converted); err != nil {
			return fmt.Errorf("field %s: %w", attribute.Name, err)
		}
	}
	for _, standard := range applicationStandardWritableFields(objectKind) {
		field, present := input.Fields[standard]
		if !present {
			continue
		}
		converted, err := metadata.StandardValueToBSL(field)
		if err != nil {
			return fmt.Errorf("field %s: %w", standard, err)
		}
		if err := applicationRuntime.SetObjectProperty(ctx, objectValue, standard, converted); err != nil {
			return fmt.Errorf("field %s: %w", standard, err)
		}
	}
	for _, part := range tableParts {
		rows, present := input.Tables[part.Name]
		if !present {
			continue
		}
		objectRows := make([]metadata.ObjectRow, len(rows))
		for index, row := range rows {
			values := make(map[uuid.UUID]metadata.Value, len(row))
			for _, attribute := range part.Attributes {
				if field, ok := row[attribute.Name]; ok {
					values[attribute.ID] = field
				}
			}
			objectRows[index] = metadata.ObjectRow{Values: values}
		}
		if err := applicationRuntime.SetObjectTableRows(ctx, objectValue, part.Name, objectRows); err != nil {
			return fmt.Errorf("table part %s: %w", part.Name, err)
		}
	}
	return nil
}

// applicationStandardWritableFields uses the same English names as
// auto_form.go's systemFormField (Code/Description/Number/Date) - the
// dataPath ML App's generated form actually sends - not the Russian BSL
// property names (Код/Наименование/Номер/Дата). GetObjectProperty and
// SetObjectProperty already accept both bilingually via propertyName().
func applicationStandardWritableFields(objectKind metadata.Kind) []string {
	switch objectKind {
	case metadata.CatalogKind:
		return []string{"Code", "Description"}
	case metadata.DocumentKind:
		return []string{"Number", "Date"}
	default:
		return nil
	}
}

func readApplicationObjectState(ctx context.Context, applicationRuntime *metadata.Runtime, catalog *metadata.Catalog, objectKind metadata.Kind, name string, value bytecode.Value, isNew bool) (ApplicationObjectState, error) {
	objectValue, ok := value.AsRuntimeObject()
	if !ok {
		return ApplicationObjectState{}, fmt.Errorf("application object is not addressable")
	}
	result := ApplicationObjectState{IsNew: isNew, Fields: map[string]metadata.Value{}, Tables: map[string][]map[string]metadata.Value{}}
	reference, err := applicationObjectReference(ctx, applicationRuntime, objectValue)
	if err != nil {
		return ApplicationObjectState{}, err
	}
	result.Reference = reference
	var attributes []metadata.Attribute
	var tableParts []metadata.TablePart
	switch objectKind {
	case metadata.CatalogKind:
		definition, _ := catalog.CatalogDefinition(name)
		attributes, tableParts = definition.Attributes, definition.TableParts
		if err := readApplicationStandardFields(ctx, applicationRuntime, objectValue, []string{"Code", "Description", "DeletionMark"}, result.Fields); err != nil {
			return ApplicationObjectState{}, err
		}
	case metadata.DocumentKind:
		definition, _ := catalog.DocumentDefinition(name)
		attributes, tableParts = definition.Attributes, definition.TableParts
		if err := readApplicationStandardFields(ctx, applicationRuntime, objectValue, []string{"Number", "Date", "Posted", "DeletionMark"}, result.Fields); err != nil {
			return ApplicationObjectState{}, err
		}
	}
	if mark, ok := result.Fields["DeletionMark"]; ok {
		result.DeletionMark = mark.Data == "true"
		delete(result.Fields, "DeletionMark")
	}
	if posted, ok := result.Fields["Posted"]; ok {
		result.Posted = posted.Data == "true"
		delete(result.Fields, "Posted")
	}
	for _, attribute := range attributes {
		raw, err := applicationRuntime.GetObjectProperty(ctx, objectValue, attribute.Name)
		if err != nil {
			return ApplicationObjectState{}, err
		}
		if raw.Kind() == bytecode.UndefinedKind {
			continue
		}
		converted, err := applicationRuntime.ValueFromBSL(attribute.Types, raw, attribute.Name)
		if err != nil {
			return ApplicationObjectState{}, err
		}
		result.Fields[attribute.Name] = converted
	}
	for _, part := range tableParts {
		rows, err := readApplicationTableRows(ctx, applicationRuntime, objectValue, part)
		if err != nil {
			return ApplicationObjectState{}, err
		}
		result.Tables[part.Name] = rows
	}
	return result, nil
}

func readApplicationStandardFields(ctx context.Context, applicationRuntime *metadata.Runtime, objectValue bytecode.RuntimeObject, names []string, target map[string]metadata.Value) error {
	for _, name := range names {
		raw, err := applicationRuntime.GetObjectProperty(ctx, objectValue, name)
		if err != nil {
			return err
		}
		target[name] = metadata.StandardValueFromBSL(raw)
	}
	return nil
}

func readApplicationTableRows(ctx context.Context, applicationRuntime *metadata.Runtime, objectValue bytecode.RuntimeObject, part metadata.TablePart) ([]map[string]metadata.Value, error) {
	table, err := applicationRuntime.GetObjectProperty(ctx, objectValue, part.Name)
	if err != nil {
		return nil, err
	}
	length, ok := bytecode.CollectionLength(table)
	if !ok {
		return nil, fmt.Errorf("table part %s is not a collection", part.Name)
	}
	rows := make([]map[string]metadata.Value, length)
	for index := range length {
		row, ok := bytecode.CollectionElement(table, index)
		if !ok {
			return nil, fmt.Errorf("table part %s row %d is invalid", part.Name, index+1)
		}
		values := make(map[string]metadata.Value, len(part.Attributes))
		for _, attribute := range part.Attributes {
			cell, err := bytecode.CollectionProperty(row, attribute.Name)
			if err != nil {
				return nil, err
			}
			if cell.Kind() == bytecode.UndefinedKind {
				continue
			}
			converted, err := applicationRuntime.ValueFromBSL(attribute.Types, cell, attribute.Name)
			if err != nil {
				return nil, err
			}
			values[attribute.Name] = converted
		}
		rows[index] = values
	}
	return rows, nil
}

func applicationObjectReference(ctx context.Context, applicationRuntime *metadata.Runtime, objectValue bytecode.RuntimeObject) (string, error) {
	referenceValue, err := applicationRuntime.GetObjectProperty(ctx, objectValue, "Ссылка")
	if err != nil {
		return "", err
	}
	referenceObject, ok := referenceValue.AsRuntimeObject()
	if !ok {
		return "", fmt.Errorf("application object reference is not addressable")
	}
	uuidValue, err := applicationRuntime.GetObjectProperty(ctx, referenceObject, "UUID")
	if err != nil {
		return "", err
	}
	text, _ := uuidValue.AsString()
	return text, nil
}
