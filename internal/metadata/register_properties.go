package metadata

import "fmt"

// Properties the four kinds of register have and the model did not carry. Which
// kind has which is not a guess: the syntax assistant's own property list for
// each of the four says so, and it matters, because the sample in the
// demonstration configuration is two accumulation registers, two accounting ones
// and one calculation register - far too small to conclude anything from silence.
//
//	EditType, EnableTotalsSliceFirst, EnableTotalsSliceLast, MainFilterOnPeriod
//	                                        only the information register
//	EnableTotalsSplitting     the accumulation and the accounting register
//	PeriodAdjustmentLength                  only the accounting register

// EditType is how a user enters and edits a row: in a dialog of its own, in the
// list, or either way. The prototype shares this one enumeration across many
// kinds of object - a task, a business process, a chart of calculation types and
// more - so it is declared here once and reused as those kinds arrive.
type EditType string

const (
	EditInDialog EditType = "in-dialog"
	EditInList   EditType = "in-list"
	EditBothWays EditType = "both-ways"
)

func validEditType(mode EditType) bool {
	switch mode {
	case "", EditInDialog, EditInList, EditBothWays:
		return true
	default:
		return false
	}
}

// maxPeriodAdjustment is the prototype's ceiling on the period refinement, and
// the help states it outright: an integer from 0 to 3.
const maxPeriodAdjustment = 3

// validatePeriodAdjustmentLength checks the refinement of an accounting
// register's period. The refinement orders entries on the time axis beyond their
// period: of two entries with equal periods, the one with the smaller refinement
// is the earlier. Zero means the register does not support refinement at all,
// which is what both accounting registers of the demonstration configuration say.
//
// Note the name. The plan for this point called it "длина периода корректировки",
// a period of correction; the help calls it ДлинаУточненияПериода, the length of
// the period's refinement. They are not the same idea, and the second one is the
// prototype's.
func validatePeriodAdjustmentLength(length int) []string {
	if length < 0 || length > maxPeriodAdjustment {
		return []string{fmt.Sprintf("period_adjustment_length must be 0..%d", maxPeriodAdjustment)}
	}
	return nil
}

// InformationRegisterTotals is what an information register says about the extra
// totals tables that answer a slice quickly.
//
// A slice of the last records, or of the first, can always be computed from the
// records themselves; these settings ask the platform to keep a table of them
// as well, so that a query by the slice reads one row instead of searching. ML
// computes slices from the records and keeps no such table yet, so the settings
// are carried and not acted on - the report's second category - rather than read
// and dropped.
type InformationRegisterTotals struct {
	SliceFirst bool `yaml:"slice_first,omitempty" json:"sliceFirst,omitempty"`
	SliceLast  bool `yaml:"slice_last,omitempty" json:"sliceLast,omitempty"`
}

// validateInformationRegisterProperties checks the four properties that belong
// to an information register and to no other register.
//
// It deliberately does not require a periodicity for the slices or for the
// period filter, although in the demonstration configuration every register that
// sets one of them is periodic and all 234 non-periodic ones leave all three off.
// The help states no such rule, and the export proves nothing by silence: a rule
// invented here would refuse a configuration the prototype accepts, and refusing
// a valid configuration is worse than carrying a setting that says little.
func validateInformationRegisterProperties(value InformationRegisterDefinition) []string {
	var issues []string
	if !validEditType(value.EditType) {
		issues = append(issues, "edit_type must be in-dialog, in-list or both-ways")
	}
	return issues
}
