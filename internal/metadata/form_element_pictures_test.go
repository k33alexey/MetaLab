package metadata

import (
	"os"
	"path/filepath"
	"reflect"
	"slices"
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

// A picture of an element's own may be a set of variants, as a common picture
// is: a folder in the element's folder holding the files the variants name -
// the prototype's Picture.zip, a button of lombard1 with eight densities and
// the 8.2 image. The folder is checked as the folder of a common picture, and
// the set is read back as written.
//
// Defect caught: the set refused, so that the form holding it does not load
// (lombard1 ПарсингЦеныНоменклатуры.ФормаСписка); the folder of the set or
// a file a variant names missing, or the set kept as a file, loading clean and
// drawing a blank place; a file of the set that is no image, or a folder
// inside it, kept silently; the folder found only in the case it was written
// in; the variants lost on the way back.
func TestAPictureOfAnElementMayBeASetOfVariants(t *testing.T) {
	t.Parallel()
	const button = "items:\n  - {id: c0de0000-0000-4000-8000-000000990001, name: Создать, kind: button, picture: {file: Picture, variants: [" +
		"{file: 100.png, density: 100, glyph_width: 16, glyph_height: 16}, {file: 200.png, density: 200}," +
		" {file: Picture.png, density: 100, interface: \"8.2\"}, {file: Picture.png, density: 100, interface: 8.2-ordinary-application}]}}\n"
	set := []string{"Создать/Picture/100.png", "Создать/Picture/200.png", "Создать/Picture/Picture.png"}
	for name, test := range map[string]struct {
		files []string
		want  string
	}{
		"набор на месте":       {set, ""},
		"в другом регистре":    {[]string{"создать/PICTURE/100.PNG", "создать/PICTURE/200.png", "создать/PICTURE/picture.png"}, ""},
		"лишняя картинка":      {append(slices.Clone(set), "Создать/Picture/300.png"), ""},
		"папки набора нет":     {nil, "element Создать picture is shown with the variants of picture Picture, and its folder holds no folder of them"},
		"набор файлом":         {[]string{"Создать/Picture"}, "element Создать picture is shown with the variants of picture Picture, and its folder holds no folder of them"},
		"варианта нет":         {set[:2], "element Создать picture variants[2] draws Picture.png, which is not in its folder"},
		"не картинка в наборе": {append(slices.Clone(set), "Создать/Picture/manifest.xml"), `element Создать picture keeps "manifest.xml", which is not an image`},
		"папка в наборе":       {append(slices.Clone(set), "Создать/Picture/a/100.png"), `element Создать picture keeps "a", which is not one of its images`},
		"файл рядом с набором": {append(slices.Clone(set), "Создать/Picture.png"), `element Создать keeps "Picture.png", which its pictures do not draw`},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			root := formReferencesProject(t)
			path := filepath.Join(root, "metadata", string(CatalogKind), "Номенклатура", "forms", "ФормаЭлемента", project.FormMetadataFile)
			content, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			writeFile(t, path, strings.Replace(string(content), "attributes:\n", button+"attributes:\n", 1))
			for _, file := range test.files {
				writeFile(t, filepath.Join(filepath.Dir(path), project.FormItemsDirectory, file), "image")
			}
			_, err = Load(root)
			switch {
			case test.want != "":
				if err == nil || !strings.Contains(err.Error(), "catalog Номенклатура form ФормаЭлемента: "+test.want) {
					t.Fatalf("err = %v, want %q", err, test.want)
				}
				return
			case err != nil:
				t.Fatal(err)
			}
			source, err := os.Open(path)
			if err != nil {
				t.Fatal(err)
			}
			defer source.Close()
			form, err := DecodeManagedForm(path, source, managedFormConfiguration())
			if err != nil {
				t.Fatal(err)
			}
			picture := form.FormItems()[0].Picture
			want := []PictureVariant{{File: "100.png", Density: 100, GlyphWidth: 16, GlyphHeight: 16}, {File: "200.png", Density: 200},
				{File: "Picture.png", Density: 100, Interface: Version82PictureInterface}, {File: "Picture.png", Density: 100, Interface: Version82OrdinaryInterface}}
			if picture == nil || picture.File != "Picture" || !reflect.DeepEqual(picture.Variants, want) {
				t.Fatalf("picture = %+v", picture)
			}
		})
	}
}
