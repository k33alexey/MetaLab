package metadata

import (
	"maps"
	"slices"
	"testing"

	"github.com/k33alexey/MetaLab/internal/kinds"
	"github.com/k33alexey/MetaLab/internal/uuid"
)

// TestTypesNameTheirKinds catches a type of a type description that names an
// object of the wrong kind, or of none: a reference checked against the index
// of another kind refuses every right value and lets a wrong one through, and
// a type left out of the table is not checked at all. The expectation is
// written out here from the help (8.3.27: СправочникСсылка, СправочникОбъект,
// СправочникМенеджер, РегистрСведенийНаборЗаписей, ... - each type of the
// classes of a kind), not read from kindInfos.
func TestTypesNameTheirKinds(t *testing.T) {
	t.Parallel()
	expected := map[TypeKind]Kind{
		CatalogType: CatalogKind, CatalogObjectType: CatalogKind, CatalogManagerType: CatalogKind,
		DocumentType: DocumentKind, DocumentObjectType: DocumentKind, DocumentManagerType: DocumentKind,
		EnumerationType: EnumerationKind, EnumerationManagerType: EnumerationKind,
		CharacteristicTypesType: ChartOfCharacteristicTypesKind, CharacteristicTypesObjectType: ChartOfCharacteristicTypesKind,
		CharacteristicTypesManagerType: ChartOfCharacteristicTypesKind, CharacteristicSet: ChartOfCharacteristicTypesKind,
		AccountType: ChartOfAccountsKind, AccountsObjectType: ChartOfAccountsKind, AccountsManagerType: ChartOfAccountsKind,
		CalculationTypeType: ChartOfCalculationTypesKind, CalculationTypesObjectType: ChartOfCalculationTypesKind,
		CalculationTypesManagerType: ChartOfCalculationTypesKind,
		BusinessProcessType:         BusinessProcessKind, RoutePointType: BusinessProcessKind,
		BusinessProcessObjectType: BusinessProcessKind, BusinessProcessManagerType: BusinessProcessKind,
		TaskType: TaskKind, TaskObjectType: TaskKind, TaskManagerType: TaskKind,
		ExchangePlanType: ExchangePlanKind, ExchangePlanObjectType: ExchangePlanKind, ExchangePlanManagerType: ExchangePlanKind,
		InformationRegisterRecordSetType: InformationRegisterKind, InformationRegisterRecordManagerType: InformationRegisterKind,
		InformationRegisterManagerType:    InformationRegisterKind,
		AccumulationRegisterRecordSetType: AccumulationRegisterKind, AccumulationRegisterManagerType: AccumulationRegisterKind,
		AccountingRegisterRecordSetType: AccountingRegisterKind, AccountingRegisterManagerType: AccountingRegisterKind,
		CalculationRegisterRecordSetType: CalculationRegisterKind, CalculationRegisterManagerType: CalculationRegisterKind,
		SequenceRecordSetType: SequenceKind, RecalculationRecordSetType: RecalculationKind,
		ConstantValueManagerType: ConstantKind, DocumentJournalManagerType: DocumentJournalKind,
		ReportObjectType: ReportKind, DataProcessorObjectType: DataProcessorKind,
		ReportManagerType: ReportKind, DataProcessorManagerType: DataProcessorKind,
		SettingsStorageManagerType: SettingsStorageKind, FilterCriterionManagerType: FilterCriterionKind,
		DefinedType:       DefinedTypeKind,
		ExternalTableType: ExternalDataSourceTableKind, ExternalDimensionTableType: ExternalDimensionTableKind,
	}
	if !maps.Equal(typeOwners, expected) {
		for kind, owner := range expected {
			if typeOwners[kind] != owner {
				t.Errorf("%s names an object of %q, the prototype's of %s", kind, typeOwners[kind], owner)
			}
		}
		for kind := range typeOwners {
			if _, ok := expected[kind]; !ok {
				t.Errorf("%s names an object of %s, and names none", kind, typeOwners[kind])
			}
		}
	}
	// What lives in memory is the object, the record set and the managers;
	// a reference, a defined type and a characteristic are stored.
	for kind := range expected {
		inMemory := slices.Contains([]TypeKind{CatalogObjectType, CatalogManagerType, DocumentObjectType, DocumentManagerType,
			EnumerationManagerType, CharacteristicTypesObjectType, CharacteristicTypesManagerType, AccountsObjectType,
			AccountsManagerType, CalculationTypesObjectType, CalculationTypesManagerType, BusinessProcessObjectType,
			BusinessProcessManagerType, TaskObjectType, TaskManagerType, ExchangePlanObjectType, ExchangePlanManagerType,
			InformationRegisterRecordSetType, InformationRegisterRecordManagerType, InformationRegisterManagerType,
			AccumulationRegisterRecordSetType, AccumulationRegisterManagerType, AccountingRegisterRecordSetType,
			AccountingRegisterManagerType, CalculationRegisterRecordSetType, CalculationRegisterManagerType,
			SequenceRecordSetType, RecalculationRecordSetType, ConstantValueManagerType, DocumentJournalManagerType,
			ReportObjectType, DataProcessorObjectType, ReportManagerType, DataProcessorManagerType,
			SettingsStorageManagerType, FilterCriterionManagerType}, kind)
		if isObjectType(kind) != inMemory {
			t.Errorf("%s lives in memory: %t, the prototype's: %t", kind, isObjectType(kind), inMemory)
		}
	}
}

// TestSetsHoldTheirReferences catches a set expanding to the references of
// another kind, or a set the model no longer knows as one: an attribute typed
// "any catalog" would then accept documents, or be stored in a column of its
// own and rebuilt the day a second catalog appears.
func TestSetsHoldTheirReferences(t *testing.T) {
	t.Parallel()
	expected := map[TypeKind]TypeKind{
		CatalogSet: CatalogType, DocumentSet: DocumentType, EnumerationSet: EnumerationType,
		CharacteristicTypesSet: CharacteristicTypesType, AccountSet: AccountType, CalculationTypeSet: CalculationTypeType,
		BusinessProcessSet: BusinessProcessType, RoutePointSet: RoutePointType, TaskSet: TaskType, ExchangePlanSet: ExchangePlanType,
	}
	got := map[TypeKind]TypeKind{}
	for _, item := range kindInfos {
		for _, reference := range item.references {
			if reference.set != "" {
				got[reference.set] = reference.kind
			}
		}
	}
	if !maps.Equal(got, expected) {
		t.Errorf("sets hold %v, the prototype's %v", got, expected)
	}
	for set := range expected {
		if !referenceSets[set] || !IsTypeSet(set) {
			t.Errorf("%s is not a set", set)
		}
	}
	if len(referenceSets) != len(expected)+1 || !referenceSets[AnyReferenceSet] {
		t.Errorf("the sets are %v", referenceSets)
	}
}

// TestEveryReferenceIsNamed catches a reference type of the model without
// its name in the kinds package, or the names and the types of one kind out
// of step: the type editor would offer a reference under another's name.
func TestEveryReferenceIsNamed(t *testing.T) {
	t.Parallel()
	for _, item := range kindInfos {
		if names := kinds.Must(item.kind).References(); len(names) != len(item.references) {
			t.Errorf("%s has %d reference types and %d names of them", item.kind, len(item.references), len(names))
		}
	}
}

// TestEveryKindFindsItsObjects catches an object found by the index of
// another kind, or not found by its own: every object of the composite
// project, which holds every kind the loader reads, must be found as an
// object of its kind and of no other. Identifiers are unique across a
// configuration, so a hit under another kind is a wrong index, not a
// coincidence.
func TestEveryKindFindsItsObjects(t *testing.T) {
	t.Parallel()
	catalog, err := Load(compositeProject(t))
	if err != nil {
		t.Fatal(err)
	}
	objects := map[Kind][]uuid.UUID{}
	for _, item := range kindInfos {
		list, ok := catalog.objectsOf(item.kind)
		if !ok {
			continue
		}
		for index := range list.Len() {
			objects[item.kind] = append(objects[item.kind], list.Index(index).FieldByName("ID").Interface().(uuid.UUID))
		}
	}
	for _, source := range catalog.ExternalDataSources {
		for _, table := range source.Tables {
			objects[ExternalDataSourceTableKind] = append(objects[ExternalDataSourceTableKind], table.ID)
		}
		for _, function := range source.Functions {
			objects[ExternalFunctionKind] = append(objects[ExternalFunctionKind], function.ID)
		}
		for _, cube := range source.Cubes {
			objects[ExternalCubeKind] = append(objects[ExternalCubeKind], cube.ID)
			for _, table := range cube.DimensionTables {
				objects[ExternalDimensionTableKind] = append(objects[ExternalDimensionTableKind], table.ID)
			}
		}
	}
	for _, register := range catalog.CalculationRegisters {
		for _, recalculation := range register.Recalculations {
			objects[RecalculationKind] = append(objects[RecalculationKind], recalculation.ID)
		}
	}
	for id, object := range catalog.objects {
		if object.kind == CommonFormKind {
			objects[CommonFormKind] = append(objects[CommonFormKind], id)
		}
	}
	for _, language := range catalog.Project.Languages {
		objects[LanguageKind] = append(objects[LanguageKind], language.ID)
	}
	for _, item := range kindInfos {
		if item.has == nil {
			t.Errorf("%s finds no object", item.kind)
		}
		if item.has != nil && len(objects[item.kind]) == 0 {
			t.Errorf("the composite project holds no %s, so finding one is not tested", item.kind)
		}
	}
	for kind, ids := range objects {
		for _, id := range ids {
			for _, other := range kindInfos {
				if found := catalog.hasObject(other.kind, id); found != (other.kind == kind) {
					t.Errorf("%s %s found as %s: %t", kind, id, other.kind, found)
				}
			}
			for _, reference := range kindInfoMust(kind).references {
				if found, known := catalog.typeNamesObject(reference.kind, id); !found || !known {
					t.Errorf("%s of %s %s: found %t, known %t", reference.kind, kind, id, found, known)
				}
			}
		}
	}
}

func kindInfoMust(kind Kind) kindInfo {
	item, _ := kindInfoOf(kind)
	return item
}

// TestEveryKindSaysWhatMayNameIt catches a kind that says nothing of a use
// (a new kind left out of rights, registration or functional options by
// omission), and a use given to the wrong kinds: a right on a recalculation
// refused (demo-base and energy give them), a registration of an enumeration
// accepted, a kind the exports never show taken silently instead of with a
// note, a right on a dimension of a cube offered as on one of a register. The expectation is written out here, not read from kindUsesOf: what
// the exports of the configurator write (the Rights.xml of every role, the
// Content.xml of every exchange plan, the Content of every functional option
// of the four checked, demo-base and energy), and for registration the help
// besides - the kinds whose data has ОбменДанными (8.3.27).
func TestEveryKindSaysWhatMayNameIt(t *testing.T) {
	t.Parallel()
	shown := map[string][]Kind{
		"rights": {SubsystemKind, SessionParameterKind, CommonAttributeKind, ExchangePlanKind, FilterCriterionKind,
			CommonFormKind, CommonCommandKind, ConstantKind, CatalogKind, DocumentKind, DocumentJournalKind, SequenceKind,
			ReportKind, DataProcessorKind, ChartOfCharacteristicTypesKind, ChartOfAccountsKind, ChartOfCalculationTypesKind,
			InformationRegisterKind, AccumulationRegisterKind, AccountingRegisterKind, CalculationRegisterKind,
			RecalculationKind, BusinessProcessKind, TaskKind, ExternalDataSourceKind, ExternalDataSourceTableKind,
			ExternalCubeKind, ExternalDimensionTableKind, WebServiceKind, HTTPServiceKind},
		"registration": {ConstantKind, CatalogKind, DocumentKind, SequenceKind, ChartOfCharacteristicTypesKind,
			ChartOfAccountsKind, ChartOfCalculationTypesKind, BusinessProcessKind, TaskKind, InformationRegisterKind,
			AccumulationRegisterKind, AccountingRegisterKind, CalculationRegisterKind, RecalculationKind},
		"option": {AccountingRegisterKind, AccumulationRegisterKind, BusinessProcessKind, CalculationRegisterKind,
			CatalogKind, ChartOfAccountsKind, ChartOfCalculationTypesKind, ChartOfCharacteristicTypesKind,
			CommonAttributeKind, CommonCommandKind, CommonFormKind, ConstantKind, DataProcessorKind, DocumentKind,
			DocumentJournalKind, EnumerationKind, ExchangePlanKind, FilterCriterionKind, InformationRegisterKind,
			ReportKind, SubsystemKind, TaskKind},
		// The kinds the exports restrict access to (8318 restrictions of 2526
		// roles, survey of 12.10.2026).
		"restriction": {CatalogKind, DocumentKind, DocumentJournalKind, ChartOfCharacteristicTypesKind, InformationRegisterKind,
			AccumulationRegisterKind, AccountingRegisterKind, CalculationRegisterKind, BusinessProcessKind, TaskKind},
		// The kinds the exports give rights on a part of (survey of the
		// Rights.xml of 13.10.2026).
		"parts": {CatalogKind, DocumentKind, DocumentJournalKind, ChartOfCharacteristicTypesKind, ChartOfAccountsKind,
			ChartOfCalculationTypesKind, ExchangePlanKind, BusinessProcessKind, TaskKind, ReportKind, DataProcessorKind,
			InformationRegisterKind, AccumulationRegisterKind, AccountingRegisterKind, CalculationRegisterKind,
			ExternalDataSourceTableKind, WebServiceKind, HTTPServiceKind},
	}
	// The kinds whose parts no export gives rights on although the model
	// holds them: the commands of an enumeration and a filter criterion, the
	// dimensions, resources and commands of a cube, the fields and commands of
	// a dimension table (help: ВнешнийИсточникДанныхКубЗапись,
	// ВнешнийИсточникДанныхКубТаблицаИзмеренияОбъект). The rest hold no part a
	// right is given on.
	partsUnshown := []Kind{EnumerationKind, FilterCriterionKind, ExternalCubeKind, ExternalDimensionTableKind}
	// The common kinds the window «Выбор объекта» of the content of a
	// functional option does not offer (opis of the session parameter,
	// 14.10.2026).
	optionRefused := []Kind{RoleKind, SessionParameterKind, CommonModuleKind, EventSubscriptionKind, ScheduledJobKind,
		CommandGroupKind, CommonTemplateKind, XDTOPackageKind, WebServiceKind, WSReferenceKind, WebSocketClientKind,
		HTTPServiceKind, CommonPictureKind, StyleItemKind, StyleKind, FunctionalOptionKind, FunctionalOptionParameterKind,
		SettingsStorageKind, DefinedTypeKind, LanguageKind}
	// The help restricts reading, adding, changing and deleting only: a kind
	// offered none of the four keeps no data to restrict.
	keepsData := func(kind Kind) bool {
		for _, right := range kindRightsOf[kind] {
			if right == PermissionRead || right == PermissionInsert || right == PermissionUpdate || right == PermissionDelete {
				return true
			}
		}
		return false
	}
	// The help gives ОбменДанными to the data of these and of no other kind;
	// of them, only an exchange plan is in no export's content.
	exchanged := append(slices.Clone(shown["registration"]), ExchangePlanKind)
	for _, item := range kinds.All() {
		info, ok := kindInfoOf(item.Kind)
		if !ok {
			t.Errorf("%s has no descriptor in the model", item.Kind)
			continue
		}
		for use, got := range map[string]presence{"rights": info.uses.rights, "registration": info.uses.registration,
			"option": info.uses.option, "restriction": info.uses.restriction, "parts": info.uses.parts} {
			expected := unproven
			switch {
			case slices.Contains(shown[use], item.Kind):
				expected = present
			case use == "option" && slices.Contains(optionRefused, item.Kind):
				expected = absent
			case use == "registration" && !slices.Contains(exchanged, item.Kind):
				expected = absent
			case use == "restriction" && !keepsData(item.Kind):
				expected = absent
			case use == "parts" && !slices.Contains(partsUnshown, item.Kind):
				expected = absent
			}
			switch {
			case got == unsaid:
				t.Errorf("%s says nothing of its %s: every kind says whether the prototype has it", item.Kind, use)
			case got != expected:
				t.Errorf("%s of %s is %d, the prototype's %d", use, item.Kind, got, expected)
			}
		}
	}
}

// TestTheWizardOffersThePrototypesKinds catches the wizard «Новое
// ограничение доступа» offering on its first step a kind the prototype's
// does not, or leaving out one it does (review 2.464, Р1): the wizard
// offered whatever restricts - a sequence, a recalculation, the table, the
// cube and the table of dimensions of an external source. The kinds are
// written out here from the seventh pass of 22.10.2026
// (vse-ogranicheniya-dostupa.md, «Что мастер предлагает»), not read from
// kindUsesOf; a kind saying nothing of it fails, so a new kind is not left
// out by omission.
func TestTheWizardOffersThePrototypesKinds(t *testing.T) {
	t.Parallel()
	offered := []Kind{CatalogKind, DocumentKind, DocumentJournalKind, ChartOfCharacteristicTypesKind, ChartOfAccountsKind,
		ChartOfCalculationTypesKind, InformationRegisterKind, AccumulationRegisterKind, AccountingRegisterKind,
		CalculationRegisterKind, BusinessProcessKind, TaskKind, ExchangePlanKind}
	for _, item := range kinds.All() {
		got := usesOf(item.Kind).wizard
		want := absent
		if slices.Contains(offered, item.Kind) {
			want = present
		}
		switch {
		case got == unsaid:
			t.Errorf("%s says nothing of the wizard of a new restriction", item.Kind)
		case got != want:
			t.Errorf("the wizard of a new restriction offers %s: %d, the prototype's %d", item.Kind, got, want)
		}
	}
}
