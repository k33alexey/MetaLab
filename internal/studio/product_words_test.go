package studio

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// The product names no «конфигурация», «конфигуратор» and no 1С in any of
// the three languages of its interface (ML-STUDIO.md, «„Проект“ вместо
// „конфигурации“»; the owner, 15, 19 and 24.10.2026): the texts of Studio,
// of Manager and of the application are read here as their files hold
// them - the strings and the text of the pages, comments left out, which
// are the developers' and may name the prototype. A string of letters,
// digits, dots, slashes, dashes and underscores alone - a file, an address,
// a key - is not a text a person reads.
var (
	productForbiddenWords = regexp.MustCompile(`(?i)конфигурац|конфигуратор|конфігурац|конфігуратор|configuration|configurator|(?:^|[^0-9A-Za-zА-Яа-яЁё#])1[СC](?:$|[^0-9A-Za-zА-Яа-яЁё])`)
	productBlockComments  = regexp.MustCompile(`(?s)/\*.*?\*/|<!--.*?-->`)
	productLineComments   = regexp.MustCompile("(?m)(^|[^:'\"`\\\\])//[^\n]*")
	productTexts          = regexp.MustCompile("'(?:[^'\\\\\n]|\\\\.)*'|\"(?:[^\"\\\\\n]|\\\\.)*\"|`(?:[^`\\\\]|\\\\.)*`|>[^<>]+<")
	productTechnical      = regexp.MustCompile(`^[A-Za-z0-9./_-]*$`)
)

// TestTheProductNamesNoConfigurationAnd1C catches a text of the interface
// that names the prototype or says «конфигурация» where the product says
// «проект», «ML Studio», «BSL»: «Свернуть дерево конфигурации», «Библиотека
// картинок конфигурации», the sections of the root of the project.
func TestTheProductNamesNoConfigurationAnd1C(t *testing.T) {
	t.Parallel()
	var files []string
	for _, folder := range []string{"ui", filepath.Join("..", "manager", "ui"), filepath.Join("..", "mlapp", "ui")} {
		for _, pattern := range []string{"*.js", "*.html"} {
			found, err := filepath.Glob(filepath.Join(folder, pattern))
			if err != nil {
				t.Fatal(err)
			}
			files = append(files, found...)
		}
	}
	if len(files) < 20 {
		t.Fatalf("the files of the interface are not where the test looks: %v", files)
	}
	for _, file := range files {
		content, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		// A comment is cut out line for line, so a text keeps its line.
		code := productBlockComments.ReplaceAllStringFunc(string(content), func(comment string) string {
			return strings.Repeat("\n", strings.Count(comment, "\n"))
		})
		code = productLineComments.ReplaceAllString(code, "$1")
		for number, line := range strings.Split(code, "\n") {
			for _, text := range productTexts.FindAllString(line, -1) {
				inner := strings.Trim(text, "'\"`<>")
				if productForbiddenWords.MatchString(inner) && !productTechnical.MatchString(inner) {
					t.Errorf("%s:%d: %s", file, number+1, inner)
				}
			}
		}
	}
}

// productGoTexts are the strings of Go - interpreted and raw - and
// productGoComments its comments; productGoTechnical is a word with no
// space - a key, a file, an address (the platform's namespace
// http://v8.1c.ru/), a route, a name in the language
// («ОбновлениеКонфигурацииБазыДанных» is the right's name in BSL and
// stays) - which no person reads as a text.
var (
	productGoTexts     = regexp.MustCompile("\"(?:[^\"\\\\\n]|\\\\.)*\"|`[^`]*`")
	productGoComments  = regexp.MustCompile("(?s)/\\*.*?\\*/|//[^\n]*")
	productGoMarkers   = regexp.MustCompile("\x00([0-9]+)\x01")
	productGoTechnical = regexp.MustCompile(`^(?:(?:GET|PUT|POST|DELETE) )?[\p{L}\p{N}./_*:-]*$`)
)

// TestTheModelSaysNoConfigurationAnd1C catches a refusal or a note of the
// model - what the import report and Studio show - that names the
// prototype or its «конфигурация»: «which is not in the configuration»,
// «Конфигуратор такое сохраняет», «выгрузка конфигуратора» (BLOCKS 2с,
// «„проект“ вместо „конфигурации“: сообщения модели и отчёт импорта»).
// The packages read are those whose words reach the person: the model,
// the project, Studio. A struct tag is a key, not a text.
func TestTheModelSaysNoConfigurationAnd1C(t *testing.T) {
	t.Parallel()
	var files []string
	for _, folder := range []string{filepath.Join("..", "metadata"), filepath.Join("..", "project"), "."} {
		found, err := filepath.Glob(filepath.Join(folder, "*.go"))
		if err != nil {
			t.Fatal(err)
		}
		for _, file := range found {
			if !strings.HasSuffix(file, "_test.go") {
				files = append(files, file)
			}
		}
	}
	if len(files) < 100 {
		t.Fatalf("the files of the model are not where the test looks: %d", len(files))
	}
	for _, file := range files {
		content, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		// Strings first, so a «//» inside one is not taken for a comment;
		// each leaves its number, and a comment - cut out line for line -
		// takes the numbers of the quotes inside it away with it.
		code := string(content)
		var texts []string
		code = productGoTexts.ReplaceAllStringFunc(code, func(text string) string {
			texts = append(texts, text)
			return fmt.Sprintf("\x00%d\x01", len(texts)-1) + strings.Repeat("\n", strings.Count(text, "\n"))
		})
		code = productGoComments.ReplaceAllStringFunc(code, func(comment string) string {
			return strings.Repeat("\n", strings.Count(comment, "\n"))
		})
		for number, line := range strings.Split(code, "\n") {
			for _, marker := range productGoMarkers.FindAllStringSubmatch(line, -1) {
				index, _ := strconv.Atoi(marker[1])
				text := texts[index]
				inner := strings.Trim(text, "\"`")
				if strings.HasPrefix(text, "`") && (strings.Contains(inner, "json:") || strings.Contains(inner, "yaml:")) {
					continue
				}
				if productForbiddenWords.MatchString(inner) && !productGoTechnical.MatchString(inner) {
					t.Errorf("%s:%d: %s", file, number+1, inner)
				}
			}
		}
	}
}
