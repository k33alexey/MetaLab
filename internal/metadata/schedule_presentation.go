package metadata

import (
	"strconv"
	"strings"
	"time"
)

// Presentation is the schedule in words, as the prototype writes it under
// the pages of the window «Расписание» (docs/block-00/reglamentnoe-zadanie.md,
// «Досъёмка 28.10.2026»; snapshots configurator-2026-10-28/
// v3-v4-raspisanie-spisok/037-069): «Выполнять: », the days, «; » and the
// times of a day. The days are the dates, then, after a space, the
// repetition of days and, after commas, the days of the week, the months,
// the repetition of weeks, the day of the month and the day of the week in
// the month. The times are those of each detailed schedule of a day, joined
// by « и », or the schedule's own when it has none. A schedule of nothing
// runs one day, once that day. The schedule of a job left out is that.
func (schedule *JobSchedule) Presentation() string {
	if schedule == nil {
		schedule = &JobSchedule{}
	}
	return "Выполнять: " + schedule.daysPresentation() + "; " + schedule.timesPresentation()
}

var (
	monthsNamed = []string{"Январь", "Февраль", "Март", "Апрель", "Май", "Июнь", "Июль", "Август", "Сентябрь", "Октябрь", "Ноябрь", "Декабрь"}
	monthsOf    = []string{"января", "февраля", "марта", "апреля", "мая", "июня", "июля", "августа", "сентября", "октября", "ноября", "декабря"}
	weekDays    = []string{"понедельник", "вторник", "среда", "четверг", "пятница", "суббота", "воскресенье"}
)

func (schedule *JobSchedule) daysPresentation() string {
	var dates []string
	if schedule.BeginDate != "" {
		dates = append(dates, "с "+scheduleDate(schedule.BeginDate))
	}
	if schedule.EndDate != "" {
		dates = append(dates, "по "+scheduleDate(schedule.EndDate))
	}
	var parts []string
	switch schedule.DaysRepeatPeriod {
	case 0:
		parts = append(parts, "один день")
	case 1:
		parts = append(parts, "каждый день")
	default:
		parts = append(parts, "каждый "+groupedNumber(schedule.DaysRepeatPeriod)+"-й день")
	}
	// Every day of the week or month, or none, says nothing of them.
	if count := len(schedule.WeekDays); count > 0 && count < len(weekDays) {
		parts = append(parts, "по дням недели ["+strings.Join(namedNumbers(schedule.WeekDays, weekDays), ", ")+"]")
	}
	if count := len(schedule.Months); count > 0 && count < len(monthsNamed) {
		parts = append(parts, "по месяцам ["+strings.Join(namedNumbers(schedule.Months, monthsNamed), ", ")+"]")
	}
	if schedule.WeeksPeriod > 1 {
		parts = append(parts, "каждые "+groupedNumber(schedule.WeeksPeriod)+" недели")
	}
	if day := schedule.DayInMonth; day > 0 {
		parts = append(parts, strconv.Itoa(day)+"-го числа месяца")
	} else if day < 0 {
		parts = append(parts, strconv.Itoa(-day)+"-го дня с конца месяца")
	}
	if day := schedule.WeekDayInMonth; day > 0 {
		parts = append(parts, strconv.Itoa(day)+"-го дня недели в месяце")
	} else if day < 0 {
		parts = append(parts, strconv.Itoa(-day)+"-го дня недели с конца месяца")
	}
	return strings.Join(append(dates, strings.Join(parts, ", ")), " ")
}

func (schedule *JobSchedule) timesPresentation() string {
	if len(schedule.DetailedDailySchedules) > 0 {
		times := make([]string, len(schedule.DetailedDailySchedules))
		for index := range schedule.DetailedDailySchedules {
			times[index] = schedule.DetailedDailySchedules[index].timesPresentation()
		}
		return strings.Join(times, " и ")
	}
	var words []string
	if schedule.BeginTime != "" {
		words = append(words, "с "+schedule.BeginTime)
	}
	if schedule.EndTime != "" {
		words = append(words, "по "+schedule.EndTime)
	}
	if schedule.RepeatPeriodInDay > 0 {
		words = append(words, "каждые "+groupedNumber(schedule.RepeatPeriodInDay)+" секунд")
	} else {
		words = append(words, "один раз в день")
	}
	parts := []string{strings.Join(words, " ")}
	if schedule.CompletionTime != "" {
		parts = append(parts, "завершать после "+schedule.CompletionTime)
	}
	if schedule.CompletionInterval > 0 {
		parts = append(parts, "завершать через "+groupedNumber(schedule.CompletionInterval)+" секунд")
	}
	if schedule.RepeatPause > 0 {
		parts = append(parts, "повторять после завершения через "+groupedNumber(schedule.RepeatPause)+" секунд")
	}
	return strings.Join(parts, ", ")
}

// scheduleDate is a date of a schedule in words, «1 ноября 2026». A date the
// model would not take is written as it is.
func scheduleDate(date string) string {
	day, err := time.Parse(time.DateOnly, date)
	if err != nil {
		return date
	}
	return strconv.Itoa(day.Day()) + " " + monthsOf[day.Month()-1] + " " + strconv.Itoa(day.Year())
}

// namedNumbers names the days of the week or the months by their numbers,
// counted from one, in the order of the calendar.
func namedNumbers(numbers []int, names []string) []string {
	var named []string
	for number, name := range names {
		for _, given := range numbers {
			if given == number+1 {
				named = append(named, name)
				break
			}
		}
	}
	return named
}

// groupedNumber writes a number with its digits in groups of three parted by
// a space - «3 600», «1 410 065 407» - which the prototype's text breaks a
// line at, so it is an ordinary space.
func groupedNumber(number int) string {
	digits := strconv.Itoa(number)
	var grouped strings.Builder
	for index, digit := range digits {
		if index > 0 && (len(digits)-index)%3 == 0 {
			grouped.WriteByte(' ')
		}
		grouped.WriteRune(digit)
	}
	return grouped.String()
}
