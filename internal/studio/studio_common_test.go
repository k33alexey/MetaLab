package studio

import (
	"bytes"
	"context"
	"encoding/json"
	"os/exec"
	"testing"
	"time"

	"github.com/k33alexey/MetaLab/internal/project"
)

// resolveInNode runs resolveLocalizedText of studio-common.js on every case
// read from stdin and prints the answers in the same order.
const resolveInNode = `
const {readFileSync} = await import('node:fs');
const vm = await import('node:vm');
const context = vm.createContext({});
vm.runInContext(readFileSync(process.argv[1], 'utf8'), context);
const cases = JSON.parse(readFileSync(0, 'utf8'));
context.cases = cases;
process.stdout.write(JSON.stringify(vm.runInContext(
  'cases.map(item => resolveLocalizedText(item.text, item.language, item.defaultLanguage, item.languages))', context)));
`

// Studio picks the translation it shows by the server's chain, not one of its
// own. Defect caught: the JS chain drifting from project.LocalizedText.Resolve
// - the editors used to take the default language and then whatever value came
// first in the map, so a title could show in one language in Studio and in
// another in the application. The expectation is written out by hand and both
// sides are held to it, so a change to either shows here.
func TestStudioResolvesTextLikeTheServer(t *testing.T) {
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("Node.js is required for this test")
	}
	languages := func(codes ...string) []project.Language {
		result := make([]project.Language, len(codes))
		for index, code := range codes {
			result[index] = project.Language{Code: code}
		}
		return result
	}
	type resolveCase struct {
		name            string
		want            string
		text            project.LocalizedText
		language        string
		defaultLanguage string
		languages       []project.Language
	}
	cases := []resolveCase{
		{"the language asked for", "Goods", project.LocalizedText{"ru": "Товары", "en": "Goods"}, "en", "ru", languages("ru", "en")},
		{"blank counts as missing", "Товары", project.LocalizedText{"ru": "Товары", "en": "  "}, "en", "ru", languages("ru", "en")},
		{"value is trimmed", "Товары", project.LocalizedText{"ru": "  Товары  "}, "ru", "ru", languages("ru")},
		{"region falls back to its language", "Товари", project.LocalizedText{"uk": "Товари", "ru": "Товары"}, "uk-UA", "ru", languages("ru", "uk")},
		{"language finds a stored region", "Товари", project.LocalizedText{"uk-UA": "Товари", "ru": "Товары"}, "uk", "ru", languages("ru", "uk")},
		{"case of the region does not matter", "Товари", project.LocalizedText{"uk": "Товари", "ru": "Товары"}, "UK-ua", "ru", languages("ru", "uk")},
		{"default before the configured order", "Товары", project.LocalizedText{"en": "Goods", "ru": "Товары"}, "de", "ru", languages("en", "ru")},
		{"configured order, not the sorted one", "Товары", project.LocalizedText{"de": "Waren", "ru": "Товары"}, "fr", "en", languages("en", "ru", "de")},
		{"any value when no language has one", "A", project.LocalizedText{"zz": "Z", "aa": "A"}, "fr", "en", languages("en")},
		{"empty language and default", "Товары", project.LocalizedText{"ru": "Товары"}, "", "", nil},
		{"nothing at all", "", project.LocalizedText{"ru": " "}, "ru", "ru", languages("ru")},
		{"no text", "", nil, "ru", "ru", languages("ru")},
	}
	type jsLanguage struct {
		Code string `json:"code"`
	}
	type jsCase struct {
		Text            project.LocalizedText `json:"text"`
		Language        string                `json:"language"`
		DefaultLanguage string                `json:"defaultLanguage"`
		Languages       []jsLanguage          `json:"languages"`
	}
	input := make([]jsCase, len(cases))
	for index, item := range cases {
		input[index] = jsCase{Text: item.text, Language: item.language, DefaultLanguage: item.defaultLanguage, Languages: []jsLanguage{}}
		for _, language := range item.languages {
			input[index].Languages = append(input[index].Languages, jsLanguage{Code: language.Code})
		}
	}
	payload, err := json.Marshal(input)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, node, "--input-type=module", "-e", resolveInNode, "ui/studio-common.js")
	command.Stdin = bytes.NewReader(payload)
	output, err := command.Output()
	if err != nil {
		t.Fatalf("node: %v", err)
	}
	var answers []string
	if err := json.Unmarshal(output, &answers); err != nil {
		t.Fatalf("node answered %q: %v", output, err)
	}
	if len(answers) != len(cases) {
		t.Fatalf("node answered %d cases of %d", len(answers), len(cases))
	}
	for index, item := range cases {
		server := item.text.Resolve(item.language, item.defaultLanguage, item.languages)
		if server != item.want {
			t.Errorf("%s: the server shows %q, want %q", item.name, server, item.want)
		}
		if answers[index] != item.want {
			t.Errorf("%s: Studio shows %q, want %q", item.name, answers[index], item.want)
		}
	}
}
