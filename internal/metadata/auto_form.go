package metadata

import (
	"fmt"
	"sort"
	"strings"

	"github.com/k33alexey/MetaLab/internal/uuid"
)

// FormKind identifies the three standard managed forms of a reference object.
type FormKind string

const (
	ObjectForm FormKind = "object"
	ListForm   FormKind = "list"
	ChoiceForm FormKind = "choice"
	CommonForm FormKind = "common"
)

// FormDescriptor is the first server-side form model consumed by ML App.
// A custom form keeps its stable source UUID; otherwise fields and commands are generated.
type FormDescriptor struct {
	Kind       FormKind
	ObjectKind Kind
	ObjectID   uuid.UUID
	ObjectName string
	Title      string
	SourceID   *uuid.UUID
	Generated  bool
	Fields     []FormField
	TableParts []FormTablePart
	Commands   []FormCommand
	List       ListSettings
}

type FormField struct {
	Name     string
	Title    string
	Types    []Type
	Required bool
	ReadOnly bool
}

type FormTablePart struct {
	Name    string
	Title   string
	Columns []FormField
}

type FormCommand struct {
	Name  string
	Title string
}

// CatalogForm resolves a custom form or creates the standard catalog form.
func (catalog *Catalog) CatalogForm(name string, kind FormKind, language string) (FormDescriptor, error) {
	definition, ok := catalog.CatalogDefinition(name)
	if !ok {
		return FormDescriptor{}, fmt.Errorf("unknown catalog %q", name)
	}
	form, err := catalog.baseForm(CatalogKind, definition.ID, definition.Name, definition.Title, definition.Forms.ObjectForms, kind, language)
	form.List = definition.List
	form.List.SearchFields = effectiveListSearchFields(definition.List, []string{"Description", "Code"})
	if err != nil || !form.Generated {
		return form, err
	}
	form.Fields = []FormField{
		systemFormField("Code", definition.Code.Type, language, kind != ObjectForm),
		systemFormField("Description", StringType, language, kind != ObjectForm),
	}
	if kind == ObjectForm {
		form.Fields = append(form.Fields, catalog.attributeFormFields(definition.Attributes, language)...)
		form.TableParts = catalog.tablePartForms(definition.TableParts, language)
		form.Commands = standardObjectCommands(language, false, false)
	} else {
		form.Commands = standardListCommands(language, kind)
	}
	return form, nil
}

// DocumentForm resolves a custom form or creates the standard document form.
func (catalog *Catalog) DocumentForm(name string, kind FormKind, language string) (FormDescriptor, error) {
	definition, ok := catalog.DocumentDefinition(name)
	if !ok {
		return FormDescriptor{}, fmt.Errorf("unknown document %q", name)
	}
	form, err := catalog.baseForm(DocumentKind, definition.ID, definition.Name, definition.Title, definition.Forms, kind, language)
	form.List = definition.List
	form.List.SearchFields = effectiveListSearchFields(definition.List, []string{"Number"})
	if err != nil || !form.Generated {
		return form, err
	}
	form.Fields = []FormField{
		systemFormField("Number", definition.Number.Type, language, kind != ObjectForm),
		systemFormField("Date", DateType, language, kind != ObjectForm),
		systemFormField("Posted", BooleanType, language, true),
	}
	if kind == ObjectForm {
		form.Fields = append(form.Fields, catalog.attributeFormFields(definition.Attributes, language)...)
		form.TableParts = catalog.tablePartForms(definition.TableParts, language)
		form.Commands = standardObjectCommands(language, definition.Posting, catalog.documentHasMovements(definition.ID))
	} else {
		form.Commands = standardListCommands(language, kind)
	}
	return form, nil
}

func (catalog *Catalog) baseForm(objectKind Kind, id uuid.UUID, name string, title LocalizedText, forms ObjectForms, kind FormKind, language string) (FormDescriptor, error) {
	if kind != ObjectForm && kind != ListForm && kind != ChoiceForm {
		return FormDescriptor{}, fmt.Errorf("invalid form kind %q", kind)
	}
	result := FormDescriptor{
		Kind: kind, ObjectKind: objectKind, ObjectID: id, ObjectName: name,
		Title: catalog.resolveTitle(title, language), Generated: true,
	}
	if result.Title == "" {
		result.Title = name
	}
	var slot string
	switch kind {
	case ObjectForm:
		slot = forms.Object
	case ListForm:
		slot = forms.List
	case ChoiceForm:
		slot = forms.Choice
	}
	// The slot names the form; the identifier comes from the form itself. That
	// is the part that travels: roles, the portal and ML App all refer to a
	// form by identifier, and none of them has to learn where the file lies.
	if id, ok := catalog.ObjectFormID(objectKind, name, slot); ok {
		result.SourceID, result.Generated = &id, false
	}
	return result, nil
}

// objectFormIndex is what one object's forms folder held: the object's name as
// written, and its forms by folded name.
type objectFormIndex struct {
	object string
	forms  map[string]objectFormRef
}

// objectFormRef is one form of an object: the name its folder was called, and
// the identifier the form declared inside itself.
type objectFormRef struct {
	name string
	id   uuid.UUID
}

// ObjectFormNames returns the names of the forms one object keeps, as written
// and sorted. The folder is the list of forms, so this is the only answer to
// "what forms does this object have".
func (catalog *Catalog) ObjectFormNames(objectKind Kind, object string) []string {
	index, ok := catalog.objectForms[objectKind][strings.ToLower(object)]
	if !ok {
		return nil
	}
	result := make([]string, 0, len(index.forms))
	for _, form := range index.forms {
		result = append(result, form.name)
	}
	sort.Strings(result)
	return result
}

// ObjectForms flattens the index of what forms each object keeps: every form
// of every object, with the object it belongs to and the identifier the form
// declared. It is what a published snapshot carries, and what anything else
// asking "which forms exist" goes through - the folders are the only place
// that knows. Sorted, because a snapshot is compared byte for byte against the
// one already applied to the database.
func (catalog *Catalog) ObjectForms() []RuntimeObjectForm {
	var result []RuntimeObjectForm
	for objectKind, objects := range catalog.objectForms {
		for _, index := range objects {
			for _, form := range index.forms {
				result = append(result, RuntimeObjectForm{
					ObjectKind: objectKind, Object: index.object, Name: form.name, ID: form.id,
				})
			}
		}
	}
	sort.Slice(result, func(left, right int) bool {
		if result[left].ObjectKind != result[right].ObjectKind {
			return result[left].ObjectKind < result[right].ObjectKind
		}
		if result[left].Object != result[right].Object {
			return result[left].Object < result[right].Object
		}
		return result[left].Name < result[right].Name
	})
	return result
}

// indexRuntimeObjectForms rebuilds the index out of what a published snapshot
// carried, so a running application resolves a slot to a form exactly as
// Studio does against the folders on disk.
func (catalog *Catalog) indexRuntimeObjectForms(forms []RuntimeObjectForm) {
	for _, form := range forms {
		if catalog.objectForms == nil {
			catalog.objectForms = map[Kind]map[string]objectFormIndex{}
		}
		if catalog.objectForms[form.ObjectKind] == nil {
			catalog.objectForms[form.ObjectKind] = map[string]objectFormIndex{}
		}
		object := strings.ToLower(form.Object)
		index, ok := catalog.objectForms[form.ObjectKind][object]
		if !ok {
			index = objectFormIndex{object: form.Object, forms: map[string]objectFormRef{}}
		}
		index.forms[strings.ToLower(form.Name)] = objectFormRef{name: form.Name, id: form.ID}
		catalog.objectForms[form.ObjectKind][object] = index
	}
}

// ObjectFormID returns the identifier of one of an object's own forms, found
// by the name a slot calls it. It reports false when the slot names nothing,
// which is how an object says it has no form of that role and takes the one
// the platform generates instead.
func (catalog *Catalog) ObjectFormID(objectKind Kind, object, form string) (uuid.UUID, bool) {
	if form == "" {
		return uuid.UUID{}, false
	}
	found, ok := catalog.objectForms[objectKind][strings.ToLower(object)].forms[strings.ToLower(form)]
	return found.id, ok
}

func (catalog *Catalog) documentHasMovements(documentID uuid.UUID) bool {
	for _, register := range catalog.InformationRegisters {
		if register.WriteMode == InformationRegisterRecorder && containsUUID(register.Recorders, documentID) {
			return true
		}
	}
	for _, register := range catalog.AccumulationRegisters {
		if containsUUID(register.Recorders, documentID) {
			return true
		}
	}
	return false
}

func containsUUID(values []uuid.UUID, expected uuid.UUID) bool {
	for _, value := range values {
		if value == expected {
			return true
		}
	}
	return false
}

func (catalog *Catalog) attributeFormFields(attributes []Attribute, language string) []FormField {
	result := make([]FormField, len(attributes))
	for index, attribute := range attributes {
		title := catalog.resolveTitle(attribute.Title, language)
		if title == "" {
			title = attribute.Name
		}
		result[index] = FormField{Name: attribute.Name, Title: title, Types: cloneTypes(attribute.Types), Required: attribute.FillChecking.checked()}
	}
	return result
}

func (catalog *Catalog) tablePartForms(parts []TablePart, language string) []FormTablePart {
	result := make([]FormTablePart, len(parts))
	for index, part := range parts {
		title := catalog.resolveTitle(part.Title, language)
		if title == "" {
			title = part.Name
		}
		result[index] = FormTablePart{Name: part.Name, Title: title, Columns: catalog.attributeFormFields(part.Attributes, language)}
	}
	return result
}

func systemFormField(name string, kind TypeKind, language string, readOnly bool) FormField {
	return FormField{Name: name, Title: formText(language, strings.ToLower(name)), Types: []Type{{Kind: kind}}, ReadOnly: readOnly}
}

func standardObjectCommands(language string, posting, movements bool) []FormCommand {
	result := []FormCommand{
		{Name: "Save", Title: formText(language, "save")},
		{Name: "SaveAndClose", Title: formText(language, "save-and-close")},
	}
	if posting {
		result = append(result,
			FormCommand{Name: "Post", Title: formText(language, "post")},
			FormCommand{Name: "UndoPosting", Title: formText(language, "undo-posting")},
		)
	}
	if movements {
		result = append(result, FormCommand{Name: "Movements", Title: formText(language, "movements")})
	}
	return append(result,
		FormCommand{Name: "SetDeletionMark", Title: formText(language, "deletion-mark")},
		FormCommand{Name: "Close", Title: formText(language, "close")},
	)
}

func standardListCommands(language string, kind FormKind) []FormCommand {
	if kind == ChoiceForm {
		return []FormCommand{{Name: "Choose", Title: formText(language, "choose")}, {Name: "Refresh", Title: formText(language, "refresh")}}
	}
	return []FormCommand{{Name: "Create", Title: formText(language, "create")}, {Name: "Refresh", Title: formText(language, "refresh")}}
}

func formText(language, key string) string {
	texts := map[string]map[string]string{
		"ru": {"code": "Код", "description": "Наименование", "number": "Номер", "date": "Дата", "posted": "Проведён", "save": "Записать", "save-and-close": "Записать и закрыть", "close": "Закрыть", "post": "Провести", "undo-posting": "Отменить проведение", "movements": "Движения документа", "deletion-mark": "Пометка удаления", "create": "Создать", "refresh": "Обновить", "choose": "Выбрать"},
		"uk": {"code": "Код", "description": "Найменування", "number": "Номер", "date": "Дата", "posted": "Проведений", "save": "Записати", "save-and-close": "Записати й закрити", "close": "Закрити", "post": "Провести", "undo-posting": "Скасувати проведення", "movements": "Рухи документа", "deletion-mark": "Позначка видалення", "create": "Створити", "refresh": "Оновити", "choose": "Вибрати"},
		"en": {"code": "Code", "description": "Description", "number": "Number", "date": "Date", "posted": "Posted", "save": "Save", "save-and-close": "Save and close", "close": "Close", "post": "Post", "undo-posting": "Undo posting", "movements": "Document movements", "deletion-mark": "Deletion mark", "create": "Create", "refresh": "Refresh", "choose": "Choose"},
	}
	if text := texts[strings.ToLower(language)][key]; text != "" {
		return text
	}
	return texts["en"][key]
}

// resolveTitle answers with the project's own fallback chain rather than with
// whatever translation happens to be stored first - see LocalizedText.Resolve.
func (catalog *Catalog) resolveTitle(text LocalizedText, language string) string {
	if catalog == nil {
		return text.Resolve(language, "", nil)
	}
	return text.Resolve(language, catalog.Project.DefaultLanguage, catalog.Project.Languages)
}
