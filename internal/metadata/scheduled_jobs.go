package metadata

import (
	"fmt"
	"io"
	"strings"

	"github.com/k33alexey/MetaLab/internal/project"
	"github.com/k33alexey/MetaLab/internal/uuid"
)

// ScheduledJobKind holds the work an application does by itself, without
// anybody asking.
const ScheduledJobKind Kind = "scheduled-jobs"

const (
	// maxRestartCount and maxRestartInterval bound what a developer may ask
	// for after a job fails. The interval is in seconds.
	maxRestartCount    = 1000
	maxRestartInterval = 86_400
)

// ScheduledJobDefinition describes one scheduled job: a procedure the platform
// runs on its own.
//
// The schedule is deliberately absent. It is data of the database, not
// metadata: the same configuration runs nightly in one installation and hourly
// in another, and the administrator changes it without a developer.
type ScheduledJobDefinition struct {
	Format  int           `yaml:"format" json:"format"`
	ID      uuid.UUID     `yaml:"id" json:"id"`
	Name    string        `yaml:"name" json:"name"`
	Title   LocalizedText `yaml:"title" json:"title"`
	Comment string        `yaml:"comment,omitempty" json:"comment,omitempty"`
	// Description is how the job calls itself to the administrator watching
	// the jobs run. It is one string rather than a translated text, because
	// that is what the prototype keeps and what an administrator's list shows.
	Description string `yaml:"description,omitempty" json:"description,omitempty"`
	// Module and Procedure name what the job runs. It is always a procedure of
	// a common module: a job has no object of its own to run inside.
	Module    uuid.UUID `yaml:"module" json:"module"`
	Procedure string    `yaml:"procedure" json:"procedure"`
	// Key groups jobs that must not run at the same time. It is not unique and
	// is not meant to be: sharing one key is how two jobs are kept from
	// running together.
	Key string `yaml:"key,omitempty" json:"key,omitempty"`
	// Use says whether the job is allowed to start at all.
	Use bool `yaml:"use,omitempty" json:"use,omitempty"`
	// Predefined jobs are created in the database by the platform itself, so
	// that an installation has them without anybody adding them.
	Predefined               bool `yaml:"predefined,omitempty" json:"predefined,omitempty"`
	RestartCountOnFailure    int  `yaml:"restart_count_on_failure,omitempty" json:"restartCountOnFailure,omitempty"`
	RestartIntervalOnFailure int  `yaml:"restart_interval_on_failure,omitempty" json:"restartIntervalOnFailure,omitempty"`
}

// DecodeScheduledJob reads and validates one scheduled job.
func DecodeScheduledJob(source string, reader io.Reader, configuration project.Project) (ScheduledJobDefinition, error) {
	var value ScheduledJobDefinition
	if err := decodeStrict(source, reader, &value); err != nil {
		return ScheduledJobDefinition{}, err
	}
	issues := validateBase(value.Format, value.ID, value.Name, value.Title, configuration)
	if value.Module.IsZero() {
		issues = append(issues, "module is required: a job runs a procedure of a common module")
	}
	if !validIdentifier(value.Procedure) {
		issues = append(issues, "procedure must be a valid identifier")
	}
	if value.RestartCountOnFailure < 0 || value.RestartCountOnFailure > maxRestartCount {
		issues = append(issues, fmt.Sprintf("restart_count_on_failure must be 0..%d", maxRestartCount))
	}
	if value.RestartIntervalOnFailure < 0 || value.RestartIntervalOnFailure > maxRestartInterval {
		issues = append(issues, fmt.Sprintf("restart_interval_on_failure must be 0..%d seconds", maxRestartInterval))
	}
	// An interval with no repeats is not refused, though it reads as a
	// contradiction: five jobs of the reference configuration are written that
	// way, and refusing them would refuse the configuration.
	if err := issuesError(source, value.Format, issues); err != nil {
		return ScheduledJobDefinition{}, err
	}
	return value, nil
}

func cloneScheduledJob(value ScheduledJobDefinition) ScheduledJobDefinition {
	value.Title = cloneTitle(value.Title)
	return value
}

// ScheduledJob returns one scheduled job by name, folded case.
func (catalog *Catalog) ScheduledJob(name string) (ScheduledJobDefinition, bool) {
	index, ok := catalog.scheduledJobByName[strings.ToLower(name)]
	if !ok {
		return ScheduledJobDefinition{}, false
	}
	return cloneScheduledJob(catalog.ScheduledJobs[index]), true
}

// validateScheduledJobReferences checks that every job has something to run.
// A job runs with nobody watching, so a method that is not there fails at
// three in the morning rather than when the configuration is saved.
func (catalog *Catalog) validateScheduledJobReferences() error {
	for _, item := range catalog.ScheduledJobs {
		index, ok := catalog.commonModuleByID[item.Module]
		if !ok {
			return fmt.Errorf("scheduled job %s runs a procedure of unknown common module %s", item.Name, item.Module)
		}
		// The job runs in a background job on the server, and a module that
		// cannot run there cannot hold what it calls.
		if module := catalog.CommonModules[index]; !module.Server {
			return fmt.Errorf("scheduled job %s runs %s.%s, and that module is not a server module",
				item.Name, module.Name, item.Procedure)
		}
	}
	return nil
}
