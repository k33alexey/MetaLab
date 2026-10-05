package metadata

import (
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/k33alexey/MetaLab/internal/uuid"
)

// Edges of a catalog's description that sit beside the ceilings test: the
// number of levels when levels are not limited, the name of a predefined
// item, a number of a single digit.
//
// Defect caught: five levels refused when the limit is off, a predefined name
// of exactly 255 characters refused, a number of length 1 refused.
func TestCatalogDescriptionEdges(t *testing.T) {
	t.Parallel()
	catalog := ceilingCatalog("{type: string, length: 9}", "100")
	predefined := func(name string) string {
		return catalog + "predefined:\n  - {id: " + uuid.MustNew().String() + ", name: " + name + ", code: \"001\"}\n"
	}
	for name, test := range map[string]struct {
		body string
		says string
	}{
		"пять уровней без ограничения":    {catalog + "hierarchy: {enabled: true, kind: items, level_count: 5}\n", ""},
		"шесть уровней без ограничения":   {catalog + "hierarchy: {enabled: true, kind: items, level_count: 6}\n", "hierarchy.level_count must be 0..5"},
		"предопределённый с именем в 255": {predefined("П" + strings.Repeat("р", 254)), ""},
		"предопределённый с именем в 256": {predefined("П" + strings.Repeat("р", 255)), "predefined[0].name must be a valid identifier of at most 255 characters"},
		"реквизит числом в одну цифру":    {catalog + "attributes:\n  - {id: " + uuid.MustNew().String() + ", name: Цифра, title: {ru: Цифра}, types: [{kind: number, precision: 1}]}\n", ""},
		"реквизит числом без цифр":        {catalog + "attributes:\n  - {id: " + uuid.MustNew().String() + ", name: Цифра, title: {ru: Цифра}, types: [{kind: number, precision: 0}]}\n", "precision must be 1..32"},
		"поиск по коду, когда код есть":   {catalog + "list: {search_fields: [code]}\n", ""},
		"поиск по коду, когда кода нет":   {ceilingCatalog("{type: string, length: 0}", "100") + "list: {search_fields: [code]}\n", "list.search_fields[0] references an unknown field"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			_, err := DecodeCatalog("object.yaml", strings.NewReader(test.body), metadataConfiguration())
			if test.says == "" && err != nil {
				t.Fatalf("refused: %v", err)
			}
			if test.says != "" && (err == nil || !strings.Contains(err.Error(), test.says)) {
				t.Fatalf("want %q, got %v", test.says, err)
			}
		})
	}
}

// A document, a business process or a task with its number switched off has
// no number to search a list by; with a number it has.
//
// Defect caught: search by a number that is not there accepted, or by one
// that is there refused.
func TestListSearchByNumberFollowsTheNumber(t *testing.T) {
	t.Parallel()
	search := func(length int) []string {
		var found []string
		issues := validateNumberedObjectShape(numberedObjectShape{
			kind: DocumentKind, number: DocumentNumber{Type: StringType, Length: length},
			list: ListSettings{SearchFields: []string{"number"}}, reservedName: reservedDocumentObjectName,
		}, metadataConfiguration())
		for _, issue := range issues {
			if strings.HasPrefix(issue, "list.") {
				found = append(found, issue)
			}
		}
		return found
	}
	if found := search(9); len(found) != 0 {
		t.Fatalf("search by a number that is there: %v", found)
	}
	if found := search(0); !slices.Equal(found, []string{"list.search_fields[0] references an unknown field"}) {
		t.Fatalf("search by a number that is switched off: %v", found)
	}
}

// Predefined items that are each other's parents are a ring, and counting
// their levels must still end: the ring is reported, not walked forever.
//
// Defect caught: the walk of parents never stopping on a ring.
func TestPredefinedRingEnds(t *testing.T) {
	t.Parallel()
	body := ceilingCatalog("{type: string, length: 9}", "100") +
		"hierarchy: {enabled: true, kind: items, limit_levels: true, level_count: 5}\n" +
		"predefined:\n" +
		"  - {id: " + uuid.MustNew().String() + ", name: Первый, parent: Второй, code: \"001\"}\n" +
		"  - {id: " + uuid.MustNew().String() + ", name: Второй, parent: Первый, code: \"002\"}\n"
	done := make(chan error, 1)
	go func() {
		_, err := DecodeCatalog("object.yaml", strings.NewReader(body), metadataConfiguration())
		done <- err
	}()
	select {
	case err := <-done:
		if err == nil || !strings.Contains(err.Error(), "makes a ring through") {
			t.Fatalf("a ring of predefined items: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("counting the levels of a ring never ended")
	}
}

// What follows the description in its file is refused with words that say
// what it is: a second document, or text that is not YAML at all.
//
// Defect caught: the two refusals swapped, so a broken tail is reported as a
// second document and the other way round.
func TestTrailingYAMLIsNamed(t *testing.T) {
	t.Parallel()
	var target map[string]any
	for tail, says := range map[string]string{
		"---\nformat: 1\n": "multiple YAML documents are not allowed",
		"---\n: : [\n":     "decode trailing YAML",
	} {
		err := decodeStrict("object.yaml", strings.NewReader("format: 1\n"+tail), &target)
		if err == nil || !strings.Contains(err.Error(), says) {
			t.Fatalf("tail %q: want %q, got %v", tail, says, err)
		}
	}
}
