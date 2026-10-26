package studio

import (
	"slices"

	"github.com/k33alexey/MetaLab/internal/kinds"
	"github.com/k33alexey/MetaLab/internal/metadata"
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
}

// objectChoiceTree is the tree of the window «Выбор объекта» offering the
// objects of the kinds offered: their branches in the order of the project
// tree, those of «Общие» inside it (branchPlaces), each with its objects in
// the tree's order.
func objectChoiceTree(loaded *metadata.Catalog, offered []metadata.Kind) []ObjectChoiceBranch {
	var result []ObjectChoiceBranch
	common := ObjectChoiceBranch{Title: commonBranch.Title, Icon: commonBranch.Icon}
	for _, descriptor := range kinds.Branches() {
		if !slices.Contains(offered, descriptor.Kind) {
			continue
		}
		branch := ObjectChoiceBranch{Title: descriptor.Titles, Icon: descriptor.Icon}
		for _, object := range loaded.KindObjects(descriptor.Kind) {
			branch.Objects = append(branch.Objects, ObjectChoiceItem{ID: object.ID, Name: object.Name,
				FullName: descriptor.Name.Russian + "." + object.Name, Icon: descriptor.Icon})
		}
		if branchPlaces[string(descriptor.Kind)] == commonPlace {
			common.Children = append(common.Children, branch)
		} else {
			result = append(result, branch)
		}
	}
	if len(common.Children) > 0 {
		result = append([]ObjectChoiceBranch{common}, result...)
	}
	return result
}
