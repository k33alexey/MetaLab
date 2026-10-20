package metadata

import (
	"maps"
	"slices"
	"testing"
)

// rightsMarkedWith is, by right, every right the window of a role marked
// together with it on an object of a kind, written out from the pass of
// 20.10.2026 with the owner (materials/configurator-2026-10-20/prava/zapis.md,
// «Карта зависимостей прав», snapshots prava/33-41), not read off
// rightPrerequisites: what the window marked, not the steps it took.
type rightsMarkedWith map[PermissionOperation][]PermissionOperation

// with is the map and more beside it.
func (marked rightsMarkedWith) with(more rightsMarkedWith) rightsMarkedWith {
	result := maps.Clone(marked)
	maps.Copy(result, more)
	return result
}

// The map of the pass, kind by kind.
var (
	// A catalog («Бригады», prava/36-38).
	catalogMarked = rightsMarkedWith{
		PermissionInsert: {PermissionRead}, PermissionUpdate: {PermissionRead},
		PermissionDelete: {PermissionRead}, PermissionView: {PermissionRead},
		PermissionInteractiveInsert:            {PermissionRead, PermissionInsert, PermissionUpdate, PermissionView, PermissionEdit},
		PermissionEdit:                         {PermissionRead, PermissionUpdate, PermissionView},
		PermissionInteractiveDelete:            {PermissionRead, PermissionUpdate, PermissionDelete, PermissionView, PermissionEdit},
		PermissionInteractiveSetDeletionMark:   {PermissionRead, PermissionUpdate, PermissionView, PermissionEdit},
		PermissionInteractiveClearDeletionMark: {PermissionRead, PermissionUpdate, PermissionView, PermissionEdit},
		PermissionInteractiveDeleteMarked:      {PermissionRead, PermissionUpdate, PermissionDelete, PermissionView, PermissionEdit},
		PermissionInputByString:                {PermissionRead, PermissionView},
		PermissionInteractiveDeletePredefinedData: {PermissionRead, PermissionUpdate, PermissionDelete, PermissionView,
			PermissionEdit, PermissionInteractiveDelete},
		PermissionInteractiveSetDeletionMarkPredefinedData: {PermissionRead, PermissionUpdate, PermissionView, PermissionEdit,
			PermissionInteractiveSetDeletionMark},
		PermissionInteractiveClearDeletionMarkPredefinedData: {PermissionRead, PermissionUpdate, PermissionView, PermissionEdit,
			PermissionInteractiveClearDeletionMark},
		PermissionInteractiveDeleteMarkedPredefinedData: {PermissionRead, PermissionUpdate, PermissionDelete, PermissionView,
			PermissionEdit, PermissionInteractiveDeleteMarked},
	}.with(historyMarked)
	historyMarked = rightsMarkedWith{
		PermissionReadDataHistory:                 {PermissionRead},
		PermissionReadDataHistoryOfMissingData:    {PermissionRead, PermissionReadDataHistory},
		PermissionUpdateDataHistory:               {PermissionRead, PermissionReadDataHistory},
		PermissionUpdateDataHistoryOfMissingData:  {PermissionRead, PermissionReadDataHistory, PermissionReadDataHistoryOfMissingData, PermissionUpdateDataHistory},
		PermissionUpdateDataHistoryVersionComment: {PermissionRead, PermissionReadDataHistory},
		PermissionViewDataHistory:                 {PermissionRead, PermissionView, PermissionReadDataHistory},
		PermissionEditDataHistoryVersionComment: {PermissionRead, PermissionView, PermissionReadDataHistory,
			PermissionUpdateDataHistoryVersionComment},
		PermissionSwitchToDataHistoryVersion: {PermissionRead, PermissionView},
	}
	// A document (АвансовыйОтчет, prava/40): the catalog's and the posting.
	documentMarked = catalogMarked.with(rightsMarkedWith{
		PermissionPosting: {PermissionRead, PermissionUpdate}, PermissionUndoPosting: {PermissionRead, PermissionUpdate},
		PermissionInteractivePosting: {PermissionRead, PermissionUpdate, PermissionPosting, PermissionView, PermissionEdit},
		PermissionInteractivePostingRegular: {PermissionRead, PermissionUpdate, PermissionPosting, PermissionView, PermissionEdit,
			PermissionInteractivePosting},
		PermissionInteractiveUndoPosting:    {PermissionRead, PermissionUpdate, PermissionUndoPosting, PermissionView, PermissionEdit},
		PermissionInteractiveChangeOfPosted: {PermissionRead, PermissionUpdate, PermissionView, PermissionEdit},
	})
	// The registers and the constant.
	registerMarked = rightsMarkedWith{
		PermissionUpdate: {PermissionRead}, PermissionView: {PermissionRead},
		PermissionEdit: {PermissionRead, PermissionUpdate, PermissionView},
	}.with(historyMarked)
	// A register of information marks the update of the history of missing
	// data without the reading of it (prava/39).
	informationRegisterMarked = registerMarked.with(rightsMarkedWith{
		PermissionUpdateDataHistoryOfMissingData: {PermissionRead, PermissionReadDataHistory, PermissionUpdateDataHistory},
	})
	usedMarked = rightsMarkedWith{PermissionView: {PermissionUse}}
	readMarked = rightsMarkedWith{PermissionView: {PermissionRead}}
	// A table of an external source, by the owner's word of 20.10.2026: the
	// catalog's for the rights it has.
	sourceTableMarked = catalogMarked
)

// The window of a role marks with a right the rights the prototype's does
// and nothing more, on every kind and on the configuration, as the pass
// wrote them; the kinds it did not open by the owner's word of 20.10.2026 -
// a kind with the rights of one opened as that one, a sequence and a
// recalculation the reading with the update, a table of a source as a
// catalog, the source itself nothing. Every kind a role gives rights on is
// named here, a kind gaining rights failing until it is. Defects caught: a
// right missing from what another marks or one too many; the viewing of a
// report marking the reading; the reading of the history of missing data
// marked in a register of information; a kind left without its map.
func TestTheWindowOfARoleMarksTheRightsARightNeeds(t *testing.T) {
	t.Parallel()
	byKind := map[Kind]rightsMarkedWith{
		"":           {PermissionAdministration: {PermissionDataAdministration}},
		CatalogKind:  catalogMarked,
		DocumentKind: documentMarked,
		BusinessProcessKind: catalogMarked.with(rightsMarkedWith{
			PermissionInteractiveActivate: {PermissionRead, PermissionUpdate}, PermissionStart: {PermissionRead, PermissionUpdate},
			PermissionInteractiveStart: {PermissionRead, PermissionUpdate, PermissionStart}}),
		TaskKind: catalogMarked.with(rightsMarkedWith{
			PermissionInteractiveActivate: {PermissionRead, PermissionUpdate}, PermissionExecute: {PermissionRead, PermissionUpdate},
			PermissionInteractiveExecute: {PermissionRead, PermissionUpdate, PermissionExecute}}),
		ChartOfAccountsKind:            catalogMarked,
		ChartOfCharacteristicTypesKind: catalogMarked, ChartOfCalculationTypesKind: catalogMarked, ExchangePlanKind: catalogMarked,
		InformationRegisterKind:  informationRegisterMarked,
		AccumulationRegisterKind: registerMarked, AccountingRegisterKind: registerMarked, CalculationRegisterKind: registerMarked,
		ConstantKind: registerMarked,
		ReportKind:   usedMarked, DataProcessorKind: usedMarked, ExternalFunctionKind: usedMarked,
		DocumentJournalKind: readMarked, ExternalCubeKind: readMarked, ExternalDimensionTableKind: readMarked,
		CommonAttributeKind: {PermissionEdit: {PermissionView}},
		SequenceKind:        {PermissionUpdate: {PermissionRead}}, RecalculationKind: {PermissionUpdate: {PermissionRead}},
		ExternalDataSourceTableKind: sourceTableMarked,
		ExternalDataSourceKind:      {},
		SessionParameterKind:        {}, SubsystemKind: {}, FilterCriterionKind: {}, CommonCommandKind: {}, CommonFormKind: {},
		WebServiceKind: {}, HTTPServiceKind: {},
	}
	offered := map[Kind][]PermissionOperation{"": configurationRights}
	for _, item := range kindInfos {
		if item.uses.offered != nil {
			offered[item.kind] = item.uses.offered
		}
	}
	for kind, rights := range offered {
		want, ok := byKind[kind]
		if !ok {
			t.Errorf("%q: what the window marks with its rights is not written here", kind)
			continue
		}
		for _, right := range rights {
			got := slices.DeleteFunc(rightsRequired(kind, right), func(other PermissionOperation) bool { return !slices.Contains(rights, other) })
			expected := slices.DeleteFunc(slices.Clone(want[right]), func(other PermissionOperation) bool { return !slices.Contains(rights, other) })
			if !slices.Equal(got, inPrototypeOrder(expected)) {
				t.Errorf("%q: %s marks %v, want %v", kind, right, got, inPrototypeOrder(expected))
			}
		}
	}
}

// A part takes of what its right needs only the rights of parts: the
// editing of an attribute marks its viewing, nothing of the object
// (prava/zapis.md: «Редактирование» of an attribute of a task marked nothing
// of the task). Defects caught: the editing of a part marking nothing; a part
// told to mark a right it has not.
func TestTheEditingOfAPartMarksItsViewing(t *testing.T) {
	t.Parallel()
	for _, item := range kindInfos {
		for _, group := range partGroups {
			rights := offeredPartRights(item.kind, group.group)
			for _, right := range rights {
				got := slices.DeleteFunc(rightsRequired(item.kind, right), func(other PermissionOperation) bool { return !slices.Contains(rights, other) })
				var want []PermissionOperation
				if right == PermissionEdit && slices.Contains(rights, PermissionView) {
					want = []PermissionOperation{PermissionView}
				}
				if !slices.Equal(got, want) {
					t.Errorf("%s, %s: %s marks %v, want %v", item.kind, group.group, right, got, want)
				}
			}
		}
	}
}

// Clearing a right clears every right needing it, as the window of the
// prototype did (prava/41: «Изменение» cleared on a chart of accounts with
// every right marked; prava/23: «Чтение» cleared on the branch of catalogs),
// written out from the snapshots: what stays marked is the rest. Defects
// caught: a right needing the cleared one left marked; one needing it not
// cleared through another (the predefined through their plain right).
func TestClearingARightClearsTheRightsNeedingIt(t *testing.T) {
	t.Parallel()
	needing := func(kind Kind, cleared PermissionOperation) []PermissionOperation {
		var result []PermissionOperation
		for _, right := range usesOf(kind).offered {
			if slices.Contains(rightsRequired(kind, right), cleared) {
				result = append(result, right)
			}
		}
		return result
	}
	updateNeeded := []PermissionOperation{PermissionInteractiveInsert, PermissionEdit, PermissionInteractiveDelete,
		PermissionInteractiveSetDeletionMark, PermissionInteractiveClearDeletionMark, PermissionInteractiveDeleteMarked,
		PermissionInteractiveDeletePredefinedData, PermissionInteractiveSetDeletionMarkPredefinedData,
		PermissionInteractiveClearDeletionMarkPredefinedData, PermissionInteractiveDeleteMarkedPredefinedData}
	if got := needing(ChartOfAccountsKind, PermissionUpdate); !slices.Equal(got, inPrototypeOrder(updateNeeded)) {
		t.Errorf("«Изменение» cleared on a chart of accounts clears %v, want %v", got, inPrototypeOrder(updateNeeded))
	}
	kept := []PermissionOperation{PermissionRead, PermissionUpdateDataHistorySettings}
	for _, right := range usesOf(CatalogKind).offered {
		if cleared := slices.Contains(needing(CatalogKind, PermissionRead), right); cleared == slices.Contains(kept, right) {
			t.Errorf("«Чтение» cleared on a catalog: %s cleared %v", right, cleared)
		}
	}
}
