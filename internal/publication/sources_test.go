package publication

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"

	"github.com/k33alexey/MetaLab/internal/metadata"
	"github.com/k33alexey/MetaLab/internal/project"
	"github.com/k33alexey/MetaLab/internal/uuid"
)

// Inspection is what "Сохранить данные" reads the project through, so the
// same directory has to produce the same manifest every time: the content
// digest is what tells a database whether anything changed at all.
func TestInspectIsDeterministicOverProjectSources(t *testing.T) {
	t.Parallel()
	root := publicationProject(t)
	modulePath, _ := project.ModulePath(uuid.MustNew())
	if err := os.WriteFile(filepath.Join(root, filepath.FromSlash(modulePath)), []byte("Процедура Тест()\nКонецПроцедуры\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	first, err := inspect(context.Background(), root, SourceState{GitCommit: "0123456789abcdef0123456789abcdef01234567"})
	if err != nil {
		t.Fatal(err)
	}
	second, err := inspect(context.Background(), root, SourceState{GitCommit: "0123456789abcdef0123456789abcdef01234567"})
	if err != nil || !reflect.DeepEqual(first, second) {
		t.Fatalf("second inspection differs: %+v error=%v", second, err)
	}
	if len(first.ContentSHA256) != 64 || first.Format != CurrentPackageFormat || first.ProjectName != "PackageDemo" {
		t.Fatalf("manifest = %+v", first)
	}
	if !slicesContainPath(first.Files, project.ConfigurationFile) || !slicesContainPath(first.Files, modulePath) {
		t.Fatalf("files = %+v", first.Files)
	}
	if err := os.WriteFile(filepath.Join(root, filepath.FromSlash(modulePath)), []byte("Процедура Другой()\nКонецПроцедуры\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	changed, err := inspect(context.Background(), root, SourceState{})
	if err != nil || changed.ContentSHA256 == first.ContentSHA256 {
		t.Fatalf("source change did not change the content digest: %+v error=%v", changed, err)
	}
}

func TestInspectRejectsMalformedAndUnsupportedSources(t *testing.T) {
	t.Parallel()
	root := publicationProject(t)
	formID := uuid.MustNew()
	formPath, _ := project.ObjectFormPath("documents", "Продажа", "Invalid")
	writeSourceFile(t, root, formPath, []byte("format: 1\nid: "+formID.String()+"\nname: Invalid\ntitle: {ru: Invalid}\nkind: unsupported\n"))
	if _, err := inspect(context.Background(), root, SourceState{}); err == nil {
		t.Fatal("inspect accepted a malformed managed form")
	}
	if err := os.RemoveAll(filepath.Join(root, "metadata", "documents")); err != nil {
		t.Fatal(err)
	}

	constantID := uuid.MustNew()
	constantPath, _ := project.ObjectMetadataPath("constants", "Invalid")
	writeSourceFile(t, root, constantPath, []byte("format: 1\nid: "+constantID.String()+"\nname: Invalid\ntitle: {de: Ungültig}\ntypes: [{kind: boolean}]\n"))
	if _, err := inspect(context.Background(), root, SourceState{}); err == nil || !strings.Contains(err.Error(), "unconfigured language") {
		t.Fatalf("unconfigured language error = %v", err)
	}
	if err := os.Remove(filepath.Join(root, filepath.FromSlash(constantPath))); err != nil {
		t.Fatal(err)
	}

	if _, err := inspect(context.Background(), root, SourceState{GitCommit: "short"}); err == nil {
		t.Fatal("inspect accepted a shortened Git commit")
	}
}

// The manifest carries the identity of everything the database has to create,
// so a missing kind here means silently missing tables after saving.
func TestInspectCarriesSchemaIdentityOfEveryStoredKind(t *testing.T) {
	t.Parallel()
	root := publicationProject(t)
	constantID := uuid.MustNew()
	constantPath, _ := project.ObjectMetadataPath("constants", "Режим")
	writeSourceFile(t, root, constantPath, []byte("format: 1\nid: "+constantID.String()+"\nname: Режим\ntitle: {ru: Режим}\ntypes: [{kind: boolean}]\n"))

	catalogID, attributeID := uuid.MustNew(), uuid.MustNew()
	catalogPath, _ := project.ObjectMetadataPath("catalogs", "Товары")
	writeSourceFile(t, root, catalogPath, []byte("format: 1\nid: "+catalogID.String()+"\nname: Товары\ntitle: {ru: Товары}\n"+
		"code: {type: string, length: 9, auto: true, unique: true}\ndescription_length: 250\n"+
		"attributes:\n  - id: "+attributeID.String()+"\n    name: Артикул\n    title: {ru: Артикул}\n    types: [{kind: string, length: 32}]\n    indexing: index\n"))

	documentID, formID := uuid.MustNew(), uuid.MustNew()
	modulePath, _ := project.ObjectModulePath("documents", "Продажа", project.ObjectModuleFile)
	writeSourceFile(t, root, modulePath, []byte("Процедура ПриЗаписи(Отказ)\nКонецПроцедуры\n"))
	formPath, _ := project.ObjectFormPath("documents", "Продажа", "DocumentForm")
	writeSourceFile(t, root, formPath, managedFormYAML(t, formID, "DocumentForm"))
	// The register the document writes into is named by the document, so its
	// identifier is needed before the document is written.
	accumulationID, accumulationDimensionID, accumulationResourceID := uuid.MustNew(), uuid.MustNew(), uuid.MustNew()
	documentPath, _ := project.ObjectMetadataPath("documents", "Продажа")
	writeSourceFile(t, root, documentPath, []byte("format: 1\nid: "+documentID.String()+"\nname: Продажа\ntitle: {ru: Продажа}\n"+
		"number: {type: string, length: 11, auto: false, unique: true, periodicity: year}\nposting: {allowed: true}\n"+
		"movements: ["+accumulationID.String()+"]\n"+
		"forms: {object: DocumentForm}\n"))

	informationID, informationDimensionID, informationResourceID := uuid.MustNew(), uuid.MustNew(), uuid.MustNew()
	informationPath, _ := project.ObjectMetadataPath("information-registers", "КурсыВалют")
	writeSourceFile(t, root, informationPath, []byte("format: 1\nid: "+informationID.String()+"\nname: КурсыВалют\ntitle: {ru: Курсы валют}\n"+
		"write_mode: independent\nperiodicity: day\n"+
		"dimensions:\n  - id: "+informationDimensionID.String()+"\n    name: Валюта\n    title: {ru: Валюта}\n    types: [{kind: catalog, reference: "+catalogID.String()+"}]\n"+
		"resources:\n  - id: "+informationResourceID.String()+"\n    name: Курс\n    title: {ru: Курс}\n    types: [{kind: number, precision: 15, scale: 4}]\n"))

	accumulationPath, _ := project.ObjectMetadataPath("accumulation-registers", "Продажи")
	writeSourceFile(t, root, accumulationPath, []byte("format: 1\nid: "+accumulationID.String()+"\nname: Продажи\ntitle: {ru: Продажи}\nkind: turnover\n"+
		"dimensions:\n  - id: "+accumulationDimensionID.String()+"\n    name: Товар\n    title: {ru: Товар}\n    types: [{kind: string, length: 100}]\n"+
		"resources:\n  - id: "+accumulationResourceID.String()+"\n    name: Сумма\n    title: {ru: Сумма}\n    types: [{kind: number, precision: 15, scale: 2}]\n"))

	manifest, err := inspect(context.Background(), root, SourceState{})
	if err != nil {
		t.Fatal(err)
	}
	identities := map[string][]uuid.UUID{
		"constant":              {constantID},
		"catalog":               {catalogID},
		"document":              {documentID},
		"information register":  {informationID},
		"accumulation register": {accumulationID},
	}
	got := map[string][]uuid.UUID{
		"constant":              manifest.ConstantIDs,
		"catalog":               manifest.CatalogIDs,
		"document":              manifest.DocumentIDs,
		"information register":  manifest.InformationRegisterIDs,
		"accumulation register": manifest.AccumulationRegisterIDs,
	}
	for kind, want := range identities {
		if !reflect.DeepEqual(got[kind], want) {
			t.Fatalf("%s identities = %v, want %v", kind, got[kind], want)
		}
	}
	if len(manifest.SchemaSHA256) != 64 || manifest.Runtime.Project.Name != "PackageDemo" {
		t.Fatalf("manifest = %+v", manifest)
	}
}

func TestInspectHonoursCancellationAndRejectsSymlinks(t *testing.T) {
	root := publicationProject(t)
	modulePath, _ := project.ModulePath(uuid.MustNew())
	writeSourceFile(t, root, modulePath, []byte("Процедура Тест()\nКонецПроцедуры\n"))
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := inspect(ctx, root, SourceState{}); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled inspect() error = %v", err)
	}
	if runtime.GOOS == "windows" {
		return
	}
	linkPath, _ := project.ModulePath(uuid.MustNew())
	if err := os.Symlink(filepath.Join(root, project.ConfigurationFile), filepath.Join(root, filepath.FromSlash(linkPath))); err != nil {
		t.Fatal(err)
	}
	if _, err := inspect(context.Background(), root, SourceState{}); err == nil {
		t.Fatal("inspect accepted a symbolic link")
	}
}

func slicesContainPath(entries []FileEntry, path string) bool {
	for _, entry := range entries {
		if entry.Path == path {
			return true
		}
	}
	return false
}

func writeSourceFile(t *testing.T, root, relative string, content []byte) {
	t.Helper()
	absolute := filepath.Join(root, filepath.FromSlash(relative))
	if err := os.MkdirAll(filepath.Dir(absolute), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(absolute, content, 0o644); err != nil {
		t.Fatal(err)
	}
}

func managedFormYAML(t *testing.T, id uuid.UUID, name string) []byte {
	t.Helper()
	var content bytes.Buffer
	form := metadata.ManagedForm{Format: metadata.CurrentFormat, ID: id, Name: name, Title: metadata.LocalizedText{"ru": name}, Kind: metadata.ObjectForm}
	if err := metadata.Encode(&content, form); err != nil {
		t.Fatal(err)
	}
	return content.Bytes()
}

func publicationProject(t *testing.T) string {
	t.Helper()
	root := filepath.Join(t.TempDir(), "project")
	configuration := project.Project{
		Format: project.CurrentFormat, ID: uuid.MustNew(), Name: "PackageDemo", Title: project.LocalizedText{"ru": "Package Demo"},
		DefaultLanguage: "ru", Languages: []project.Language{{ID: uuid.MustNew(), Name: "Русский", Title: project.LocalizedText{"ru": "Русский"}, Code: "ru"}},
	}
	if err := project.Initialize(root, configuration); err != nil {
		t.Fatal(err)
	}
	return root
}

// Everything an object keeps in a folder of its own - a command, a form, a
// template - is now named after itself, and the shape of the path is what says
// so before anything is read. A publication that accepted the old shapes would
// carry files no loader can place.
func TestObjectFolderPathShapes(t *testing.T) {
	t.Parallel()
	for name, test := range map[string]struct {
		relative  string
		directory bool
		accepted  bool
	}{
		"папка макета по имени": {"metadata/catalogs/Товары/templates/ПечатнаяФорма", true, true},
		"содержимое макета":     {"metadata/catalogs/Товары/templates/ПечатнаяФорма/content.yaml", false, true},
		"папка макета по идентификатору": {
			"metadata/catalogs/Товары/templates/ce000000-0000-4000-8000-000000000100", true, false},
		"папка формы по имени": {"metadata/catalogs/Товары/forms/ФормаЭлемента", true, true},
		"описание формы":       {"metadata/catalogs/Товары/forms/ФормаЭлемента/form.yaml", false, true},
		"модуль формы":         {"metadata/catalogs/Товары/forms/ФормаЭлемента/МодульФормы.bsl", false, true},
		"посторонний файл у формы": {
			"metadata/catalogs/Товары/forms/ФормаЭлемента/заметки.txt", false, false},
		"папка команды":    {"metadata/catalogs/Товары/commands/Пересчитать", true, true},
		"модуль команды":   {"metadata/catalogs/Товары/commands/Пересчитать/МодульКоманды.bsl", false, true},
		"модуль объекта":   {"metadata/catalogs/Товары/МодульОбъекта.bsl", false, true},
		"описание объекта": {"metadata/catalogs/Товары/object.yaml", false, true},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			err := validateObjectFolderSourcePath(strings.Split(test.relative, "/"), test.relative, test.directory)
			if test.accepted && err != nil {
				t.Fatalf("%s was refused: %v", test.relative, err)
			}
			if !test.accepted && err == nil {
				t.Fatalf("%s was accepted", test.relative)
			}
		})
	}
}

// A common template and a common picture keep their content in the folder that
// is the template or the picture itself. Publication walks that folder, so the
// shape of those paths has to be one it knows - otherwise a project carrying a
// printed form or an icon would refuse to publish at all.
func TestCommonTemplateAndPicturePathShapes(t *testing.T) {
	t.Parallel()
	for name, test := range map[string]struct {
		relative  string
		directory bool
		accepted  bool
	}{
		"папка общего макета":      {"metadata/common-templates/ПечатнаяФорма", true, true},
		"описание общего макета":   {"metadata/common-templates/ПечатнаяФорма/object.yaml", false, true},
		"содержимое общего макета": {"metadata/common-templates/ПечатнаяФорма/content.yaml", false, true},
		"документ на язык":         {"metadata/common-templates/Инструкция/ru.html", false, true},
		"посторонний файл у общего макета": {
			"metadata/common-templates/ПечатнаяФорма/заметки.txt", false, false},
		"папка общей картинки":    {"metadata/common-pictures/Печать", true, true},
		"описание общей картинки": {"metadata/common-pictures/Печать/object.yaml", false, true},
		"образ картинки":          {"metadata/common-pictures/Печать/100.png", false, true},
		"образ картинки плотнее":  {"metadata/common-pictures/Печать/200.svg", false, true},
		"образ не с лестницы":     {"metadata/common-pictures/Печать/110.png", false, false},
		"образ без плотности":     {"metadata/common-pictures/Печать/Печать.png", false, false},
		// The schema of an XDTO package and the service description of a WS
		// reference are content under a fixed name, and nothing else is.
		"схема пакета XDTO":         {"metadata/xdto-packages/Обмен/content.xml", false, true},
		"описание WS-ссылки":        {"metadata/ws-references/Склад/definition.xml", false, true},
		"чужое имя у пакета XDTO":   {"metadata/xdto-packages/Обмен/schema.xml", false, false},
		"схема пакета у WS-ссылки":  {"metadata/ws-references/Склад/content.xml", false, false},
		"описание WS у пакета XDTO": {"metadata/xdto-packages/Обмен/definition.xml", false, false},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			err := validateSourcePath(test.relative, test.directory)
			if test.accepted && err != nil {
				t.Fatalf("%s was refused: %v", test.relative, err)
			}
			if !test.accepted && err == nil {
				t.Fatalf("%s was accepted", test.relative)
			}
		})
	}
}

// A table of an external data source keeps the files a catalog keeps, one level
// deeper, and publication walks it. Catches a project with a table refusing to
// publish at all - which it did - and the depth letting through what a
// catalog's folder would refuse.
func TestSubordinateObjectPathShapes(t *testing.T) {
	t.Parallel()
	const table = "metadata/external-data-sources/Склад/tables/Товары"
	for name, test := range map[string]struct {
		relative  string
		directory bool
		accepted  bool
	}{
		"папка источника":          {"metadata/external-data-sources/Склад", true, true},
		"описание источника":       {"metadata/external-data-sources/Склад/object.yaml", false, true},
		"коллекция таблиц":         {"metadata/external-data-sources/Склад/tables", true, true},
		"папка таблицы":            {table, true, true},
		"описание таблицы":         {table + "/object.yaml", false, true},
		"модуль объекта таблицы":   {table + "/МодульОбъекта.bsl", false, true},
		"папка форм таблицы":       {table + "/forms", true, true},
		"форма таблицы":            {table + "/forms/ФормаСписка/form.yaml", false, true},
		"модуль формы таблицы":     {table + "/forms/ФормаСписка/МодульФормы.bsl", false, true},
		"модуль команды таблицы":   {table + "/commands/Печать/МодульКоманды.bsl", false, true},
		"макет таблицы":            {table + "/templates/Макет/content.yaml", false, true},
		"чужая коллекция":          {"metadata/external-data-sources/Склад/cubes", true, false},
		"файл в коллекции":         {"metadata/external-data-sources/Склад/tables/Товары.yaml", false, false},
		"посторонний файл таблицы": {table + "/заметки.txt", false, false},
		"не модуль у таблицы":      {table + "/МодульСервиса.bsl", false, false},
		"не форма в формах":        {table + "/forms/ФормаСписка/заметки.yaml", false, false},
		"таблица с точкой в имени": {"metadata/external-data-sources/Склад/tables/Тов.ары", true, false},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			err := validateSourcePath(test.relative, test.directory)
			if test.accepted && err != nil {
				t.Fatalf("%s was refused: %v", test.relative, err)
			}
			if !test.accepted && err == nil {
				t.Fatalf("%s was accepted", test.relative)
			}
		})
	}
	if name, ok := objectFolderFormName(table + "/forms/ФормаСписка/form.yaml"); !ok || name != "ФормаСписка" {
		t.Fatalf("a form of a table is not recognised as a form: %q %v", name, ok)
	}
}

// A project with a table of an external data source publishes whole: the
// table's description, its module and its form go into the package, and the
// form into the published snapshot under the table's address. Catches the
// table's files missing from the package and its form missing from what the
// server knows, each of which a check of path shapes alone would not see.
func TestInspectPublishesATableOfAnExternalDataSource(t *testing.T) {
	t.Parallel()
	root := publicationProject(t)
	const table = "metadata/external-data-sources/Склад/tables/Товары"
	for relative, content := range map[string]string{
		"metadata/external-data-sources/Склад/object.yaml": "format: 1\nid: fa000000-0000-4000-8000-000000000001\nname: Склад\ntitle: {ru: Склад}\n",
		table + "/object.yaml": "format: 1\nid: fa000000-0000-4000-8000-000000000002\nname: Товары\ntitle: {ru: Товары}\n" +
			"name_in_data_source: dbo.Goods\ndata_type: non-object\nforms: {list: ФормаСписка}\n",
		table + "/МодульНабораЗаписей.bsl":           "Процедура ПередЗаписью(Отказ, Замещение)\nКонецПроцедуры\n",
		table + "/forms/ФормаСписка/form.yaml":       "format: 1\nid: fa000000-0000-4000-8000-000000000003\nname: ФормаСписка\ntitle: {ru: Список}\nkind: list\n",
		table + "/forms/ФормаСписка/МодульФормы.bsl": "\n",
	} {
		path := filepath.Join(root, filepath.FromSlash(relative))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	manifest, err := inspect(context.Background(), root, SourceState{})
	if err != nil {
		t.Fatalf("a project with a table of a source did not publish: %v", err)
	}
	for _, relative := range []string{table + "/object.yaml", table + "/МодульНабораЗаписей.bsl", table + "/forms/ФормаСписка/form.yaml"} {
		if !slicesContainPath(manifest.Files, relative) {
			t.Errorf("the package misses %s", relative)
		}
	}
	found := false
	for _, form := range manifest.Runtime.ObjectForms {
		if form.ObjectKind == metadata.ExternalDataSourceTableKind && form.Object == "Склад.Товары" && form.Name == "ФормаСписка" {
			found = true
		}
	}
	if !found {
		t.Fatalf("the published snapshot does not know the table's form: %+v", manifest.Runtime.ObjectForms)
	}
}
