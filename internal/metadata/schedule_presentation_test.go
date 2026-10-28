package metadata

import "testing"

// A schedule is written in the prototype's words. Each text is copied from
// the window «Расписание» of the configurator (docs/block-00/
// reglamentnoe-zadanie.md, «Досъёмка 28.10.2026»; snapshots configurator-
// 2026-10-28/v3-v4-raspisanie-spisok and reglamentnoe-zadanie/02), not built
// from the code. Defects caught: a part missing, out of its order or with
// the wrong joint - a comma after the dates, « и » between parts of one day;
// every day of the week or month written out; a number not grouped; the
// times of the schedule written beside detailed ones; a day from the end of
// the month written as from its start.
func TestAScheduleIsWrittenInThePrototypesWords(t *testing.T) {
	t.Parallel()
	every := []int{1, 2, 3, 4, 5, 6, 7}
	months := []int{2, 3, 4, 5, 6, 7, 8, 9, 10, 11}
	daily := JobSchedule{BeginTime: "03:00:00", RepeatPeriodInDay: 3600, CompletionTime: "05:00:00", CompletionInterval: 120, RepeatPause: 60}
	for name, test := range map[string]struct {
		schedule *JobSchedule
		want     string
	}{
		"нет расписания":   {nil, "Выполнять: один день; один раз в день"},
		"пустое":           {&JobSchedule{}, "Выполнять: один день; один раз в день"},
		"задание ERP (02)": {&JobSchedule{DayInMonth: 1, BeginTime: "02:00:00", EndTime: "06:00:00"}, "Выполнять: один день, 1-го числа месяца; с 02:00:00 по 06:00:00 один раз в день"},
		"даты, все дни недели (047)": {&JobSchedule{BeginDate: "2027-02-01", EndDate: "2027-01-15", DaysRepeatPeriod: 3, WeekDays: every, WeeksPeriod: 1},
			"Выполнять: с 1 февраля 2027 по 15 января 2027 каждый 3-й день; один раз в день"},
		"повтор недель (051)": {&JobSchedule{BeginDate: "2027-02-01", EndDate: "2027-01-15", DaysRepeatPeriod: 3, WeeksPeriod: 2},
			"Выполнять: с 1 февраля 2027 по 15 января 2027 каждый 3-й день, каждые 2 недели; один раз в день"},
		"месяцы и дни с конца (063)": {&JobSchedule{BeginDate: "2027-02-01", EndDate: "2027-01-15", DaysRepeatPeriod: 3, Months: months, WeeksPeriod: 2,
			DayInMonth: -31, WeekDayInMonth: -5},
			"Выполнять: с 1 февраля 2027 по 15 января 2027 каждый 3-й день, по месяцам [Февраль, Март, Апрель, Май, Июнь, Июль, Август, Сентябрь, Октябрь, Ноябрь], " +
				"каждые 2 недели, 31-го дня с конца месяца, 5-го дня недели с конца месяца; один раз в день"},
		"день недели (069)": {&JobSchedule{DaysRepeatPeriod: 3, WeekDays: []int{1}, Months: append([]int{1}, months...), DayInMonth: 1, WeekDayInMonth: 2},
			"Выполнять: каждый 3-й день, по дням недели [понедельник], по месяцам [Январь, Февраль, Март, Апрель, Май, Июнь, Июль, Август, Сентябрь, Октябрь, Ноябрь], " +
				"1-го числа месяца, 2-го дня недели в месяце; один раз в день"},
		"каждый день, только начало": {&JobSchedule{BeginDate: "2026-11-01", DaysRepeatPeriod: 1}, "Выполнять: с 1 ноября 2026 каждый день; один раз в день"},
		"дневное (043)": {&JobSchedule{BeginDate: "2027-02-01", EndDate: "2027-01-15", DaysRepeatPeriod: 3, BeginTime: "20:00:00", RepeatPeriodInDay: 1410065407,
			CompletionTime: "05:00:00", CompletionInterval: 120, RepeatPause: 60},
			"Выполнять: с 1 февраля 2027 по 15 января 2027 каждый 3-й день; с 20:00:00 каждые 1 410 065 407 секунд, завершать после 05:00:00, " +
				"завершать через 120 секунд, повторять после завершения через 60 секунд"},
		"детальное расписание дня (039)": {&JobSchedule{BeginDate: "2027-02-01", EndDate: "2027-01-15", DaysRepeatPeriod: 3, BeginTime: "23:00:00",
			DetailedDailySchedules: []JobSchedule{daily, {BeginTime: "10:00:00", RepeatPeriodInDay: 3600, CompletionTime: "05:00:00", CompletionInterval: 120, RepeatPause: 60}}},
			"Выполнять: с 1 февраля 2027 по 15 января 2027 каждый 3-й день; с 03:00:00 каждые 3 600 секунд, завершать после 05:00:00, завершать через 120 секунд, " +
				"повторять после завершения через 60 секунд и с 10:00:00 каждые 3 600 секунд, завершать после 05:00:00, завершать через 120 секунд, " +
				"повторять после завершения через 60 секунд"},
	} {
		if got := test.schedule.Presentation(); got != test.want {
			t.Errorf("%s:\n got %s\nwant %s", name, got, test.want)
		}
	}
}
