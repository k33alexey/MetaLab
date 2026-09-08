package metadata

import (
	"fmt"
	"strings"

	"github.com/k33alexey/MetaLab/internal/uuid"
)

// FormKind identifies the three standard managed forms of a reference object.
type FormKind string

const (
	ObjectForm FormKind = "object"
	ListForm   FormKind = "list"
	ChoiceForm FormKind = "choice"
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
	form, err := catalog.baseForm(CatalogKind, definition.ID, definition.Name, definition.Title, definition.Forms, kind, language)
	if err != nil || !form.Generated {
		return form, err
	}
	form.Fields = []FormField{
		systemFormField("Code", definition.Code.Type, language, kind != ObjectForm),
		systemFormField("Description", StringType, language, kind != ObjectForm),
	}
	if kind == ObjectForm {
		form.Fields = append(form.Fields, attributeFormFields(definition.Attributes, language)...)
		form.TableParts = tablePartForms(definition.TableParts, language)
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
	if err != nil || !form.Generated {
		return form, err
	}
	form.Fields = []FormField{
		systemFormField("Number", definition.Number.Type, language, kind != ObjectForm),
		systemFormField("Date", DateType, language, kind != ObjectForm),
		systemFormField("Posted", BooleanType, language, true),
	}
	if kind == ObjectForm {
		form.Fields = append(form.Fields, attributeFormFields(definition.Attributes, language)...)
		form.TableParts = tablePartForms(definition.TableParts, language)
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
		Title: title.Resolve(language, catalog.Project.Languages), Generated: true,
	}
	if result.Title == "" {
		result.Title = name
	}
	var source *uuid.UUID
	switch kind {
	case ObjectForm:
		source = forms.Object
	case ListForm:
		source = forms.List
	case ChoiceForm:
		source = forms.Choice
	}
	if source != nil {
		copy := *source
		result.SourceID, result.Generated = &copy, false
	}
	return result, nil
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

func attributeFormFields(attributes []Attribute, language string) []FormField {
	result := make([]FormField, len(attributes))
	for index, attribute := range attributes {
		title := attribute.Title.Resolve(language, nil)
		if title == "" {
			title = attribute.Name
		}
		result[index] = FormField{Name: attribute.Name, Title: title, Types: cloneTypes(attribute.Types), Required: attribute.Required}
	}
	return result
}

func tablePartForms(parts []TablePart, language string) []FormTablePart {
	result := make([]FormTablePart, len(parts))
	for index, part := range parts {
		title := part.Title.Resolve(language, nil)
		if title == "" {
			title = part.Name
		}
		result[index] = FormTablePart{Name: part.Name, Title: title, Columns: attributeFormFields(part.Attributes, language)}
	}
	return result
}

func systemFormField(name string, kind TypeKind, language string, readOnly bool) FormField {
	return FormField{Name: name, Title: formText(language, strings.ToLower(name)), Types: []Type{{Kind: kind}}, ReadOnly: readOnly}
}

func standardObjectCommands(language string, posting, movements bool) []FormCommand {
	result := []FormCommand{{Name: "Save", Title: formText(language, "save")}}
	if posting {
		result = append(result,
			FormCommand{Name: "Post", Title: formText(language, "post")},
			FormCommand{Name: "UndoPosting", Title: formText(language, "undo-posting")},
		)
	}
	if movements {
		result = append(result, FormCommand{Name: "Movements", Title: formText(language, "movements")})
	}
	return append(result, FormCommand{Name: "SetDeletionMark", Title: formText(language, "deletion-mark")})
}

func standardListCommands(language string, kind FormKind) []FormCommand {
	if kind == ChoiceForm {
		return []FormCommand{{Name: "Choose", Title: formText(language, "choose")}, {Name: "Refresh", Title: formText(language, "refresh")}}
	}
	return []FormCommand{{Name: "Create", Title: formText(language, "create")}, {Name: "Refresh", Title: formText(language, "refresh")}}
}

func formText(language, key string) string {
	texts := map[string]map[string]string{
		"ru": {"code": "Код", "description": "Наименование", "number": "Номер", "date": "Дата", "posted": "Проведён", "save": "Записать", "post": "Провести", "undo-posting": "Отменить проведение", "movements": "Движения документа", "deletion-mark": "Пометка удаления", "create": "Создать", "refresh": "Обновить", "choose": "Выбрать"},
		"uk": {"code": "Код", "description": "Найменування", "number": "Номер", "date": "Дата", "posted": "Проведений", "save": "Записати", "post": "Провести", "undo-posting": "Скасувати проведення", "movements": "Рухи документа", "deletion-mark": "Позначка видалення", "create": "Створити", "refresh": "Оновити", "choose": "Вибрати"},
		"en": {"code": "Code", "description": "Description", "number": "Number", "date": "Date", "posted": "Posted", "save": "Save", "post": "Post", "undo-posting": "Undo posting", "movements": "Document movements", "deletion-mark": "Deletion mark", "create": "Create", "refresh": "Refresh", "choose": "Choose"},
	}
	if text := texts[strings.ToLower(language)][key]; text != "" {
		return text
	}
	return texts["en"][key]
}
