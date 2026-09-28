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
