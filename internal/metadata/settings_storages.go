package metadata

import (
	"fmt"
	"io"
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
type SettingsStorageForms struct {
	Save          string `yaml:"save,omitempty" json:"save,omitempty"`
	Load          string `yaml:"load,omitempty" json:"load,omitempty"`
	AuxiliarySave string `yaml:"auxiliary_save,omitempty" json:"auxiliarySave,omitempty"`
	AuxiliaryLoad string `yaml:"auxiliary_load,omitempty" json:"auxiliaryLoad,omitempty"`
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
}

// DecodeSettingsStorage reads and validates one settings storage.
func DecodeSettingsStorage(source string, reader io.Reader, manifest project.Project) (SettingsStorageDefinition, error) {
	var value SettingsStorageDefinition
	if err := decodeStrict(source, reader, &value); err != nil {
		return SettingsStorageDefinition{}, err
	}
	issues := validateBase(value.Format, value.ID, value.Name, value.Title, manifest)
	issues = append(issues, validateFormSlots(map[string]string{
		"forms.save": value.Forms.Save, "forms.load": value.Forms.Load,
		"forms.auxiliary_save": value.Forms.AuxiliarySave, "forms.auxiliary_load": value.Forms.AuxiliaryLoad,
	})...)
	// An auxiliary form stands beside a main one; alone it is a form nothing
	// ever opens.
	if value.Forms.AuxiliarySave != "" && value.Forms.Save == "" {
		issues = append(issues, "forms.auxiliary_save stands beside forms.save, which is not named")
	}
	if value.Forms.AuxiliaryLoad != "" && value.Forms.Load == "" {
		issues = append(issues, "forms.auxiliary_load stands beside forms.load, which is not named")
	}
	if err := issuesError(source, value.Format, issues); err != nil {
		return SettingsStorageDefinition{}, err
	}
	return value, nil
}

func cloneSettingsStorage(value SettingsStorageDefinition) SettingsStorageDefinition {
	value.Title = cloneTitle(value.Title)
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

// validateSettingsStorageReferences checks everything that names a storage.
// Until this kind existed those names could not be resolved at all, so a report
// could keep its variants in a storage that was never there.
func (catalog *Catalog) validateSettingsStorageReferences() error {
	for _, item := range catalog.Reports {
		for name, id := range map[string]*uuid.UUID{
			"variants storage": item.VariantsStorage, "settings storage": item.SettingsStorage,
		} {
			if id == nil {
				continue
			}
			if _, ok := catalog.settingsStorageByID[*id]; !ok {
				return fmt.Errorf("report %s keeps its %s in %s, which is not in the configuration", item.Name, name, id)
			}
		}
	}
	return nil
}
