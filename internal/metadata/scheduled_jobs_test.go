package metadata

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const (
	jobID            = "d4000000-0000-4000-8000-000000000001"
	jobModule        = "d4000000-0000-4000-8000-000000000002"
	jobModuleSource  = "d4000000-0000-4000-8000-000000000003"
	jobClientModule  = "d4000000-0000-4000-8000-000000000004"
	jobClientSource  = "d4000000-0000-4000-8000-000000000005"
	jobMissingModule = "d4000000-0000-4000-8000-0000000000ff"
)

// jobProject writes a server common module for a job to run, and a client-only
// one beside it.
func jobProject(t *testing.T) string {
	t.Helper()
	root := metadataProject(t)
	for _, kind := range []Kind{CommonModuleKind, ScheduledJobKind} {
		if err := os.MkdirAll(filepath.Join(root, "metadata", string(kind)), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	for _, source := range []string{jobModuleSource, jobClientSource} {
		if err := os.WriteFile(filepath.Join(root, "modules", source+".bsl"),
			[]byte("Процедура ЗагрузитьКурсы() Экспорт\nКонецПроцедуры\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	writeMetadata(t, root, CommonModuleKind, jobModule, `format: 1
id: `+jobModule+`
name: РаботаСКурсами
title: {ru: Работа с курсами}
server: true
module: `+jobModuleSource+`
`)
	writeMetadata(t, root, CommonModuleKind, jobClientModule, `format: 1
id: `+jobClientModule+`
name: РаботаНаКлиенте
title: {ru: Работа на клиенте}
client: true
module: `+jobClientSource+`
`)
	return root
}

// A scheduled job is a procedure the platform runs by itself. Everything the
// administrator needs to understand a run has to survive - what it runs, under
// what name it shows itself, and what happens when it fails.
func TestScheduledJobKeepsWhatItRunsAndHowItRecovers(t *testing.T) {
	t.Parallel()
	root := jobProject(t)
	writeMetadata(t, root, ScheduledJobKind, jobID, `format: 1
id: `+jobID+`
name: ЗагрузкаКурсовВалют
title: {ru: Загрузка курсов валют}
comment: Ходит во внешний источник
description: Загрузка курсов валют
module: `+jobModule+`
procedure: ЗагрузитьКурсы
key: КурсыВалют
use: true
predefined: true
restart_count_on_failure: 10
restart_interval_on_failure: 600
`)
	catalog, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	job, ok := catalog.ScheduledJob("ЗагрузкаКурсовВалют")
	if !ok {
		t.Fatal("the scheduled job did not load")
	}
	switch {
	case job.Module.String() != jobModule || job.Procedure != "ЗагрузитьКурсы":
		t.Fatalf("what the job runs was lost: %+v", job)
	case job.Description != "Загрузка курсов валют":
		t.Fatalf("the name the administrator sees was lost: %+v", job)
	case job.Key != "КурсыВалют" || !job.Use || !job.Predefined:
		t.Fatalf("how the job is run was lost: %+v", job)
	case job.RestartCountOnFailure != 10 || job.RestartIntervalOnFailure != 600:
		t.Fatalf("what happens after a failure was lost: %+v", job)
	}
}

// A job runs with nobody watching. A method that is not there fails at three
// in the morning instead of when the configuration is saved.
func TestScheduledJobMustHaveSomethingToRun(t *testing.T) {
	t.Parallel()
	for name, broken := range map[string]struct{ body, want string }{
		"модуля не существует": {"module: " + jobMissingModule + "\nprocedure: ЗагрузитьКурсы",
			"unknown common module"},
		"модуль не серверный": {"module: " + jobClientModule + "\nprocedure: ЗагрузитьКурсы",
			"is not a server module"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			root := jobProject(t)
			writeMetadata(t, root, ScheduledJobKind, jobID, `format: 1
id: `+jobID+`
name: Задание
title: {ru: Задание}
`+broken.body+`
`)
			_, err := Load(root)
			if err == nil {
				t.Fatalf("%s: accepted", name)
			}
			if !strings.Contains(err.Error(), broken.want) {
				t.Fatalf("%s: refused for another reason: %v", name, err)
			}
		})
	}
}

// Two things that look like contradictions are deliberately allowed, because
// the reference configuration is written that way and refusing them would
// refuse the configuration: an interval to wait before repeating when no
// repeats are asked for, and a key shared by several jobs - which is how jobs
// are kept from running at the same time.
func TestScheduledJobsAllowWhatTheReferenceConfigurationWrites(t *testing.T) {
	t.Parallel()
	root := jobProject(t)
	writeMetadata(t, root, ScheduledJobKind, jobID, `format: 1
id: `+jobID+`
name: ОтложенноеОбновление
title: {ru: Отложенное обновление}
module: `+jobModule+`
procedure: ЗагрузитьКурсы
key: ОбщийКлюч
restart_count_on_failure: 0
restart_interval_on_failure: 90
`)
	writeMetadata(t, root, ScheduledJobKind, "d4000000-0000-4000-8000-000000000010", `format: 1
id: d4000000-0000-4000-8000-000000000010
name: РассылкаОтчетов
title: {ru: Рассылка отчётов}
module: `+jobModule+`
procedure: ЗагрузитьКурсы
key: ОбщийКлюч
`)
	if _, err := Load(root); err != nil {
		t.Fatal(err)
	}
}

// What is checked in the job itself is the shape of what it says.
func TestBrokenScheduledJobsAreRefused(t *testing.T) {
	t.Parallel()
	for name, broken := range map[string]struct{ body, want string }{
		"модуль не назван": {"procedure: ЗагрузитьКурсы", "module is required"},
		"метод не назван":  {"module: " + jobModule, "procedure must be a valid identifier"},
		"метод не идентификатор": {"module: " + jobModule + "\nprocedure: \"Загрузить курсы\"",
			"procedure must be a valid identifier"},
		"повторов отрицательное число": {"module: " + jobModule + "\nprocedure: Загрузить\nrestart_count_on_failure: -1",
			"restart_count_on_failure must not be negative"},
		"интервал отрицательный": {"module: " + jobModule + "\nprocedure: Загрузить\nrestart_interval_on_failure: -5",
			"restart_interval_on_failure must not be negative"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			_, err := DecodeScheduledJob("job.yaml", strings.NewReader(`format: 1
id: `+jobID+`
name: Задание
title: {ru: Задание}
`+broken.body+`
`), metadataConfiguration())
			if err == nil {
				t.Fatalf("%s: accepted", name)
			}
			if !strings.Contains(err.Error(), broken.want) {
				t.Fatalf("%s: refused for another reason: %v", name, err)
			}
		})
	}
}

// A job is shipped with the schedule the developer gave it, and the designer
// saves that schedule with the job - 74, 36 and 62 jobs of the configurations
// being moved have one. Defect caught: the model said a schedule is data of
// the database and carried none, so "every night at two" arrived as "never".
func TestScheduledJobCarriesItsSchedule(t *testing.T) {
	t.Parallel()
	root := jobProject(t)
	writeMetadata(t, root, ScheduledJobKind, jobID, `format: 1
id: `+jobID+`
name: ЗагрузкаКурсовВалют
title: {ru: Загрузка курсов валют}
module: `+jobModule+`
procedure: ЗагрузитьКурсы
schedule:
  begin_date: "2021-05-01"
  begin_time: "02:00:00"
  end_time: "06:00:00"
  completion_time: "04:00:00"
  completion_interval: 3600
  repeat_period_in_day: 600
  repeat_pause: 60
  week_day_in_month: -1
  day_in_month: -1
  weeks_period: 1
  days_repeat_period: 1
  week_days: [6, 7]
  months: [1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12]
  detailed_daily_schedules:
    - {begin_time: "01:00:00", week_days: [6]}
    - {begin_time: "14:00:00", week_days: [7]}
`)
	catalog, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	job, _ := catalog.ScheduledJob("ЗагрузкаКурсовВалют")
	schedule := job.Schedule
	if schedule == nil || schedule.BeginDate != "2021-05-01" || schedule.BeginTime != "02:00:00" || schedule.CompletionInterval != 3600 ||
		schedule.RepeatPeriodInDay != 600 || schedule.DayInMonth != -1 || len(schedule.WeekDays) != 2 || len(schedule.Months) != 12 {
		t.Fatalf("the schedule was lost: %+v", schedule)
	}
	if len(schedule.DetailedDailySchedules) != 2 || schedule.DetailedDailySchedules[1].BeginTime != "14:00:00" {
		t.Fatalf("the schedules of single days were lost: %+v", schedule.DetailedDailySchedules)
	}
	schedule.WeekDays[0] = 1
	schedule.DetailedDailySchedules[0].WeekDays[0] = 1
	again, _ := catalog.ScheduledJob("ЗагрузкаКурсовВалют")
	if again.Schedule.WeekDays[0] != 6 || again.Schedule.DetailedDailySchedules[0].WeekDays[0] != 6 {
		t.Fatal("the schedule handed out is shared with the catalog")
	}
}

// A schedule the calendar cannot keep is refused: a date or a time that is not
// one, a negative period, a thirty-second day, a sixth week, an eighth day of
// the week, a month twice - at any depth.
func TestScheduledJobScheduleRefusesWhatTheCalendarHasNot(t *testing.T) {
	t.Parallel()
	for name, broken := range map[string]struct{ schedule, want string }{
		"дата":              {`begin_date: "01.05.2021"`, "schedule.begin_date must be a date"},
		"время":             {`end_time: "25:00:00"`, "schedule.end_time must be a time of day"},
		"отрицательный":     {`repeat_pause: -1`, "schedule.repeat_pause must not be negative"},
		"день месяца":       {`day_in_month: 32`, "schedule.day_in_month must count at most 31"},
		"неделя месяца":     {`week_day_in_month: -6`, "schedule.week_day_in_month must count at most five"},
		"день недели":       {`week_days: [8]`, "schedule.week_days must be numbers from 1 to 7"},
		"месяц дважды":      {`months: [1, 1]`, "schedule.months must be numbers from 1 to 12"},
		"день в расписании": {`detailed_daily_schedules: [{begin_time: "1:00"}]`, "schedule.detailed_daily_schedules[0].begin_time"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			root := jobProject(t)
			writeMetadata(t, root, ScheduledJobKind, jobID, `format: 1
id: `+jobID+`
name: ЗагрузкаКурсовВалют
title: {ru: Загрузка курсов валют}
module: `+jobModule+`
procedure: ЗагрузитьКурсы
schedule: {`+broken.schedule+`}
`)
			_, err := Load(root)
			if err == nil || !strings.Contains(err.Error(), broken.want) {
				t.Fatalf("error = %v, want one saying %q", err, broken.want)
			}
		})
	}
}
