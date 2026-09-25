package metadata

import (
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/k33alexey/MetaLab/internal/project"
	"github.com/k33alexey/MetaLab/internal/uuid"
)

// CommonTemplateKind holds the prepared documents that belong to no object:
// the same ten kinds of template, offered by the configuration itself.
const CommonTemplateKind Kind = "common-templates"

// CommonTemplateDefinition is a template belonging to no object.
//
// It is the same thing as a template of an object - the same ten kinds, the
// same content in a folder of its own - and it carries that sameness by
// reusing the type, so the two cannot drift apart. The only difference is
// where it stands: nothing owns it, so its folder is the template itself
// rather than one folder among an object's templates.
type CommonTemplateDefinition struct {
	Format         int `yaml:"format" json:"format"`
	ObjectTemplate `yaml:",inline"`
}

// DecodeCommonTemplate reads and validates one common template.
func DecodeCommonTemplate(source string, reader io.Reader, manifest project.Project) (CommonTemplateDefinition, error) {
	var value CommonTemplateDefinition
	if err := decodeStrict(source, reader, &value); err != nil {
		return CommonTemplateDefinition{}, err
	}
	issues := validateBase(value.Format, value.ID, value.Name, value.Title, manifest)
	if !validTemplateKind(value.Kind) {
		issues = append(issues, "kind is not a kind of template")
	}
	if err := issuesError(source, value.Format, issues); err != nil {
		return CommonTemplateDefinition{}, err
	}
	return value, nil
}

func cloneCommonTemplate(value CommonTemplateDefinition) CommonTemplateDefinition {
	value.ObjectTemplate = cloneObjectTemplates([]ObjectTemplate{value.ObjectTemplate})[0]
	return value
}

// CommonTemplate returns one common template by name, folded case.
func (catalog *Catalog) CommonTemplate(name string) (CommonTemplateDefinition, bool) {
	index, ok := catalog.commonTemplateByName[strings.ToLower(name)]
	if !ok {
		return CommonTemplateDefinition{}, false
	}
	return cloneCommonTemplate(catalog.CommonTemplates[index]), true
}

// CommonTemplateByID returns one common template by identifier.
func (catalog *Catalog) CommonTemplateByID(id uuid.UUID) (CommonTemplateDefinition, bool) {
	index, ok := catalog.commonTemplateByID[id]
	if !ok {
		return CommonTemplateDefinition{}, false
	}
	return cloneCommonTemplate(catalog.CommonTemplates[index]), true
}

// validateCommonTemplateFiles checks the folder of every common template: its
// own description, and the content its kind allows beside it.
//
// The content may be missing entirely, for the same reason an object's
// template may have none: no editor writes it yet, and the reference
// configuration carries templates whose content is empty. What is checked is
// that nothing else lies there, so content written later lands where the
// platform will look for it.
func (catalog *Catalog) validateCommonTemplateFiles(root string) error {
	if root == "" {
		return nil
	}
	for _, item := range catalog.CommonTemplates {
		directory := filepath.Join(root, "metadata", string(CommonTemplateKind), item.Name)
		entries, err := os.ReadDir(directory)
		if err != nil {
			return fmt.Errorf("common template %s: %w", item.Name, err)
		}
		for _, entry := range entries {
			if entry.IsDir() || entry.Type()&fs.ModeSymlink != 0 {
				return fmt.Errorf("common template %s keeps %q, which is not a file of its content",
					item.Name, entry.Name())
			}
			if entry.Name() == project.ObjectMetadataFile {
				continue
			}
			if !templateContentFileName(item.Kind, entry.Name()) {
				return fmt.Errorf("common template %s is a %s and cannot hold %q",
					item.Name, item.Kind, entry.Name())
			}
		}
	}
	return nil
}
