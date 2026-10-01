package metadata

import (
	"fmt"
	"io"
	"slices"
	"strings"
	"time"

	"github.com/k33alexey/MetaLab/internal/project"
	"github.com/k33alexey/MetaLab/internal/uuid"
)

// ScheduledJobKind holds the work an application does by itself, without
// anybody asking.
const ScheduledJobKind Kind = "scheduled-jobs"

// ScheduledJobDefinition describes one scheduled job: a procedure the platform
// runs on its own.
//
// The schedule here is the one the configuration ships with. The
// administrator of a running database changes it without a developer, and the
// same configuration then runs nightly in one installation and hourly in
// another - but a predefined job starts from the schedule the developer gave
// it, and the designer saves that schedule with the job: 74, 36 and 62 jobs of
// the configurations being moved have one. Without it a job transferred
// "every night at two" would wait for an administrator to remember that.
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
	// Schedule is when the job runs until the administrator says otherwise.
	// Without one the job runs only when started by hand.
	Schedule *JobSchedule `yaml:"schedule,omitempty" json:"schedule,omitempty"`
}

// JobSchedule is the prototype's schedule of a job, all fifteen properties the
// help gives it. Dates are written YYYY-MM-DD and times HH:MM:SS; a date or a
// time left out is not set, which the prototype writes as its empty date.
// Periods and pauses are in seconds.
type JobSchedule struct {
	BeginDate string `yaml:"begin_date,omitempty" json:"beginDate,omitempty"`
	EndDate   string `yaml:"end_date,omitempty" json:"endDate,omitempty"`
	BeginTime string `yaml:"begin_time,omitempty" json:"beginTime,omitempty"`
	EndTime   string `yaml:"end_time,omitempty" json:"endTime,omitempty"`
	// CompletionTime and CompletionInterval stop a run that is still going:
	// at a time of day, or that many seconds after it began.
	CompletionTime     string `yaml:"completion_time,omitempty" json:"completionTime,omitempty"`
	CompletionInterval int    `yaml:"completion_interval,omitempty" json:"completionInterval,omitempty"`
	// RepeatPeriodInDay repeats the job within a day, and RepeatPause is the
	// least time between the end of one run and the start of the next.
	RepeatPeriodInDay int `yaml:"repeat_period_in_day,omitempty" json:"repeatPeriodInDay,omitempty"`
	RepeatPause       int `yaml:"repeat_pause,omitempty" json:"repeatPause,omitempty"`
	// WeekDayInMonth and DayInMonth count from the start of the month when
	// positive and from its end when negative: -1 is the last one.
	WeekDayInMonth int `yaml:"week_day_in_month,omitempty" json:"weekDayInMonth,omitempty"`
	DayInMonth     int `yaml:"day_in_month,omitempty" json:"dayInMonth,omitempty"`
	// WeeksPeriod and DaysRepeatPeriod are every how many weeks and days.
	WeeksPeriod      int `yaml:"weeks_period,omitempty" json:"weeksPeriod,omitempty"`
	DaysRepeatPeriod int `yaml:"days_repeat_period,omitempty" json:"daysRepeatPeriod,omitempty"`
	// WeekDays are 1 for Monday to 7 for Sunday, Months 1 to 12.
	WeekDays []int `yaml:"week_days,omitempty" json:"weekDays,omitempty"`
	Months   []int `yaml:"months,omitempty" json:"months,omitempty"`
	// DetailedDailySchedules are schedules of their own for some days of the
	// week - two runs on Saturday where every other day has one.
	DetailedDailySchedules []JobSchedule `yaml:"detailed_daily_schedules,omitempty" json:"detailedDailySchedules,omitempty"`
}

// validateJobSchedule checks a schedule for what the calendar allows. There is
// no fifth weekday of a month past the fifth and no thirty-second day, and a
// period or a pause is never negative; nothing else is bounded.
func validateJobSchedule(path string, schedule *JobSchedule) []string {
	if schedule == nil {
		return nil
	}
	var issues []string
	for name, value := range map[string]string{"begin_date": schedule.BeginDate, "end_date": schedule.EndDate} {
		if value == "" {
			continue
		}
		if _, err := time.Parse(time.DateOnly, value); err != nil {
			issues = append(issues, path+"."+name+" must be a date written YYYY-MM-DD")
		}
	}
	for name, value := range map[string]string{"begin_time": schedule.BeginTime, "end_time": schedule.EndTime, "completion_time": schedule.CompletionTime} {
		if value == "" {
			continue
		}
		if _, err := time.Parse(time.TimeOnly, value); err != nil {
			issues = append(issues, path+"."+name+" must be a time of day written HH:MM:SS")
		}
	}
	for name, value := range map[string]int{"completion_interval": schedule.CompletionInterval, "repeat_period_in_day": schedule.RepeatPeriodInDay,
		"repeat_pause": schedule.RepeatPause, "weeks_period": schedule.WeeksPeriod, "days_repeat_period": schedule.DaysRepeatPeriod} {
		if value < 0 {
			issues = append(issues, path+"."+name+" must not be negative")
		}
	}
	if schedule.WeekDayInMonth < -5 || schedule.WeekDayInMonth > 5 {
		issues = append(issues, path+".week_day_in_month must count at most five weeks from either end of the month")
	}
	if schedule.DayInMonth < -31 || schedule.DayInMonth > 31 {
		issues = append(issues, path+".day_in_month must count at most 31 days from either end of the month")
	}
	issues = append(issues, validateCalendarNumbers(path+".week_days", schedule.WeekDays, 7)...)
	issues = append(issues, validateCalendarNumbers(path+".months", schedule.Months, 12)...)
	for index := range schedule.DetailedDailySchedules {
		issues = append(issues, validateJobSchedule(fmt.Sprintf("%s.detailed_daily_schedules[%d]", path, index), &schedule.DetailedDailySchedules[index])...)
	}
	return issues
}

func validateCalendarNumbers(path string, numbers []int, last int) []string {
	seen := map[int]bool{}
	for _, number := range numbers {
		if number < 1 || number > last || seen[number] {
			return []string{fmt.Sprintf("%s must be numbers from 1 to %d, each once", path, last)}
		}
		seen[number] = true
	}
	return nil
}

func cloneJobSchedule(schedule *JobSchedule) *JobSchedule {
	if schedule == nil {
		return nil
	}
	value := *schedule
	value.WeekDays = slices.Clone(value.WeekDays)
	value.Months = slices.Clone(value.Months)
	value.DetailedDailySchedules = slices.Clone(value.DetailedDailySchedules)
	for index := range value.DetailedDailySchedules {
		value.DetailedDailySchedules[index] = *cloneJobSchedule(&value.DetailedDailySchedules[index])
	}
	return &value
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
	// No ceiling on either: the help names none, and the configurations being
	// moved go up to 10 repeats and an hour.
	if value.RestartCountOnFailure < 0 {
		issues = append(issues, "restart_count_on_failure must not be negative")
	}
	if value.RestartIntervalOnFailure < 0 {
		issues = append(issues, "restart_interval_on_failure must not be negative")
	}
	issues = append(issues, validateJobSchedule("schedule", value.Schedule)...)
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
	value.Schedule = cloneJobSchedule(value.Schedule)
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
