package metadata

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/k33alexey/MetaLab/internal/project"
	"github.com/k33alexey/MetaLab/internal/uuid"
)

// A reference to a picture names the point whose colour is drawn transparent,
// the way a common picture does: the prototype writes it beside common
// commands, command groups and subsystems (0, 3, 9, 0 and 1 times). Defect
// caught: the reference had no place for the point, and decoding refused a
// description that carried it - or, read past, the point was lost and another
// colour went transparent.
func TestPictureReferenceKeepsItsTransparentPixel(t *testing.T) {
	t.Parallel()
	root := metadataProject(t)
	picture := "picture: {standard: Обмен, load_transparent: true, transparent_pixel: {x: 7, y: 5}}\n"
	writeCommonCommand(t, root, "Обменяться", "format: 1\nid: "+commonCommandID+"\nname: Обменяться\ntitle: {ru: Обменяться}\n"+picture, true)
	writeMetadata(t, root, CommandGroupKind, commandGroupID, "format: 1\nid: "+commandGroupID+"\nname: Обмены\ntitle: {ru: Обмены}\n"+
		"category: actions-panel\n"+picture)
	subsystem := uuid.MustNew().String()
	writeMetadata(t, root, SubsystemKind, subsystem, "format: 1\nid: "+subsystem+"\nname: Продажи\ntitle: {ru: Продажи}\n"+picture)
	catalogID := uuid.MustNew().String()
	writeMetadata(t, root, CatalogKind, catalogID, "format: 1\nid: "+catalogID+"\nname: Товары\ntitle: {ru: Товары}\n"+
		"code: {type: string, length: 9}\ndescription_length: 150\n"+
		"commands:\n  - {id: "+uuid.MustNew().String()+", name: Подбор, title: {ru: Подбор}, "+strings.TrimSuffix(picture, "\n")+"}\n")
	writeCommandModule(t, root, CatalogKind, "Товары", "Подбор")
	catalog, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	holders := map[string]*PictureReference{}
	if command, ok := catalog.CommonCommand("Обменяться"); ok {
		holders["общая команда"] = command.Picture
	}
	for _, group := range catalog.CommandGroups {
		holders["группа команд"] = group.Picture
	}
	for _, item := range catalog.Subsystems {
		holders["подсистема"] = item.Picture
	}
	for _, item := range catalog.Catalogs {
		for _, command := range item.Commands {
			holders["команда объекта"] = command.Picture
		}
	}
	for _, owner := range []string{"общая команда", "группа команд", "подсистема", "команда объекта"} {
		picture := holders[owner]
		if picture == nil || picture.TransparentPixel == nil || *picture.TransparentPixel != (PicturePixel{X: 7, Y: 5}) {
			t.Errorf("%s lost the transparent point of its picture: %+v", owner, picture)
		}
	}
}

// The point is checked the way a common picture's is: it belongs to a picture
// loaded transparent and lies inside the image. Defect caught: the reference
// took the point without the checks its twin in a common picture has, and the
// two disagreed on one type of the prototype.
func TestPictureReferenceChecksItsTransparentPixel(t *testing.T) {
	t.Parallel()
	for name, test := range map[string]struct{ picture, refusal string }{
		"точка без прозрачности": {"{standard: Обмен, transparent_pixel: {x: 1, y: 1}}",
			"picture.transparent_pixel belongs to a picture loaded transparent only"},
		"точка за краем": {"{standard: Обмен, load_transparent: true, transparent_pixel: {x: 1, y: -1}}",
			"picture.transparent_pixel must be a point inside the image"},
		// Nothing left to carry: written as no reference at all.
		"пустая ссылка": {"{}", "picture must name a standard picture or a common picture, or carry what is left of one"},
		"две картинки": {"{standard: Обмен, common: " + commonPictureID + "}",
			"picture names both a standard picture and a common picture"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			id := uuid.MustNew().String()
			_, err := DecodeSubsystem("subsystem.yaml", strings.NewReader("format: 1\nid: "+id+"\nname: Продажи\ntitle: {ru: Продажи}\npicture: "+test.picture+"\n"),
				project.Project{})
			if err == nil || !strings.Contains(err.Error(), test.refusal) {
				t.Fatalf("refused for another reason or not at all: %v", err)
			}
		})
	}
}

// A reference to «picture 0» - the picture removed, its settings left - is
// carried with the settings (3, 2, 6, 0 and 0 such references, 8 with
// settings), and whatever stands beside it is drawn as without a picture.
// Defects caught: such a reference was refused for naming no picture; or,
// carried, a command asking to be shown as its picture was drawn as an empty
// button.
func TestPictureSettingsWithoutPictureAreCarriedAndDrawnAsNone(t *testing.T) {
	t.Parallel()
	root := metadataProject(t)
	writeCommonCommand(t, root, "Обменяться", "format: 1\nid: "+commonCommandID+"\nname: Обменяться\ntitle: {ru: Обменяться}\n"+
		"representation: picture\npicture: {load_transparent: true, transparent_pixel: {x: 0, y: 15}}\n", true)
	catalog, err := Load(root)
	if err != nil {
		t.Fatalf("a reference left with its settings was refused: %v", err)
	}
	command, _ := catalog.CommonCommand("Обменяться")
	if command.Picture == nil || !command.Picture.LoadTransparent || command.Picture.TransparentPixel == nil {
		t.Fatalf("the settings left by the picture were lost: %+v", command.Picture)
	}
	if command.Picture.Names() {
		t.Fatal("a reference without a picture reports a picture to draw")
	}
	if shown := command.Representation.ShownAs(command.Picture); shown != CommandText {
		t.Fatalf("a command shown as a picture it no longer has is drawn as %q, not as its title", shown)
	}
	kinds := map[NoteKind]string{}
	for _, note := range catalog.Notes() {
		kinds[note.Kind] = note.Written
	}
	if kinds[NotePictureSettingsLeft] != "transparent_pixel 0,15" {
		t.Fatalf("the settings without a picture carry no note naming them: %v", kinds)
	}
	if _, ok := kinds[NotePictureWithoutPicture]; !ok {
		t.Fatalf("the command shown as a picture it has not got carries no note: %v", kinds)
	}
}

// A snapshot owns its references: the point of a subsystem's picture changed
// by the caller after the snapshot was taken does not change the snapshot.
// Defect caught: the subsystem was copied without its picture, and the
// snapshot shared it with the caller.
func TestSnapshotOwnsThePictureOfASubsystem(t *testing.T) {
	t.Parallel()
	id := uuid.MustNew()
	subsystems := []SubsystemDefinition{{Format: 1, ID: id, Name: "Продажи", Title: LocalizedText{"ru": "Продажи"},
		Picture: &PictureReference{Standard: "Обмен", LoadTransparent: true, TransparentPixel: &PicturePixel{X: 1, Y: 2}}}}
	catalog, err := NewCatalogSnapshotWithEventSubscriptions(project.Project{}, nil, nil, nil, nil, nil, nil, nil, nil, subsystems, nil, nil, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	subsystems[0].Picture.TransparentPixel.X = 99
	subsystems[0].Picture.Standard = "Другая"
	if picture := catalog.Subsystems[0].Picture; picture.TransparentPixel.X != 1 || picture.Standard != "Обмен" {
		t.Fatalf("the snapshot shares the picture of the subsystem with its caller: %+v", picture)
	}
}

// A graphical schema keeps the pictures of its items beside its content, a
// folder per item, as the prototype does (Items/Декорация11/Picture.png, erp
// 2) - in an object's template and in a common one alike. Defect caught: the
// folder was refused, and with it the data processor holding the schema.
func TestGraphicalSchemaKeepsThePicturesOfItsItems(t *testing.T) {
	t.Parallel()
	place := func(t *testing.T, directory, relative string, folder bool) {
		t.Helper()
		path := filepath.Join(directory, project.SchemaItemsDirectory, relative)
		if folder {
			if err := os.MkdirAll(path, 0o755); err != nil {
				t.Fatal(err)
			}
			return
		}
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("образ"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	declare := func(t *testing.T, root string, kind TemplateKind) string {
		writeMetadata(t, root, CatalogKind, templateCatalog, "format: 1\nid: "+templateCatalog+"\nname: Контрагенты\ntitle: {ru: Контрагенты}\n"+
			"code: {type: string, length: 9, auto: true}\ndescription_length: 150\n"+
			"templates:\n  - {id: "+templateIdentifier(0)+", name: Методика, title: {ru: Методика}, kind: "+string(kind)+"}\n")
		directory := filepath.Join(root, "metadata", string(CatalogKind), "Контрагенты", "templates", "Методика")
		if err := os.MkdirAll(directory, 0o755); err != nil {
			t.Fatal(err)
		}
		return directory
	}
	common := func(t *testing.T, root string, kind TemplateKind) string {
		writeCommonTemplate(t, root, commonTemplateID, "Методика", kind)
		return filepath.Join(root, "metadata", string(CommonTemplateKind), "Методика")
	}
	for owner, make := range map[string]func(*testing.T, string, TemplateKind) string{"макет объекта": declare, "общий макет": common} {
		t.Run(owner, func(t *testing.T) {
			t.Parallel()
			root := metadataProject(t)
			directory := make(t, root, GraphicalSchema)
			writeText(t, directory, "", "content.yaml", "{}\n")
			place(t, directory, "Декорация11/Picture.png", false)
			place(t, directory, "Декорация 12/Picture.png", false)
			if _, err := Load(root); err != nil {
				t.Fatalf("a graphical schema with the pictures of its items was refused: %v", err)
			}

			for name, test := range map[string]struct {
				kind     TemplateKind
				relative string
				folder   bool
				refusal  string
			}{
				// Only a graphical schema has items with pictures.
				"картинки элементов у табличного макета": {SpreadsheetTemplate, "Декорация11/Picture.png", false, "not a file of its content"},
				"файл прямо в папке элементов":           {GraphicalSchema, "Picture.png", false, "each item keeps a folder named by it"},
				"папка внутри элемента":                  {GraphicalSchema, "Декорация11/вложено/Picture.png", false, `keeps "вложено", which is not a picture file`},
				"скрытый файл у элемента":                {GraphicalSchema, "Декорация11/.DS_Store", false, `keeps ".DS_Store", which is not a picture file`},
				"скрытая папка элемента":                 {GraphicalSchema, ".Декорация11", true, "each item keeps a folder named by it"},
			} {
				t.Run(name, func(t *testing.T) {
					t.Parallel()
					root := metadataProject(t)
					directory := make(t, root, test.kind)
					place(t, directory, test.relative, test.folder)
					_, err := Load(root)
					if err == nil || !strings.Contains(err.Error(), test.refusal) {
						t.Fatalf("refused for another reason or not at all: %v", err)
					}
				})
			}
		})
	}
}
