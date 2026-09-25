package metadata

import (
	"fmt"
	"io"
	"slices"
	"strings"

	"github.com/k33alexey/MetaLab/internal/project"
	"github.com/k33alexey/MetaLab/internal/uuid"
)

const maxJournalColumns = 128

// JournalColumn is one column of a journal. It has no data of its own: it
// names, per kind of document, the attribute to show there. The attributes need
// not share a name - a journal exists precisely to put "storage" of one
// document and "source storage" of another under one heading.
type JournalColumn struct {
	ID      uuid.UUID     `yaml:"id" json:"id"`
	Name    string        `yaml:"name" json:"name"`
	Title   LocalizedText `yaml:"title" json:"title"`
	Indexed bool          `yaml:"indexed,omitempty" json:"indexed,omitempty"`
	// References are the attributes this column shows, one per kind of
	// document at most.
	References []uuid.UUID `yaml:"references,omitempty" json:"references,omitempty"`
}

// DocumentJournalDefinition is a common list of documents of several kinds.
//
// It stores nothing of its own: it shows documents, and every column of it is
// an attribute of one of them.
type DocumentJournalDefinition struct {
	Format    int              `yaml:"format" json:"format"`
	ID        uuid.UUID        `yaml:"id" json:"id"`
	Name      string           `yaml:"name" json:"name"`
	Title     LocalizedText    `yaml:"title" json:"title"`
	Documents []uuid.UUID      `yaml:"documents,omitempty" json:"documents,omitempty"`
	Columns   []JournalColumn  `yaml:"columns,omitempty" json:"columns,omitempty"`
	Forms     SingleRoleForms  `yaml:"forms,omitempty" json:"forms,omitempty"`
	Commands  []ObjectCommand  `yaml:"commands,omitempty" json:"commands,omitempty"`
	Templates []ObjectTemplate `yaml:"templates,omitempty" json:"templates,omitempty"`
	List      ListSettings     `yaml:"list,omitempty" json:"list,omitempty"`
}

// DecodeDocumentJournal reads and validates one journal.
func DecodeDocumentJournal(source string, reader io.Reader, configuration project.Project) (DocumentJournalDefinition, error) {
	var value DocumentJournalDefinition
	if err := decodeStrict(source, reader, &value); err != nil {
		return DocumentJournalDefinition{}, err
	}
	issues := validateBase(value.Format, value.ID, value.Name, value.Title, configuration)
	if len(value.Documents) == 0 {
		issues = append(issues, "documents must name at least one kind of document: a journal of nothing shows nothing")
	}
	issues = append(issues, validateUniqueIDs("documents", value.Documents)...)
	if len(value.Columns) > maxJournalColumns {
		issues = append(issues, fmt.Sprintf("columns must not contain more than %d items", maxJournalColumns))
	}
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
		issues = append(issues, validateUniqueIDs(prefix+".references", column.References)...)
		if len(column.References) == 0 {
			issues = append(issues, prefix+" shows no attribute of any document, so it is an empty column")
		}
	}
	// A journal shows documents, so the fields a list of it can be searched by
	// are the ones every document has.
	issues = append(issues, validateListSettings(value.List, nil, map[string]TypeKind{
		"number": StringType, "date": DateType,
	})...)
	issues = append(issues, validateFormSlots(value.Forms.slots())...)
	issues = append(issues, validateObjectCommands(value.Commands, value.ID, configuration)...)
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
	}
	value.Forms = cloneFormSet(value.Forms)
	value.List.SearchFields = slices.Clone(value.List.SearchFields)
	value.Commands = cloneObjectCommands(value.Commands)
	value.Templates = cloneObjectTemplates(value.Templates)
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
