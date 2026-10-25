package metadata

import (
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/k33alexey/MetaLab/internal/uuid"
)

// A project holding at once what the iterations of block 2б taught the model
// one at a time reads, and every place of the window of the type offers in it
// only what the model then takes: a second source with an object table keyed
// by binary data (2.541, 2.546), a field referring to the dimension table of
// the first source (2.538, noted), a function of a number and a string
// (2.544); a cube there whose dimension table has a field of binary data
// referring to the object table of the first source (2.545, 2.549, noted) and
// a resource of a defined type of «Число + Строка» (2.548); a defined type of
// the manager of any settings storage (2.543, unnoted); two calculation
// registers with a recalculation of one name, which the window of the source
// of a subscription tells apart by the register (2.547). What the closure of
// the block checks here is that the rules live together: each was tested
// alone.
//
// Defect caught: two rules of the block that refuse each other's project; a
// place of the window offering, in a project of every kind together, what its
// model refuses; a note left on what the model knows, or missing on a
// reference to another source.
func TestAProjectOfEveryPlaceOfTheWindowReadsAndOffersWhatItTakes(t *testing.T) {
	t.Parallel()
	root := compositeProject(t)
	id := func() string { return uuid.MustNew().String() }
	head := func(id, name string) string {
		return "format: 1\nid: " + id + "\nname: " + name + "\ntitle: {ru: " + name + "}\n"
	}

	measure, storages := id(), id()
	writeMetadata(t, root, DefinedTypeKind, measure, head(measure, "Мера")+
		"types: [{kind: number, precision: 15, scale: 2}, {kind: string, length: 10}]\n")
	writeMetadata(t, root, DefinedTypeKind, storages, head(storages, "Хранилища")+"types: [{kind: settings-storage-manager}]\n")

	// The second source: an object table keyed by binary data, a field of
	// it referring to the first source's dimension table, a function of the
	// simple types, a cube.
	accounts, accountsKey, cube, dimensionTable := id(), id(), id(), id()
	writeExternalSource(t, root, otherSource, "Бухгалтерия", "functions:\n"+
		"  - {id: "+id()+", name: Остаток, title: {ru: Остаток}, return_value: true, "+
		"types: [{kind: number, precision: 15, scale: 2}, {kind: string, length: 10}]}\n")
	writeExternalTable(t, root, "Бухгалтерия", accounts, "Счета", "name_in_data_source: accounts\ndata_type: object\nkey_fields: [Ключ]\nfields:\n"+
		"  - {id: "+accountsKey+", name: Ключ, title: {ru: Ключ}, types: [{kind: binary-data, length: 16, fixed_length: true}], name_in_data_source: key}\n"+
		"  - {id: "+id()+", name: Товар, title: {ru: Товар}, types: [{kind: external-data-source-dimension-table, reference: "+goodsDimTable+"}], "+
		"name_in_data_source: goods}\n")
	cubeDirectory := filepath.Join(root, "metadata", string(ExternalDataSourceKind), "Бухгалтерия", ExternalDataSourceCubesDirectory, "Обороты")
	writeFile(t, filepath.Join(cubeDirectory, "object.yaml"), head(cube, "Обороты")+"name_in_data_source: Turnovers\n"+
		"dimensions:\n  - {id: "+id()+", name: Счет, title: {ru: Счет}, types: [{kind: external-data-source-dimension-table, reference: "+dimensionTable+"}]}\n"+
		"resources:\n  - {id: "+id()+", name: Сумма, title: {ru: Сумма}, types: [{kind: defined-type, reference: "+measure+"}], name_in_data_source: amount}\n")
	writeFile(t, filepath.Join(cubeDirectory, ExternalDimensionTablesDirectory, "Счета", "object.yaml"), head(dimensionTable, "Счета")+
		"name_in_data_source: Accounts\nfields:\n"+
		"  - {id: "+id()+", name: Файл, title: {ru: Файл}, types: [{kind: binary-data, length: 16}], name_in_data_source: file}\n"+
		"  - {id: "+id()+", name: Товар, title: {ru: Товар}, types: [{kind: external-data-source-table, reference: "+goodsTable+"}], name_in_data_source: goods}\n")

	// A second calculation register with a recalculation of the name the
	// composite project's register gives its own.
	deductions := id()
	writeMetadata(t, root, CalculationRegisterKind, deductions, head(deductions, "Удержания")+
		"chart_of_calculation_types: "+cmpCalculationTypes+"\nperiodicity: month\n"+
		"resources:\n  - {id: "+id()+", name: Сумма, title: {ru: Сумма}, types: [{kind: number, precision: 15, scale: 2}]}\n"+
		"recalculations:\n  - {id: "+id()+", name: Перерасчет, title: {ru: Перерасчёт}}\n")

	catalog, err := Load(root)
	if err != nil {
		t.Fatalf("the project of every place was refused: %v", err)
	}
	var noted []string
	for _, note := range catalog.Notes() {
		if strings.Contains(note.Where, "Хранилища") || strings.Contains(note.Where, "Мера") {
			t.Errorf("noted what the model knows: %+v", note)
		}
		if note.Kind == NoteForeignSourceReference {
			noted = append(noted, note.Where)
		}
	}
	if want := []string{"external-data-sources Бухгалтерия cubes Обороты dimension-tables Счета fields Товар",
		"external-data-sources Бухгалтерия tables Счета fields Товар"}; !slices.Equal(slices.Sorted(slices.Values(noted)), want) {
		t.Errorf("noted as references to another source %v, want %v", noted, want)
	}

	everyOfferedTypeIsAccepted(t, catalog)

	offer, _ := catalog.TypeOffer(SubscriptionSourceChoice)
	for _, row := range offer.Rows {
		if row.Kind != RecalculationRecordSetType {
			continue
		}
		if got := slices.Sorted(slices.Values(names(row.Objects))); !slices.Equal(got, []string{"Начисления.Перерасчет", "Удержания.Перерасчет"}) {
			t.Errorf("the recalculations of the window of a subscription are %v", got)
		}
	}
}
