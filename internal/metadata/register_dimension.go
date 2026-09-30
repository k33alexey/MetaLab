package metadata

import "github.com/k33alexey/MetaLab/internal/uuid"

// RegisterDimension is a dimension of a register, and it is a kind of metadata
// object of its own in the prototype: «ОбъектМетаданных: Измерение» carries 44
// properties where «ОбъектМетаданных: Реквизит» carries 38, and the difference
// is not decoration. Nothing here changes the type of the value - that stays
// `types`, as for any field. What is here is the rest of the property palette a
// dimension has and an attribute does not.
//
// Why a structure of its own rather than four more fields on Attribute: an
// attribute of a catalog has none of these in the prototype, and putting them
// there would give them to it - a difference in the composition of metadata,
// which we do not keep anywhere.
//
// Resources and attributes of a register stay as attributes, and that is not a
// simplification. In the export a resource of an information register carries 28
// properties and an attribute 28, with not one property between them either way,
// and both sets are the catalog attribute's minus «использование», which a field
// of a register is refused anyway.
type RegisterDimension struct {
	Attribute `yaml:",inline" json:",inline"`
	// DenyIncompleteValues refuses an empty value in this dimension, and the
	// help gives it to the dimensions of all four kinds of register.
	DenyIncompleteValues bool `yaml:"deny_incomplete_values,omitempty" json:"denyIncompleteValues,omitempty"`
	// Master and MainFilter belong to a dimension of an information register
	// and to no other. Of the 498 dimensions of
	// the demonstration configuration 153 are master and 463 are the main filter.
	Master     bool `yaml:"master,omitempty" json:"master,omitempty"`
	MainFilter bool `yaml:"main_filter,omitempty" json:"mainFilter,omitempty"`
	// TypeReduction says what happens to the data when the configuration is
	// updated and a value cannot be converted to the new type without loss - a
	// number narrowed, a moment replaced by a date. All 498 dimensions of the
	// export say «transform values».
	// The setting is the one standard attributes already carry - see
	// TypeReduction in standard_attributes.go, which describes the same choice
	// of three.
	TypeReduction TypeReduction `yaml:"type_reduction,omitempty" json:"typeReduction,omitempty"`
	// UseInTotals belongs to a dimension of an accumulation register and is not
	// used by a register of balances: «для регистра накопления остатков это
	// свойство не используется».
	//
	// It is a pointer and not a boolean on purpose. Every dimension of the export
	// says «true», and the platform's own default is to use the dimension in
	// totals - so an absent value means «use», not «do not». A plain boolean
	// would read an omitted line as a refusal and quietly drop the dimension out
	// of every total.
	UseInTotals *bool `yaml:"use_in_totals,omitempty" json:"useInTotals,omitempty"`
	// Base and ScheduleLink belong to a dimension of a calculation register.
	// Base says a record is tied to its base by this dimension: without it
	// there is nothing to say whose base to gather, and one person's salary
	// would be computed from everybody's bonuses. ScheduleLink is the
	// dimension of the schedule this one corresponds to - the register knows
	// where the schedule's value and date are, and this is what tells it whose
	// value that is.
	Base         bool       `yaml:"base,omitempty" json:"base,omitempty"`
	ScheduleLink *uuid.UUID `yaml:"schedule_link,omitempty" json:"scheduleLink,omitempty"`
}

// usesInTotals answers the question the pointer leaves open: a dimension nobody
// spoke about is used in totals.
func (dimension RegisterDimension) usesInTotals() bool {
	return dimension.UseInTotals == nil || *dimension.UseInTotals
}

// RegisterDimensionAttributes is the dimensions as plain attributes, for the
// checks, the storage and the editors that do not care which kind of field this
// is. Exported because Studio shows the fields of a register in one list and has
// the same need.
func RegisterDimensionAttributes(dimensions []RegisterDimension) []Attribute {
	result := make([]Attribute, 0, len(dimensions))
	for _, dimension := range dimensions {
		result = append(result, dimension.Attribute)
	}
	return result
}

func cloneRegisterDimensions(dimensions []RegisterDimension) []RegisterDimension {
	result := make([]RegisterDimension, len(dimensions))
	copy(result, dimensions)
	for index := range result {
		result[index].Attribute = cloneAttribute(result[index].Attribute)
		if result[index].UseInTotals != nil {
			value := *result[index].UseInTotals
			result[index].UseInTotals = &value
		}
		if result[index].ScheduleLink != nil {
			id := *result[index].ScheduleLink
			result[index].ScheduleLink = &id
		}
	}
	return result
}

// dimensionProperties is which of the properties of a dimension this kind of
// register has. The prototype has one metadata object for the dimension of all
// four kinds, and what a kind may set of it is applicability, not a different
// type. ЗапрещатьНезаполненныеЗначения is not among them: it belongs to the
// dimensions of all four registers and needs no permission.
type dimensionProperties struct {
	master        bool
	mainFilter    bool
	typeReduction bool
	useInTotals   bool
	base          bool
	scheduleLink  bool
}

// informationRegisterDimensions and accumulationRegisterDimensions are the two
// answers the prototype gives. Ведущее and ОсновнойОтбор belong to the
// dimensions of information registers; ИспользованиеВИтогах belongs to the
// dimensions of accumulation registers and means nothing for a register of
// balances - so a balance register allows none of the four.
func informationRegisterDimensions() dimensionProperties {
	return dimensionProperties{master: true, mainFilter: true, typeReduction: true}
}

func accumulationRegisterDimensions(balance bool) dimensionProperties {
	return dimensionProperties{useInTotals: !balance}
}

// calculationRegisterDimensions is the answer for a calculation register: the
// base and the link to the schedule, and nothing of the other kinds.
func calculationRegisterDimensions() dimensionProperties {
	return dimensionProperties{base: true, scheduleLink: true}
}

// validateRegisterDimension refuses a property the owner does not have. A
// property nobody reads is worse than a missing one: it reads as a setting and
// changes nothing, and the developer who wrote it goes looking for the effect.
func validateRegisterDimension(prefix string, dimension RegisterDimension, allowed dimensionProperties) []string {
	var issues []string
	if !validTypeReduction(dimension.TypeReduction) {
		issues = append(issues, prefix+".type_reduction must be deny, transform-values or delete-data")
	}
	for name, refused := range map[string]bool{
		"master":         dimension.Master && !allowed.master,
		"main_filter":    dimension.MainFilter && !allowed.mainFilter,
		"type_reduction": dimension.TypeReduction != "" && !allowed.typeReduction,
		"use_in_totals":  dimension.UseInTotals != nil && !allowed.useInTotals,
		"base":           dimension.Base && !allowed.base,
		"schedule_link":  dimension.ScheduleLink != nil && !allowed.scheduleLink,
	} {
		if refused {
			issues = append(issues, prefix+"."+name+" does not belong to a dimension of this register")
		}
	}
	return issues
}
