package metadata

import (
	"fmt"
	"io"
	"slices"
	"strings"

	"github.com/k33alexey/MetaLab/internal/project"
	"github.com/k33alexey/MetaLab/internal/schemadiff"
	"github.com/k33alexey/MetaLab/internal/uuid"
)

const maxSequenceDimensions = 16

// SequenceDimension narrows the boundary. Without dimensions a sequence keeps
// one boundary for everything, and a document posted late by one product
// declares every later document out of order. With them the boundary is kept
// per combination of values, and only what actually depends on the late
// document has to be posted again.
//
// A dimension that is not mapped anywhere follows nothing: the maps are where
// the value is read from when a document is posted, and where the same value
// sits in the movements the boundary follows.
type SequenceDimension struct {
	ID    uuid.UUID     `yaml:"id" json:"id"`
	Name  string        `yaml:"name" json:"name"`
	Title LocalizedText `yaml:"title" json:"title"`
	Types []Type        `yaml:"types" json:"types"`
	// DocumentAttributes are the attributes the value is taken from. An
	// attribute of a table part is as good as one of the document itself: the
	// product a boundary is kept by lives in the lines, not in the header.
	DocumentAttributes []uuid.UUID `yaml:"document_attributes,omitempty" json:"documentAttributes,omitempty"`
	// RegisterDimensions are where the same value sits in the movements.
	RegisterDimensions []uuid.UUID `yaml:"register_dimensions,omitempty" json:"registerDimensions,omitempty"`
}

// SequenceDefinition is the control of posting order: a document posted after
// the fact leaves everything later than it in need of posting again.
type SequenceDefinition struct {
	Format int           `yaml:"format" json:"format"`
	ID     uuid.UUID     `yaml:"id" json:"id"`
	Name   string        `yaml:"name" json:"name"`
	Title  LocalizedText `yaml:"title" json:"title"`
	// Documents are the kinds of document the sequence follows.
	Documents []uuid.UUID `yaml:"documents,omitempty" json:"documents,omitempty"`
	// Movements are the registers whose records the boundary is watched by.
	Movements []uuid.UUID `yaml:"movements,omitempty" json:"movements,omitempty"`
	// MoveBoundaryOnPosting moves the boundary forward as documents are posted
	// in order, instead of leaving it where it was.
	MoveBoundaryOnPosting bool                `yaml:"move_boundary_on_posting,omitempty" json:"moveBoundaryOnPosting,omitempty"`
	Dimensions            []SequenceDimension `yaml:"dimensions,omitempty" json:"dimensions,omitempty"`
}

// DecodeSequence reads and validates one sequence.
func DecodeSequence(source string, reader io.Reader, manifest project.Project) (SequenceDefinition, error) {
	var value SequenceDefinition
	if err := decodeStrict(source, reader, &value); err != nil {
		return SequenceDefinition{}, err
	}
	issues := validateBase(value.Format, value.ID, value.Name, value.Title, manifest)
	// A sequence over no documents watches nothing happen.
	if len(value.Documents) == 0 {
		issues = append(issues, "documents must name at least one kind of document: a sequence over nothing follows nothing")
	}
	issues = append(issues, validateUniqueIDs("documents", value.Documents)...)
	issues = append(issues, validateUniqueIDs("movements", value.Movements)...)
	if len(value.Dimensions) > maxSequenceDimensions {
		issues = append(issues, fmt.Sprintf("dimensions must not contain more than %d items", maxSequenceDimensions))
	}
	names, ids := map[string]bool{}, map[uuid.UUID]bool{}
	for index, dimension := range value.Dimensions {
		prefix := fmt.Sprintf("dimensions[%d]", index)
		if dimension.ID.IsZero() {
			issues = append(issues, prefix+".id must be a non-zero UUID")
		}
		if ids[dimension.ID] {
			issues = append(issues, prefix+".id must be unique")
		}
		ids[dimension.ID] = true
		if !validIdentifier(dimension.Name) {
			issues = append(issues, prefix+".name must be a valid identifier")
		}
		folded := strings.ToLower(dimension.Name)
		if names[folded] {
			issues = append(issues, prefix+".name must be unique")
		}
		names[folded] = true
		issues = append(issues, validateTitle(prefix+".title", dimension.Title, manifest)...)
		issues = append(issues, validateTypes(prefix+".types", dimension.Types, value.ID)...)
		issues = append(issues, validateUniqueIDs(prefix+".document_attributes", dimension.DocumentAttributes)...)
		issues = append(issues, validateUniqueIDs(prefix+".register_dimensions", dimension.RegisterDimensions)...)
		if len(dimension.DocumentAttributes) == 0 {
			issues = append(issues, prefix+" is taken from no attribute of any document, so it never gets a value")
		}
	}
	if err := issuesError(source, value.Format, issues); err != nil {
		return SequenceDefinition{}, err
	}
	return value, nil
}

func validateUniqueIDs(path string, ids []uuid.UUID) []string {
	var issues []string
	seen := map[uuid.UUID]bool{}
	for index, id := range ids {
		if id.IsZero() {
			issues = append(issues, fmt.Sprintf("%s[%d] must be a non-zero UUID", path, index))
		}
		if seen[id] {
			issues = append(issues, fmt.Sprintf("%s[%d] repeats %s", path, index, id))
		}
		seen[id] = true
	}
	return issues
}

func cloneSequence(value SequenceDefinition) SequenceDefinition {
	value.Title = cloneTitle(value.Title)
	value.Documents = slices.Clone(value.Documents)
	value.Movements = slices.Clone(value.Movements)
	value.Dimensions = slices.Clone(value.Dimensions)
	for index := range value.Dimensions {
		dimension := &value.Dimensions[index]
		dimension.Title = cloneTitle(dimension.Title)
		dimension.Types = cloneTypes(dimension.Types)
		dimension.DocumentAttributes = slices.Clone(dimension.DocumentAttributes)
		dimension.RegisterDimensions = slices.Clone(dimension.RegisterDimensions)
	}
	return value
}

// Sequence returns one sequence by name, folded case.
func (catalog *Catalog) Sequence(name string) (SequenceDefinition, bool) {
	index, ok := catalog.sequenceByName[strings.ToLower(name)]
	if !ok {
		return SequenceDefinition{}, false
	}
	return cloneSequence(catalog.Sequences[index]), true
}

// SequenceByID returns one sequence by its identifier.
func (catalog *Catalog) SequenceByID(id uuid.UUID) (SequenceDefinition, bool) {
	index, ok := catalog.sequenceByID[id]
	if !ok {
		return SequenceDefinition{}, false
	}
	return cloneSequence(catalog.Sequences[index]), true
}

// sequenceTable is where a posted document is written down as belonging to the
// sequence. The boundary itself is maintained by posting and arrives with it;
// what is described here is the record, which is what the sequence stores.
func (catalog *Catalog) sequenceTable(definition SequenceDefinition) (schemadiff.Table, error) {
	tableName, err := PhysicalCatalogTable(definition.ID)
	if err != nil {
		return schemadiff.Table{}, err
	}
	table := schemadiff.Table{
		Name: tableName,
		Columns: []schemadiff.Column{
			{Name: "period", Type: "timestamp with time zone", Nullable: false},
			// The document that made the record, kept the way every recorder
			// is kept: which kind of document and which one of them.
			{Name: "recorder_type", Type: "uuid", Nullable: false},
			{Name: "recorder_ref", Type: "uuid", Nullable: false},
		},
		Constraints: []schemadiff.Constraint{
			{Name: physicalObjectName("pk", definition.ID), Type: "primary_key", Definition: "PRIMARY KEY (recorder_type, recorder_ref)"},
		},
		Indexes: []schemadiff.Index{
			// The boundary is found by walking the records in order, so the
			// order is the one thing this table is always read by.
			{Name: physicalObjectName("ip", definition.ID), Method: "btree", Keys: []string{"period"}},
		},
	}
	for _, dimension := range definition.Dimensions {
		if err := catalog.appendAttributeSchema(&table, Attribute{
			ID: dimension.ID, Name: dimension.Name, Title: dimension.Title, Types: dimension.Types, Indexed: true,
		}); err != nil {
			return schemadiff.Table{}, fmt.Errorf("sequence %s dimension %s: %w", definition.Name, dimension.Name, err)
		}
	}
	return table, nil
}
