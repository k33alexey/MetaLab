package metadata

import (
	"io"
	"slices"
	"strings"

	"github.com/k33alexey/MetaLab/internal/kinds"
	"github.com/k33alexey/MetaLab/internal/project"
	"github.com/k33alexey/MetaLab/internal/uuid"
)

// SettingsStorageKind holds the places user settings are saved to and read
// back from.
const SettingsStorageKind = kinds.SettingsStorage

// SettingsStorageForms are the two things a user does with saved settings:
// save the current ones under a name, and pick a saved one to load.
type SettingsStorageForms struct {
	Save string `yaml:"save,omitempty" json:"save,omitempty"`
	Load string `yaml:"load,omitempty" json:"load,omitempty"`
}

func (forms SettingsStorageForms) slots() []formSlot {
	return []formSlot{
		{"forms.save", forms.Save},
		{"forms.load", forms.Load},
	}
}

// SettingsStorageDefinition describes one settings storage: where the settings
// of forms, lists, reports and report variants are kept, and how the user is
// shown them.
//
// It keeps no data of its own and is shown in no list - the platform generates
// a manager for it and nothing else. That is why it has no commands: there is
// no list to offer them beside.
type SettingsStorageDefinition struct {
	Format  int           `yaml:"format" json:"format"`
	ID      uuid.UUID     `yaml:"id" json:"id"`
	Name    string        `yaml:"name" json:"name"`
	Title   LocalizedText `yaml:"title" json:"title"`
	Comment string        `yaml:"comment,omitempty" json:"comment,omitempty"`
	// The manager module of a storage is МодульМенеджера.bsl in the storage's
	// own folder, and it is not named here: the file is the declaration. It
	// holds the four procedures the platform calls to save, load and describe
	// settings, and without it the storage does nothing at all - the platform
	// has nowhere to put what the user saved.
	Forms SettingsStorageForms `yaml:"forms,omitempty" json:"forms,omitempty"`
	// Templates are the storage's own templates - what its forms print the
	// saved settings with.
	Templates         []ObjectTemplate `yaml:"templates,omitempty" json:"templates,omitempty"`
	SubordinatesOrder `yaml:",inline" json:",inline"`
}

// DecodeSettingsStorage reads and validates one settings storage.
func DecodeSettingsStorage(source string, reader io.Reader, configuration project.Project) (SettingsStorageDefinition, error) {
	return decodeDescription(source, reader, configuration, func(value SettingsStorageDefinition, configuration project.Project) []string {
		var issues []string
		issues = append(issues, validateFormSlots(value.Forms.slots())...)
		issues = append(issues, validateObjectTemplates(value.Templates, configuration)...)
		return issues
	})
}

func cloneSettingsStorage(value SettingsStorageDefinition) SettingsStorageDefinition {
	value.Title = cloneTitle(value.Title)
	value.Templates = cloneObjectTemplates(value.Templates)
	value.SubordinatesOrder = value.SubordinatesOrder.clone()
	return value
}

// SettingsStorage returns one settings storage by name, folded case.
func (catalog *Catalog) SettingsStorage(name string) (SettingsStorageDefinition, bool) {
	index, ok := catalog.settingsStorageByName[strings.ToLower(name)]
	if !ok {
		return SettingsStorageDefinition{}, false
	}
	return cloneSettingsStorage(catalog.SettingsStorages[index]), true
}

// validateSettingsStorageReferences resolves everything a report names by
// identifier beside its templates: the storages of its variants and settings,
// and its main schema. One that points at nothing is carried and noted, not
// refused - the prototype writes a reference to what is gone by its bare
// identifier and keeps it (A3; a form of a report, erp СверкаДанныхОУиБУ).
func (catalog *Catalog) validateSettingsStorageReferences() error {
	for _, item := range catalog.Reports {
		for _, storage := range []struct {
			name string
			id   *uuid.UUID
		}{{"variants storage", item.VariantsStorage}, {"settings storage", item.SettingsStorage}} {
			if storage.id == nil {
				continue
			}
			if _, ok := catalog.settingsStorageByID[*storage.id]; !ok {
				catalog.noteUnresolved("report "+item.Name+" "+storage.name, *storage.id)
			}
		}
		if item.MainSchema != nil && !item.MainSchema.IsZero() && !slices.ContainsFunc(item.Templates,
			func(template ObjectTemplate) bool { return template.ID == *item.MainSchema }) {
			catalog.noteUnresolved("report "+item.Name+" main schema", *item.MainSchema)
		}
	}
	return nil
}
