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
			"restart_count_on_failure must be 0..1000"},
		"интервал больше суток": {"module: " + jobModule + "\nprocedure: Загрузить\nrestart_interval_on_failure: 90000",
			"restart_interval_on_failure must be 0..86400 seconds"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			_, err := DecodeScheduledJob("job.yaml", strings.NewReader(`format: 1
id: `+jobID+`
name: Задание
title: {ru: Задание}
`+broken.body+`
`), metadataManifest())
			if err == nil {
				t.Fatalf("%s: accepted", name)
			}
			if !strings.Contains(err.Error(), broken.want) {
				t.Fatalf("%s: refused for another reason: %v", name, err)
			}
		})
	}
}
