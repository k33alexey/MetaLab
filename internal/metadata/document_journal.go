package metadata

import (
	"fmt"
	"io"
	"slices"
	"strings"

	"github.com/k33alexey/MetaLab/internal/project"
	"github.com/k33alexey/MetaLab/internal/uuid"
)

// JournalColumn is one column of a journal. It has no data of its own: it
// names, per kind of document, the attribute to show there. The attributes need
// not share a name - a journal exists precisely to put "storage" of one
// document and "source storage" of another under one heading.
type JournalColumn struct {
	ID    uuid.UUID     `yaml:"id" json:"id"`
	Name  string        `yaml:"name" json:"name"`
	Title LocalizedText `yaml:"title" json:"title"`
	// Comment is the developer's note on the column. «ОбъектМетаданных: Графа»
	// has seven properties and this was the only one of them we did not carry;
	// the export writes it on all 27 columns of the demonstration configuration.
	Comment  string    `yaml:"comment,omitempty" json:"comment,omitempty"`
	Indexing IndexMode `yaml:"indexing,omitempty" json:"indexing,omitempty"`
	// References are the attributes this column shows, one per kind of
	// document at most.
	References []uuid.UUID `yaml:"references,omitempty" json:"references,omitempty"`
	// StandardReferences are the standard fields - the date, the number - the
	// column shows, named by document and name; see DocumentStandardField.
	// One document is shown once across both lists.
	StandardReferences []DocumentStandardField `yaml:"standard_references,omitempty" json:"standardReferences,omitempty"`
}

// DocumentJournalDefinition is a common list of documents of several kinds.
//
// It stores nothing of its own: it shows documents, and every column of it is
// an attribute of one of them.
type DocumentJournalDefinition struct {
	Format int           `yaml:"format" json:"format"`
	ID     uuid.UUID     `yaml:"id" json:"id"`
	Name   string        `yaml:"name" json:"name"`
	Title  LocalizedText `yaml:"title" json:"title"` // ListPresentations and the help flag: a row of this kind is not an object a
	// person opens, so there is a list to name and no object - see
	// object_presentation.go.
	ListPresentations     `yaml:",inline" json:",inline"`
	IncludeHelpInContents bool `yaml:"include_help_in_contents,omitempty" json:"includeHelpInContents,omitempty"`

	Documents []uuid.UUID     `yaml:"documents,omitempty" json:"documents,omitempty"`
	Columns   []JournalColumn `yaml:"columns,omitempty" json:"columns,omitempty"`
	// AdditionalIndexes are the indexes this object asks the database for
	// beside the ones the platform builds - see additional_indexes.go.
	AdditionalIndexes  []AdditionalIndex   `yaml:"additional_indexes,omitempty" json:"additionalIndexes,omitempty"`
	StandardAttributes []StandardAttribute `yaml:"standard_attributes,omitempty" json:"standardAttributes,omitempty"`
	Forms              SingleRoleForms     `yaml:"forms,omitempty" json:"forms,omitempty"`
	Commands           []ObjectCommand     `yaml:"commands,omitempty" json:"commands,omitempty"`
	Templates          []ObjectTemplate    `yaml:"templates,omitempty" json:"templates,omitempty"`
	List               ListSettings        `yaml:"list,omitempty" json:"list,omitempty"`
}

// DecodeDocumentJournal reads and validates one journal.
func DecodeDocumentJournal(source string, reader io.Reader, configuration project.Project) (DocumentJournalDefinition, error) {
	var value DocumentJournalDefinition
	if err := decodeStrict(source, reader, &value); err != nil {
		return DocumentJournalDefinition{}, err
	}
	issues := validateBase(value.Format, value.ID, value.Name, value.Title, configuration)
	// A journal of no documents, and a column that shows no attribute, are
	// what a journal is right after it is made; nothing shows the
	// configurator refusing to save them. Both are carried, show nothing,
	// and are notes.
	issues = append(issues, validateUniqueIDs("documents", value.Documents)...)
	issues = append(issues, validateAdditionalIndexes(value.AdditionalIndexes, DocumentJournalKind,
		recordIndexTables(standardFieldsOfKind(DocumentJournalKind), journalColumnNames(value.Columns)))...)
	names, ids := map[string]bool{}, map[uuid.UUID]bool{}
	for index, column := range value.Columns {
		prefix := fmt.Sprintf("columns[%d]", index)
		if column.ID.IsZero() {
			issues = append(issues, prefix+".id must be a non-zero UUID")
		}
		if ids[column.ID] {
			issues = append(issues, prefix+".id must be unique")
		}
		ids[column.ID] = true
		if !validIdentifier(column.Name) {
			issues = append(issues, prefix+".name must be a valid identifier")
		}
		folded := strings.ToLower(column.Name)
		if names[folded] {
			issues = append(issues, prefix+".name must be unique")
		}
		names[folded] = true
		issues = append(issues, validateTitle(prefix+".title", column.Title, configuration)...)
		if !validIndexMode(column.Indexing) {
			issues = append(issues, prefix+".indexing must be dont-index, index or index-with-additional-order")
		}
		issues = append(issues, validateUniqueIDs(prefix+".references", column.References)...)
		issues = append(issues, validateDocumentStandardFields(prefix+".standard_references", column.StandardReferences)...)
	}
	// A journal shows documents, so the fields a list of it can be searched by
	// are the ones every document has.
	issues = append(issues, validateListSettings(value.List, nil, map[string]TypeKind{
		"number": StringType, "date": DateType,
	})...)
	issues = append(issues, validateListPresentations(value.ListPresentations, configuration)...)
	issues = append(issues, validateStandardAttributes("standard_attributes", value.StandardAttributes, standardFieldsOfKind(DocumentJournalKind), configuration)...)
	issues = append(issues, validateFieldLinks(standardFieldsOfKind(DocumentJournalKind), nil, nil, standardAttributeChoices("standard_attributes", value.StandardAttributes)...)...)
	issues = append(issues, validateFormSlots(value.Forms.slots())...)
	issues = append(issues, validateObjectCommands(value.Commands, configuration)...)
	issues = append(issues, validateObjectTemplates(value.Templates, configuration)...)
	if err := issuesError(source, value.Format, issues); err != nil {
		return DocumentJournalDefinition{}, err
	}
	return value, nil
}

func cloneDocumentJournal(value DocumentJournalDefinition) DocumentJournalDefinition {
	value.Title = cloneTitle(value.Title)
	value.Documents = slices.Clone(value.Documents)
	value.Columns = slices.Clone(value.Columns)
	for index := range value.Columns {
		value.Columns[index].Title = cloneTitle(value.Columns[index].Title)
		value.Columns[index].References = slices.Clone(value.Columns[index].References)
		value.Columns[index].StandardReferences = slices.Clone(value.Columns[index].StandardReferences)
	}
	value.Forms = cloneFormSet(value.Forms)
	value.List.SearchFields = slices.Clone(value.List.SearchFields)
	value.Commands = cloneObjectCommands(value.Commands)
	value.Templates = cloneObjectTemplates(value.Templates)
	value.StandardAttributes = cloneStandardAttributes(value.StandardAttributes)
	value.ListPresentations = cloneListPresentations(value.ListPresentations)
	return value
}

// DocumentJournal returns one journal by name, folded case.
func (catalog *Catalog) DocumentJournal(name string) (DocumentJournalDefinition, bool) {
	index, ok := catalog.documentJournalByName[strings.ToLower(name)]
	if !ok {
		return DocumentJournalDefinition{}, false
	}
	return cloneDocumentJournal(catalog.DocumentJournals[index]), true
}

// DocumentJournalByID returns one journal by its identifier.
func (catalog *Catalog) DocumentJournalByID(id uuid.UUID) (DocumentJournalDefinition, bool) {
	index, ok := catalog.documentJournalByID[id]
	if !ok {
		return DocumentJournalDefinition{}, false
	}
	return cloneDocumentJournal(catalog.DocumentJournals[index]), true
}
