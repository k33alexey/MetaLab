package studio

import (
	"os"
	"path/filepath"
	"regexp"
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
