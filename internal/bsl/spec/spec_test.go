package spec

import (
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf8"
)

func TestCatalogIsCompleteAndConsistent(t *testing.T) {
	t.Parallel()

	catalog, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if catalog.Version != 1 {
		t.Fatalf("catalog version = %d, want 1", catalog.Version)
	}
	if len(catalog.Features) < 20 {
		t.Fatalf("catalog has only %d features", len(catalog.Features))
	}
	if len(catalog.Keywords) < 35 {
		t.Fatalf("catalog has only %d keyword pairs", len(catalog.Keywords))
	}

	keywordIDs := make(map[string]bool, len(catalog.Keywords))
	aliases := make(map[string]string, len(catalog.Keywords)*2)
	for _, keyword := range catalog.Keywords {
		if keyword.ID == "" || keyword.Russian == "" || keyword.English == "" || keyword.Category == "" {
			t.Fatalf("keyword has an empty field: %+v", keyword)
		}
		if keywordIDs[keyword.ID] {
			t.Fatalf("duplicate keyword ID %q", keyword.ID)
		}
		keywordIDs[keyword.ID] = true
		for _, alias := range []string{keyword.Russian, keyword.English} {
			folded := strings.ToLower(alias)
			if owner, exists := aliases[folded]; exists && owner != keyword.ID {
				t.Fatalf("keyword alias %q belongs to both %q and %q", alias, owner, keyword.ID)
			}
			aliases[folded] = keyword.ID
		}
	}

	allowedCategories := map[string]bool{
		"lexical":      true,
		"preprocessor": true,
		"declaration":  true,
		"statement":    true,
		"expression":   true,
		"extension":    true,
	}
	ids := make(map[string]bool, len(catalog.Features))
	coveredCorpus := make(map[string]bool)
	for _, feature := range catalog.Features {
		if feature.ID == "" || feature.Title == "" {
			t.Fatalf("feature has empty identity: %+v", feature)
		}
		if ids[feature.ID] {
			t.Fatalf("duplicate feature ID %q", feature.ID)
		}
		ids[feature.ID] = true
		if !allowedCategories[feature.Category] {
			t.Fatalf("feature %q has unknown category %q", feature.ID, feature.Category)
		}
		if len(feature.Corpus) == 0 {
			t.Fatalf("feature %q has no conformance corpus", feature.ID)
		}
		for _, name := range feature.Corpus {
			if filepath.Base(name) != name || filepath.Ext(name) != ".bsl" {
				t.Fatalf("feature %q has unsafe corpus name %q", feature.ID, name)
			}
			coveredCorpus[name] = true
		}
	}

	entries, err := files.ReadDir("corpus")
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if entry.IsDir() {
			t.Fatalf("unexpected corpus directory %q", entry.Name())
		}
		if !coveredCorpus[entry.Name()] {
			t.Fatalf("corpus %q is not referenced by the catalog", entry.Name())
		}
	}
}

func TestCorpusIsPortableUTF8(t *testing.T) {
	t.Parallel()

	catalog, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	checked := make(map[string]bool)
	for _, feature := range catalog.Features {
		for _, name := range feature.Corpus {
			if checked[name] {
				continue
			}
			checked[name] = true

			data, err := Corpus(name)
			if err != nil {
				t.Fatal(err)
			}
			if len(data) == 0 || !utf8.Valid(data) {
				t.Fatalf("corpus %q must be non-empty UTF-8", name)
			}
			if strings.HasPrefix(string(data), "\ufeff") {
				t.Fatalf("corpus %q must not contain UTF-8 BOM", name)
			}
			if !strings.HasSuffix(string(data), "\n") {
				t.Fatalf("corpus %q must end with a newline", name)
			}
		}
	}
}

func TestCorpusRejectsTraversal(t *testing.T) {
	t.Parallel()

	if _, err := Corpus("../syntax.yaml"); err == nil {
		t.Fatal("Corpus() accepted traversal path")
	}
}
