package metadata

import (
	"fmt"
	"io"
	"slices"
	"sort"
	"strings"

	"github.com/k33alexey/MetaLab/internal/project"
	"github.com/k33alexey/MetaLab/internal/uuid"
)

const CommonAttributeKind Kind = "common-attributes"

// CommonAttributeDefinition is physically propagated as an ordinary Attribute
// into every object it targets (the 1С 7.7 model), not composed as a virtual
// layer above objects. Propagation happens once, in Catalog.indexAndValidate,
// so schema generation, BSL property dispatch and forms all see it exactly
// like a natively declared attribute.
type CommonAttributeDefinition struct {
	Format       int           `yaml:"format"`
	ID           uuid.UUID     `yaml:"id"`
	Name         string        `yaml:"name"`
	Title        LocalizedText `yaml:"title"`
	Types        []Type        `yaml:"types"`
	FillChecking FillCheck     `yaml:"fill_checking,omitempty"`
	Indexing     IndexMode     `yaml:"indexing,omitempty"`
	// FullTextSearch is the field's own flag, which a common attribute carries
	// like any other field - see full_text_search.go. The prototype gives it to
	// a common attribute and the demonstration configuration writes it on all
	// six of them.
	FullTextSearch FullTextSearchMode `yaml:"full_text_search,omitempty"`
	// DataHistory is the field's half of data history, which a common
	// attribute carries the same way - see data_history.go. Only the
	// participation flag: the two booleans beside it belong to the object,
	// not to a field. The demonstration configuration writes it on all six
	// common attributes, and on all six it is Использовать.
	DataHistory DataHistoryMode `yaml:"data_history,omitempty"`
	// Comment, Presentation, Choice and Filling are the rest of what any field
	// carries. ТЗ asks for «полный набор свойств реквизита» here, and it asks
	// for a reason: a common attribute becomes an ordinary attribute of every
	// object it targets, so a format or a choice form it cannot hold is a
	// format or a choice form none of those objects get. All six common
	// attributes of the demonstration configuration carry them.
	Comment      string            `yaml:"comment,omitempty"`
	Presentation FieldPresentation `yaml:"presentation,omitempty"`
	Choice       FieldChoice       `yaml:"choice,omitempty"`
	Filling      FieldFilling      `yaml:"filling,omitempty"`
	Objects      []uuid.UUID       `yaml:"objects"`
}

func DecodeCommonAttribute(source string, reader io.Reader, configuration project.Project) (CommonAttributeDefinition, error) {
	var value CommonAttributeDefinition
	if err := decodeStrict(source, reader, &value); err != nil {
		return CommonAttributeDefinition{}, err
	}
	if err := ValidateCommonAttribute(source, value, configuration); err != nil {
		return CommonAttributeDefinition{}, err
	}
	return value, nil
}

// ValidateCommonAttribute checks structure only; object existence, object
// kind and name collisions are checked catalog-wide by propagateCommonAttributes.
func ValidateCommonAttribute(source string, value CommonAttributeDefinition, configuration project.Project) error {
	issues := validateBase(value.Format, value.ID, value.Name, value.Title, configuration)
	issues = append(issues, validateFullTextSearch("full_text_search", value.FullTextSearch)...)
	issues = append(issues, validateDataHistory(DataHistorySettings{DataHistory: value.DataHistory})...)
	issues = append(issues, validateTypes("types", value.Types, uuid.UUID{})...)
	if !validFillCheck(value.FillChecking) {
		issues = append(issues, "fill_checking must be dont-check or show-error")
	}
	if !validIndexMode(value.Indexing) {
		issues = append(issues, "indexing must be dont-index, index or index-with-additional-order")
	}
	// The settings of the field are checked the way any field's are. The paths
	// carry no prefix: the file is the field, there is no field of an object to
	// name. A bound of another type than the field is compared with nothing and
	// rejects nothing, and it would reject nothing in every object the attribute
	// is propagated into.
	issues = append(issues, validateValueSettings(Attribute{
		Types: value.Types, Presentation: value.Presentation, Choice: value.Choice,
	}, configuration)...)
	if filling := value.Filling.Value; filling != nil {
		issues = append(issues, validateFieldBound("filling.value", *filling, value.Types)...)
	}
	// A common attribute is declared on its own, away from the objects it will
	// join, so there are no sibling fields for a choice parameter link to take
	// its value from - and which siblings it would have depends on the object,
	// which is exactly why the prototype does not let it draw one.
	issues = append(issues, validateFieldLinks(nil, nil, choiceHolder{"", value.Choice})...)
	if len(value.Objects) == 0 {
		issues = append(issues, "objects must list at least one target object")
	}
	seen := make(map[uuid.UUID]bool, len(value.Objects))
	for index, target := range value.Objects {
		prefix := fmt.Sprintf("objects[%d]", index)
		if target.IsZero() {
			issues = append(issues, prefix+" must be a non-zero UUID")
		}
		if seen[target] {
			issues = append(issues, prefix+" must be unique")
		}
		seen[target] = true
	}
	return issuesError(source, value.Format, issues)
}

func (catalog *Catalog) CommonAttribute(name string) (CommonAttributeDefinition, bool) {
	index, ok := catalog.commonAttributeByName[strings.ToLower(name)]
	if !ok {
		return CommonAttributeDefinition{}, false
	}
	return cloneCommonAttributeDefinition(catalog.CommonAttributes[index]), true
}

func (catalog *Catalog) CommonAttributeByID(id uuid.UUID) (CommonAttributeDefinition, bool) {
	index, ok := catalog.commonAttributeByID[id]
	if !ok {
		return CommonAttributeDefinition{}, false
	}
	return cloneCommonAttributeDefinition(catalog.CommonAttributes[index]), true
}

func cloneCommonAttributeDefinition(value CommonAttributeDefinition) CommonAttributeDefinition {
	value.Title, value.Types = cloneTitle(value.Title), cloneTypes(value.Types)
	value.Objects = slices.Clone(value.Objects)
	settings := cloneFieldFilling(cloneFieldSettings(Attribute{
		Presentation: value.Presentation, Choice: value.Choice, Filling: value.Filling,
	}))
	value.Presentation, value.Choice, value.Filling = settings.Presentation, settings.Choice, settings.Filling
	return value
}

// commonAttributeTarget locates one object a common attribute can be
// propagated into: only kinds that already carry an Attributes list.
type commonAttributeTarget struct {
	kind  string
	index int
}

// propagateCommonAttributes physically appends every common attribute to the
// Attributes list of each object it targets. It must run before any other
// indexing in Catalog.indexAndValidate so downstream validation, schema
// generation and BSL property dispatch all see the final, propagated shape.
func (catalog *Catalog) propagateCommonAttributes() error {
	if len(catalog.CommonAttributes) == 0 {
		return nil
	}
	sort.Slice(catalog.CommonAttributes, func(i, j int) bool {
		return catalog.CommonAttributes[i].ID.String() < catalog.CommonAttributes[j].ID.String()
	})
	targets := make(map[uuid.UUID]commonAttributeTarget, len(catalog.Catalogs)+len(catalog.Documents)+len(catalog.InformationRegisters)+len(catalog.AccumulationRegisters))
	for index, item := range catalog.Catalogs {
		targets[item.ID] = commonAttributeTarget{"catalog", index}
	}
	for index, item := range catalog.Documents {
		targets[item.ID] = commonAttributeTarget{"document", index}
	}
	for index, item := range catalog.InformationRegisters {
		targets[item.ID] = commonAttributeTarget{"information register", index}
	}
	for index, item := range catalog.AccumulationRegisters {
		targets[item.ID] = commonAttributeTarget{"accumulation register", index}
	}
	for _, common := range catalog.CommonAttributes {
		attribute := Attribute{
			ID: common.ID, Name: common.Name, Title: cloneTitle(common.Title), Types: cloneTypes(common.Types),
			FillChecking: common.FillChecking, Indexing: common.Indexing,
			// A common attribute becomes an ordinary attribute of every object
			// it targets, so both field flags have to travel with it: kept on
			// the definition alone they would describe a field nothing sees.
			// Each is the field's two-valued pair written in the three-valued
			// type an attribute uses - see attribute_storage.go, which refuses
			// the third value for exactly these two.
			FullTextSearch: UsageMode(common.FullTextSearch),
			DataHistory:    UsageMode(common.DataHistory),
			// The rest of the field travels with it for the same reason: a
			// format kept on the definition alone formats nothing.
			Comment: common.Comment, Presentation: common.Presentation,
			Choice: common.Choice, Filling: common.Filling,
		}
		for _, objectID := range common.Objects {
			location, ok := targets[objectID]
			if !ok {
				return fmt.Errorf("common attribute %s references unknown object %s", common.Name, objectID)
			}
			name, names, ids := catalog.objectFieldName(location), catalog.objectFieldNames(location), catalog.objectFieldIDs(location)
			if ids[common.ID] {
				// Already propagated: indexAndValidate may run again on already
				// validated data (for example RuntimeSnapshot round-trips), and
				// propagation must be idempotent rather than duplicate the field.
				continue
			}
			if names[strings.ToLower(common.Name)] {
				return fmt.Errorf("common attribute %s collides with an existing field of %s %s", common.Name, location.kind, name)
			}
			// A copy per object, not the one attribute in all of them. The
			// attribute is built once outside the loop, so every object it
			// joins would otherwise share the very same maps - its title, its
			// format, its choice parameters. Nothing changes a propagated
			// attribute in place today, so nothing has gone wrong yet; the copy
			// is here so that the first thing that does change one changes it
			// in one object rather than in all of them at once.
			catalog.appendPropagatedAttribute(location, cloneAttribute(attribute))
		}
	}
	return nil
}

func (catalog *Catalog) objectFieldName(location commonAttributeTarget) string {
	switch location.kind {
	case "catalog":
		return catalog.Catalogs[location.index].Name
	case "document":
		return catalog.Documents[location.index].Name
	case "information register":
		return catalog.InformationRegisters[location.index].Name
	case "accumulation register":
		return catalog.AccumulationRegisters[location.index].Name
	}
	return ""
}

func (catalog *Catalog) objectFields(location commonAttributeTarget) []Attribute {
	var fields []Attribute
	switch location.kind {
	case "catalog":
		definition := catalog.Catalogs[location.index]
		fields = append(fields, definition.Attributes...)
		for _, part := range definition.TableParts {
			fields = append(fields, Attribute{Name: part.Name})
		}
	case "document":
		definition := catalog.Documents[location.index]
		fields = append(fields, definition.Attributes...)
		for _, part := range definition.TableParts {
			fields = append(fields, Attribute{Name: part.Name})
		}
	case "information register":
		definition := catalog.InformationRegisters[location.index]
		fields = append(fields, definition.Attributes...)
		fields = append(fields, definition.Dimensions...)
		fields = append(fields, definition.Resources...)
	case "accumulation register":
		definition := catalog.AccumulationRegisters[location.index]
		fields = append(fields, definition.Attributes...)
		fields = append(fields, definition.Dimensions...)
		fields = append(fields, definition.Resources...)
	}
	return fields
}

func (catalog *Catalog) objectFieldNames(location commonAttributeTarget) map[string]bool {
	used := make(map[string]bool)
	for _, field := range catalog.objectFields(location) {
		used[strings.ToLower(field.Name)] = true
	}
	return used
}

// objectFieldIDs reports the IDs already present among an object's fields,
// used to make propagation idempotent when indexAndValidate runs again on
// already-propagated data (table part pseudo-entries carry a zero ID and are
// intentionally absent here).
func (catalog *Catalog) objectFieldIDs(location commonAttributeTarget) map[uuid.UUID]bool {
	used := make(map[uuid.UUID]bool)
	for _, field := range catalog.objectFields(location) {
		if !field.ID.IsZero() {
			used[field.ID] = true
		}
	}
	return used
}

func (catalog *Catalog) appendPropagatedAttribute(location commonAttributeTarget, attribute Attribute) {
	switch location.kind {
	case "catalog":
		catalog.Catalogs[location.index].Attributes = append(catalog.Catalogs[location.index].Attributes, attribute)
	case "document":
		catalog.Documents[location.index].Attributes = append(catalog.Documents[location.index].Attributes, attribute)
	case "information register":
		catalog.InformationRegisters[location.index].Attributes = append(catalog.InformationRegisters[location.index].Attributes, attribute)
	case "accumulation register":
		catalog.AccumulationRegisters[location.index].Attributes = append(catalog.AccumulationRegisters[location.index].Attributes, attribute)
	}
}
