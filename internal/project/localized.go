package project

import (
	"sort"
	"strings"
)

// LocalizedText is one text in as many languages as the project has. It lives
// here, beside the languages themselves, because the configuration root is the
// thing that both declares the languages and needs its own texts in them: its
// synonym, its brief and detailed information, its copyright and its
// addresses. Everything else in a configuration is described the same way, and
// the metadata package names this very type rather than one of its own.
type LocalizedText map[string]string

// Resolve returns a translation by a fixed chain: the language asked for, then
// the same language without its region, then the project's default language,
// then the configured languages in order, and finally any non-empty value at
// all.
//
// The chain is fixed on purpose. It used to fall through the configured list
// straight away, so which translation a user saw when theirs was empty depended
// on the order languages happened to be stored in - two projects with the same
// languages could answer differently, and neither answer was the project's own
// default. What a reader gets when their language is missing is a decision, and
// it belongs to the project.
func (text LocalizedText) Resolve(language, defaultLanguage string, configured []Language) string {
	if value := strings.TrimSpace(text[language]); value != "" {
		return value
	}
	// "uk-UA" reads a translation stored as "uk", and the other way round: a
	// region is a refinement of a language, not a different one.
	if base, _, ok := strings.Cut(language, "-"); ok && base != "" {
		if value := strings.TrimSpace(text[base]); value != "" {
			return value
		}
	}
	// A stored "uk-UA" answers a request for "uk" as well: the two are the same
	// language, and which side carries the region is an accident of how each was
	// written down.
	if requested := LanguageBase(language); requested != "" {
		for _, key := range sortedKeys(text) {
			if LanguageBase(key) != requested {
				continue
			}
			if value := strings.TrimSpace(text[key]); value != "" {
				return value
			}
		}
	}
	if value := strings.TrimSpace(text[defaultLanguage]); value != "" {
		return value
	}
	for _, item := range configured {
		if value := strings.TrimSpace(text[item.Code]); value != "" {
			return value
		}
	}
	for _, key := range sortedKeys(text) {
		if value := strings.TrimSpace(text[key]); value != "" {
			return value
		}
	}
	return ""
}

// Clone copies a text so that handing one out never hands out the map behind it.
func (text LocalizedText) Clone() LocalizedText {
	if text == nil {
		return nil
	}
	copied := make(LocalizedText, len(text))
	for language, value := range text {
		copied[language] = value
	}
	return copied
}

// sortedKeys keeps every fallback that walks the stored translations
// deterministic - two readers with the same data must get the same answer.
func sortedKeys(text LocalizedText) []string {
	keys := make([]string, 0, len(text))
	for key := range text {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

// LanguageBase drops the region from a locale code: "uk-UA" and "uk" are the
// same language.
func LanguageBase(code string) string {
	base, _, _ := strings.Cut(strings.ToLower(strings.TrimSpace(code)), "-")
	return base
}
