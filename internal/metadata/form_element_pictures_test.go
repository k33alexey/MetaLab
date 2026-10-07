package metadata

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/k33alexey/MetaLab/internal/project"
)

// pictureElements is a table whose column is drawn with two pictures of its
// own, in the header and in the footer.
func pictureElements(column string) string {
	return "items:\n  - {id: c0de0000-0000-4000-8000-000000990001, name: Таблица, kind: table, children: [" +
		"{id: c0de0000-0000-4000-8000-000000990002, name: " + column + ", kind: input-field," +
		" header_picture: {file: HeaderPicture.png}, footer_picture: {file: FooterPicture.gif, load_transparent: true}}]}\n"
}

// The pictures of the elements of a form drawn from a file of their own lie in
// the form's folder, a folder per element named by it, as the prototype keeps
// them; the folder is checked against the form both ways, for a form of an
// object and for a common form alike.
//
// Defect caught: a picture of an element's own refused, so that 84, 66 and 77
// of them in erp, acc and sb are lost at import; a folder or file found only
// in the case it was written in, though the prototype looks a picture up
// without regard to case; a reference to a file the folder does not hold, or
// an element renamed away from its folder, loading clean and drawing a blank
// place; a file no picture draws, the folder of an element the form does not
// have, or of one with no picture of its own, kept silently; something deeper
// than a picture in an element's folder, even under the name of a picture in
// another case; the items folder being a file.
func TestFormElementPicturesLieInTheFormFolder(t *testing.T) {
	t.Parallel()
	type layout func(t *testing.T, items string)
	put := func(names ...string) layout {
		return func(t *testing.T, items string) {
			for _, name := range names {
				writeFile(t, filepath.Join(items, name), "image")
			}
		}
	}
	both := put("Поле/HeaderPicture.png", "Поле/FooterPicture.gif")
	for name, test := range map[string]struct {
		column string
		files  layout
		want   string
	}{
		"оба файла":                      {"Поле", both, ""},
		"в другом регистре":              {"Поле", put("поле/headerpicture.PNG", "поле/FOOTERPICTURE.gif"), ""},
		"файла нет":                      {"Поле", put("Поле/HeaderPicture.png"), "element Поле footer_picture is shown with picture file FooterPicture.gif, which its folder does not hold"},
		"папки нет":                      {"Поле", nil, "element Поле header_picture is shown with picture file HeaderPicture.png, which its folder does not hold"},
		"элемент переименован":           {"Сумма", both, "element Сумма header_picture is shown with picture file HeaderPicture.png"},
		"лишний файл":                    {"Поле", put("Поле/HeaderPicture.png", "Поле/FooterPicture.gif", "Поле/Picture.png"), `element Поле keeps "Picture.png", which its pictures do not draw`},
		"папка чужого элемента":          {"Поле", put("Поле/HeaderPicture.png", "Поле/FooterPicture.gif", "Нет/Picture.png"), `keeps "Нет" among the pictures of its elements, and the form has no element of that name`},
		"папка элемента без картинок":    {"Поле", put("Поле/HeaderPicture.png", "Поле/FooterPicture.gif", "Таблица/Picture.png"), `element Таблица keeps "Picture.png", which its pictures do not draw`},
		"глубже элемента":                {"Поле", put("Поле/HeaderPicture.png", "Поле/FooterPicture.gif", "Поле/a/HeaderPicture.png"), `element Поле keeps "a", which its pictures do not draw`},
		"папка под именем картинки":      {"Поле", put("Поле/HeaderPicture.png", "Поле/FooterPicture.gif", "Поле/HEADERPICTURE.PNG/a.png"), `element Поле keeps "HEADERPICTURE.PNG", which its pictures do not draw`},
		"файл прямо в папке элементов":   {"Поле", put("Поле/HeaderPicture.png", "Поле/FooterPicture.gif", "HeaderPicture.png"), `keeps "HeaderPicture.png" among the pictures of its elements`},
		"папка элементов файлом":         {"Поле", func(t *testing.T, items string) { writeFile(t, items, "image") }, "element Поле header_picture is shown with picture file"},
		"папка элементов файлом, пустая": {"", func(t *testing.T, items string) { writeFile(t, items, "image") }, `keeps "items", and the pictures of its elements are a folder`},
	} {
		for _, common := range []bool{false, true} {
			where := "форма объекта"
			if common {
				where = "общая форма"
			}
			t.Run(name+"/"+where, func(t *testing.T) {
				t.Parallel()
				root := formReferencesProject(t)
				// A folder beside a file of the same name in another case is
				// possible only where the case of a name tells files apart.
				if name == "папка под именем картинки" && !caseSensitive(t, root) {
					t.Skip("the file system does not tell names apart by case")
				}
				var path, prefix string
				if common {
					path = filepath.Join(root, "metadata", "common-forms", "АдреснаяКнига", project.FormMetadataFile)
					prefix = "common form АдреснаяКнига: "
				} else {
					path = filepath.Join(root, "metadata", string(CatalogKind), "Номенклатура", "forms", "ФормаЭлемента", project.FormMetadataFile)
					prefix = "catalog Номенклатура form ФормаЭлемента: "
				}
				if test.column != "" {
					content, err := os.ReadFile(path)
					if err != nil {
						t.Fatal(err)
					}
					writeFile(t, path, strings.Replace(string(content), "attributes:\n", pictureElements(test.column)+"attributes:\n", 1))
				}
				if test.files != nil {
					test.files(t, filepath.Join(filepath.Dir(path), project.FormItemsDirectory))
				}
				_, err := Load(root)
				switch {
				case test.want == "" && err != nil:
					t.Fatal(err)
				case test.want != "" && (err == nil || !strings.Contains(err.Error(), prefix+test.want)):
					t.Fatalf("err = %v, want %q", err, prefix+test.want)
				}
			})
		}
	}
}

// caseSensitive reports whether the file system under the folder tells names
// apart by case.
func caseSensitive(t *testing.T, directory string) bool {
	t.Helper()
	writeFile(t, filepath.Join(directory, "Регистр"), "")
	_, err := os.Stat(filepath.Join(directory, "рЕГИСТР"))
	if err := os.Remove(filepath.Join(directory, "Регистр")); err != nil {
		t.Fatal(err)
	}
	return err != nil
}
