package metadata

import (
	"io"
	"slices"
	"strings"

	"github.com/k33alexey/MetaLab/internal/project"
	"github.com/k33alexey/MetaLab/internal/uuid"
)

// SettingsStorageKind holds the places user settings are saved to and read
// back from.
const SettingsStorageKind Kind = "settings-storages"

// SettingsStorageForms are the two things a user does with saved settings:
// save the current ones under a name, and pick a saved one to load. Each has
// an auxiliary form beside it.
//
// Beside, not behind: the platform reaches for the auxiliary form exactly when
// the main one is missing or does not fit the client it has to be shown in, so
// an auxiliary form named with no main form beside it is the ordinary case and
// not a mistake.
type SettingsStorageForms struct {
	Save          string `yaml:"save,omitempty" json:"save,omitempty"`
	Load          string `yaml:"load,omitempty" json:"load,omitempty"`
	AuxiliarySave string `yaml:"auxiliary_save,omitempty" json:"auxiliarySave,omitempty"`
	AuxiliaryLoad string `yaml:"auxiliary_load,omitempty" json:"auxiliaryLoad,omitempty"`
}

func (forms SettingsStorageForms) slots() []formSlot {
	return []formSlot{
		{"forms.save", forms.Save},
		{"forms.load", forms.Load},
		{"forms.auxiliary_save", forms.AuxiliarySave},
		{"forms.auxiliary_load", forms.AuxiliaryLoad},
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
	Templates []ObjectTemplate `yaml:"templates,omitempty" json:"templates,omitempty"`
}

// DecodeSettingsStorage reads and validates one settings storage.
func DecodeSettingsStorage(source string, reader io.Reader, configuration project.Project) (SettingsStorageDefinition, error) {
	var value SettingsStorageDefinition
	if err := decodeStrict(source, reader, &value); err != nil {
		return SettingsStorageDefinition{}, err
	}
	issues := validateBase(value.Format, value.ID, value.Name, value.Title, configuration)
	issues = append(issues, validateFormSlots(value.Forms.slots())...)
	issues = append(issues, validateObjectTemplates(value.Templates, configuration)...)
	if err := issuesError(source, value.Format, issues); err != nil {
		return SettingsStorageDefinition{}, err
	}
	return value, nil
}

func cloneSettingsStorage(value SettingsStorageDefinition) SettingsStorageDefinition {
	value.Title = cloneTitle(value.Title)
	value.Templates = cloneObjectTemplates(value.Templates)
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
