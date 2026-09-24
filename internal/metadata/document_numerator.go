package metadata

import (
	"io"
	"strings"

	"github.com/k33alexey/MetaLab/internal/project"
	"github.com/k33alexey/MetaLab/internal/uuid"
)

// NumeratorDefinition is one numbering shared by several kinds of document, so
// that an invoice and a delivery note draw from one run of numbers instead of
// each counting from one.
//
// Automatic numbering is deliberately not here: whether a number is assigned by
// the platform or typed by hand is a property of the document, and two
// documents sharing a numerator may well differ in it.
type NumeratorDefinition struct {
	Format int            `yaml:"format" json:"format"`
	ID     uuid.UUID      `yaml:"id" json:"id"`
	Name   string         `yaml:"name" json:"name"`
	Title  LocalizedText  `yaml:"title" json:"title"`
	Number DocumentNumber `yaml:"number" json:"number"`
}

// DecodeNumerator reads and validates one numerator.
func DecodeNumerator(source string, reader io.Reader, manifest project.Project) (NumeratorDefinition, error) {
	var value NumeratorDefinition
	if err := decodeStrict(source, reader, &value); err != nil {
		return NumeratorDefinition{}, err
	}
	issues := validateBase(value.Format, value.ID, value.Name, value.Title, manifest)
	issues = append(issues, validateNumberShape(value.Number)...)
	if value.Number.Auto {
		issues = append(issues, "number.auto belongs to the document, not to the numerator it shares")
	}
	if err := issuesError(source, value.Format, issues); err != nil {
		return NumeratorDefinition{}, err
	}
	return value, nil
}

func cloneNumerator(value NumeratorDefinition) NumeratorDefinition {
	value.Title = cloneTitle(value.Title)
	return value
}

// Numerator returns one numerator by name, folded case.
func (catalog *Catalog) Numerator(name string) (NumeratorDefinition, bool) {
	index, ok := catalog.numeratorByName[strings.ToLower(name)]
	if !ok {
		return NumeratorDefinition{}, false
	}
	return cloneNumerator(catalog.Numerators[index]), true
}

// NumeratorByID returns one numerator by its identifier.
func (catalog *Catalog) NumeratorByID(id uuid.UUID) (NumeratorDefinition, bool) {
	index, ok := catalog.numeratorByID[id]
	if !ok {
		return NumeratorDefinition{}, false
	}
	return cloneNumerator(catalog.Numerators[index]), true
}
