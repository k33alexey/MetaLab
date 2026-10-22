package metadata

import (
	"slices"
	"testing"

	"github.com/k33alexey/MetaLab/internal/icons"
	"github.com/k33alexey/MetaLab/internal/uuid"
)

// Every kind shows under an object the branches the prototype's tree does, in
// its order (inventory of block 00, «Ветки под объектом», snapshots derevo/
// of 19.10.2026); the expectation is written here from the snapshots, not
// read off partGroups. Defects caught: a kind with parts shown without a
// branch of them, or with one it has not (the fields a criterion of filter
// searches by shown as «Поля»); branches out of the prototype's order (the
// flags of a chart of accounts after its table parts, the addressing of a task
// after its attributes); commands or standard attributes given a branch of
// parts beside the tree's «Команды».
func TestEveryKindShowsTheBranchesOfThePrototypesTree(t *testing.T) {
	t.Parallel()
	ordinary := []PartGroup{AttributesPart, TablePartsPart}
	register := []PartGroup{DimensionsPart, ResourcesPart, AttributesPart}
	want := map[Kind][]PartGroup{
		CatalogKind: ordinary, DocumentKind: ordinary, ChartOfCharacteristicTypesKind: ordinary, ChartOfCalculationTypesKind: ordinary,
		ExchangePlanKind: ordinary, BusinessProcessKind: ordinary, ReportKind: ordinary, DataProcessorKind: ordinary,
		ChartOfAccountsKind:     {AttributesPart, AccountingFlagsPart, ExtDimensionAccountingFlagsPart, TablePartsPart},
		TaskKind:                {AddressingAttributesPart, AttributesPart, TablePartsPart},
		InformationRegisterKind: register, AccumulationRegisterKind: register, AccountingRegisterKind: register, CalculationRegisterKind: register,
		ExternalDataSourceTableKind: {FieldsPart}, ExternalDimensionTableKind: {FieldsPart}, ExternalCubeKind: {DimensionsPart, ResourcesPart},
		// Shown inline, with no node of the branch (derevo/11a, 11b).
		HTTPServiceKind: {URLTemplatesPart}, WebServiceKind: {OperationsPart},
	}
	for _, item := range kindInfos {
		if got := branchesOf(item.kind); !slices.Equal(got, want[item.kind]) {
			t.Errorf("%s shows the branches %v, want %v", item.kind, got, want[item.kind])
		}
	}
}

// The branches of an object hold what it declares of each sort, a table part
// with its fields, and not a common attribute taken into it; a catalog that
// is not there gives every branch empty. Defects caught: a part put in the
// branch of another sort; the fields of a table part left out or shown as the
// object's own; a common attribute shown as the object's; the tree breaking
// on a project that does not load.
func TestObjectBranchesHoldWhatTheObjectDeclares(t *testing.T) {
	t.Parallel()
	id := func() uuid.UUID { return uuid.MustNew() }
	chart, task, attribute, common, flag, subcontoFlag, part, partField, addressing, taskAttribute := id(), id(), id(), id(), id(), id(), id(), id(), id(), id()
	catalog := &Catalog{
		chartOfAccountsByID: map[uuid.UUID]int{chart: 0},
		ChartsOfAccounts: []ChartOfAccountsDefinition{{ID: chart,
			Attributes:                  []Attribute{{ID: attribute, Name: "Вид"}, {ID: common, Name: "Организация"}},
			AccountingFlags:             []AccountingFlag{{Attribute: Attribute{ID: flag, Name: "Валютный"}}},
			ExtDimensionAccountingFlags: []AccountingFlag{{Attribute: Attribute{ID: subcontoFlag, Name: "Суммовой"}}},
			TableParts:                  []TablePart{{ID: part, Name: "ВидыСубконто", Attributes: []Attribute{{ID: partField, Name: "ВидСубконто"}}}}}},
		taskByID: map[uuid.UUID]int{task: 0},
		Tasks: []TaskDefinition{{ID: task, Attributes: []Attribute{{ID: taskAttribute, Name: "Описание"}},
			AddressingAttributes: []AddressingAttribute{{ID: addressing, Name: "Исполнитель"}}}},
		commonAttributeByID: map[uuid.UUID]int{common: 0},
	}
	type held struct {
		group PartGroup
		names []string
	}
	shown := func(branches []ObjectBranch) []held {
		var result []held
		for _, branch := range branches {
			var names []string
			for _, item := range branch.Parts {
				names = append(names, item.Name)
				for _, inside := range item.Parts {
					names = append(names, item.Name+"."+inside.Name)
				}
			}
			result = append(result, held{branch.Group, names})
		}
		return result
	}
	for _, check := range []struct {
		name string
		got  []held
		want []held
	}{
		{"a chart of accounts", shown(catalog.ObjectBranches(ChartOfAccountsKind, chart)), []held{{AttributesPart, []string{"Вид"}},
			{AccountingFlagsPart, []string{"Валютный"}}, {ExtDimensionAccountingFlagsPart, []string{"Суммовой"}},
			{TablePartsPart, []string{"ВидыСубконто", "ВидыСубконто.ВидСубконто"}}}},
		{"a task", shown(catalog.ObjectBranches(TaskKind, task)), []held{{AddressingAttributesPart, []string{"Исполнитель"}},
			{AttributesPart, []string{"Описание"}}, {TablePartsPart, nil}}},
		{"no catalog", shown((*Catalog)(nil).ObjectBranches(TaskKind, task)), []held{{AddressingAttributesPart, nil}, {AttributesPart, nil}, {TablePartsPart, nil}}},
	} {
		if !slices.EqualFunc(check.got, check.want, func(a, b held) bool { return a.group == b.group && slices.Equal(a.names, b.names) }) {
			t.Errorf("%s shows %v, want %v", check.name, check.got, check.want)
		}
	}
}

// Every branch of the tree is drawn by an icon the set has. Defects caught: a
// sort of part or a held list given an icon the set does not draw, which
// would stand in the tree as an empty square, branch and parts alike.
func TestEveryBranchOfTheTreeIsDrawn(t *testing.T) {
	t.Parallel()
	for _, sort := range partGroups {
		if sort.icon != "" && !icons.Has(sort.icon) {
			t.Errorf("%s names icon %q, which the set does not draw", sort.group, sort.icon)
		}
	}
	for kind, composition := range kindCompositionOf {
		for _, list := range composition.held {
			if !icons.Has(list.icon) {
				t.Errorf("%s shows %s by icon %q, which the set does not draw", kind, list.key, list.icon)
			}
		}
	}
}

// What an object keeps beside its parts stands in the tree as the prototype
// shows it (inventory of block 00, «Ветки под объектом», snapshots derevo/07,
// 10, 11a, 11b, 12b): «Значения» under an enumeration, «Графы» under a
// journal, «Функции» under a source, «Перерасчеты» under a calculation
// register with the dimensions of each recalculation right under it (05b),
// each after the branches of parts; the
// templates of an HTTP service with their methods and the operations of a
// web service with their parameters right under the service; each by its
// icon, in the order of the description and not by the alphabet. Captions,
// icons and orders are written here from the snapshots. Defects caught: a
// list left out of the tree or given the caption or icon of another; a
// service's parts put in a branch the prototype has not; the methods of a
// template or the parameters of an operation lost or put under another;
// the declared order sorted away (НеКлассифицирован after СКласс, as in
// derevo/07); an object the catalog has not reading the lists of another.
func TestObjectBranchesShowWhatTheObjectHoldsBesideItsParts(t *testing.T) {
	t.Parallel()
	id := func() uuid.UUID { return uuid.MustNew() }
	enumeration, journal, source, httpService, webService, register := id(), id(), id(), id(), id(), id()
	template, other, get, post, operation, parameter := id(), id(), id(), id(), id(), id()
	catalog := &Catalog{
		calculationRegisterByID: map[uuid.UUID]int{register: 0},
		CalculationRegisters: []CalculationRegisterDefinition{{ID: register, Dimensions: []RegisterDimension{{Attribute: Attribute{ID: id(), Name: "Сотрудник"}}},
			Recalculations: []Recalculation{{ID: id(), Name: "Перерасчет", Dimensions: []RecalculationDimension{{ID: id(), Name: "Измерение1"}}},
				{ID: id(), Name: "Второй"}}}},
		enumerationByID: map[uuid.UUID]int{enumeration: 0},
		Enumerations: []Enumeration{{ID: enumeration, Values: []EnumerationValue{
			{ID: id(), Name: "АКласс"}, {ID: id(), Name: "СКласс"}, {ID: id(), Name: "НеКлассифицирован"}}}},
		documentJournalByID:    map[uuid.UUID]int{journal: 0},
		DocumentJournals:       []DocumentJournalDefinition{{ID: journal, Columns: []JournalColumn{{ID: id(), Name: "Сумма"}, {ID: id(), Name: "Организация"}}}},
		externalDataSourceByID: map[uuid.UUID]int{source: 0},
		ExternalDataSources:    []ExternalDataSourceDefinition{{ID: source, Functions: []ExternalFunction{{ID: id(), Name: "Остаток"}}}},
		httpServiceByID:        map[uuid.UUID]int{httpService: 0},
		HTTPServices: []HTTPServiceDefinition{{ID: httpService, Templates: []URLTemplate{
			{ID: template, Name: "ХранилищеИИдентификатор", Methods: []HTTPServiceMethod{{ID: get, Name: "GET"}, {ID: post, Name: "POST"}}},
			{ID: other, Name: "Получить"}}}},
		webServiceByID: map[uuid.UUID]int{webService: 0},
		WebServices: []WebServiceDefinition{{ID: webService, Operations: []WebServiceOperation{{ID: operation, Name: "FillCatalogs",
			Parameters: []WebServiceParameter{{ID: parameter, Name: "MobileDeviceID"}}}}}},
	}
	type shown struct {
		title, icon string
		inline      bool
		items       []string
	}
	view := func(branches []ObjectBranch) []shown {
		var result []shown
		for _, branch := range branches {
			item := shown{title: branch.Title, icon: branch.Icon, inline: branch.Inline}
			for _, part := range branch.Parts {
				item.items = append(item.items, part.Name+" "+part.Icon)
				for _, inside := range part.Parts {
					item.items = append(item.items, part.Name+"."+inside.Name+" "+inside.Icon)
				}
			}
			result = append(result, item)
		}
		return result
	}
	same := func(a, b shown) bool {
		return a.title == b.title && a.icon == b.icon && a.inline == b.inline && slices.Equal(a.items, b.items)
	}
	for _, check := range []struct {
		name string
		got  []shown
		want []shown
	}{
		{"an enumeration", view(catalog.ObjectBranches(EnumerationKind, enumeration)), []shown{{"Значения", "enumeration-value", false,
			[]string{"АКласс enumeration-value", "СКласс enumeration-value", "НеКлассифицирован enumeration-value"}}}},
		{"a journal", view(catalog.ObjectBranches(DocumentJournalKind, journal)), []shown{{"Графы", "journal-column", false,
			[]string{"Сумма journal-column", "Организация journal-column"}}}},
		{"a source", view(catalog.ObjectBranches(ExternalDataSourceKind, source)), []shown{{"Функции", "external-function", false,
			[]string{"Остаток external-function"}}}},
		{"an HTTP service", view(catalog.ObjectBranches(HTTPServiceKind, httpService)), []shown{{"Шаблоны URL", "url-template", true,
			[]string{"ХранилищеИИдентификатор url-template", "ХранилищеИИдентификатор.GET http-method", "ХранилищеИИдентификатор.POST http-method",
				"Получить url-template"}}}},
		{"a web service", view(catalog.ObjectBranches(WebServiceKind, webService)), []shown{{"Операции", "web-service-operation", true,
			[]string{"FillCatalogs web-service-operation", "FillCatalogs.MobileDeviceID parameter"}}}},
		{"a calculation register", view(catalog.ObjectBranches(CalculationRegisterKind, register)), []shown{{"Измерения", "dimension", false,
			[]string{"Сотрудник dimension"}}, {"Ресурсы", "resource", false, nil}, {"Реквизиты", "attribute", false, nil},
			{"Перерасчёты", "recalculation", false, []string{"Перерасчет recalculation", "Перерасчет.Измерение1 dimension", "Второй recalculation"}}}},
		{"an enumeration the catalog has not", view(catalog.ObjectBranches(EnumerationKind, id())), []shown{{"Значения", "enumeration-value", false, nil}}},
		{"no catalog", view((*Catalog)(nil).ObjectBranches(DocumentJournalKind, journal)), []shown{{"Графы", "journal-column", false, nil}}},
	} {
		if !slices.EqualFunc(check.got, check.want, same) {
			t.Errorf("%s shows %v, want %v", check.name, check.got, check.want)
		}
	}
}

// The values of an enumeration and the columns of a journal have identifiers
// of their object's own and are checked with the rest of what it holds, while
// the functions of a source are objects of their own kind, which the catalog
// checks already. Defects caught: a value or a column whose identifier
// another thing of the configuration repeats left unchecked once the tree's
// lists came to name them; a function checked twice, once as an object and
// once as what its source holds.
func TestHeldListsGiveTheIdentitiesOfTheirObjects(t *testing.T) {
	t.Parallel()
	enumeration, journal, source, value, column, function := uuid.MustNew(), uuid.MustNew(), uuid.MustNew(), uuid.MustNew(), uuid.MustNew(), uuid.MustNew()
	catalog := &Catalog{
		enumerationByID:        map[uuid.UUID]int{enumeration: 0},
		Enumerations:           []Enumeration{{ID: enumeration, Values: []EnumerationValue{{ID: value, Name: "АКласс"}}}},
		documentJournalByID:    map[uuid.UUID]int{journal: 0},
		DocumentJournals:       []DocumentJournalDefinition{{ID: journal, Columns: []JournalColumn{{ID: column, Name: "Сумма"}}}},
		externalDataSourceByID: map[uuid.UUID]int{source: 0},
		ExternalDataSources:    []ExternalDataSourceDefinition{{ID: source, Functions: []ExternalFunction{{ID: function, Name: "Остаток"}}}},
	}
	for _, check := range []struct {
		name string
		got  []heldIdentity
		want []heldIdentity
	}{
		{"an enumeration", catalog.heldIdentities(EnumerationKind, enumeration), []heldIdentity{{value, "value", "АКласс"}}},
		{"a journal", catalog.heldIdentities(DocumentJournalKind, journal), []heldIdentity{{column, "column", "Сумма"}}},
		{"a source", catalog.heldIdentities(ExternalDataSourceKind, source), nil},
	} {
		if !slices.Equal(check.got, check.want) {
			t.Errorf("%s holds the identities %v, want %v", check.name, check.got, check.want)
		}
	}
}
