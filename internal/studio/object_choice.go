package studio

import (
	"path"
	"slices"

	"github.com/k33alexey/MetaLab/internal/kinds"
	"github.com/k33alexey/MetaLab/internal/metadata"
	"github.com/k33alexey/MetaLab/internal/project"
	"github.com/k33alexey/MetaLab/internal/uuid"
)

// ObjectChoiceBranch is a branch of the platform's window «Выбор объекта»
// (docs/block-00/spravochnik.md, snapshots vladelcy-pyat-vidov,
// vladelcy-vybor-obekta): the branch of a kind with its objects, or «Общие»
// with the branches of its kinds, as the project tree stands them.
type ObjectChoiceBranch struct {
	Title    string               `json:"title"`
	Icon     string               `json:"icon"`
	Children []ObjectChoiceBranch `json:"children,omitempty"`
	Objects  []ObjectChoiceItem   `json:"objects,omitempty"`
}

// ObjectChoiceItem is an object the window offers: its name, its full name
// as a list of chosen objects shows it (Справочник.Контрагенты) and its icon.
type ObjectChoiceItem struct {
	ID       uuid.UUID `json:"id"`
	Name     string    `json:"name"`
	FullName string    `json:"fullName"`
	Icon     string    `json:"icon"`
	// Path is where the object is described, for a window that opens it -
	// the magnifier of a main form; none for an object of no folder.
	Path string `json:"path,omitempty"`
	// Items are what the window offers inside the object: its forms in
	// «Выбор управляемой формы» (homePageFormTree).
	Items []ObjectChoiceItem `json:"items,omitempty"`
}

// objectChoiceTree is the tree of the window «Выбор объекта» offering the
// objects of the kinds offered: their branches in the order of the project
// tree, those of «Общие» inside it and those standing in the branch of
// another kind - numerators in «Документы» - inside that branch before its
// objects (branchPlaces), each with its objects in the tree's order.
func objectChoiceTree(loaded *metadata.Catalog, offered []metadata.Kind) []ObjectChoiceBranch {
	return shapedObjectChoiceTree(loaded, offered, nil)
}

// shapedObjectChoiceTree is objectChoiceTree whose objects shape gives what
// they hold or leaves out, answering false; nil keeps every object as it is.
func shapedObjectChoiceTree(loaded *metadata.Catalog, offered []metadata.Kind, shape func(metadata.Kind, *ObjectChoiceItem) bool) []ObjectChoiceBranch {
	var result []ObjectChoiceBranch
	common := ObjectChoiceBranch{Title: commonBranch.Title, Icon: commonBranch.Icon}
	at := map[string]int{}
	type within struct {
		place  string
		branch ObjectChoiceBranch
	}
	var inside []within
	for _, descriptor := range kinds.Branches() {
		if !slices.Contains(offered, descriptor.Kind) {
			continue
		}
		branch := ObjectChoiceBranch{Title: descriptor.Titles, Icon: descriptor.Icon}
		for _, object := range loaded.KindObjects(descriptor.Kind) {
			item := ObjectChoiceItem{ID: object.ID, Name: object.Name, FullName: descriptor.Name.Russian + "." + object.Name, Icon: descriptor.Icon}
			if directory, err := project.ObjectDirectory(string(descriptor.Kind), object.Name); err == nil && descriptor.Layout().KeepsFolder() {
				item.Path = path.Join(directory, project.DescriptionFile(string(descriptor.Kind)))
			}
			if shape != nil && !shape(descriptor.Kind, &item) {
				continue
			}
			branch.Objects = append(branch.Objects, item)
		}
		switch place := branchPlaces[string(descriptor.Kind)]; place {
		case commonPlace:
			common.Children = append(common.Children, branch)
		case "":
			at[string(descriptor.Kind)] = len(result)
			result = append(result, branch)
		default:
			inside = append(inside, within{place, branch})
		}
	}
	for _, item := range inside {
		if owner, ok := at[item.place]; ok {
			result[owner].Children = append(result[owner].Children, item.branch)
		} else {
			result = append(result, item.branch)
		}
	}
	if len(common.Children) > 0 {
		result = append([]ObjectChoiceBranch{common}, result...)
	}
	return result
}
