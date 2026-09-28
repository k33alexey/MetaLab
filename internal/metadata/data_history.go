package metadata

// История данных — the prototype's own versioning of written data, carried by
// the model and not executed by the platform.
//
// Three properties travel together, and the syntax assistant gives all three
// to exactly ten kinds: a catalog, a document, the three charts, an exchange
// plan, a business process, a task, an information register and a constant.
// The demonstration configuration writes them on exactly those ten and on
// six hundred and twenty-nine objects in total. The other three registers, a
// sequence and a recalculation have nothing of the kind - and that is the
// difference from the full-text flag, which the four registers do have.
//
// The field's half of the mechanism is older than this file: an attribute, a
// dimension and a resource have the participation flag and only it. The two
// booleans below belong to the object alone. The requirements said "the same
// three properties are available on an attribute", and that was wrong - a
// field has one.
//
// Why it is carried and not executed is written out in METADATA-OBJECTS.md:
// data history keeps a version of every write of every included object with
// no retention limit, which on real volumes grows faster than the data it
// describes. What people actually turned it on for - seeing earlier values
// and who changed them - is ML's own change history instead. An object
// arriving with data history switched on must not silently start writing
// versions here, and must not silently lose the fact that it asked to.

// DataHistoryMode says whether the platform keeps the history of a value's
// changes. Two values, and there is no third: ИспользованиеИсторииДанных has
// Использовать and НеИспользовать, the same pair the field's flag has.
type DataHistoryMode string

const (
	DataHistoryUse     DataHistoryMode = "use"
	DataHistoryDontUse DataHistoryMode = "dont-use"
)

// DataHistorySettings is the object's half of the mechanism. It is embedded
// inline so that the keys stay flat in the file - a constant has been written
// with data_history beside its other properties since block 1, and the two
// that join it now are properties of the object in the same way.
type DataHistorySettings struct {
	DataHistory DataHistoryMode `yaml:"data_history,omitempty" json:"dataHistory,omitempty"`
	// UpdateDataHistoryImmediatelyAfterWrite asks for the version to be built
	// once the transaction is over, in a background job nobody waits for,
	// instead of on the schedule the history is otherwise updated by.
	UpdateDataHistoryImmediatelyAfterWrite bool `yaml:"update_data_history_immediately_after_write,omitempty" json:"updateDataHistoryImmediatelyAfterWrite,omitempty"`
	// ExecuteAfterDataHistoryVersionWriteProcessing lets the manager module
	// see a version once it is written, through the event
	// ОбработкаПослеЗаписиВерсийИсторииДанных.
	ExecuteAfterDataHistoryVersionWriteProcessing bool `yaml:"execute_after_data_history_version_write_processing,omitempty" json:"executeAfterDataHistoryVersionWriteProcessing,omitempty"`
}

func validDataHistoryMode(mode DataHistoryMode) bool {
	switch mode {
	case "", DataHistoryUse, DataHistoryDontUse:
		return true
	default:
		return false
	}
}

// validateDataHistory checks the participation flag and deliberately checks
// nothing about the two booleans beside it.
//
// The pair that asks to be refused is a flag switched on while the history is
// off: it reads as working and does nothing. It is left alone, for the reason
// the field's full-text flag is left alone inside an object outside the index
// - the setting is ready for the day the history is switched on, and the top
// flag beside it already says plainly that the day has not come. The
// demonstration configuration gives no evidence either way: all six hundred
// and twenty-nine objects have the history off and both booleans false, so a
// refusal here would fire only on configurations we have never seen, and
// refusing a real one costs more than carrying an idle setting.
//
// The field-level pair - an attribute in the history inside an object that is
// not - is written three thousand six hundred and thirty-five times in that
// same configuration against no object at all, so refusing it was never an
// option.
func validateDataHistory(settings DataHistorySettings) []string {
	if validDataHistoryMode(settings.DataHistory) {
		return nil
	}
	return []string{"data_history must be use or dont-use"}
}
