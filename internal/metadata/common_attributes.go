package metadata

import (
	"fmt"
	"io"
	"reflect"
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
	// The eight properties of data separation. In the prototype the same
	// metadata object carries a second, unrelated mechanism: one database shared
	// between independent companies, each with its own users and authentication,
	// told apart by the value of this attribute. We have one company per
	// database and do not do it - **but we carry all eight**, because a property
	// missing from the model disappears on import, and that is a loss of
	// meaning, while a property carried and not executed imports whole and the
	// report says «carried, not implemented». The reason is in
	// METADATA-OBJECTS.md.
	//
	// The two session parameters are how the platform learns which area it is
	// in: one holds the value of the separator for this session, the other says
	// whether separation is in force. Both separators of the demonstration
	// configuration name both.
	DataSeparation                    SeparationMode   `yaml:"data_separation,omitempty"`
	SeparatedDataUse                  SeparatedDataUse `yaml:"separated_data_use,omitempty"`
	DataSeparationValue               *uuid.UUID       `yaml:"data_separation_value,omitempty"`
	DataSeparationUse                 *uuid.UUID       `yaml:"data_separation_use,omitempty"`
	UsersSeparation                   SeparationMode   `yaml:"users_separation,omitempty"`
	AuthenticationSeparation          SeparationMode   `yaml:"authentication_separation,omitempty"`
	ConfigurationExtensionsSeparation SeparationMode   `yaml:"configuration_extensions_separation,omitempty"`
	// ConditionalSeparation is the eighth: a boolean constant whose value says
	// whether the attribute separates anything in this session. It is the
	// attribute's own, beside the condition an item of the composition may
	// carry for one object, and the help gives it a constant only. All three
	// configurations being moved set it on their two separators. Carried, not
	// executed, for the reason above.
	ConditionalSeparation *uuid.UUID `yaml:"conditional_separation,omitempty"`
}

// SeparationMode is the two-valued answer four of the eight give: separate by
// this attribute, or do not.
type SeparationMode string

const (
	SeparationDontUse  SeparationMode = "dont-use"
	SeparationSeparate SeparationMode = "separate"
)

// SeparatedDataUse is the level of separation, and it has no «do not» among its
// values: the prototype writes «independently» even on an attribute that
// separates nothing, which is why an empty value here means nothing was said
// rather than nothing is done.
type SeparatedDataUse string

const (
	SeparatedDataIndependently                  SeparatedDataUse = "independently"
	SeparatedDataIndependentlyAndSimultaneously SeparatedDataUse = "independently-and-simultaneously"
)

func validSeparationMode(value SeparationMode) bool {
	switch value {
	case "", SeparationDontUse, SeparationSeparate:
		return true
	default:
		return false
	}
}

func validSeparatedDataUse(value SeparatedDataUse) bool {
	switch value {
	case "", SeparatedDataIndependently, SeparatedDataIndependentlyAndSimultaneously:
		return true
	default:
		return false
	}
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
	for name, mode := range map[string]SeparationMode{
		"data_separation": value.DataSeparation, "users_separation": value.UsersSeparation,
		"authentication_separation":           value.AuthenticationSeparation,
		"configuration_extensions_separation": value.ConfigurationExtensionsSeparation,
	} {
		if !validSeparationMode(mode) {
			issues = append(issues, name+" must be dont-use or separate")
		}
	}
	if !validSeparatedDataUse(value.SeparatedDataUse) {
		issues = append(issues, "separated_data_use must be independently or independently-and-simultaneously")
	}
	for name, parameter := range map[string]*uuid.UUID{
		"data_separation_value": value.DataSeparationValue, "data_separation_use": value.DataSeparationUse,
	} {
		if parameter != nil && parameter.IsZero() {
			issues = append(issues, name+" must be a non-zero UUID")
		}
	}
	// Separation cannot work without the two session parameters: they are how
	// the platform learns which area this session is in and whether separation
	// is in force at all. Both separators of the demonstration configuration
	// name both.
	if value.DataSeparation == SeparationSeparate {
		if value.DataSeparationValue == nil || value.DataSeparationUse == nil {
			issues = append(issues, "data_separation needs data_separation_value and data_separation_use: without them nothing says which area the session is in")
		}
	}
	if value.ConditionalSeparation != nil {
		switch {
		case value.ConditionalSeparation.IsZero():
			issues = append(issues, "conditional_separation must be a non-zero UUID")
		case value.reaches(*value.ConditionalSeparation):
			issues = append(issues, "conditional_separation names a constant the attribute reaches: the condition would have to be read to decide whether to read it")
		}
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
		issues = append(issues, validateConditionalSeparation(prefix, item.ConditionalSeparation, value.reaches)...)
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
// that object is separated, and reading it needs the answer. Outside means not
// reached by the attribute, not unlisted: the configurations being moved list
// the condition with «do not use» when auto-use is on, and an item further
// down the list counts as much as one above it.
func validateConditionalSeparation(prefix string, separation *ConditionalSeparation, inside func(uuid.UUID) bool) []string {
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
		if !separation.Object.IsZero() && inside(*separation.Object) {
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
	if separation.Constant != nil && !separation.Constant.IsZero() && inside(*separation.Constant) {
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
	for _, parameter := range []**uuid.UUID{&value.DataSeparationValue, &value.DataSeparationUse, &value.ConditionalSeparation} {
		if *parameter != nil {
			id := **parameter
			*parameter = &id
		}
	}
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
	// sub is the recalculation within its calculation register, for the one
	// kind that lies inside another.
	sub int
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
	// Which kinds these are is the prototype's own list, from the configurator
	// page «Состав общего реквизита»: catalogs, documents, sequences, charts of
	// characteristic types, charts of accounts, charts of calculation types,
	// business processes, tasks, information, accumulation, accounting and
	// calculation registers, recalculations, exchange plans.
	//
	// All fourteen are here. A sequence and a recalculation keep no list of
	// attributes, and the help says where the field goes on them: it is a field
	// of their record - ПоследовательностьЗапись.<Имя>.<Имя общего реквизита>,
	// ПерерасчетЗапись.<Имя>.<Имя общего реквизита>, read only on the second -
	// kept in a list of its own beside their dimensions. And the help puts a
	// condition on it that no other kind has: only an attribute that does not
	// separate data, or one that separates it «независимо и совместно»,
	// becomes such a field. See recordField.
	//
	// Four more kinds join a composition only when the attribute separates data:
	// constants, scheduled jobs, users of the database and document journals,
	// the last implicitly. They get no field - a journal stores no data of
	// its own, a constant is one value, a job has no table - so
	// membership means separation for them and nothing else.
	var ordered []commonAttributeTarget
	targets := map[uuid.UUID]commonAttributeTarget{}
	add := func(id uuid.UUID, kind string, index int) {
		location := commonAttributeTarget{kind: kind, index: index}
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
	for index, item := range catalog.ChartsOfCharacteristicTypes {
		add(item.ID, "chart of characteristic types", index)
	}
	for index, item := range catalog.ChartsOfAccounts {
		add(item.ID, "chart of accounts", index)
	}
	for index, item := range catalog.ChartsOfCalculationTypes {
		add(item.ID, "chart of calculation types", index)
	}
	for index, item := range catalog.ExchangePlans {
		add(item.ID, "exchange plan", index)
	}
	for index, item := range catalog.AccountingRegisters {
		add(item.ID, "accounting register", index)
	}
	for index, item := range catalog.CalculationRegisters {
		add(item.ID, "calculation register", index)
	}
	for index, item := range catalog.Sequences {
		add(item.ID, "sequence", index)
	}
	for index, register := range catalog.CalculationRegisters {
		for sub, recalculation := range register.Recalculations {
			location := commonAttributeTarget{kind: "recalculation", index: index, sub: sub}
			targets[recalculation.ID] = location
			ordered = append(ordered, location)
		}
	}
	known := catalog.knownObjectIDs()
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
		// An item naming nothing at all is caught before anything is
		// propagated. Naming something that cannot hold a field is not an
		// error: in the real configuration compositions name constants,
		// scheduled jobs, charts of characteristic types and exchange plans -
		// 138 constants and 70 scheduled jobs among six common attributes -
		// and for those, being in the composition does not mean getting a
		// field. There is no field to get: a constant is one value and a
		// scheduled job has no table. It means being separated by the area,
		// which is the half of the mechanism we carry and do not execute.
		//
		// Refusing them would reject a real configuration outright, which is
		// the worst thing an importer can do.
		for _, item := range common.Content {
			if !known[item.Metadata] {
				return fmt.Errorf("common attribute %s references unknown object %s", common.Name, item.Metadata)
			}
		}
		for _, location := range ordered {
			if !common.reaches(catalog.objectID(location)) {
				continue
			}
			if (location.kind == "sequence" || location.kind == "recalculation") && !common.recordField() {
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

// validateConditionalSeparationReferences resolves what every condition of
// every composition names, and it is a pass over the whole catalog because one
// file cannot answer it: the condition points at a constant or at an attribute
// of another object.
//
// Three things are checked, and all three come from the prototype's own
// description of the property. The boolean has to exist - a condition pointing
// at nothing separates nothing and says it separates. It has to be boolean -
// a boolean constant or a boolean attribute. And the object it lives on has to
// be the one whose reference type is the type of the common attribute. That
// last one is what ties the condition to the data area
// it decides about, and it also means a register cannot hold one - a register
// forms no reference.
func (catalog *Catalog) validateConditionalSeparationReferences() error {
	for _, common := range catalog.CommonAttributes {
		for index, item := range common.Content {
			separation := item.ConditionalSeparation
			if separation == nil {
				continue
			}
			where := fmt.Sprintf("common attribute %s content[%d].conditional_separation", common.Name, index)
			if separation.Constant != nil {
				constant, ok := catalog.ConstantByID(*separation.Constant)
				if !ok {
					return fmt.Errorf("%s names unknown constant %s", where, separation.Constant)
				}
				if !isBooleanOnly(constant.Types) {
					return fmt.Errorf("%s names constant %s, which is not boolean", where, constant.Name)
				}
				continue
			}
			if separation.Object == nil || separation.Attribute == nil {
				continue
			}
			name, fields, reference, ok := catalog.referableObject(*separation.Object)
			if !ok {
				return fmt.Errorf("%s names unknown object %s", where, separation.Object)
			}
			field, ok := findAttributeByID(fields, *separation.Attribute)
			if !ok {
				return fmt.Errorf("%s names an attribute that %s does not have", where, name)
			}
			if !isBooleanOnly(field.Types) {
				return fmt.Errorf("%s names attribute %s.%s, which is not boolean", where, name, field.Name)
			}
			if !typesReference(common.Types, reference, *separation.Object) {
				return fmt.Errorf("%s lives on %s, whose reference is not among the types of the common attribute", where, name)
			}
		}
		if common.ConditionalSeparation != nil {
			constant, ok := catalog.ConstantByID(*common.ConditionalSeparation)
			if !ok {
				return fmt.Errorf("common attribute %s conditional_separation names unknown constant %s", common.Name, common.ConditionalSeparation)
			}
			if !isBooleanOnly(constant.Types) {
				return fmt.Errorf("common attribute %s conditional_separation names constant %s, which is not boolean", common.Name, constant.Name)
			}
		}
		// The two session parameters of separation point outside the file as
		// well, and a parameter that is not there leaves the platform with no
		// way to learn which area the session is in.
		for name, parameter := range map[string]*uuid.UUID{
			"data_separation_value": common.DataSeparationValue, "data_separation_use": common.DataSeparationUse,
		} {
			if parameter == nil {
				continue
			}
			if _, ok := catalog.SessionParameterByID(*parameter); !ok {
				return fmt.Errorf("common attribute %s %s names unknown session parameter %s", common.Name, name, parameter)
			}
		}
	}
	return nil
}

func isBooleanOnly(types []Type) bool {
	return len(types) == 1 && types[0].Kind == BooleanType
}

func findAttributeByID(fields []Attribute, id uuid.UUID) (Attribute, bool) {
	for _, field := range fields {
		if field.ID == id {
			return field, true
		}
	}
	return Attribute{}, false
}

func typesReference(types []Type, kind TypeKind, id uuid.UUID) bool {
	for _, item := range types {
		if item.Kind == kind && item.Reference != nil && *item.Reference == id {
			return true
		}
	}
	return false
}

// referableObject is one object that forms a reference type, with its own
// attributes. Registers are absent on purpose: a register record is not
// referable, so a condition cannot live on one.
func (catalog *Catalog) referableObject(id uuid.UUID) (string, []Attribute, TypeKind, bool) {
	for _, item := range catalog.Catalogs {
		if item.ID == id {
			return item.Name, item.Attributes, CatalogType, true
		}
	}
	for _, item := range catalog.Documents {
		if item.ID == id {
			return item.Name, item.Attributes, DocumentType, true
		}
	}
	for _, item := range catalog.BusinessProcesses {
		if item.ID == id {
			return item.Name, item.Attributes, BusinessProcessType, true
		}
	}
	for _, item := range catalog.Tasks {
		if item.ID == id {
			return item.Name, item.Attributes, TaskType, true
		}
	}
	return "", nil, "", false
}

// knownObjectIDs is every metadata object of the configuration, of every kind.
//
// It is gathered by walking the collections of the catalog rather than by
// listing them, and that is deliberate: the list would be twenty-odd names
// today and would quietly fall behind on the first kind added, turning a
// perfectly good object into an "unknown object" at load time. A collection
// added to the catalog is known here the moment it is added.
// OwnAttributes returns the attributes an object declares itself, leaving out
// the common attributes propagation has added to the same list.
//
// The prototype keeps the two apart where anybody can see: the collection of
// attributes of an object's metadata does not hold the common attributes it is
// in - established on the platform on 30.09.2026 with
// Метаданные.Справочники.<Имя>.Реквизиты - and its configurator does not show
// them among the object's attributes either. Propagation puts them into one
// list because storage, queries and the object in code all need the field
// there; whatever answers «what are this object's attributes» takes this
// instead.
func (catalog *Catalog) OwnAttributes(attributes []Attribute) []Attribute {
	if catalog == nil || len(catalog.CommonAttributes) == 0 {
		return attributes
	}
	common := make(map[uuid.UUID]bool, len(catalog.CommonAttributes))
	for _, item := range catalog.CommonAttributes {
		common[item.ID] = true
	}
	own := make([]Attribute, 0, len(attributes))
	for _, attribute := range attributes {
		if !common[attribute.ID] {
			own = append(own, attribute)
		}
	}
	return own
}

// recordField says whether the attribute becomes a field of the record of a
// sequence or a recalculation it reaches. The help gives the rule in one
// sentence for both: an attribute that does not separate data does, and one
// that separates it does only when separated data is used «независимо и
// совместно». A separator used independently alone keeps its area apart and
// adds no field there.
func (value CommonAttributeDefinition) recordField() bool {
	return value.DataSeparation != SeparationSeparate || value.SeparatedDataUse == SeparatedDataIndependentlyAndSimultaneously
}

func (catalog *Catalog) knownObjectIDs() map[uuid.UUID]bool {
	known := map[uuid.UUID]bool{}
	// A recalculation lies inside its calculation register, and a composition
	// names it all the same: it is one of the fourteen kinds.
	for _, register := range catalog.CalculationRegisters {
		for _, recalculation := range register.Recalculations {
			known[recalculation.ID] = true
		}
	}
	value := reflect.ValueOf(*catalog)
	for index := 0; index < value.NumField(); index++ {
		field := value.Field(index)
		if !field.CanInterface() || field.Kind() != reflect.Slice {
			continue
		}
		for item := 0; item < field.Len(); item++ {
			element := field.Index(item)
			if element.Kind() != reflect.Struct {
				break
			}
			id := element.FieldByName("ID")
			if !id.IsValid() || id.Type() != reflect.TypeOf(uuid.UUID{}) {
				break
			}
			known[id.Interface().(uuid.UUID)] = true
		}
	}
	return known
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
	case "chart of characteristic types":
		return catalog.ChartsOfCharacteristicTypes[location.index].Name
	case "chart of accounts":
		return catalog.ChartsOfAccounts[location.index].Name
	case "chart of calculation types":
		return catalog.ChartsOfCalculationTypes[location.index].Name
	case "exchange plan":
		return catalog.ExchangePlans[location.index].Name
	case "accounting register":
		return catalog.AccountingRegisters[location.index].Name
	case "calculation register":
		return catalog.CalculationRegisters[location.index].Name
	case "sequence":
		return catalog.Sequences[location.index].Name
	case "recalculation":
		register := catalog.CalculationRegisters[location.index]
		return register.Name + "." + register.Recalculations[location.sub].Name
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
	case "chart of characteristic types":
		return catalog.ChartsOfCharacteristicTypes[location.index].ID
	case "chart of accounts":
		return catalog.ChartsOfAccounts[location.index].ID
	case "chart of calculation types":
		return catalog.ChartsOfCalculationTypes[location.index].ID
	case "exchange plan":
		return catalog.ExchangePlans[location.index].ID
	case "accounting register":
		return catalog.AccountingRegisters[location.index].ID
	case "calculation register":
		return catalog.CalculationRegisters[location.index].ID
	case "sequence":
		return catalog.Sequences[location.index].ID
	case "recalculation":
		return catalog.CalculationRegisters[location.index].Recalculations[location.sub].ID
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
		fields = append(fields, RegisterDimensionAttributes(definition.Dimensions)...)
		fields = append(fields, definition.Resources...)
	case "accumulation register":
		definition := catalog.AccumulationRegisters[location.index]
		fields = append(fields, definition.Attributes...)
		fields = append(fields, RegisterDimensionAttributes(definition.Dimensions)...)
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
	case "chart of characteristic types":
		definition := catalog.ChartsOfCharacteristicTypes[location.index]
		fields = append(fields, definition.Attributes...)
		fields = append(fields, tablePartNames(definition.TableParts)...)
	case "chart of accounts":
		definition := catalog.ChartsOfAccounts[location.index]
		fields = append(fields, definition.Attributes...)
		fields = append(fields, tablePartNames(definition.TableParts)...)
		// The flags of a chart of accounts are fields of its own table, and a
		// name taken by one of them is taken.
		for _, flag := range definition.AccountingFlags {
			fields = append(fields, Attribute{ID: flag.ID, Name: flag.Name})
		}
	case "chart of calculation types":
		definition := catalog.ChartsOfCalculationTypes[location.index]
		fields = append(fields, definition.Attributes...)
		fields = append(fields, tablePartNames(definition.TableParts)...)
	case "exchange plan":
		definition := catalog.ExchangePlans[location.index]
		fields = append(fields, definition.Attributes...)
		fields = append(fields, tablePartNames(definition.TableParts)...)
	case "accounting register":
		definition := catalog.AccountingRegisters[location.index]
		fields = append(fields, definition.Attributes...)
		fields = append(fields, RegisterDimensionAttributes(definition.Dimensions)...)
		fields = append(fields, accountingResourceAttributes(definition.Resources)...)
	case "calculation register":
		definition := catalog.CalculationRegisters[location.index]
		fields = append(fields, definition.Attributes...)
		fields = append(fields, definition.Resources...)
		for _, dimension := range definition.Dimensions {
			fields = append(fields, dimension.Attribute)
		}
	case "sequence":
		definition := catalog.Sequences[location.index]
		for _, dimension := range definition.Dimensions {
			fields = append(fields, Attribute{ID: dimension.ID, Name: dimension.Name})
		}
		fields = append(fields, definition.CommonAttributeFields...)
	case "recalculation":
		definition := catalog.CalculationRegisters[location.index].Recalculations[location.sub]
		for _, dimension := range definition.Dimensions {
			fields = append(fields, Attribute{ID: dimension.ID, Name: dimension.Name})
		}
		fields = append(fields, definition.CommonAttributeFields...)
	}
	return fields
}

func tablePartNames(parts []TablePart) []Attribute {
	fields := make([]Attribute, 0, len(parts))
	for _, part := range parts {
		fields = append(fields, Attribute{Name: part.Name})
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
	case "chart of characteristic types":
		catalog.ChartsOfCharacteristicTypes[location.index].Attributes = append(catalog.ChartsOfCharacteristicTypes[location.index].Attributes, attribute)
	case "chart of accounts":
		catalog.ChartsOfAccounts[location.index].Attributes = append(catalog.ChartsOfAccounts[location.index].Attributes, attribute)
	case "chart of calculation types":
		catalog.ChartsOfCalculationTypes[location.index].Attributes = append(catalog.ChartsOfCalculationTypes[location.index].Attributes, attribute)
	case "exchange plan":
		catalog.ExchangePlans[location.index].Attributes = append(catalog.ExchangePlans[location.index].Attributes, attribute)
	case "accounting register":
		catalog.AccountingRegisters[location.index].Attributes = append(catalog.AccountingRegisters[location.index].Attributes, attribute)
	case "calculation register":
		catalog.CalculationRegisters[location.index].Attributes = append(catalog.CalculationRegisters[location.index].Attributes, attribute)
	case "sequence":
		catalog.Sequences[location.index].CommonAttributeFields = append(catalog.Sequences[location.index].CommonAttributeFields, attribute)
	case "recalculation":
		recalculation := &catalog.CalculationRegisters[location.index].Recalculations[location.sub]
		recalculation.CommonAttributeFields = append(recalculation.CommonAttributeFields, attribute)
	}
}
