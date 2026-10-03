package metadata

import (
	"fmt"
	"strings"
)

// Агрегаты регистра накопления — a mechanism for speeding up totals that the
// model did not have and the map did not name.
//
// It is not a property of the register but a subordinate entity: a register
// carries a collection of aggregates, and each of them is a precomputed cut of
// the movements by a subset of dimensions at a chosen periodicity. The syntax
// assistant gives АгрегатРегистраНакопления exactly three properties -
// Измерения, Периодичность, Использование - and no name and no identifier
// among them. An aggregate is therefore identified by what it holds, which is
// why two of them holding the same thing are refused below.
//
// The demonstration configuration uses aggregates nowhere, so it could not
// have shown the gap and cannot confirm the shape. Everything here comes from
// the syntax assistant of 8.3.27.
//
// **Declared here, switched on in the data.** The metadata declares which
// aggregates exist; whether the register runs on aggregates at all is a
// setting of the working database, set by УстановитьРежимАгрегатов, and the
// help is explicit that a register with no aggregates in the configuration
// raises an exception when asked to switch. So the declaration is the
// precondition, not the switch, and it belongs here while the switch does not.
//
// **Aggregates and totals are alternatives, not layers.** The same help says
// switching the mode on clears the totals, and switching it off clears the
// aggregates and recomputes the totals. That is worth writing down beside the
// register's own AllowTotalsSplitting: a register running on aggregates has no
// totals to split. It is not a contradiction in the configuration, because
// both modes are data of the base and both may be permitted at once - see
// register_totals_mode.go - and it is not refused for that reason.
//
// **Open question: which kind of register may carry them.** The help gives
// Агрегаты to ОбъектМетаданных: РегистрНакопления without distinguishing a
// turnover register from a balance one, and says nothing anywhere else. The
// wording "instead of totals the aggregates are used" reads as a turnover
// mechanism, but reading is not knowing, and refusing a balance register with
// aggregates would refuse a configuration on a guess. Stored as written;
// settled by a run on the platform, like the numbering of period fields in
// data composition and the difference between the two kinds of indexing.

// AggregatePeriodicity is how coarsely an aggregate folds time.
//
// Seven values, and the set is the aggregate's own: it is neither the
// periodicity of an information register - there is no week here and no second
// - nor the period of anything else. Авто leaves the choice to the platform.
type AggregatePeriodicity string

const (
	AggregatePeriodicityAuto        AggregatePeriodicity = "auto"
	AggregatePeriodicityNonPeriodic AggregatePeriodicity = "non-periodic"
	AggregatePeriodicityDay         AggregatePeriodicity = "day"
	AggregatePeriodicityMonth       AggregatePeriodicity = "month"
	AggregatePeriodicityQuarter     AggregatePeriodicity = "quarter"
	AggregatePeriodicityHalfYear    AggregatePeriodicity = "half-year"
	AggregatePeriodicityYear        AggregatePeriodicity = "year"
)

func validAggregatePeriodicity(value AggregatePeriodicity) bool {
	switch value {
	case "", AggregatePeriodicityAuto, AggregatePeriodicityNonPeriodic, AggregatePeriodicityDay,
		AggregatePeriodicityMonth, AggregatePeriodicityQuarter, AggregatePeriodicityHalfYear,
		AggregatePeriodicityYear:
		return true
	default:
		return false
	}
}

// AggregateUse says when a query is allowed to answer from this aggregate.
// Two values: always, or at the platform's discretion.
type AggregateUse string

const (
	AggregateUseAlways AggregateUse = "always"
	AggregateUseAuto   AggregateUse = "auto"
)

func validAggregateUse(value AggregateUse) bool {
	switch value {
	case "", AggregateUseAlways, AggregateUseAuto:
		return true
	default:
		return false
	}
}

// AccumulationRegisterAggregate is one precomputed cut of the movements.
//
// Dimensions are named, not listed by identifier, because they are the
// register's own dimensions and nothing else: an aggregate over a field the
// register has not got is an aggregate that can never be built.
type AccumulationRegisterAggregate struct {
	Periodicity AggregatePeriodicity `yaml:"periodicity,omitempty" json:"periodicity,omitempty"`
	Use         AggregateUse         `yaml:"use,omitempty" json:"use,omitempty"`
	Dimensions  []string             `yaml:"dimensions,omitempty" json:"dimensions,omitempty"`
}

// validateAccumulationAggregates resolves every aggregate against the register
// that carries it.
//
// A dimension that is not there is refused, for the reason any dangling name
// is refused: it describes a cut nothing can build. Two aggregates holding the
// same dimensions at the same periodicity are not: an aggregate has no name of
// its own, so the copy is one answer stored twice, but nothing shows the
// configurator refusing to save it. The copy is carried and is a note
// (NoteDuplicateAggregate).
//
// An aggregate over no dimensions at all is not refused: folded only by
// period, it is the coarsest cut there is and the most effective one when a
// report asks for totals and nothing else.
func validateAccumulationAggregates(aggregates []AccumulationRegisterAggregate, dimensions []Attribute) []string {
	if len(aggregates) == 0 {
		return nil
	}
	declared := make(map[string]bool, len(dimensions))
	for _, dimension := range dimensions {
		declared[strings.ToLower(dimension.Name)] = true
	}
	var issues []string
	seen := make(map[string]int, len(aggregates))
	for index, aggregate := range aggregates {
		prefix := fmt.Sprintf("aggregates[%d]", index)
		if !validAggregatePeriodicity(aggregate.Periodicity) {
			issues = append(issues, prefix+".periodicity must be auto, non-periodic, day, month, quarter, half-year or year")
		}
		if !validAggregateUse(aggregate.Use) {
			issues = append(issues, prefix+".use must be always or auto")
		}
		within := make(map[string]bool, len(aggregate.Dimensions))
		folded := make([]string, 0, len(aggregate.Dimensions))
		for position, name := range aggregate.Dimensions {
			at := fmt.Sprintf("%s.dimensions[%d]", prefix, position)
			switch key := strings.ToLower(name); {
			case name == "":
				issues = append(issues, at+" must name a dimension")
			case !declared[key]:
				issues = append(issues, at+" names "+name+", which is not a dimension of this register")
			case within[key]:
				issues = append(issues, at+" repeats "+name)
			default:
				within[key] = true
				folded = append(folded, key)
			}
		}
		// The order of dimensions inside an aggregate says nothing - the same
		// set folded the same way is the same cut - so the key is sorted.
		key := strings.Join(sortedStrings(folded), ",") + "|" + string(aggregate.Periodicity)
		// The same cut twice is carried and noted: nothing shows the
		// configurator refusing it, and a second copy computes the same.
		if _, exists := seen[key]; !exists {
			seen[key] = index
		}
	}
	return issues
}

func sortedStrings(values []string) []string {
	sorted := append([]string(nil), values...)
	for i := 1; i < len(sorted); i++ {
		for j := i; j > 0 && sorted[j] < sorted[j-1]; j-- {
			sorted[j], sorted[j-1] = sorted[j-1], sorted[j]
		}
	}
	return sorted
}

func cloneAccumulationAggregates(values []AccumulationRegisterAggregate) []AccumulationRegisterAggregate {
	if values == nil {
		return nil
	}
	cloned := make([]AccumulationRegisterAggregate, len(values))
	for index, value := range values {
		value.Dimensions = append([]string(nil), value.Dimensions...)
		cloned[index] = value
	}
	return cloned
}
