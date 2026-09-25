package metadata

import (
	"fmt"

	"github.com/k33alexey/MetaLab/internal/project"
	"github.com/k33alexey/MetaLab/internal/uuid"
)

// constantKindModules are the modules a constant may keep beside its own
// description: the one the platform calls around the value itself, and the one
// of its manager.
var constantKindModules = []string{project.ValueModuleFile, project.ManagerModuleFile}

// validateConstantFiles checks the folder of every constant and resolves the
// form each one names.
//
// Neither module is required. In the reference configuration most constants
// have none: a constant without code is simply a value nobody needed to check
// or watch, which is the ordinary case.
//
// The form is a common form, so it is looked up among them - and they are read
// only if some constant actually names one, because reading every common form
// to answer a question nobody asked is work for nothing.
func (catalog *Catalog) validateConstantFiles(root string) error {
	if root == "" {
		return nil
	}
	var forms map[uuid.UUID]bool
	for _, item := range catalog.Constants {
		if _, err := namedFolderContents(root, ConstantKind, "constant", item.Name); err != nil {
			return err
		}
		if item.DefaultForm == nil {
			continue
		}
		if forms == nil {
			read, err := ReadCommonForms(root, catalog.Project)
			if err != nil {
				return err
			}
			forms = make(map[uuid.UUID]bool, len(read))
			for _, form := range read {
				forms[form.ID] = true
			}
		}
		if !forms[*item.DefaultForm] {
			return fmt.Errorf("constant %s is opened by unknown common form %s", item.Name, item.DefaultForm)
		}
	}
	return nil
}
