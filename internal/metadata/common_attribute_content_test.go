package metadata

import (
	"strings"
	"testing"

	"github.com/k33alexey/MetaLab/internal/uuid"
)

// contentCatalog builds a catalog holding the objects a common attribute can
// reach, with no validation in the way: what is under test is propagation, and
// propagation reads names and identifiers only.
func contentCatalog(common CommonAttributeDefinition, fields ...Attribute) *Catalog {
	return &Catalog{
		Catalogs: []CatalogDefinition{
			{ID: contentFirst, Name: "Товары", Attributes: fields},
			{ID: contentSecond, Name: "Склады"},
		},
		Documents:             []DocumentDefinition{{ID: contentDocument, Name: "Продажа"}},
		InformationRegisters:  []InformationRegisterDefinition{{ID: contentInformation, Name: "Цены"}},
		AccumulationRegisters: []AccumulationRegisterDefinition{{ID: contentAccumulation, Name: "Остатки"}},
		BusinessProcesses:     []BusinessProcessDefinition{{ID: contentProcess, Name: "Согласование"}},
		Tasks:                 []TaskDefinition{{ID: contentTask, Name: "ЗадачаИсполнителя"}},
		CommonAttributes:      []CommonAttributeDefinition{common},
	}
}

var (
	contentFirst        = uuid.MustNew()
	contentSecond       = uuid.MustNew()
	contentDocument     = uuid.MustNew()
	contentInformation  = uuid.MustNew()
	contentAccumulation = uuid.MustNew()
	contentProcess      = uuid.MustNew()
	contentTask         = uuid.MustNew()
)

func contentAttribute(autoUse CommonAttributeAutoUse, items ...CommonAttributeContentItem) CommonAttributeDefinition {
	return CommonAttributeDefinition{
		Format: CurrentFormat, ID: uuid.MustNew(), Name: "Организация",
		Title: LocalizedText{"ru": "Организация"}, Types: []Type{{Kind: StringType, Length: 100}},
		AutoUse: autoUse, Content: items,
	}
}

func reached(t *testing.T, catalog *Catalog) map[string]bool {
	t.Helper()
	if err := catalog.propagateCommonAttributes(); err != nil {
		t.Fatalf("propagation failed: %v", err)
	}
	got := map[string]bool{}
	for _, item := range catalog.Catalogs {
		got[item.Name] = len(item.Attributes) > 0 && item.Attributes[len(item.Attributes)-1].Name == "Организация"
	}
	for _, item := range catalog.Documents {
		got[item.Name] = len(item.Attributes) > 0
	}
	for _, item := range catalog.InformationRegisters {
		got[item.Name] = len(item.Attributes) > 0
	}
	for _, item := range catalog.AccumulationRegisters {
		got[item.Name] = len(item.Attributes) > 0
	}
	for _, item := range catalog.BusinessProcesses {
		got[item.Name] = len(item.Attributes) > 0
	}
	for _, item := range catalog.Tasks {
		got[item.Name] = len(item.Attributes) > 0
	}
	return got
}

// The defect this catches is an inversion, and it is not hypothetical: of the
// six common attributes of the demonstration configuration five are «only these,
// explicitly» and the sixth is the other way round - auto-use on, and all 331
// items marked «do not use», so its composition is a list of exceptions. Read as
// a list of the included, that attribute would be given to precisely the 331
// objects that must not have it, silently and with the right count of objects.
func TestCommonAttributeContentIsAVerdictPerObject(t *testing.T) {
	t.Parallel()
	t.Run("только перечисленные", func(t *testing.T) {
		got := reached(t, contentCatalog(contentAttribute(CommonAttributeAutoUseDontUse,
			CommonAttributeContentItem{Metadata: contentFirst, Use: CommonAttributeUseUse})))
		if !got["Товары"] || got["Склады"] || got["Продажа"] {
			t.Fatalf("reached = %v", got)
		}
	})
	t.Run("список исключений", func(t *testing.T) {
		got := reached(t, contentCatalog(contentAttribute(CommonAttributeAutoUseUse,
			CommonAttributeContentItem{Metadata: contentFirst, Use: CommonAttributeUseDontUse})))
		if got["Товары"] {
			t.Fatal("the one object the composition excludes got the attribute: the exclusion list was read as an inclusion list")
		}
		if !got["Склады"] || !got["Продажа"] || !got["Цены"] {
			t.Fatalf("reached = %v", got)
		}
	})
	t.Run("авто решается автоиспользованием", func(t *testing.T) {
		got := reached(t, contentCatalog(contentAttribute(CommonAttributeAutoUseUse,
			CommonAttributeContentItem{Metadata: contentFirst, Use: CommonAttributeUseAuto})))
		if !got["Товары"] {
			t.Fatalf("an item marked auto was not decided by auto_use: %v", got)
		}
	})
	t.Run("объект, о котором состав молчит", func(t *testing.T) {
		got := reached(t, contentCatalog(contentAttribute(CommonAttributeAutoUseDontUse,
			CommonAttributeContentItem{Metadata: contentFirst, Use: CommonAttributeUseUse})))
		if got["Склады"] {
			t.Fatal("an object the composition never mentions grew the attribute while auto_use says not to")
		}
	})
}

// The kinds are the prototype's list, checked in its configurator: catalogs,
// documents, document journals, information registers, accumulation registers,
// business processes, tasks. Journals keep columns rather than attributes and
// have a point of their own; the other six are here.
func TestCommonAttributeReachesEveryKindOfItsComposition(t *testing.T) {
	t.Parallel()
	got := reached(t, contentCatalog(contentAttribute(CommonAttributeAutoUseUse)))
	for _, name := range []string{"Товары", "Склады", "Продажа", "Цены", "Остатки", "Согласование", "ЗадачаИсполнителя"} {
		if !got[name] {
			t.Fatalf("%s did not get the attribute: %v", name, got)
		}
	}
}

// With auto-use the collision appears where nobody listed anything: the object
// was never named in the composition, and it already has a field of that name.
// Two fields of one name in one table is not a thing a table can hold.
func TestCommonAttributeCollidesInAnObjectNobodyListed(t *testing.T) {
	t.Parallel()
	catalog := contentCatalog(contentAttribute(CommonAttributeAutoUseUse),
		Attribute{ID: uuid.MustNew(), Name: "Организация"})
	err := catalog.propagateCommonAttributes()
	if err == nil {
		t.Fatal("the attribute was propagated into an object that already had a field of that name")
	}
	if !strings.Contains(err.Error(), "collides") {
		t.Fatalf("error = %v", err)
	}
}

// A condition names one boolean, and it stands outside the composition: a
// condition kept on a separated object would have to be read to decide whether
// that object is separated, and reading it needs the answer.
func TestConditionalSeparationNamesOneBooleanOutsideTheComposition(t *testing.T) {
	t.Parallel()
	constant, other, field := uuid.MustNew(), uuid.MustNew(), uuid.MustNew()
	good := contentAttribute(CommonAttributeAutoUseDontUse,
		CommonAttributeContentItem{Metadata: contentFirst, Use: CommonAttributeUseUse,
			ConditionalSeparation: &ConditionalSeparation{Constant: &constant}},
		CommonAttributeContentItem{Metadata: contentSecond, Use: CommonAttributeUseUse,
			ConditionalSeparation: &ConditionalSeparation{Object: &other, Attribute: &field}},
	)
	if err := ValidateCommonAttribute("common-attribute.yaml", good, metadataConfiguration()); err != nil {
		t.Fatalf("a valid condition was refused: %v", err)
	}
	cases := map[string]*ConditionalSeparation{
		"ни константы, ни реквизита": {},
		"и константа, и реквизит":    {Constant: &constant, Object: &other, Attribute: &field},
		"объект без реквизита":       {Object: &other},
		"объект входит в состав":     {Object: &contentFirst, Attribute: &field},
		"константа входит в состав":  {Constant: &contentFirst},
	}
	for name, separation := range cases {
		t.Run(name, func(t *testing.T) {
			attribute := contentAttribute(CommonAttributeAutoUseDontUse,
				CommonAttributeContentItem{Metadata: contentFirst, Use: CommonAttributeUseUse},
				CommonAttributeContentItem{Metadata: contentSecond, Use: CommonAttributeUseUse, ConditionalSeparation: separation},
			)
			if err := ValidateCommonAttribute("common-attribute.yaml", attribute, metadataConfiguration()); err == nil {
				t.Fatal("the condition was accepted")
			}
		})
	}
}

// TestCommonAttributeReachesTheSchemaByAutoUse is here because auto-use makes
// one line of one file change hundreds of tables: an attribute that reaches
// every object gets a column in every object's table. The prototype does the
// same - that is how a separator of data areas works - but it is the first
// setting of ours with that reach, so the reach is checked rather than assumed.
func TestCommonAttributeReachesTheSchemaByAutoUse(t *testing.T) {
	t.Parallel()
	first := CatalogDefinition{Format: CurrentFormat, ID: uuid.MustNew(), Name: "Товары", Title: LocalizedText{"ru": "Товары"},
		Code: CatalogCode{Type: StringType, Length: 9}, DescriptionLength: 100}
	second := CatalogDefinition{Format: CurrentFormat, ID: uuid.MustNew(), Name: "Склады", Title: LocalizedText{"ru": "Склады"},
		Code: CatalogCode{Type: StringType, Length: 9}, DescriptionLength: 100}
	excluded := CatalogDefinition{Format: CurrentFormat, ID: uuid.MustNew(), Name: "Файлы", Title: LocalizedText{"ru": "Файлы"},
		Code: CatalogCode{Type: StringType, Length: 9}, DescriptionLength: 100}
	common := CommonAttributeDefinition{
		Format: CurrentFormat, ID: uuid.MustNew(), Name: "Организация", Title: LocalizedText{"ru": "Организация"},
		Types: []Type{{Kind: StringType, Length: 100}}, AutoUse: CommonAttributeAutoUseUse,
		Content: []CommonAttributeContentItem{{Metadata: excluded.ID, Use: CommonAttributeUseDontUse}},
	}
	catalog, err := NewCatalogSnapshotWithCommonAttributes(metadataConfiguration(), nil, nil, nil,
		[]CatalogDefinition{first, second, excluded}, nil, nil, nil, nil, nil, nil, []CommonAttributeDefinition{common})
	if err != nil {
		t.Fatal(err)
	}
	schema, err := catalog.ApplicationSchema()
	if err != nil {
		t.Fatal(err)
	}
	column, err := PhysicalAttributeColumn(common.ID)
	if err != nil {
		t.Fatal(err)
	}
	carries := map[string]bool{}
	for _, definition := range []CatalogDefinition{first, second, excluded} {
		table, err := PhysicalCatalogTable(definition.ID)
		if err != nil {
			t.Fatal(err)
		}
		for _, candidate := range schema.Tables {
			if candidate.Name != table {
				continue
			}
			for _, field := range candidate.Columns {
				if field.Name == column {
					carries[definition.Name] = true
				}
			}
		}
	}
	if !carries["Товары"] || !carries["Склады"] {
		t.Fatalf("the attribute reached the objects and not their tables: %v", carries)
	}
	if carries["Файлы"] {
		t.Fatal("the object the composition excludes got a column all the same")
	}
}

// A condition points outside the file it is written in, so what it points at can
// only be checked with the whole configuration in hand. Three things are checked
// and each closes a different silence: a condition pointing at nothing separates
// nothing while saying it separates; a condition on a value that is not boolean
// has no true or false to read; and a condition living on an object the
// attribute does not reference is not tied to the data it decides about.
func TestConditionalSeparationIsResolvedAcrossTheConfiguration(t *testing.T) {
	t.Parallel()
	areas := CatalogDefinition{Format: CurrentFormat, ID: uuid.MustNew(), Name: "Организации", Title: LocalizedText{"ru": "Организации"},
		Code: CatalogCode{Type: StringType, Length: 9}, DescriptionLength: 100}
	separate := Attribute{ID: uuid.MustNew(), Name: "Обособленная", Title: LocalizedText{"ru": "Обособленная"},
		Types: []Type{{Kind: BooleanType}}}
	text := Attribute{ID: uuid.MustNew(), Name: "Примечание", Title: LocalizedText{"ru": "Примечание"},
		Types: []Type{{Kind: StringType, Length: 10}}}
	areas.Attributes = []Attribute{separate, text}
	goods := CatalogDefinition{Format: CurrentFormat, ID: uuid.MustNew(), Name: "Товары", Title: LocalizedText{"ru": "Товары"},
		Code: CatalogCode{Type: StringType, Length: 9}, DescriptionLength: 100}
	flag := Constant{Format: CurrentFormat, ID: uuid.MustNew(), Name: "РазделятьУчет", Title: LocalizedText{"ru": "Разделять учёт"},
		Types: []Type{{Kind: BooleanType}}}
	number := Constant{Format: CurrentFormat, ID: uuid.MustNew(), Name: "КурсПоУмолчанию", Title: LocalizedText{"ru": "Курс"},
		Types: []Type{{Kind: NumberType, Precision: 10, Scale: 2}}}

	attribute := func(separation *ConditionalSeparation, types ...Type) CommonAttributeDefinition {
		if len(types) == 0 {
			types = []Type{{Kind: CatalogType, Reference: &areas.ID}}
		}
		return CommonAttributeDefinition{
			Format: CurrentFormat, ID: uuid.MustNew(), Name: "Организация", Title: LocalizedText{"ru": "Организация"},
			Types: types, Content: []CommonAttributeContentItem{
				{Metadata: goods.ID, Use: CommonAttributeUseUse, ConditionalSeparation: separation},
			},
		}
	}
	load := func(common CommonAttributeDefinition) error {
		_, err := NewCatalogSnapshotWithCommonAttributes(metadataConfiguration(), []Constant{flag, number}, nil, nil,
			[]CatalogDefinition{areas, goods}, nil, nil, nil, nil, nil, nil, []CommonAttributeDefinition{common})
		return err
	}

	if err := load(attribute(&ConditionalSeparation{Constant: &flag.ID})); err != nil {
		t.Fatalf("a boolean constant was refused: %v", err)
	}
	if err := load(attribute(&ConditionalSeparation{Object: &areas.ID, Attribute: &separate.ID})); err != nil {
		t.Fatalf("a boolean attribute of the referenced object was refused: %v", err)
	}

	unknown := uuid.MustNew()
	cases := map[string]struct {
		separation *ConditionalSeparation
		types      []Type
		message    string
	}{
		"неизвестная константа":       {&ConditionalSeparation{Constant: &unknown}, nil, "unknown constant"},
		"константа не булева":         {&ConditionalSeparation{Constant: &number.ID}, nil, "not boolean"},
		"неизвестный объект":          {&ConditionalSeparation{Object: &unknown, Attribute: &separate.ID}, nil, "unknown object"},
		"нет такого реквизита":        {&ConditionalSeparation{Object: &areas.ID, Attribute: &unknown}, nil, "does not have"},
		"реквизит не булев":           {&ConditionalSeparation{Object: &areas.ID, Attribute: &text.ID}, nil, "not boolean"},
		"объект не в типах реквизита": {&ConditionalSeparation{Object: &areas.ID, Attribute: &separate.ID}, []Type{{Kind: StringType, Length: 100}}, "reference is not among the types"},
	}
	for name, testCase := range cases {
		t.Run(name, func(t *testing.T) {
			err := load(attribute(testCase.separation, testCase.types...))
			if err == nil {
				t.Fatal("the condition was accepted")
			}
			if !strings.Contains(err.Error(), testCase.message) {
				t.Fatalf("error = %q, expected it to name %q", err, testCase.message)
			}
		})
	}
}

// The composition of a real configuration names things that cannot hold a field
// at all. Among the six common attributes of the demonstration configuration the
// items name 142 information registers and 50 catalogs - which do take a field -
// and also 138 constants, 70 scheduled jobs, 3 charts of characteristic types
// and one exchange plan, which do not: a constant is one value, a scheduled job
// has no table. For those, being in the composition means being separated by the
// data area, which is the half of the mechanism we carry and do not execute.
//
// The defect this catches is the one that would hurt most: refusing such a
// composition outright, so a real configuration could not be read at all.
func TestCommonAttributeCompositionMayNameWhatCannotHoldAField(t *testing.T) {
	t.Parallel()
	goods := CatalogDefinition{Format: CurrentFormat, ID: uuid.MustNew(), Name: "Товары", Title: LocalizedText{"ru": "Товары"},
		Code: CatalogCode{Type: StringType, Length: 9}, DescriptionLength: 100}
	flag := Constant{Format: CurrentFormat, ID: uuid.MustNew(), Name: "РазделятьУчет", Title: LocalizedText{"ru": "Разделять учёт"},
		Types: []Type{{Kind: BooleanType}}}
	common := CommonAttributeDefinition{
		Format: CurrentFormat, ID: uuid.MustNew(), Name: "Организация", Title: LocalizedText{"ru": "Организация"},
		Types: []Type{{Kind: StringType, Length: 100}},
		Content: []CommonAttributeContentItem{
			{Metadata: goods.ID, Use: CommonAttributeUseUse},
			{Metadata: flag.ID, Use: CommonAttributeUseUse},
		},
	}
	catalog, err := NewCatalogSnapshotWithCommonAttributes(metadataConfiguration(), []Constant{flag}, nil, nil,
		[]CatalogDefinition{goods}, nil, nil, nil, nil, nil, nil, []CommonAttributeDefinition{common})
	if err != nil {
		t.Fatalf("a composition naming a constant was refused: %v", err)
	}
	loaded, ok := catalog.CatalogDefinition("Товары")
	if !ok || len(loaded.Attributes) != 1 {
		t.Fatalf("the catalog of the composition did not get the field: %+v", loaded.Attributes)
	}

	// A name that belongs to nothing is still an error: it separates nothing and
	// gives the field to nobody, while reading as though it did both.
	dangling := common
	dangling.Content = append([]CommonAttributeContentItem{}, common.Content...)
	dangling.Content[1].Metadata = uuid.MustNew()
	_, err = NewCatalogSnapshotWithCommonAttributes(metadataConfiguration(), []Constant{flag}, nil, nil,
		[]CatalogDefinition{goods}, nil, nil, nil, nil, nil, nil, []CommonAttributeDefinition{dangling})
	if err == nil || !strings.Contains(err.Error(), "unknown object") {
		t.Fatalf("error = %v", err)
	}
}

// The seven properties of data separation are carried and not executed: in the
// prototype the same metadata object carries a second mechanism - one database
// shared between independent companies - and we have one company per database.
// Carried, because a property missing from the model disappears on import, and
// that is a loss of meaning, while carried-and-not-executed imports whole.
func TestCommonAttributeCarriesDataSeparation(t *testing.T) {
	t.Parallel()
	areaValue := SessionParameter{Format: CurrentFormat, ID: uuid.MustNew(), Name: "ОбластьДанныхЗначение",
		Title: LocalizedText{"ru": "Область данных: значение"}, Types: []Type{{Kind: StringType, Length: 20}}}
	areaUse := SessionParameter{Format: CurrentFormat, ID: uuid.MustNew(), Name: "ОбластьДанныхИспользование",
		Title: LocalizedText{"ru": "Область данных: использование"}, Types: []Type{{Kind: BooleanType}}}
	goods := CatalogDefinition{Format: CurrentFormat, ID: uuid.MustNew(), Name: "Товары", Title: LocalizedText{"ru": "Товары"},
		Code: CatalogCode{Type: StringType, Length: 9}, DescriptionLength: 100}
	separator := func() CommonAttributeDefinition {
		return CommonAttributeDefinition{
			Format: CurrentFormat, ID: uuid.MustNew(), Name: "ОбластьДанных", Title: LocalizedText{"ru": "Область данных"},
			Types:          []Type{{Kind: StringType, Length: 20}},
			Content:        []CommonAttributeContentItem{{Metadata: goods.ID, Use: CommonAttributeUseUse}},
			DataSeparation: SeparationSeparate, SeparatedDataUse: SeparatedDataIndependently,
			DataSeparationValue: &areaValue.ID, DataSeparationUse: &areaUse.ID,
			UsersSeparation: SeparationSeparate, AuthenticationSeparation: SeparationSeparate,
			ConfigurationExtensionsSeparation: SeparationSeparate,
		}
	}
	load := func(common CommonAttributeDefinition) error {
		_, err := NewCatalogSnapshotWithCommonAttributes(metadataConfiguration(), nil, nil, nil,
			[]CatalogDefinition{goods}, nil, nil, nil, nil, nil,
			[]SessionParameter{areaValue, areaUse}, []CommonAttributeDefinition{common})
		return err
	}
	if err := load(separator()); err != nil {
		t.Fatalf("a separator of data areas was refused: %v", err)
	}

	// The shape of the four common attributes of the real configuration that
	// separate nothing: the separation is off and the properties are written all
	// the same - «independently» among them, which has no «do not» to write. A
	// rule that demanded they be empty would refuse four real attributes out of
	// six.
	quiet := separator()
	quiet.DataSeparation = SeparationDontUse
	quiet.UsersSeparation, quiet.AuthenticationSeparation = SeparationDontUse, SeparationDontUse
	quiet.ConfigurationExtensionsSeparation = SeparationDontUse
	quiet.DataSeparationValue, quiet.DataSeparationUse = nil, nil
	if err := load(quiet); err != nil {
		t.Fatalf("an attribute that separates nothing was refused: %v", err)
	}

	// The shape of the file is checked when the file is read, and what the file
	// points at when the whole configuration is in hand. The cases are split the
	// same way.
	local := map[string]func(*CommonAttributeDefinition){
		"неизвестный режим разделения": func(a *CommonAttributeDefinition) { a.DataSeparation = "иногда" },
		"неизвестный уровень":          func(a *CommonAttributeDefinition) { a.SeparatedDataUse = "иногда" },
		// Separation cannot work without the parameters: nothing would say which
		// area the session is in.
		"разделение без параметров сеанса": func(a *CommonAttributeDefinition) {
			a.DataSeparationValue, a.DataSeparationUse = nil, nil
		},
	}
	for name, mutate := range local {
		t.Run(name, func(t *testing.T) {
			common := separator()
			mutate(&common)
			if err := ValidateCommonAttribute("common-attribute.yaml", common, metadataConfiguration()); err == nil {
				t.Fatal("the separation was accepted")
			}
		})
	}
	t.Run("неизвестный параметр сеанса", func(t *testing.T) {
		unknown := uuid.MustNew()
		common := separator()
		common.DataSeparationValue = &unknown
		if err := load(common); err == nil {
			t.Fatal("an unknown session parameter was accepted")
		}
	})
}
