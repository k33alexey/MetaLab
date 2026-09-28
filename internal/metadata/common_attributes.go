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
	// AutoUse decides the objects the composition says nothing definite about:
	// those it does not mention at all, and those it marks «auto». The
	// prototype's default is not to use them, and that is the safe way round -
	// an object nobody spoke about does not silently grow a column.
	AutoUse CommonAttributeAutoUse `yaml:"auto_use,omitempty"`
	// Content is the composition, and it is not a list of the objects that get
	// the attribute: it is a list of objects with a verdict on each. That
	// difference is the whole of it. Of the six common attributes of the
	// demonstration configuration five are «only these, explicitly», and the
	// sixth - the separator of the main data area - is the other way round:
	// auto-use on, and all 331 items marked «do not use», so the composition
	// works as a list of exceptions. Read as a list of the included, it would
	// give the attribute to precisely the objects that must not have it.
	Content []CommonAttributeContentItem `yaml:"content,omitempty"`
}

// CommonAttributeUse is the verdict of one item of the composition.
type CommonAttributeUse string

const (
	CommonAttributeUseAuto    CommonAttributeUse = "auto"
	CommonAttributeUseUse     CommonAttributeUse = "use"
	CommonAttributeUseDontUse CommonAttributeUse = "dont-use"
)

// CommonAttributeAutoUse is what «auto» means for this attribute.
type CommonAttributeAutoUse string

const (
	CommonAttributeAutoUseDontUse CommonAttributeAutoUse = "dont-use"
	CommonAttributeAutoUseUse     CommonAttributeAutoUse = "use"
)

// CommonAttributeContentItem is one object of the composition and what the
// attribute does with it.
type CommonAttributeContentItem struct {
	Metadata uuid.UUID          `yaml:"metadata"`
	Use      CommonAttributeUse `yaml:"use,omitempty"`
	// ConditionalSeparation is the condition under which the attribute
	// separates this object's data. It is part of the composition and not of
	// the attribute, because the condition is per object - see
	// ConditionalSeparation. We carry it and do not act on it: separation of
	// data is not ours, and the reason is in METADATA-OBJECTS.md.
	ConditionalSeparation *ConditionalSeparation `yaml:"conditional_separation,omitempty"`
}

// ConditionalSeparation names the boolean that decides whether the attribute
// separates one object's data: either a constant, or an attribute of an object.
//
// Both must stand outside the composition, and that is the prototype's rule,
// not our caution: a condition that lived on a separated object would have to
// be read to decide whether to separate it, and reading it needs the answer.
type ConditionalSeparation struct {
	Constant *uuid.UUID `yaml:"constant,omitempty"`
	// Object and Attribute are named together: an attribute of an object, and
	// the object is needed because the condition may live anywhere.
	Object    *uuid.UUID `yaml:"object,omitempty"`
	Attribute *uuid.UUID `yaml:"attribute,omitempty"`
}

func validCommonAttributeUse(value CommonAttributeUse) bool {
	switch value {
	case "", CommonAttributeUseAuto, CommonAttributeUseUse, CommonAttributeUseDontUse:
		return true
	default:
		return false
	}
}

func validCommonAttributeAutoUse(value CommonAttributeAutoUse) bool {
	switch value {
	case "", CommonAttributeAutoUseDontUse, CommonAttributeAutoUseUse:
		return true
	default:
		return false
	}
}

// reaches says whether this object gets the attribute. An object the
// composition mentions is decided by its item; one it does not mention is
// decided by auto-use, the same as an item that says «auto».
func (value CommonAttributeDefinition) reaches(objectID uuid.UUID) bool {
	for _, item := range value.Content {
		if item.Metadata != objectID {
			continue
		}
		switch item.Use {
		case CommonAttributeUseUse:
			return true
		case CommonAttributeUseDontUse:
			return false
		default:
			return value.AutoUse == CommonAttributeAutoUseUse
		}
	}
	return value.AutoUse == CommonAttributeAutoUseUse
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
	if !validCommonAttributeAutoUse(value.AutoUse) {
		issues = append(issues, "auto_use must be use or dont-use")
	}
	seen := make(map[uuid.UUID]bool, len(value.Content))
	reaches := value.AutoUse == CommonAttributeAutoUseUse
	for index, item := range value.Content {
		prefix := fmt.Sprintf("content[%d]", index)
		if item.Metadata.IsZero() {
			issues = append(issues, prefix+".metadata must be a non-zero UUID")
		}
		if seen[item.Metadata] {
			issues = append(issues, prefix+".metadata must be unique")
		}
		seen[item.Metadata] = true
		if !validCommonAttributeUse(item.Use) {
			issues = append(issues, prefix+".use must be auto, use or dont-use")
		}
		if item.Use == CommonAttributeUseUse {
			reaches = true
		}
		issues = append(issues, validateConditionalSeparation(prefix, item.ConditionalSeparation, seen)...)
	}
	// An attribute that reaches nobody is a field declared and given to no
	// object: the setting reads as working and does nothing.
	if !reaches {
		issues = append(issues, "the composition reaches no object: either list an object with use or set auto_use to use")
	}
	return issuesError(source, value.Format, issues)
}

// validateConditionalSeparation checks the condition of one item: it names one
// boolean and not two, and what it names stands outside the composition.
//
// Outside the composition is the prototype's rule and it is not a formality: a
// condition kept on a separated object would have to be read to decide whether
// that object is separated, and reading it needs the answer. `inside` holds the
// objects the composition has named so far, which is why the items are checked
// in order.
func validateConditionalSeparation(prefix string, separation *ConditionalSeparation, inside map[uuid.UUID]bool) []string {
	if separation == nil {
		return nil
	}
	path := prefix + ".conditional_separation"
	var issues []string
	named := 0
	if separation.Constant != nil {
		named++
		if separation.Constant.IsZero() {
			issues = append(issues, path+".constant must be a non-zero UUID")
		}
	}
	switch {
	case separation.Object != nil && separation.Attribute != nil:
		named++
		if separation.Object.IsZero() || separation.Attribute.IsZero() {
			issues = append(issues, path+".object and .attribute must be non-zero UUIDs")
		}
		if inside[*separation.Object] {
			issues = append(issues, path+".object is in the composition: the condition would have to be read to decide whether to read it")
		}
	case separation.Object != nil || separation.Attribute != nil:
		issues = append(issues, path+" names an object without an attribute or the other way round")
	}
	switch named {
	case 1:
	case 0:
		issues = append(issues, path+" must name a constant or an attribute of an object")
	default:
		issues = append(issues, path+" names both a constant and an attribute, and a condition is one boolean")
	}
	if separation.Constant != nil && inside[*separation.Constant] {
		issues = append(issues, path+".constant is in the composition")
	}
	return issues
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
	value.Content = slices.Clone(value.Content)
	for index := range value.Content {
		if separation := value.Content[index].ConditionalSeparation; separation != nil {
			copied := *separation
			for _, field := range []**uuid.UUID{&copied.Constant, &copied.Object, &copied.Attribute} {
				if *field != nil {
					id := **field
					*field = &id
				}
			}
			value.Content[index].ConditionalSeparation = &copied
		}
	}
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
	// The kinds that can take a common attribute, in a fixed order. The order
	// matters now in a way it did not before: with auto-use the attribute
	// reaches objects the composition never mentions, so propagation walks the
	// objects rather than the list, and walking a map would put the field in a
	// different place on every run.
	//
	// Which kinds these are is the prototype's list, checked in its
	// configurator: catalogs, documents, document journals, information
	// registers, accumulation registers, business processes, tasks. Registers of
	// accounting and of calculation are not among them. The document journal is
	// missing here and has a point of its own: it keeps columns rather than
	// attributes, and where the field lands in one is not yet known.
	var ordered []commonAttributeTarget
	targets := map[uuid.UUID]commonAttributeTarget{}
	add := func(id uuid.UUID, kind string, index int) {
		location := commonAttributeTarget{kind, index}
		targets[id] = location
		ordered = append(ordered, location)
	}
	for index, item := range catalog.Catalogs {
		add(item.ID, "catalog", index)
	}
	for index, item := range catalog.Documents {
		add(item.ID, "document", index)
	}
	for index, item := range catalog.InformationRegisters {
		add(item.ID, "information register", index)
	}
	for index, item := range catalog.AccumulationRegisters {
		add(item.ID, "accumulation register", index)
	}
	for index, item := range catalog.BusinessProcesses {
		add(item.ID, "business process", index)
	}
	for index, item := range catalog.Tasks {
		add(item.ID, "task", index)
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
		// An item naming an object of a kind that cannot take a common attribute
		// - or no object at all - is caught before anything is propagated.
		for _, item := range common.Content {
			if _, ok := targets[item.Metadata]; !ok {
				return fmt.Errorf("common attribute %s references unknown object %s", common.Name, item.Metadata)
			}
		}
		for _, location := range ordered {
			if !common.reaches(catalog.objectID(location)) {
				continue
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
	case "business process":
		return catalog.BusinessProcesses[location.index].Name
	case "task":
		return catalog.Tasks[location.index].Name
	}
	return ""
}

func (catalog *Catalog) objectID(location commonAttributeTarget) uuid.UUID {
	switch location.kind {
	case "catalog":
		return catalog.Catalogs[location.index].ID
	case "document":
		return catalog.Documents[location.index].ID
	case "information register":
		return catalog.InformationRegisters[location.index].ID
	case "accumulation register":
		return catalog.AccumulationRegisters[location.index].ID
	case "business process":
		return catalog.BusinessProcesses[location.index].ID
	case "task":
		return catalog.Tasks[location.index].ID
	}
	return uuid.UUID{}
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
	case "business process":
		definition := catalog.BusinessProcesses[location.index]
		fields = append(fields, definition.Attributes...)
		for _, part := range definition.TableParts {
			fields = append(fields, Attribute{Name: part.Name})
		}
	case "task":
		definition := catalog.Tasks[location.index]
		fields = append(fields, definition.Attributes...)
		for _, part := range definition.TableParts {
			fields = append(fields, Attribute{Name: part.Name})
		}
		// The addressing attributes are fields of the task as well, and a name
		// taken by one of them is taken.
		for _, addressing := range definition.AddressingAttributes {
			fields = append(fields, Attribute{ID: addressing.ID, Name: addressing.Name})
		}
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
	case "business process":
		catalog.BusinessProcesses[location.index].Attributes = append(catalog.BusinessProcesses[location.index].Attributes, attribute)
	case "task":
		catalog.Tasks[location.index].Attributes = append(catalog.Tasks[location.index].Attributes, attribute)
	}
}
