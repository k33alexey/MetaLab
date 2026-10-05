package metadata

import (
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/k33alexey/MetaLab/internal/project"
	"github.com/k33alexey/MetaLab/internal/uuid"
)

// ExternalCubeKind and ExternalDimensionTableKind name a cube of a source and a
// table of a cube's dimension where a kind is asked for. Like a table, neither
// is a folder directly under metadata/: a cube lies in its source's collection
// of cubes, a dimension table in its cube's collection of dimension tables.
const (
	ExternalCubeKind           Kind = "external-data-source-cubes"
	ExternalDimensionTableKind Kind = "external-data-source-dimension-tables"
)

// ExternalDimensionTableType is a reference to one member of a dimension of a
// cube: a row of a dimension table, which, unlike a row of the cube itself, is
// a thing - it has a reference, a presentation and possibly a parent.
const ExternalDimensionTableType TypeKind = "external-data-source-dimension-table"

// The collections the model reads, named as the project lays them out.
const (
	ExternalDataSourceCubesDirectory = "cubes"
	ExternalDimensionTablesDirectory = "dimension-tables"
)

// ExternalCubeForms are the default forms of a cube. A cube is read as a set of
// records, so it has a form of a record and a list and nothing else: the help
// gives it these two and no object or choice form.
type ExternalCubeForms struct {
	Record string `yaml:"record,omitempty" json:"record,omitempty"`
	List   string `yaml:"list,omitempty" json:"list,omitempty"`
}

func (forms ExternalCubeForms) slots() []formSlot {
	return []formSlot{{"forms.record", forms.Record}, {"forms.list", forms.List}}
}

// ExternalCube is one cube of a source: a multidimensional set of records,
// described by its dimensions, its resources and the tables its dimensions'
// members lie in.
//
// The help gives the kinds of the elements of Измерения and Ресурсы no name, and
// they were taken for fields of a source, the one kind of field a source has in
// the help. An export of the platform refuted it: a dimension and a resource
// each have a composition of their own, and neither is a field of a table -
// see ExternalCubeDimension and ExternalCubeResource.
type ExternalCube struct {
	Format int           `yaml:"format" json:"format"`
	ID     uuid.UUID     `yaml:"id" json:"id"`
	Name   string        `yaml:"name" json:"name"`
	Title  LocalizedText `yaml:"title" json:"title"`
	// ListPresentations and RecordPresentations are how the cube and one
	// record of it are named. A cube has no object, so it has no presentation
	// of one.
	ListPresentations     `yaml:",inline" json:",inline"`
	RecordPresentations   `yaml:",inline" json:",inline"`
	IncludeHelpInContents bool `yaml:"include_help_in_contents,omitempty" json:"includeHelpInContents,omitempty"`

	NameInDataSource string                  `yaml:"name_in_data_source" json:"nameInDataSource"`
	Dimensions       []ExternalCubeDimension `yaml:"dimensions,omitempty" json:"dimensions,omitempty"`
	Resources        []ExternalCubeResource  `yaml:"resources,omitempty" json:"resources,omitempty"`
	// Characteristics are carried and not resolved. The syntax assistant does
	// not give a cube any, an export writes the property on a cube all the
	// same, and a property the export writes is carried. Our characteristics
	// hang off a reference to the object they describe, and a record of a cube
	// has none; what the prototype makes of them is a question for a run on
	// the platform.
	Characteristics []ObjectCharacteristic `yaml:"characteristics,omitempty" json:"characteristics,omitempty"`
	Forms           ExternalCubeForms      `yaml:"forms,omitempty" json:"forms,omitempty"`
	Commands        []ObjectCommand        `yaml:"commands,omitempty" json:"commands,omitempty"`
	Templates       []ObjectTemplate       `yaml:"templates,omitempty" json:"templates,omitempty"`
	// DimensionTables are read from the cube's own folder of them.
	DimensionTables []ExternalDimensionTable `yaml:"-" json:"dimensionTables,omitempty"`
}

// ExternalCubeDimension is one dimension of a cube. It is a field with the whole
// palette of an attribute - bounds of the value, filling, the choice of folders
// and items, a link by type - and without indexing, full-text search, data
// history and «использование», none of which an export writes on it. Unlike a
// field of a table it has no name in the data source, no read-only flag and no
// Null allowed: a dimension is found in the other database through the
// dimension table its type refers to.
type ExternalCubeDimension struct {
	Attribute `yaml:",inline" json:",inline"`
}

// ExternalCubeResource is one resource of a cube: a measured value, named in
// the data source. It is shown and chosen like a field, and it is never
// entered - so it has no filling, no check of filling, no creation on input
// and no history of choice, no bounds, no choice of folders and items and no
// link by type. An export writes exactly this composition.
type ExternalCubeResource struct {
	Attribute        `yaml:",inline" json:",inline"`
	NameInDataSource string `yaml:"name_in_data_source" json:"nameInDataSource"`
}

// ExternalDimensionTableForms are the default forms of a dimension table: its
// members are things, so it has an object, a list and a choice form.
type ExternalDimensionTableForms struct {
	Object string `yaml:"object,omitempty" json:"object,omitempty"`
	List   string `yaml:"list,omitempty" json:"list,omitempty"`
	Choice string `yaml:"choice,omitempty" json:"choice,omitempty"`
}

func (forms ExternalDimensionTableForms) slots() []formSlot {
	return []formSlot{{"forms.object", forms.Object}, {"forms.list", forms.List}, {"forms.choice", forms.Choice}}
}

// ExternalDimensionTable is one table of a cube's dimension: the members of the
// dimension, or of one level of its hierarchy.
type ExternalDimensionTable struct {
	Format int           `yaml:"format" json:"format"`
	ID     uuid.UUID     `yaml:"id" json:"id"`
	Name   string        `yaml:"name" json:"name"`
	Title  LocalizedText `yaml:"title" json:"title"`
	// Presentations is how a member and the list of them are named.
	Presentations `yaml:",inline" json:",inline"`

	NameInDataSource string `yaml:"name_in_data_source" json:"nameInDataSource"`
	// HierarchyNameInDataSource is the hierarchy of the dimension this table
	// belongs to, as the other database names it.
	HierarchyNameInDataSource string `yaml:"hierarchy_name_in_data_source,omitempty" json:"hierarchyNameInDataSource,omitempty"`
	// LevelNumber is the level of that hierarchy the table holds, as it was
	// saved. A hierarchical table is read at level 0 whatever is written here -
	// see EffectiveLevelNumber.
	LevelNumber  int  `yaml:"level_number,omitempty" json:"levelNumber,omitempty"`
	Hierarchical bool `yaml:"hierarchical,omitempty" json:"hierarchical,omitempty"`
	// UnfilledParentValue is the text a member's parent holds when it has
	// none. The help types it as a string or Null; absent is Null, the same
	// reading as on a table.
	UnfilledParentValue *string `yaml:"unfilled_parent_value,omitempty" json:"unfilledParentValue,omitempty"`
	PresentationField   string  `yaml:"presentation_field,omitempty" json:"presentationField,omitempty"`
	QuickChoice         bool    `yaml:"quick_choice,omitempty" json:"quickChoice,omitempty"`

	Fields    []ExternalField             `yaml:"fields,omitempty" json:"fields,omitempty"`
	Forms     ExternalDimensionTableForms `yaml:"forms,omitempty" json:"forms,omitempty"`
	Commands  []ObjectCommand             `yaml:"commands,omitempty" json:"commands,omitempty"`
	Templates []ObjectTemplate            `yaml:"templates,omitempty" json:"templates,omitempty"`
}

// DecodeExternalCube reads and validates one cube, without its dimension
// tables.
func DecodeExternalCube(source string, reader io.Reader, configuration project.Project) (ExternalCube, error) {
	var value ExternalCube
	if err := decodeStrict(source, reader, &value); err != nil {
		return ExternalCube{}, err
	}
	issues := validateBase(value.Format, value.ID, value.Name, value.Title, configuration)
	issues = append(issues, validateListPresentations(value.ListPresentations, configuration)...)
	issues = append(issues, validateRecordPresentations(value.RecordPresentations, configuration)...)
	issues = append(issues, validateRequiredNameInDataSource("name_in_data_source", value.NameInDataSource, "the cube")...)
	issues = append(issues, validateObjectCommands(value.Commands, configuration)...)
	issues = append(issues, validateObjectTemplates(value.Templates, configuration)...)
	issues = append(issues, validateObjectCharacteristics(value.Characteristics)...)
	issues = append(issues, validateCubeFields(value, configuration)...)
	if err := issuesError(source, value.Format, issues); err != nil {
		return ExternalCube{}, err
	}
	return value, nil
}

// DecodeExternalDimensionTable reads and validates one table of a dimension.
func DecodeExternalDimensionTable(source string, reader io.Reader, configuration project.Project) (ExternalDimensionTable, error) {
	var value ExternalDimensionTable
	if err := decodeStrict(source, reader, &value); err != nil {
		return ExternalDimensionTable{}, err
	}
	issues := validateBase(value.Format, value.ID, value.Name, value.Title, configuration)
	issues = append(issues, validatePresentations(value.Presentations, configuration)...)
	issues = append(issues, validateRequiredNameInDataSource("name_in_data_source", value.NameInDataSource, "the dimension table")...)
	issues = append(issues, validateNameInDataSource("hierarchy_name_in_data_source", value.HierarchyNameInDataSource)...)
	if value.LevelNumber < 0 {
		issues = append(issues, "level_number must not be negative")
	}
	// A hierarchical table keeps the level it was saved with: the designer
	// saves 1 there (mdclasses, 8.3.25 and 8.3.27), while the help says such a
	// table answers 0. Both are true at once - the number written and the
	// number the application sees - so the first is carried and the second is
	// EffectiveLevelNumber.
	//
	// The value of an unfilled parent has no ceiling: the help types it as a
	// string or Null and names none.
	// Without a hierarchy the value is never read; a table of a source keeps
	// it there too (mdclasses), so it is carried and is a note.
	issues = append(issues, validateObjectCommands(value.Commands, configuration)...)
	issues = append(issues, validateObjectTemplates(value.Templates, configuration)...)
	issues = append(issues, validateExternalFieldGroups(configuration, externalFieldGroup{"fields", value.Fields})...)
	if value.PresentationField != "" && !slices.ContainsFunc(value.Fields, func(field ExternalField) bool {
		return strings.EqualFold(field.Name, value.PresentationField)
	}) {
		issues = append(issues, fmt.Sprintf("presentation_field names %q, which is not a field of this dimension table", value.PresentationField))
	}
	if err := issuesError(source, value.Format, issues); err != nil {
		return ExternalDimensionTable{}, err
	}
	return value, nil
}

// validateCubeFields checks the dimensions and the resources of a cube. They
// share one namespace: a query names either as a field of the cube, and two of
// one name would be one field read twice.
func validateCubeFields(cube ExternalCube, configuration project.Project) []string {
	var issues []string
	names, ids := map[string]bool{}, map[uuid.UUID]bool{}
	check := func(prefix string, field Attribute) {
		if field.ID.IsZero() {
			issues = append(issues, prefix+".id must be a non-zero UUID")
		} else if ids[field.ID] {
			issues = append(issues, prefix+".id is used twice")
		}
		ids[field.ID] = true
		if !validIdentifier(field.Name) || utf8.RuneCountInString(field.Name) > maxNameLength {
			issues = append(issues, prefix+".name must start with a letter, contain only letters or digits and not exceed 255 characters")
		} else if names[strings.ToLower(field.Name)] {
			issues = append(issues, prefix+".name is used twice")
		}
		names[strings.ToLower(field.Name)] = true
		issues = append(issues, validateTitle(prefix+".title", field.Title, configuration)...)
		issues = append(issues, validateExternalFieldTypes(prefix+".types", field.Types)...)
		issues = append(issues, validateFieldSettings(prefix, field, configuration)...)
		issues = append(issues, validateFieldStorage(prefix, field)...)
		for _, property := range []struct {
			set  bool
			name string
		}{
			{field.Indexing != "", "indexing"},
			{field.FullTextSearch != "", "full_text_search"},
			{field.DataHistory != "", "data_history"},
			{field.Use != "", "use"},
		} {
			if property.set {
				issues = append(issues, prefix+"."+property.name+" belongs to an attribute of the configuration's own objects, and a field of a cube has none")
			}
		}
	}
	dimensions := make([]Attribute, 0, len(cube.Dimensions))
	for index, dimension := range cube.Dimensions {
		check(fmt.Sprintf("dimensions[%d]", index), dimension.Attribute)
		dimensions = append(dimensions, dimension.Attribute)
	}
	resources := make([]Attribute, 0, len(cube.Resources))
	for index, resource := range cube.Resources {
		prefix := fmt.Sprintf("resources[%d]", index)
		check(prefix, resource.Attribute)
		issues = append(issues, validateRequiredNameInDataSource(prefix+".name_in_data_source", resource.NameInDataSource, "the measure")...)
		for _, property := range []struct {
			set  bool
			name string
		}{
			{resource.Filling.Value != nil || resource.Filling.FromFillingValue, "filling"},
			{resource.FillChecking != "", "fill_checking"},
			{resource.Choice.CreateOnInput != "", "choice.create_on_input"},
			{resource.Choice.HistoryOnInput != "", "choice.history_on_input"},
			{resource.Presentation.MinValue != nil, "presentation.min_value"},
			{resource.Presentation.MaxValue != nil, "presentation.max_value"},
			{resource.Choice.FoldersAndItems != "", "choice.folders_and_items"},
			{resource.Choice.LinkByType != nil, "choice.link_by_type"},
		} {
			if property.set {
				issues = append(issues, prefix+"."+property.name+" belongs to a field somebody enters, and a resource of a cube is a measured value nobody does")
			}
		}
		resources = append(resources, resource.Attribute)
	}
	return append(issues, validateFieldLinks(nil, []fieldGroup{{"dimensions", dimensions}, {"resources", resources}}, nil)...)
}

// cubeFields is the dimensions and the resources of a cube as fields of a
// source, for the resolution of what their types refer to.
func cubeFields(cube ExternalCube) []ExternalField {
	fields := make([]ExternalField, 0, len(cube.Dimensions)+len(cube.Resources))
	for _, dimension := range cube.Dimensions {
		fields = append(fields, ExternalField{Attribute: dimension.Attribute})
	}
	for _, resource := range cube.Resources {
		fields = append(fields, ExternalField{Attribute: resource.Attribute, NameInDataSource: resource.NameInDataSource})
	}
	return fields
}

// validateRequiredNameInDataSource checks a name of the other database that has
// to be there: a cube and a dimension table are nothing but a name of the
// other database's object.
// validateRequiredNameInDataSource checks a name in the source the help asks
// for. Left empty it is carried and is a note, as on a table: the configurator
// saves without it.
func validateRequiredNameInDataSource(path, value, what string) []string {
	return validateNameInDataSource(path, value)
}

func cloneExternalFields(fields []ExternalField) []ExternalField {
	if fields == nil {
		return nil
	}
	result := make([]ExternalField, len(fields))
	for index, field := range fields {
		result[index] = cloneExternalField(field)
	}
	return result
}

// EffectiveLevelNumber is the level the application sees. The help: a
// hierarchical table stands for the whole hierarchy and its level is 0,
// whatever number the designer saved with it. Everything that reads the level
// at run time reads it here, not from LevelNumber.
func (table ExternalDimensionTable) EffectiveLevelNumber() int {
	if table.Hierarchical {
		return 0
	}
	return table.LevelNumber
}

func cloneExternalDimensionTable(table ExternalDimensionTable) ExternalDimensionTable {
	table.Title = cloneTitle(table.Title)
	table.Presentations = clonePresentations(table.Presentations)
	if table.UnfilledParentValue != nil {
		value := *table.UnfilledParentValue
		table.UnfilledParentValue = &value
	}
	table.Fields = cloneExternalFields(table.Fields)
	table.Commands = cloneObjectCommands(table.Commands)
	table.Templates = cloneObjectTemplates(table.Templates)
	return table
}

func cloneExternalCube(cube ExternalCube) ExternalCube {
	cube.Title = cloneTitle(cube.Title)
	cube.ListPresentation = cloneTitle(cube.ListPresentation)
	cube.ExtendedListPresentation = cloneTitle(cube.ExtendedListPresentation)
	cube.Explanation = cloneTitle(cube.Explanation)
	cube.RecordPresentation = cloneTitle(cube.RecordPresentation)
	cube.ExtendedRecordPresentation = cloneTitle(cube.ExtendedRecordPresentation)
	if cube.Dimensions != nil {
		dimensions := make([]ExternalCubeDimension, len(cube.Dimensions))
		for index, dimension := range cube.Dimensions {
			dimensions[index] = ExternalCubeDimension{Attribute: cloneAttribute(dimension.Attribute)}
		}
		cube.Dimensions = dimensions
	}
	if cube.Resources != nil {
		resources := make([]ExternalCubeResource, len(cube.Resources))
		for index, resource := range cube.Resources {
			resources[index] = ExternalCubeResource{Attribute: cloneAttribute(resource.Attribute), NameInDataSource: resource.NameInDataSource}
		}
		cube.Resources = resources
	}
	cube.Characteristics = cloneObjectCharacteristics(cube.Characteristics)
	cube.Commands = cloneObjectCommands(cube.Commands)
	cube.Templates = cloneObjectTemplates(cube.Templates)
	tables := make([]ExternalDimensionTable, len(cube.DimensionTables))
	for index, table := range cube.DimensionTables {
		tables[index] = cloneExternalDimensionTable(table)
	}
	cube.DimensionTables = tables
	return cube
}

// loadExternalCubes reads the cubes of one source from its folder of cubes,
// and each cube's dimension tables from the cube's folder of them.
//
// What else lies in a cube's folder and a dimension table's - modules, forms,
// commands, templates - is checked the way any object folder is, by
// validateExternalCubeFiles once the whole configuration is read.
func loadExternalCubes(sourceDirectory, relative string, configuration project.Project) ([]ExternalCube, error) {
	directories, err := subordinateFolders(filepath.Join(sourceDirectory, ExternalDataSourceCubesDirectory),
		relative+"/"+ExternalDataSourceCubesDirectory, "a cube")
	if err != nil {
		return nil, err
	}
	cubes := make([]ExternalCube, 0, len(directories))
	for _, name := range directories {
		where := relative + "/" + ExternalDataSourceCubesDirectory + "/" + name
		directory := filepath.Join(sourceDirectory, ExternalDataSourceCubesDirectory, name)
		cube, err := decodeFolderObject(directory, where, name, "cube", func(source string, reader io.Reader) (ExternalCube, string, error) {
			value, err := DecodeExternalCube(source, reader, configuration)
			return value, value.Name, err
		})
		if err != nil {
			return nil, err
		}
		tableNames, err := subordinateFolders(filepath.Join(directory, ExternalDimensionTablesDirectory),
			where+"/"+ExternalDimensionTablesDirectory, "a dimension table")
		if err != nil {
			return nil, err
		}
		for _, tableName := range tableNames {
			table, err := decodeFolderObject(filepath.Join(directory, ExternalDimensionTablesDirectory, tableName),
				where+"/"+ExternalDimensionTablesDirectory+"/"+tableName, tableName, "dimension table",
				func(source string, reader io.Reader) (ExternalDimensionTable, string, error) {
					value, err := DecodeExternalDimensionTable(source, reader, configuration)
					return value, value.Name, err
				})
			if err != nil {
				return nil, err
			}
			cube.DimensionTables = append(cube.DimensionTables, table)
		}
		cubes = append(cubes, cube)
	}
	return cubes, nil
}

// subordinateFolders lists the folders of one collection of subordinate
// objects, refusing anything that is not a folder named as an object is. A
// collection that is not there holds nothing.
func subordinateFolders(directory, relative string, what string) ([]string, error) {
	entries, err := os.ReadDir(directory)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		where := relative + "/" + entry.Name()
		if !entry.IsDir() || entry.Type()&fs.ModeSymlink != 0 {
			return nil, fmt.Errorf("unexpected metadata source %q: %s is a folder", where, what)
		}
		if err := project.ObjectName(entry.Name()); err != nil {
			return nil, fmt.Errorf("folder %q: %w", where, err)
		}
		names = append(names, entry.Name())
	}
	return names, nil
}

// decodeFolderObject reads the description in one object's folder and checks
// that the object is named as its folder is.
func decodeFolderObject[T any](directory, relative, folder, what string, decode func(string, io.Reader) (T, string, error)) (T, error) {
	var zero T
	source := relative + "/" + project.ObjectMetadataFile
	file, err := os.Open(filepath.Join(directory, project.ObjectMetadataFile))
	if err != nil {
		return zero, fmt.Errorf("open %s: %w", source, err)
	}
	value, name, decodeErr := decode(source, file)
	file.Close()
	if decodeErr != nil {
		return zero, decodeErr
	}
	if !strings.EqualFold(name, folder) {
		return zero, fmt.Errorf("%s %s lies in a folder called %s", what, name, folder)
	}
	return value, nil
}

// validateExternalCubeFiles checks the folders of every cube and dimension
// table the way any object folder is checked. A cube is read as records, so it
// keeps a record set module; a dimension table's members are things, so it
// keeps an object module; both keep a manager module. A cube's folder keeps
// the folder of its dimension tables beside all that.
func (catalog *Catalog) validateExternalCubeFiles(root string) error {
	for _, source := range catalog.ExternalDataSources {
		for _, cube := range source.Cubes {
			name := source.Name + "." + cube.Name
			if err := catalog.validateObjectFileSources(objectFiles{root: root, directoryKind: ExternalCubeKind,
				kind: "external data source cube", name: name, modules: recordSetKindModules,
				formSlots: cube.Forms.slots(), commands: cube.Commands, templates: cube.Templates}); err != nil {
				return err
			}
			for _, table := range cube.DimensionTables {
				if err := catalog.validateObjectFileSources(objectFiles{root: root, directoryKind: ExternalDimensionTableKind,
					kind: "external data source dimension table", name: name + "." + table.Name, modules: objectKindModules,
					formSlots: table.Forms.slots(), commands: table.Commands, templates: table.Templates}); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

// ExternalCube returns one cube of a source by the names of both, folded case.
func (catalog *Catalog) ExternalCube(source, cube string) (ExternalCube, bool) {
	index, ok := catalog.externalDataSourceByName[strings.ToLower(source)]
	if !ok {
		return ExternalCube{}, false
	}
	for _, item := range catalog.ExternalDataSources[index].Cubes {
		if strings.EqualFold(item.Name, cube) {
			return cloneExternalCube(item), true
		}
	}
	return ExternalCube{}, false
}
