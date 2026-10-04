package metadata

import (
	"fmt"
	"slices"
	"strings"

	"github.com/k33alexey/MetaLab/internal/project"
	"github.com/k33alexey/MetaLab/internal/uuid"
)

// TemplateKind says what a template holds, and with it how the template is
// stored, shown and edited. There are ten, and all ten are carried: a kind the
// model does not know is a template lost without a word at import, which is
// the one outcome the conformance report calls unacceptable.
type TemplateKind string

const (
	// SpreadsheetTemplate is the printed form and the tabular report - the
	// kind most templates are.
	SpreadsheetTemplate TemplateKind = "spreadsheet"
	TextTemplate        TemplateKind = "text"
	BinaryTemplate      TemplateKind = "binary"
	// HTMLTemplate keeps one document per language rather than one document,
	// because it is read by a person and the language is the point.
	HTMLTemplate TemplateKind = "html"
	// CompositionSchema is a data composition schema. It is a kind of
	// template, not a thing standing beside them: a report's main schema names
	// one of the report's own templates.
	CompositionSchema TemplateKind = "composition-schema"
	// CompositionAppearance is the appearance a composition is drawn with.
	CompositionAppearance TemplateKind = "composition-appearance"
	// GeographicalSchema and GraphicalSchema are carried but not drawn yet -
	// the second category of the conformance report, transferred and not
	// implemented, which is what that category is for.
	GeographicalSchema TemplateKind = "geographical-schema"
	GraphicalSchema    TemplateKind = "graphical-schema"
	// ActiveDocument is an external document edited in place by another
	// program. It exists on one operating system only, and is carried for the
	// same reason as the two above.
	ActiveDocument TemplateKind = "active-document"
	// AddInTemplate holds an external component - an archive the platform
	// loads code from. It is stored as binary content but is not plain binary
	// data: what it holds is executable, and the two are told apart before
	// anything is loaded.
	AddInTemplate TemplateKind = "add-in"
)

// contentFile is the name a template's content is stored under. An HTML
// template answers with nothing, because it has a file per language instead of
// a file.
func (kind TemplateKind) contentFile() string {
	switch kind {
	case SpreadsheetTemplate:
		// Our own model, in JSON: a spreadsheet is the largest content a
		// configuration keeps - 4034 of them, 899 MiB of the prototype's XML
		// in four exports, up to 37 MB each - and it is a grid of cells only
		// its editor writes and reads. Read and written as JSON it costs a
		// fraction of what YAML does (a document twice the size of КОАТУУ:
		// 0.25 GB to read and 0.5 GB to write against 1.2 GB and 5.9 GB),
		// and a person reading its diff loses nothing (owner, 04.10.2026).
		return "content.json"
	case CompositionSchema, CompositionAppearance, GeographicalSchema, GraphicalSchema:
		// Our own model in YAML. The markup shown, printed and exported is
		// generated from it and is never the source. A composition schema
		// keeps a query a developer reads in a diff, which YAML writes as
		// it is and JSON would write on one line.
		return "content.yaml"
	case TextTemplate:
		return "content.txt"
	case BinaryTemplate, AddInTemplate, ActiveDocument:
		return "content.bin"
	default:
		return ""
	}
}

// ObjectTemplate is a prepared document an object keeps: a printed form, a
// composition schema, a text, an archive with an external component.
//
// The template itself is described by very little - what it is called and what
// it is. Everything else is its content, and the content lives in a folder of
// its own beside the object, not inside the description.
type ObjectTemplate struct {
	ID      uuid.UUID     `yaml:"id" json:"id"`
	Name    string        `yaml:"name" json:"name"`
	Title   LocalizedText `yaml:"title" json:"title"`
	Comment string        `yaml:"comment,omitempty" json:"comment,omitempty"`
	Kind    TemplateKind  `yaml:"kind" json:"kind"`
}

// validateObjectTemplates checks the templates of one object.
func validateObjectTemplates(templates []ObjectTemplate, configuration project.Project) []string {
	var issues []string
	names, ids := map[string]bool{}, map[uuid.UUID]bool{}
	for index, template := range templates {
		prefix := fmt.Sprintf("templates[%d]", index)
		if template.ID.IsZero() {
			issues = append(issues, prefix+".id must be a non-zero UUID")
		}
		if ids[template.ID] {
			issues = append(issues, prefix+".id must be unique")
		}
		ids[template.ID] = true
		if !validIdentifier(template.Name) {
			issues = append(issues, prefix+".name must be a valid identifier")
		}
		folded := strings.ToLower(template.Name)
		if names[folded] {
			issues = append(issues, prefix+".name must be unique")
		}
		names[folded] = true
		issues = append(issues, validateTitle(prefix+".title", template.Title, configuration)...)
		if !validTemplateKind(template.Kind) {
			issues = append(issues, prefix+".kind is not a kind of template")
		}
	}
	return issues
}

func validTemplateKind(kind TemplateKind) bool {
	switch kind {
	case SpreadsheetTemplate, TextTemplate, BinaryTemplate, HTMLTemplate,
		CompositionSchema, CompositionAppearance, GeographicalSchema, GraphicalSchema,
		ActiveDocument, AddInTemplate:
		return true
	default:
		return false
	}
}

func cloneObjectTemplates(templates []ObjectTemplate) []ObjectTemplate {
	templates = slices.Clone(templates)
	for index := range templates {
		templates[index].Title = cloneTitle(templates[index].Title)
	}
	return templates
}
