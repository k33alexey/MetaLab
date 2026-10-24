package studio

import (
	"encoding/json"
	"os/exec"
	"testing"

	"github.com/k33alexey/MetaLab/internal/project"
)

// presentInNode runs textPresentation of studio-common.js on every case read
// from stdin and prints the answers in the same order.
const presentInNode = `
const {readFileSync} = await import('node:fs');
const vm = await import('node:vm');
const context = vm.createContext({});
vm.runInContext(readFileSync(process.argv[1], 'utf8'), context);
const cases = JSON.parse(readFileSync(0, 'utf8'));
context.cases = cases;
process.stdout.write(JSON.stringify(vm.runInContext(
  'cases.map(item => textPresentation(item.text, item.language, item.name))', context)));
`

// Studio shows a text by the server's rule, not by one of its own: the
// translation in the language shown, and the name where there is none
// (METADATA-OBJECTS.md, «Нет перевода на языке — имя»). Defects caught: a
// chain through the default language or another stored translation coming
// back on either side - the one both sides had until 19.10.2026 showed
// «Поступление товаров и услуг» to a reader of Ukrainian - and the page
// drifting from project.LocalizedText.Presentation. The expectation is written
// out by hand, from the owner's rule and the help (a translation is tied to
// the code of a language, «ОбъектМетаданных: Язык», КодЯзыка), and both
// sides are held to it.
func TestStudioShowsTextLikeTheServer(t *testing.T) {
	const name = "ПоступлениеТоваровУслуг"
	type presentCase struct {
		title    string
		want     string
		text     project.LocalizedText
		language string
	}
	cases := []presentCase{
		{"the language shown", "Надходження товарів", project.LocalizedText{"ru": "Поступление товаров", "uk": "Надходження товарів"}, "uk"},
		{"no translation in it is the name, not the main language", name, project.LocalizedText{"ru": "Поступление товаров"}, "uk"},
		{"a translation in an undeclared language is no translation", name, project.LocalizedText{"en": "Goods receipt"}, "uk"},
		{"a translation with no language is no translation", name, project.LocalizedText{"": "Поступление товаров"}, "uk"},
		{"blank counts as none", name, project.LocalizedText{"uk": "  ", "ru": "Поступление товаров"}, "uk"},
		{"the value is trimmed", "Надходження", project.LocalizedText{"uk": "  Надходження  "}, "uk"},
		{"a region is another code", name, project.LocalizedText{"uk": "Надходження"}, "uk-UA"},
		{"a stored region is another code", name, project.LocalizedText{"uk-UA": "Надходження"}, "uk"},
		{"the code is matched as written", name, project.LocalizedText{"uk": "Надходження"}, "UK"},
		{"no language shown", name, project.LocalizedText{"ru": "Поступление товаров"}, ""},
		{"no text", name, nil, "ru"},
	}
	for _, item := range cases {
		if server := item.text.Presentation(item.language, name); server != item.want {
			t.Errorf("%s: the server shows %q, want %q", item.title, server, item.want)
		}
	}
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("Node.js is required for the page's half of this test")
	}
	type jsCase struct {
		Text     project.LocalizedText `json:"text"`
		Language string                `json:"language"`
		Name     string                `json:"name"`
	}
	input := make([]jsCase, len(cases))
	for index, item := range cases {
		input[index] = jsCase{Text: item.text, Language: item.language, Name: name}
	}
	payload, err := json.Marshal(input)
	if err != nil {
		t.Fatal(err)
	}
	output := runInNode(t, node, presentInNode, payload, "ui/studio-common.js")
	var answers []string
	if err := json.Unmarshal(output, &answers); err != nil {
		t.Fatalf("node answered %q: %v", output, err)
	}
	if len(answers) != len(cases) {
		t.Fatalf("node answered %d cases of %d", len(answers), len(cases))
	}
	for index, item := range cases {
		if answers[index] != item.want {
			t.Errorf("%s: Studio shows %q, want %q", item.title, answers[index], item.want)
		}
	}
}
