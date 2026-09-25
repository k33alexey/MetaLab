package metadata

import (
	"fmt"

	"github.com/k33alexey/MetaLab/internal/project"
	"github.com/k33alexey/MetaLab/internal/uuid"
)

// configurationDefault is one default of the root: what it is for, and the
// object it names. They are kept as a list rather than a map so that a
// configuration with two broken defaults is always told about the same one
// first - an error that moves around is an error nobody can act on.
type configurationDefault struct {
	purpose string
	id      *uuid.UUID
}

// validateConfigurationDefaults resolves the defaults of the configuration
// root: the style the application is drawn with, the roles it runs under when
// nobody is named, the common forms that stand in for the forms a report or a
// constant does not have, the template reports are painted by, and the five
// storages what a user saved is kept in.
//
// Every one of them names another object, and an object that is not there is
// not an inconvenience: the default report form is what opens when a report
// brings no form of its own, so a dangling one is a report that does not open
// at all. Found while the project is being written, it is one line in the
// editor; found at run time, it is a user in front of a window that never
// appeared.
//
// The default language is resolved with the languages themselves, because the
// root declares them and every synonym is checked against them long before a
// catalog exists. The default interface is resolved by nobody: it names an
// object of the ordinary application, which ML does not build, so it is
// carried as written and never acted upon.
func (catalog *Catalog) validateConfigurationDefaults(root string) error {
	// A catalog assembled from a snapshot holds only the kinds that snapshot
	// carried, so a default would look missing when it is merely not read.
	// Defaults are resolved against a project on disk, where every kind is.
	if root == "" {
		return nil
	}
	configuration := catalog.Project
	if id := configuration.DefaultStyle; id != nil {
		if _, ok := catalog.styleByID[*id]; !ok {
			return fmt.Errorf("the configuration is drawn with style %s, which is not in the configuration", id)
		}
	}
	if catalog.rolesLoaded {
		for _, id := range configuration.DefaultRoles {
			if _, ok := catalog.roleByID[id]; !ok {
				return fmt.Errorf("the configuration runs under role %s, which is not in the configuration", id)
			}
		}
	}
	for _, storage := range []configurationDefault{
		{"common settings", configuration.CommonSettingsStorage},
		{"user report settings", configuration.ReportsUserSettingsStorage},
		{"report variants", configuration.ReportsVariantsStorage},
		{"user dynamic list settings", configuration.DynamicListsUserSettingsStorage},
		{"form data", configuration.FormDataSettingsStorage},
	} {
		if storage.id == nil {
			continue
		}
		if _, ok := catalog.settingsStorageByID[*storage.id]; !ok {
			return fmt.Errorf("the configuration keeps %s in settings storage %s, which is not in the configuration",
				storage.purpose, storage.id)
		}
	}
	if id := configuration.DefaultReportAppearanceTemplate; id != nil {
		index, ok := catalog.commonTemplateByID[*id]
		if !ok {
			return fmt.Errorf("the configuration paints its reports with common template %s, which is not in the configuration", id)
		}
		// A template of another kind describes nothing an appearance is made
		// of: a spreadsheet is a document, not a set of colours and fonts.
		if template := catalog.CommonTemplates[index]; template.Kind != CompositionAppearance {
			return fmt.Errorf("the configuration paints its reports with common template %s, which is a %s and not a %s",
				template.Name, template.Kind, CompositionAppearance)
		}
	}
	if err := catalog.validateFullTextSearchDictionaries(); err != nil {
		return err
	}
	return catalog.validateConfigurationDefaultForms(root)
}

// validateFullTextSearchDictionaries resolves the dictionaries the full-text
// search is told about besides its own. Each is kept either in a common
// template or in a constant, and each says which, so resolving one is a lookup
// in a single place rather than a search through two.
//
// A dictionary that is not there is not a search that works slightly worse: it
// is a search that will not start, and the configuration is the only place
// where anybody still knows which dictionary was meant.
func (catalog *Catalog) validateFullTextSearchDictionaries() error {
	for _, dictionary := range catalog.Project.AdditionalFullTextSearchDictionaries {
		switch dictionary.Kind {
		case project.TemplateDictionary:
			if _, ok := catalog.commonTemplateByID[dictionary.Object]; !ok {
				return fmt.Errorf("the full-text search is given common template %s as a dictionary, which is not in the configuration",
					dictionary.Object)
			}
		case project.ConstantDictionary:
			if _, ok := catalog.constantByID[dictionary.Object]; !ok {
				return fmt.Errorf("the full-text search is given constant %s as a dictionary, which is not in the configuration",
					dictionary.Object)
			}
		}
	}
	return nil
}

// validateConfigurationDefaultForms resolves the five common forms the root
// hands out. They are read from the folders, like every common form, and only
// if the root actually names one: reading every form of a configuration to
// answer a question nobody asked is work for nothing.
func (catalog *Catalog) validateConfigurationDefaultForms(root string) error {
	configuration := catalog.Project
	var forms map[uuid.UUID]bool
	for _, form := range []configurationDefault{
		{"reports", configuration.DefaultReportForm},
		{"report settings", configuration.DefaultReportSettingsForm},
		{"report variants", configuration.DefaultReportVariantForm},
		{"constants", configuration.DefaultConstantsForm},
		{"full-text search", configuration.DefaultSearchForm},
	} {
		if form.id == nil {
			continue
		}
		if forms == nil {
			read, err := ReadCommonForms(root, configuration)
			if err != nil {
				return err
			}
			forms = make(map[uuid.UUID]bool, len(read))
			for _, common := range read {
				forms[common.ID] = true
			}
		}
		if !forms[*form.id] {
			return fmt.Errorf("the configuration opens %s with common form %s, which is not in the configuration",
				form.purpose, form.id)
		}
	}
	return nil
}
