package studio

import (
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"regexp"

	"go.yaml.in/yaml/v3"
)

// The settings of the lists of Studio - the columns shown, their order,
// place and size, the row a list opens on, its order - are the user's, not
// the project's (help, form_customizegriddlg: the settings are kept for
// each form and each user apart). They live in the user's settings of the
// operating system, a file for each configuration, and never in the
// project or in Git (the owner's decision of 19.10.2026, ML-STUDIO.md,
// «Настройка списка — стандартный механизм»). What the setting of a list
// holds and means the page alone knows (studio-list.js), and it reads a
// setting it does not understand as the standard one: the server keeps the
// setting of each list as the page sent it, an object under the key of the
// list.

// listSettingsFile is the file of a configuration's lists in the directory
// of the user's settings.
const listSettingsFile = "lists.yaml"

// listSettingsLimit is the most a request writing the setting of one list
// may carry: a setting names each column of a list once.
const listSettingsLimit = 64 << 10

// listKeyPattern is what the key of a list may be: the page names its
// lists so, and a key is a key of the file, never a path.
var listKeyPattern = regexp.MustCompile(`^[a-z][a-z0-9-]{0,63}$`)

// ErrInvalidListSettings is a setting of a list the page should not have
// sent: a key of another form, or a setting that is not an object.
var ErrInvalidListSettings = errors.New("invalid list settings")

// UserSettingsDirectory is where Studio keeps the settings of its user: the
// directory of MetaLab in the user's settings of the operating system, where
// the platform keeps its own (appconfig.DefaultPath), and in it the folder of
// Studio.
func UserSettingsDirectory() (string, error) {
	directory, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("resolve user settings directory: %w", err)
	}
	return filepath.Join(directory, "MetaLab", "studio"), nil
}

// KeepUserSettingsIn makes the workspace keep the settings of its lists in
// directory, in a folder of the configuration's identifier. A workspace not
// told so keeps them for as long as it is open.
func (workspace *Workspace) KeepUserSettingsIn(directory string) {
	workspace.listSettingsMu.Lock()
	defer workspace.listSettingsMu.Unlock()
	workspace.listSettingsDirectory = directory
}

// ListSettings answers the settings of every list the user has set up, by
// the key of the list.
func (workspace *Workspace) ListSettings() (map[string]any, error) {
	workspace.listSettingsMu.Lock()
	defer workspace.listSettingsMu.Unlock()
	return workspace.readListSettingsLocked()
}

// SaveListSettings keeps settings as the setting of the list key. The
// settings of other lists stay as they are.
func (workspace *Workspace) SaveListSettings(key string, settings map[string]any) error {
	if !listKeyPattern.MatchString(key) {
		return fmt.Errorf("%w: list key %q", ErrInvalidListSettings, key)
	}
	if settings == nil {
		return fmt.Errorf("%w: the settings of list %q are not an object", ErrInvalidListSettings, key)
	}
	workspace.listSettingsMu.Lock()
	defer workspace.listSettingsMu.Unlock()
	lists, err := workspace.readListSettingsLocked()
	// A file that does not read is no setting at all: the page has shown
	// the standard ones, and what the user sets now takes its place.
	if err != nil {
		lists = map[string]any{}
	}
	lists[key] = settings
	if workspace.listSettingsDirectory == "" {
		workspace.listSettingsMemory = lists
		return nil
	}
	directory := filepath.Join(workspace.listSettingsDirectory, workspace.configurationID.String())
	if err := os.MkdirAll(directory, 0o700); err != nil {
		return fmt.Errorf("create the directory of list settings: %w", err)
	}
	content, err := yaml.Marshal(lists)
	if err != nil {
		return fmt.Errorf("encode list settings: %w", err)
	}
	temporary, err := prepareStudioSource(directory, content)
	if err != nil {
		return err
	}
	defer os.Remove(temporary)
	if err := replaceStudioFile(temporary, filepath.Join(directory, listSettingsFile)); err != nil {
		return fmt.Errorf("replace list settings: %w", err)
	}
	return nil
}

func (workspace *Workspace) readListSettingsLocked() (map[string]any, error) {
	lists := map[string]any{}
	if workspace.listSettingsDirectory == "" {
		for key, value := range workspace.listSettingsMemory {
			lists[key] = value
		}
		return lists, nil
	}
	path := filepath.Join(workspace.listSettingsDirectory, workspace.configurationID.String(), listSettingsFile)
	content, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return lists, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read list settings: %w", err)
	}
	if err := yaml.Unmarshal(content, &lists); err != nil {
		return nil, fmt.Errorf("read list settings %s: %w", path, err)
	}
	if lists == nil {
		lists = map[string]any{}
	}
	return lists, nil
}

func registerListSettingsRoutes(routes *http.ServeMux, workspace *Workspace) {
	refuse := func(response http.ResponseWriter, err error) {
		status := http.StatusInternalServerError
		if errors.Is(err, ErrInvalidListSettings) {
			status = http.StatusBadRequest
		}
		http.Error(response, err.Error(), status)
	}
	routes.HandleFunc("GET /api/settings/lists", answerQuery(refuse, func(*http.Request) (any, error) {
		lists, err := workspace.ListSettings()
		return struct {
			Lists map[string]any `json:"lists"`
		}{lists}, err
	}))
	routes.HandleFunc("PUT /api/settings/list", answerStudio(listSettingsLimit, refuse, func(input struct {
		Key      string         `json:"key"`
		Settings map[string]any `json:"settings"`
	}) (any, error) {
		return struct{}{}, workspace.SaveListSettings(input.Key, input.Settings)
	}))
}
