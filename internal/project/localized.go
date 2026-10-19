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

// Resolve returns the translation in the language asked for, and nothing
// when there is none: a translation is tied to the code of a language of the
// configuration and to nothing else (the help, «ОбъектМетаданных: Язык»,
// КодЯзыка). A text of spaces says nothing and counts as none.
func (text LocalizedText) Resolve(language string) string {
	return strings.TrimSpace(text[language])
}

// Presentation is what a text shows in a language: the translation in that
// language, and the name of what it belongs to where there is none - not a
// translation in another language (METADATA-OBJECTS.md, «Нет перевода на
// языке — имя»; the owner's decision of 19.10.2026). Every object has a name,
// so a presentation is never empty. Before that decision a chain went on
// through the default language, the configured languages and any stored
// translation, which is not how the prototype shows its languages.
func (text LocalizedText) Presentation(language, name string) string {
	if value := text.Resolve(language); value != "" {
		return value
	}
	return name
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

// sortedKeys walks the stored translations in one order, so that the same
// text is always checked, and its faults named, the same way.
func sortedKeys(text LocalizedText) []string {
	keys := make([]string, 0, len(text))
	for key := range text {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
