package studio

import (
	"encoding/json"
	"errors"
	"net/http"
	"slices"
	"strconv"
	"strings"

	"github.com/k33alexey/MetaLab/internal/kinds"
	"github.com/k33alexey/MetaLab/internal/metadata"
	"github.com/k33alexey/MetaLab/internal/project"
)

// nameRefusals are the prototype's words at the field of a name
// (docs/block-00/obshchee.md, «Имя и синоним»; METADATA-OBJECTS.md, «Где имя
// уникально»), one for every place Studio takes a name. A name longer than
// the model keeps is a value beyond the limit of its field, which the
// prototype answers with the words of every such field (obshchee.md,
// «Пределы»).
var nameRefusals = map[metadata.NameFault]string{
	metadata.NameEmpty:         "Имя не должно быть пустым!",
	metadata.NameNotIdentifier: `Неверное имя! Имя должно состоять из одного слова, начинаться с буквы и не содержать специальных символов кроме "_".`,
	metadata.NameTooLong:       "В поле введены некорректные данные.",
	metadata.NameTaken:         "Имя не уникально!",
}

// NameCheck asks whether a name typed into the field of a name may stand
// where it is typed. Path is the description that holds it: an object's, or
// the configuration for its own name and its languages. Without At the name
// is the object's own - Name - and its branch is the tree's: the objects
// standing beside it. With At it is a part's: Description is the object as
// the editor holds it, unsaved, and At the way down to the part
// (metadata.PartNameFault).
type NameCheck struct {
	Path        string          `json:"path"`
	Name        string          `json:"name"`
	Description json.RawMessage `json:"description,omitempty"`
	At          []string        `json:"at,omitempty"`
}

// NameAnswer is the prototype's refusal of the name, empty when it fits.
type NameAnswer struct {
	Refusal string `json:"refusal"`
}

// CheckName answers a NameCheck.
func (workspace *Workspace) CheckName(check NameCheck) (NameAnswer, error) {
	workspace.mu.Lock()
	defer workspace.mu.Unlock()
	fault, err := workspace.nameFaultLocked(check)
	return NameAnswer{Refusal: nameRefusals[fault]}, err
}

func (workspace *Workspace) nameFaultLocked(check NameCheck) (metadata.NameFault, error) {
	if check.Path == project.ConfigurationFile {
		if len(check.At) == 0 {
			return metadata.NameRuleFault(check.Name), nil
		}
		return metadata.LanguageNameFault(check.Description, check.At)
	}
	if len(check.At) == 0 {
		return workspace.objectNameFaultLocked(check.Path, "", check.Name)
	}
	configuration, err := project.ValidateLayout(workspace.root)
	if err != nil {
		return metadata.NameFits, err
	}
	return metadata.PartNameFault(check.Path, check.Description, check.At, configuration)
}

// objectNameFaultLocked is the one check of the name of an object: the rule,
// then the objects standing beside it in the tree - the objects of its kind,
// those of one parent where the kind nests. relative is the object's
// description, left out of its branch; a new one has none yet, and stands at
// the top of the branch of group, the tree's node of its kind.
func (workspace *Workspace) objectNameFaultLocked(relative, group, name string) (metadata.NameFault, error) {
	if fault := metadata.NameRuleFault(name); fault != metadata.NameFits {
		return fault, nil
	}
	siblings, err := workspace.branchLocked(relative, group)
	if err != nil {
		return metadata.NameFits, err
	}
	for _, sibling := range siblings {
		if sibling.Path != relative && strings.EqualFold(sibling.Title, name) {
			return metadata.NameTaken, nil
		}
	}
	return metadata.NameFits, nil
}

// objectNameErrorLocked is objectNameFaultLocked for a save or a creation:
// the refusal is the error it is refused with.
func (workspace *Workspace) objectNameErrorLocked(relative, group, name string) error {
	fault, err := workspace.objectNameFaultLocked(relative, group, name)
	if err != nil || fault == metadata.NameFits {
		return err
	}
	return errors.New(nameRefusals[fault])
}

// branchLocked is the objects of the tree standing with the object described
// at relative, it among them; with none there, the top of group. The tree
// shows an object by its name, so a sibling's title is its name.
func (workspace *Workspace) branchLocked(relative, group string) ([]Node, error) {
	configuration, err := project.ValidateLayout(workspace.root)
	if err != nil {
		return nil, err
	}
	// The objects of a branch and their names are read from their folders;
	// what the load adds - the groups of an object, its modules - is no
	// sibling, and a project that does not load still names its objects.
	tree, err := workspace.metadataTree(nil, configuration.DefaultLanguage, configuration.Languages)
	if err != nil {
		return nil, err
	}
	var top []Node
	var find func(nodes []Node) ([]Node, bool)
	find = func(nodes []Node) ([]Node, bool) {
		for _, node := range nodes {
			if relative != "" && node.Path == relative && node.Kind == "metadata" {
				return objectNodes(nodes), true
			}
			if node.ID == group {
				top = objectNodes(node.Children)
			}
			if found, ok := find(node.Children); ok {
				return found, true
			}
		}
		return nil, false
	}
	if found, ok := find(tree); ok {
		return found, nil
	}
	return top, nil
}

// numberedName is the name «Добавить» gives what it makes: base and the
// first number from 1 that no taken name has, in any case - «Справочник1»,
// «Реквизит1» (docs/block-00/obshchee.md, «Добавить» без диалога имени).
// Its twin on the page is numberedName of studio-common.js, which names the
// parts an editor adds; TestANewNameIsNumberedLikeOnThePage runs both.
func numberedName(base string, taken []string) string {
	for index := 1; ; index++ {
		name := base + strconv.Itoa(index)
		if !slices.ContainsFunc(taken, func(other string) bool { return strings.EqualFold(other, name) }) {
			return name
		}
	}
}

// newObjectNameLocked is the name of a new object of a kind: the kind's word
// for one object, numbered among the objects at the top of its branch.
func (workspace *Workspace) newObjectNameLocked(kind string) (string, error) {
	siblings, err := workspace.branchLocked("", branchOf(kind))
	if err != nil {
		return "", err
	}
	taken := make([]string, len(siblings))
	for index, sibling := range siblings {
		taken[index] = sibling.Title
	}
	return numberedName(kinds.Must(kinds.Kind(kind)).Name.Russian, taken), nil
}

// branchOf is the node of the tree that holds the objects of a kind.
func branchOf(kind string) string { return "metadata/" + kind }

// objectNodes is the objects among nodes, without the groups of an object -
// its forms, its commands - that stand beside the objects it holds.
func objectNodes(nodes []Node) []Node {
	var result []Node
	for _, node := range nodes {
		if node.Kind == "metadata" {
			result = append(result, node)
		}
	}
	return result
}

func registerNameRoutes(routes *http.ServeMux, workspace *Workspace) {
	routes.HandleFunc("POST /api/name", answerStudio(0, writeSourceError, func(check NameCheck) (any, error) {
		return workspace.CheckName(check)
	}))
}
