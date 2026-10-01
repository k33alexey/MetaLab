package metadata

// Posting is six settings, not one flag. Whether a document may be posted at all
// is the first of them; the other five say when it may be posted, what happens
// to the records it wrote last time, which of those records get written this
// time, and whether the two operations ignore access rights.
//
// The values are the prototype's, read off the demonstration configuration and
// confirmed against the syntax assistant's own enumeration types.

// RealTimePosting is whether a document may be posted in real time - "by the
// current moment", where the platform assigns the date itself and the posting
// competes with other writers for the present. A document that is only ever
// posted after the fact denies it.
type RealTimePosting string

const (
	RealTimePostingAllow RealTimePosting = "allow"
	RealTimePostingDeny  RealTimePosting = "deny"
)

func validRealTimePosting(mode RealTimePosting) bool {
	switch mode {
	case "", RealTimePostingAllow, RealTimePostingDeny:
		return true
	default:
		return false
	}
}

// AllowsRealTimePosting resolves the setting. Unstated means allowed: that is
// the prototype's own default and what 19 of the 25 documents of the
// demonstration configuration say outright.
func AllowsRealTimePosting(mode RealTimePosting) bool {
	return mode != RealTimePostingDeny
}

// RegisterRecordsDeletion is what happens to the records a document wrote into
// registers last time. There are three answers and the difference between two of
// them is easy to miss: deleting automatically wipes the records both when
// posting starts and when posting is undone, while deleting on unposting leaves
// them in place at the start of posting - the handler then sees what it wrote
// last time and decides.
type RegisterRecordsDeletion string

const (
	// RegisterRecordsDeleteAuto wipes the records at the start of posting and on
	// unposting alike.
	RegisterRecordsDeleteAuto RegisterRecordsDeletion = "auto"
	// RegisterRecordsDeleteOnUnpost wipes them only when posting is undone. The
	// help is explicit that nothing is deleted when posting begins.
	RegisterRecordsDeleteOnUnpost RegisterRecordsDeletion = "on-unpost"
	// RegisterRecordsDeleteOff never wipes them. The handler owns the records
	// entirely.
	RegisterRecordsDeleteOff RegisterRecordsDeletion = "off"
)

func validRegisterRecordsDeletion(mode RegisterRecordsDeletion) bool {
	switch mode {
	case "", RegisterRecordsDeleteAuto, RegisterRecordsDeleteOnUnpost, RegisterRecordsDeleteOff:
		return true
	default:
		return false
	}
}

// DeletesRegisterRecords answers the only question the posting path has to ask:
// for this write, are the previous records wiped before the handler runs?
//
// Unstated means "auto", which is what ML did before the setting existed - the
// records were always wiped, on posting and on unposting both. Keeping that as
// the meaning of silence is what makes the setting an addition rather than a
// change: a document that says nothing behaves exactly as it did.
func DeletesRegisterRecords(mode RegisterRecordsDeletion, writeMode DocumentWriteMode) bool {
	switch mode {
	case RegisterRecordsDeleteOff:
		return false
	case RegisterRecordsDeleteOnUnpost:
		return writeMode == DocumentUndoPosting
	default:
		return true
	}
}

// RegisterRecordsWriting is which record sets the platform writes when posting
// finishes. Writing the modified ones means every set involved starts out marked
// for writing; writing the selected ones means the document clears that mark on
// all of them before posting begins, and the handler marks the sets it actually
// filled.
//
// ML does not act on this yet: a record set has no "write" mark of its own here,
// so there is nothing for the setting to flip. It is carried so that the
// distinction survives an import - the report's second category - rather than
// being read and dropped.
type RegisterRecordsWriting string

const (
	RegisterRecordsWriteSelected RegisterRecordsWriting = "selected"
	RegisterRecordsWriteModified RegisterRecordsWriting = "modified"
)

func validRegisterRecordsWriting(mode RegisterRecordsWriting) bool {
	switch mode {
	case "", RegisterRecordsWriteSelected, RegisterRecordsWriteModified:
		return true
	default:
		return false
	}
}

// SequenceFilling is whether the sequences a document takes part in are filled
// as it is written, or left to the code. Which sequences those are is not here:
// that composition belongs to the sequence, which names its documents - see
// SequenceDefinition.Documents. This is the mode only.
type SequenceFilling string

const (
	SequenceFillAuto SequenceFilling = "auto"
	SequenceFillOff  SequenceFilling = "off"
)

func validSequenceFilling(mode SequenceFilling) bool {
	switch mode {
	case "", SequenceFillAuto, SequenceFillOff:
		return true
	default:
		return false
	}
}

// DocumentPosting is everything a document says about being posted, gathered
// into one place because the six settings are read together and because a
// document, unlike its number, has exactly one of these.
type DocumentPosting struct {
	// Allowed is whether the document may be posted at all.
	Allowed bool `yaml:"allowed,omitempty" json:"allowed,omitempty"`
	// RealTime is whether it may be posted by the current moment.
	RealTime RealTimePosting `yaml:"real_time,omitempty" json:"realTime,omitempty"`
	// RecordsDeletion is what happens to the records written last time.
	RecordsDeletion RegisterRecordsDeletion `yaml:"records_deletion,omitempty" json:"recordsDeletion,omitempty"`
	// RecordsWriting is which record sets are written when posting finishes.
	RecordsWriting RegisterRecordsWriting `yaml:"records_writing,omitempty" json:"recordsWriting,omitempty"`
	// SequenceFilling is whether sequences are filled as the document is
	// written.
	SequenceFilling SequenceFilling `yaml:"sequence_filling,omitempty" json:"sequenceFilling,omitempty"`
	// Privileged and UnpostPrivileged let posting and unposting ignore access
	// rights. Every one of the 25 documents of the demonstration configuration
	// sets both, which is why they are carried rather than assumed off: a
	// posting that silently lost its privileges would fail on the rights of
	// whoever happened to press the button.
	//
	// ML does not act on these yet - posting runs under the caller's rights -
	// so they are model only, and that is deliberate: the setting is recorded
	// now so that the import does not lose it, and honoured when the privileged
	// mode itself arrives.
	Privileged       bool `yaml:"privileged,omitempty" json:"privileged,omitempty"`
	UnpostPrivileged bool `yaml:"unpost_privileged,omitempty" json:"unpostPrivileged,omitempty"`
}

// validateDocumentPosting checks each of the six settings against the values
// the prototype gives it.
func validateDocumentPosting(posting DocumentPosting) []string {
	var issues []string
	if !validRealTimePosting(posting.RealTime) {
		issues = append(issues, "posting.real_time must be allow or deny")
	}
	if !validRegisterRecordsDeletion(posting.RecordsDeletion) {
		issues = append(issues, "posting.records_deletion must be auto, on-unpost or off")
	}
	if !validRegisterRecordsWriting(posting.RecordsWriting) {
		issues = append(issues, "posting.records_writing must be selected or modified")
	}
	if !validSequenceFilling(posting.SequenceFilling) {
		issues = append(issues, "posting.sequence_filling must be auto or off")
	}
	// The six settings are carried whatever posting.allowed says. With posting
	// forbidden the designer greys out real-time posting and records deletion
	// but keeps their values, and leaves the privileged modes editable
	// (checked by the owner on the platform, 01.10.2026); the configurations
	// being moved write all six for every one of their 48 unposted documents,
	// with values that differ from one to the next - and seven of those
	// documents do have register records, written by code.
	return issues
}
