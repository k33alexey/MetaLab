package metadata

import (
	"fmt"
	"io"
	"slices"
	"strings"

	"github.com/k33alexey/MetaLab/internal/project"
	"github.com/k33alexey/MetaLab/internal/uuid"
)

// CalculationPeriodicity is the length of a registration period - the period an
// entry is made in, as opposed to the period it acts over. The two are
// different things, and the whole of displacement and reversal rests on the
// difference.
type CalculationPeriodicity string

const (
	CalculationPeriodDay      CalculationPeriodicity = "day"
	CalculationPeriodTenDays  CalculationPeriodicity = "ten-days"
	CalculationPeriodMonth    CalculationPeriodicity = "month"
	CalculationPeriodQuarter  CalculationPeriodicity = "quarter"
	CalculationPeriodHalfYear CalculationPeriodicity = "half-year"
	CalculationPeriodYear     CalculationPeriodicity = "year"

	maxRecalculationsPerRegister = 32
)

// CalculationRegisterDimension is a dimension of a calculation register. Beyond
// what any field carries it holds the two links that make calculation work.
type CalculationRegisterDimension struct {
	ID      uuid.UUID     `yaml:"id" json:"id"`
	Name    string        `yaml:"name" json:"name"`
	Title   LocalizedText `yaml:"title" json:"title"`
	Types   []Type        `yaml:"types" json:"types"`
	Indexed bool          `yaml:"indexed,omitempty" json:"indexed,omitempty"`
	// Base says a record is tied to its base by this dimension. Without it
	// there is nothing to say whose base to gather, and one person's salary
	// would be computed from everybody's bonuses.
	Base bool `yaml:"base,omitempty" json:"base,omitempty"`
	// ScheduleLink is the dimension of the schedule this one corresponds to.
	// The register knows where the schedule's value and date are; this is what
	// tells it whose value that is.
	ScheduleLink *uuid.UUID `yaml:"schedule_link,omitempty" json:"scheduleLink,omitempty"`
}

// RecalculationDimension carries both links a recalculation needs: which
// dimension of the register it corresponds to, and which data, when changed,
// makes the recalculation necessary.
type RecalculationDimension struct {
	ID    uuid.UUID     `yaml:"id" json:"id"`
	Name  string        `yaml:"name" json:"name"`
	Title LocalizedText `yaml:"title" json:"title"`
	// RegisterDimension is the dimension of this register the recalculation
	// finds its records by.
	RegisterDimension uuid.UUID `yaml:"register_dimension" json:"registerDimension"`
	// LeadingData are the dimensions whose change sets the recalculation off.
	// A leading dimension may belong to this register or to another one.
	LeadingData []uuid.UUID `yaml:"leading_data,omitempty" json:"leadingData,omitempty"`
}

// Recalculation says which records of a register have to be computed again once
// the data they depend on has changed. A calculation type is not named here:
// it is a field of the recalculation record, beside the recorder.
type Recalculation struct {
	ID         uuid.UUID                `yaml:"id" json:"id"`
	Name       string                   `yaml:"name" json:"name"`
	Title      LocalizedText            `yaml:"title" json:"title"`
	Dimensions []RecalculationDimension `yaml:"dimensions,omitempty" json:"dimensions,omitempty"`
}

// CalculationRegisterDefinition holds the results of calculation, with a period
// they act over and mutual displacement between them.
type CalculationRegisterDefinition struct {
	Format int           `yaml:"format" json:"format"`
	ID     uuid.UUID     `yaml:"id" json:"id"`
	Name   string        `yaml:"name" json:"name"`
	Title  LocalizedText `yaml:"title" json:"title"`
	// ChartOfCalculationTypes supplies the kinds of accrual and the rules by
	// which they compete for a period.
	ChartOfCalculationTypes uuid.UUID              `yaml:"chart_of_calculation_types" json:"chartOfCalculationTypes"`
	Periodicity             CalculationPeriodicity `yaml:"periodicity" json:"periodicity"`
	// ActionPeriod turns on the period a record acts over, as distinct from
	// the period it was registered in.
	ActionPeriod bool `yaml:"action_period,omitempty" json:"actionPeriod,omitempty"`
	// BasePeriod turns on the period a base is gathered over.
	BasePeriod bool `yaml:"base_period,omitempty" json:"basePeriod,omitempty"`
	// Schedule is the information register holding the working calendar, with
	// the resource its value is in and the dimension its date is in.
	Schedule      *uuid.UUID `yaml:"schedule,omitempty" json:"schedule,omitempty"`
	ScheduleValue *uuid.UUID `yaml:"schedule_value,omitempty" json:"scheduleValue,omitempty"`
	ScheduleDate  *uuid.UUID `yaml:"schedule_date,omitempty" json:"scheduleDate,omitempty"`

	Dimensions     []CalculationRegisterDimension `yaml:"dimensions,omitempty" json:"dimensions,omitempty"`
	Resources      []Attribute                    `yaml:"resources" json:"resources"`
	Attributes     []Attribute                    `yaml:"attributes,omitempty" json:"attributes,omitempty"`
	Recorders      []uuid.UUID                    `yaml:"recorders" json:"recorders"`
	Recalculations []Recalculation                `yaml:"recalculations,omitempty" json:"recalculations,omitempty"`
	Forms          RegisterForms                  `yaml:"forms,omitempty" json:"forms,omitempty"`
	Commands       []ObjectCommand                `yaml:"commands,omitempty" json:"commands,omitempty"`
	Templates      []ObjectTemplate               `yaml:"templates,omitempty" json:"templates,omitempty"`
}

// DecodeCalculationRegister reads and validates one calculation register.
func DecodeCalculationRegister(source string, reader io.Reader, configuration project.Project) (CalculationRegisterDefinition, error) {
	var value CalculationRegisterDefinition
	if err := decodeStrict(source, reader, &value); err != nil {
		return CalculationRegisterDefinition{}, err
	}
	issues := validateBase(value.Format, value.ID, value.Name, value.Title, configuration)
	if value.ChartOfCalculationTypes.IsZero() {
		issues = append(issues, "chart_of_calculation_types is required: without kinds of accrual there is nothing to calculate")
	}
	switch value.Periodicity {
	case CalculationPeriodDay, CalculationPeriodTenDays, CalculationPeriodMonth,
		CalculationPeriodQuarter, CalculationPeriodHalfYear, CalculationPeriodYear:
	default:
		issues = append(issues, "periodicity must be day, ten-days, month, quarter, half-year or year")
	}
	if len(value.Resources) == 0 {
		issues = append(issues, "resources must contain at least one item: a calculation with no result is not a calculation")
	}
	if len(value.Recorders) == 0 || len(value.Recorders) > 128 {
		issues = append(issues, "recorders must contain 1..128 documents")
	}
	issues = append(issues, validateUniqueIDs("recorders", value.Recorders)...)
	// The three schedule settings are one setting in three parts. Two of them
	// without the first point into nothing.
	if value.Schedule == nil && (value.ScheduleValue != nil || value.ScheduleDate != nil) {
		issues = append(issues, "schedule_value and schedule_date need the schedule they belong to")
	}
	if value.Schedule != nil && (value.ScheduleValue == nil || value.ScheduleDate == nil) {
		issues = append(issues, "a schedule needs both the resource its value is in and the dimension its date is in")
	}
	names, ids := map[string]bool{}, map[uuid.UUID]bool{}
	for index, dimension := range value.Dimensions {
		prefix := fmt.Sprintf("dimensions[%d]", index)
		issues = append(issues, validateRegisterFieldShape(prefix, dimension.ID, dimension.Name, dimension.Title, dimension.Types,
			value.ID, names, ids, configuration, reservedCalculationRegisterName)...)
		if dimension.ScheduleLink != nil && value.Schedule == nil {
			issues = append(issues, prefix+".schedule_link needs a schedule: there is no schedule for it to link to")
		}
	}
	for index, resource := range value.Resources {
		prefix := fmt.Sprintf("resources[%d]", index)
		issues = append(issues, validateRegisterFieldShape(prefix, resource.ID, resource.Name, resource.Title, resource.Types,
			value.ID, names, ids, configuration, reservedCalculationRegisterName)...)
		issues = append(issues, validateFieldSettings(prefix, resource, configuration)...)
	}
	issues = append(issues, validateAttributes("attributes", value.Attributes, configuration, func(name string) bool {
		return names[strings.ToLower(name)] || reservedCalculationRegisterName(name)
	})...)
	issues = append(issues, validateFieldLinks([]fieldGroup{
		{"resources", value.Resources}, {"attributes", value.Attributes},
	}, nil)...)
	issues = append(issues, validateRecalculations(value, configuration)...)
	issues = append(issues, validateFormSlots(value.Forms.slots())...)
	issues = append(issues, validateObjectCommands(value.Commands, value.ID, configuration)...)
	issues = append(issues, validateObjectTemplates(value.Templates, configuration)...)
	if err := issuesError(source, value.Format, issues); err != nil {
		return CalculationRegisterDefinition{}, err
	}
	return value, nil
}

// validateRegisterFieldShape checks what every named field of a register
// repeats, and keeps one set of names and identifiers across the groups: a
// dimension and a resource of one register may not share a name.
func validateRegisterFieldShape(prefix string, id uuid.UUID, name string, title LocalizedText, types []Type,
	self uuid.UUID, names map[string]bool, ids map[uuid.UUID]bool, configuration project.Project, reserved func(string) bool) []string {
	var issues []string
	if id.IsZero() {
		issues = append(issues, prefix+".id must be a non-zero UUID")
	}
	if ids[id] {
		issues = append(issues, prefix+".id must be unique")
	}
	ids[id] = true
	if !validIdentifier(name) {
		issues = append(issues, prefix+".name must be a valid identifier")
	}
	folded := strings.ToLower(name)
	if names[folded] || reserved(folded) {
		issues = append(issues, prefix+".name is taken")
	}
	names[folded] = true
	issues = append(issues, validateTitle(prefix+".title", title, configuration)...)
	issues = append(issues, validateTypes(prefix+".types", types, self)...)
	return issues
}

func validateRecalculations(value CalculationRegisterDefinition, configuration project.Project) []string {
	if len(value.Recalculations) > maxRecalculationsPerRegister {
		return []string{fmt.Sprintf("recalculations must not contain more than %d items", maxRecalculationsPerRegister)}
	}
	var issues []string
	names, ids := map[string]bool{}, map[uuid.UUID]bool{}
	for index, recalculation := range value.Recalculations {
		prefix := fmt.Sprintf("recalculations[%d]", index)
		if recalculation.ID.IsZero() {
			issues = append(issues, prefix+".id must be a non-zero UUID")
		}
		if ids[recalculation.ID] {
			issues = append(issues, prefix+".id must be unique")
		}
		ids[recalculation.ID] = true
		if !validIdentifier(recalculation.Name) {
			issues = append(issues, prefix+".name must be a valid identifier")
		}
		folded := strings.ToLower(recalculation.Name)
		if names[folded] {
			issues = append(issues, prefix+".name must be unique")
		}
		names[folded] = true
		issues = append(issues, validateTitle(prefix+".title", recalculation.Title, configuration)...)
		if len(recalculation.Dimensions) == 0 {
			issues = append(issues, prefix+" has no dimensions, so it finds no records to compute again")
		}
		inner, innerIDs := map[string]bool{}, map[uuid.UUID]bool{}
		for position, dimension := range recalculation.Dimensions {
			path := fmt.Sprintf("%s.dimensions[%d]", prefix, position)
			if dimension.ID.IsZero() {
				issues = append(issues, path+".id must be a non-zero UUID")
			}
			if innerIDs[dimension.ID] || ids[dimension.ID] {
				issues = append(issues, path+".id must be unique")
			}
			innerIDs[dimension.ID] = true
			if !validIdentifier(dimension.Name) {
				issues = append(issues, path+".name must be a valid identifier")
			}
			if inner[strings.ToLower(dimension.Name)] {
				issues = append(issues, path+".name must be unique")
			}
			inner[strings.ToLower(dimension.Name)] = true
			issues = append(issues, validateTitle(path+".title", dimension.Title, configuration)...)
			if dimension.RegisterDimension.IsZero() {
				issues = append(issues, path+".register_dimension is required: without it nothing says which records to compute again")
			}
			if len(dimension.LeadingData) == 0 {
				issues = append(issues, path+" has no leading data, so nothing ever sets it off")
			}
			issues = append(issues, validateUniqueIDs(path+".leading_data", dimension.LeadingData)...)
		}
	}
	return issues
}

// reservedCalculationRegisterName keeps the standard fields of a record: when
// it was registered and by what, the kind of accrual, the periods it acts and
// gathers its base over, and whether it reverses an earlier record.
func reservedCalculationRegisterName(name string) bool {
	switch strings.ToLower(name) {
	case "период", "period", "регистратор", "recorder", "номерстроки", "linenumber",
		"активность", "active", "видрасчета", "видрасчёта", "calculationtype",
		"периоддействия", "actionperiod",
		"периоддействияначало", "actionperiodstart", "периоддействияконец", "actionperiodend",
		"базовыйпериодначало", "baseperiodstart", "базовыйпериодконец", "baseperiodend",
		"сторно", "reversing", "recordid":
		return true
	default:
		return false
	}
}

func cloneRecalculations(items []Recalculation) []Recalculation {
	items = slices.Clone(items)
	for index := range items {
		items[index].Title = cloneTitle(items[index].Title)
		items[index].Dimensions = slices.Clone(items[index].Dimensions)
		for position := range items[index].Dimensions {
			dimension := &items[index].Dimensions[position]
			dimension.Title = cloneTitle(dimension.Title)
			dimension.LeadingData = slices.Clone(dimension.LeadingData)
		}
	}
	return items
}

func cloneCalculationRegister(value CalculationRegisterDefinition) CalculationRegisterDefinition {
	value.Title = cloneTitle(value.Title)
	value.Dimensions = slices.Clone(value.Dimensions)
	for index := range value.Dimensions {
		value.Dimensions[index].Title = cloneTitle(value.Dimensions[index].Title)
		value.Dimensions[index].Types = cloneTypes(value.Dimensions[index].Types)
		if value.Dimensions[index].ScheduleLink != nil {
			id := *value.Dimensions[index].ScheduleLink
			value.Dimensions[index].ScheduleLink = &id
		}
	}
	value.Resources = cloneAttributes(value.Resources)
	value.Attributes = cloneAttributes(value.Attributes)
	value.Recorders = slices.Clone(value.Recorders)
	value.Recalculations = cloneRecalculations(value.Recalculations)
	for _, id := range []**uuid.UUID{&value.Schedule, &value.ScheduleValue, &value.ScheduleDate} {
		if *id != nil {
			copied := **id
			*id = &copied
		}
	}
	value.Forms = cloneFormSet(value.Forms)
	value.Commands = cloneObjectCommands(value.Commands)
	value.Templates = cloneObjectTemplates(value.Templates)
	return value
}

// calculationRegisterFields is every named field of a record as a plain
// attribute, which is what they are to everything that does not calculate.
func calculationRegisterFields(item CalculationRegisterDefinition) []Attribute {
	fields := make([]Attribute, 0, len(item.Dimensions)+len(item.Resources))
	for _, dimension := range item.Dimensions {
		fields = append(fields, Attribute{ID: dimension.ID, Name: dimension.Name, Title: dimension.Title,
			Types: dimension.Types, Indexed: dimension.Indexed})
	}
	return append(fields, item.Resources...)
}

// CalculationRegister returns one calculation register by name, folded case.
func (catalog *Catalog) CalculationRegister(name string) (CalculationRegisterDefinition, bool) {
	index, ok := catalog.calculationRegisterByName[strings.ToLower(name)]
	if !ok {
		return CalculationRegisterDefinition{}, false
	}
	return cloneCalculationRegister(catalog.CalculationRegisters[index]), true
}

// CalculationRegisterByID returns one calculation register by its identifier.
func (catalog *Catalog) CalculationRegisterByID(id uuid.UUID) (CalculationRegisterDefinition, bool) {
	index, ok := catalog.calculationRegisterByID[id]
	if !ok {
		return CalculationRegisterDefinition{}, false
	}
	return cloneCalculationRegister(catalog.CalculationRegisters[index]), true
}
