package metadata

// Predefined items are described in the configuration and live in the database,
// and those are two different places: the configuration says a catalog has an
// item named ОсновнойСклад, the database holds the row. Updating the
// configuration therefore raises a question the platform has to answer - does
// the row get created and brought into line with the description, or is that the
// developer's business?
//
// PredefinedDataUpdate is that answer, and it belongs to the object because the
// answer differs per object: a classifier filled from outside must not be
// overwritten by its own description, while a catalog of statuses the code
// refers to by name must always match it.
type PredefinedDataUpdate string

const (
	// PredefinedDataUpdateAuto defers the answer. In the prototype it falls
	// through to a setting of the whole infobase and then to the kind of node
	// of a distributed infobase: a root node or a non-distributed base updates
	// automatically, a subordinate node does not.
	//
	// ML has no distributed infobase - see the exchange plan, where the flag is
	// carried as unimplemented - and the infobase-wide setting is data rather
	// than configuration, the same way a scheduled job's schedule is. So for us
	// Auto resolves to updating automatically. It is kept as its own value
	// rather than folded into AutoUpdate because it is what 112 of the 118
	// objects of the demonstration configuration actually say, and rewriting it
	// on import would be answering a question the configuration left open.
	PredefinedDataUpdateAuto PredefinedDataUpdate = "auto"
	// PredefinedDataUpdateAutomatic brings the rows into line with the
	// description whenever the configuration is updated.
	PredefinedDataUpdateAutomatic PredefinedDataUpdate = "auto-update"
	// PredefinedDataUpdateManual leaves them alone. The developer keeps the
	// predefined data current, and the platform does not touch it.
	PredefinedDataUpdateManual PredefinedDataUpdate = "dont-auto-update"
)

func validPredefinedDataUpdate(mode PredefinedDataUpdate) bool {
	switch mode {
	case "", PredefinedDataUpdateAuto, PredefinedDataUpdateAutomatic, PredefinedDataUpdateManual:
		return true
	default:
		return false
	}
}

// validatePredefinedDataUpdate checks the setting and the one thing that can be
// wrong about it beyond a misspelling: the property answers what happens to
// predefined data, so a kind that keeps none has nothing to answer about.
//
// Only four kinds keep predefined data - a catalog, a chart of characteristic
// types, a chart of accounts and a chart of calculation types - and the help
// names exactly those four managers as the ones carrying the methods that read
// and write this setting. The other reference kind, an exchange plan, has no
// predefined items at all and no field to declare this in, so a file that tries
// is refused when it is decoded rather than here.
func validatePredefinedDataUpdate(mode PredefinedDataUpdate) []string {
	if validPredefinedDataUpdate(mode) {
		return nil
	}
	return []string{"predefined_data_update must be auto, auto-update or dont-auto-update"}
}

// UpdatesPredefinedDataAutomatically resolves the setting to the answer ML acts
// on, so that callers do not each re-derive what Auto means. It is the one place
// that knows Auto resolves to yes here, and the place to change when a
// distributed infobase or an infobase-wide setting arrives.
func UpdatesPredefinedDataAutomatically(mode PredefinedDataUpdate) bool {
	return mode != PredefinedDataUpdateManual
}
