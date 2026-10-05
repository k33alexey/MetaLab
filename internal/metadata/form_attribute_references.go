package metadata

import (
	"fmt"
	"os"
	"runtime"
	"sort"
	"sync"

	"github.com/k33alexey/MetaLab/internal/uuid"
)

// formTypeList is one type description of an attribute of a form or of a
// column of one, with the place it stands.
type formTypeList struct {
	where string
	types []Type
}

// formTitle is the title of an attribute of a form or of a column of one,
// with the place it stands, kept for the notes on texts.
type formTitle struct {
	where string
	text  LocalizedText
}

// checkFormAttributes reads every form of an object whole and checks what
// the attributes of every form refer to: the objects named in their types,
// which must exist, and the functional options and the roles of their rights,
// which are carried unresolved when they are gone (UnresolvedReference). It
// runs once everything a form may refer to is loaded.
//
// The forms of erp are 4360 files; read one after another they took 1.5 s on
// an eight-core machine, read side by side 0.36 s (measured 05.10.2026). So
// they are read side by side, and checked in a fixed order afterwards, so
// that the first refusal and the order of the notes do not depend on which
// file was read first.
func (catalog *Catalog) checkFormAttributes() error {
	type job struct {
		where string
		path  string
		form  ManagedForm
		err   error
	}
	var jobs []*job
	for _, objects := range catalog.objectForms {
		for _, object := range objects {
			for _, form := range object.forms {
				if form.path == "" {
					continue
				}
				jobs = append(jobs, &job{where: fmt.Sprintf("%s %s form %s", object.kind, object.object, form.name), path: form.path})
			}
		}
	}
	sort.Slice(jobs, func(i, j int) bool { return jobs[i].where < jobs[j].where })
	work := make(chan *job)
	var group sync.WaitGroup
	for range runtime.GOMAXPROCS(0) {
		group.Add(1)
		go func() {
			defer group.Done()
			for item := range work {
				item.form, item.err = readFormDescription(item.path, catalog)
			}
		}()
	}
	for _, item := range jobs {
		work <- item
	}
	close(work)
	group.Wait()

	catalog.formTypeLists, catalog.formTitles = nil, nil
	for _, item := range jobs {
		if item.err != nil {
			return fmt.Errorf("%s: %w", item.where, item.err)
		}
		if err := catalog.resolveFormAttributes(item.where, item.form.Attributes); err != nil {
			return err
		}
	}
	common := catalog.commonFormsRead
	catalog.commonFormsRead = nil
	sort.Slice(common, func(i, j int) bool { return common[i].Name < common[j].Name })
	for _, form := range common {
		if err := catalog.resolveFormAttributes("common form "+form.Name, form.Attributes); err != nil {
			return err
		}
	}
	return nil
}

func readFormDescription(path string, catalog *Catalog) (ManagedForm, error) {
	file, err := os.Open(path)
	if err != nil {
		return ManagedForm{}, err
	}
	defer file.Close()
	return DecodeManagedForm(path, file, catalog.Project)
}

// resolveFormAttributes checks the references of the attributes of one form.
func (catalog *Catalog) resolveFormAttributes(form string, attributes []FormAttribute) error {
	for _, attribute := range attributes {
		where := form + " attribute " + attribute.Name
		catalog.keepFormText(where, attribute.Title)
		if err := catalog.resolveFormData(where, attribute.Types, attribute.FunctionalOptions, attribute.View, attribute.Edit); err != nil {
			return err
		}
		if len(attribute.ValueType) != 0 {
			if err := catalog.resolveFormData(where+" value type", attribute.ValueType, nil); err != nil {
				return err
			}
		}
		if list := attribute.DynamicList; list != nil && list.MainTable != nil {
			if _, ok := catalog.objectKindByID[list.MainTable.Object]; !ok {
				catalog.noteUnresolved(where+" main table", list.MainTable.Object)
			}
		}
		for _, column := range attribute.Columns {
			catalog.keepFormText(where+" column "+column.Name, column.Title)
			if err := catalog.resolveFormData(where+" column "+column.Name, column.Types, column.FunctionalOptions, column.View, column.Edit); err != nil {
				return err
			}
		}
		for _, additional := range attribute.AdditionalColumns {
			for _, column := range additional.Columns {
				catalog.keepFormText(where+" table "+additional.Table+" column "+column.Name, column.Title)
				if err := catalog.resolveFormData(where+" table "+additional.Table+" column "+column.Name, column.Types, column.FunctionalOptions, column.View, column.Edit); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func (catalog *Catalog) resolveFormData(where string, types []Type, options []uuid.UUID, rights ...*FormAttributeRight) error {
	if err := catalog.validateReferences(where, types); err != nil {
		return err
	}
	if len(types) != 0 {
		catalog.formTypeLists = append(catalog.formTypeLists, formTypeList{where: where, types: types})
	}
	for _, option := range options {
		if _, ok := catalog.functionalOptionByID[option]; !ok {
			catalog.noteUnresolved(where+" functional option", option)
		}
	}
	// A role is known only when the roles were loaded: the role editor reads
	// the project without them.
	if !catalog.rolesLoaded {
		return nil
	}
	for _, right := range rights {
		if right == nil {
			continue
		}
		for _, role := range right.Roles {
			if _, ok := catalog.roleByID[role.Role]; !ok {
				catalog.noteUnresolved(where+" right of role", role.Role)
			}
		}
	}
	return nil
}

func (catalog *Catalog) keepFormText(where string, text LocalizedText) {
	if len(text) != 0 {
		catalog.formTitles = append(catalog.formTitles, formTitle{where: where + " title", text: text})
	}
}
