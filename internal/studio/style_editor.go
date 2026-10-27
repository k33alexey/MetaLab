package studio

import (
	"net/http"

	"github.com/k33alexey/MetaLab/internal/metadata"
	"github.com/k33alexey/MetaLab/internal/uuid"
)

// The window of a style (docs/block-00/stil.md, «Окно»; ML-STUDIO.md, «Окно
// стиля»): at the left the name, the synonym, the comment, the subsystems
// and the preview drawn by the style; at the right every style item the
// window lists with its value in this style. Its palette opens the same
// values in a window of their own («Стиль: Открыть»). The windows choosing
// a colour, a font and a border are items of their own; «…» of a value
// stands unavailable until them.

// StyleEditorSource is a style as its window and palette read and write it.
type StyleEditorSource struct {
	windowHead
	Style metadata.StyleDefinition `json:"style"`
	// Items are the style items the window lists, in its order: the
	// platform's five, then the project's in the order of the tree.
	Items []StyleWindowItem `json:"items"`
	// Platform are all the platform's own style items, which a value may be
	// taken from, and WebColors the web palette a value may name.
	Platform  []metadata.PlatformStyleItem `json:"platform"`
	WebColors []metadata.WebColorName      `json:"webColors"`
}

// StyleWindowItem is a style item as the window of a style lists it: one of
// the platform's by its name (Standard), or one of the project's by its
// identifier with its name, synonym and own value - what a style that gives
// it none shows.
type StyleWindowItem struct {
	Standard string                   `json:"standard,omitempty"`
	Item     *uuid.UUID               `json:"item,omitempty"`
	Name     string                   `json:"name,omitempty"`
	Title    metadata.LocalizedText   `json:"title,omitempty"`
	Type     metadata.StyleItemType   `json:"type"`
	Value    *metadata.StyleItemValue `json:"value,omitempty"`
}

func (source StyleEditorSource) windowOf() (metadata.StyleDefinition, windowHead) {
	return source.Style, source.windowHead
}

func (StyleEditorSource) named(value metadata.StyleDefinition) (uuid.UUID, string) {
	return value.ID, value.Name
}

func (source StyleEditorSource) identity() (uuid.UUID, string) {
	return source.Style.ID, source.Revision
}

// StyleWindowValue is what the window of a style writes: the style and,
// when the subsystems were chosen, the subsystems marked.
type StyleWindowValue struct {
	metadata.StyleDefinition
	Subsystems *[]uuid.UUID `json:"subsystems,omitempty"`
}

// styleEditorAddress is where the window of a style is read and written.
const styleEditorAddress = "/api/style"

func (workspace *Workspace) ReadStyleEditor(relative string) (StyleEditorSource, error) {
	workspace.mu.Lock()
	defer workspace.mu.Unlock()
	return workspace.readStyleEditorLocked(relative)
}

func (workspace *Workspace) readStyleEditorLocked(relative string) (StyleEditorSource, error) {
	value, head, loaded, err := readObjectWindow(workspace, metadata.StyleKind, relative, metadata.DecodeStyle,
		func(value metadata.StyleDefinition) uuid.UUID { return value.ID })
	if err != nil {
		return StyleEditorSource{}, err
	}
	items := []StyleWindowItem{}
	for _, item := range metadata.PlatformStyleItems {
		if item.InStyleWindow {
			items = append(items, StyleWindowItem{Standard: item.Name, Type: item.Type})
		}
	}
	for _, object := range loaded.KindObjects(metadata.StyleItemKind) {
		item, _ := loaded.StyleItemByID(object.ID)
		own := item.Value
		items = append(items, StyleWindowItem{Item: &item.ID, Name: item.Name, Title: item.Title, Type: item.Type, Value: &own})
	}
	return StyleEditorSource{windowHead: head, Style: value, Items: items, Platform: metadata.PlatformStyleItems, WebColors: metadata.WebColors}, nil
}

// SaveStyleWindow writes the style and the subsystems its window marks as
// one save (saveObjectWindow).
func (workspace *Workspace) SaveStyleWindow(relative string, window StyleWindowValue, revision string) (StyleEditorSource, error) {
	return saveObjectWindowAs(workspace, relative, revision, window.StyleDefinition, window.Subsystems, workspace.readStyleEditorLocked)
}

// CreateStyle makes the next style - one giving no item a value of its own
// (stil.md, «Новый стиль») - by the one way every object is made
// (CreateObject) and opens its window.
func (workspace *Workspace) CreateStyle(name string) (StyleEditorSource, error) {
	return createObjectAs(workspace, metadata.StyleKind, name, nil, workspace.readStyleEditorLocked)
}

// DeleteStyle removes the file of a style by the one order of steps of
// every deletion.
func (workspace *Workspace) DeleteStyle(relative, expectedRevision string) error {
	return deleteObjectFolder(workspace, metadata.StyleKind, relative, expectedRevision, workspace.readStyleEditorLocked)
}

func registerStyleEditorRoutes(routes *http.ServeMux, workspace *Workspace) {
	registerObjectEditor(routes, styleEditorAddress, workspace.ReadStyleEditor, workspace.SaveStyleWindow, workspace.CreateStyle, workspace.DeleteStyle)
}
