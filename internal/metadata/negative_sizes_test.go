package metadata

import (
	"slices"
	"testing"

	"github.com/k33alexey/MetaLab/internal/project"
)

// Every number of a description that cannot be below zero is refused by its
// own key, in one wording, whichever description holds it: the form's layout
// and a scheduled job name it alone, the rest under the path of what holds
// it. Before 2.379 the steps of a route map's grid, the margins of its print
// and the ports of a transition were refused as a group, the ports and the
// periods of a schedule in an order that changed from run to run. Defects
// caught: a number of those refused under a key other than its own, or not at
// all; the refusals of one description coming in a different order each time;
// a refusal with no path starting with a dot.
func TestEveryNumberThatCannotBeNegativeIsRefusedByItsKey(t *testing.T) {
	t.Parallel()
	configuration := project.Project{DefaultLanguage: "ru", Languages: []project.Language{{Code: "ru", Name: "Русский"}}}
	cases := map[string]struct {
		issues []string
		want   []string
	}{
		"the layout of a form": {validateFormLayout(FormLayout{Width: -1, Height: -1, Scale: -1}),
			[]string{"width must not be negative", "height must not be negative", "scale must not be negative"}},
		"the schedule of a job": {validateJobSchedule("schedule", &JobSchedule{CompletionInterval: -1, RepeatPeriodInDay: -1, RepeatPause: -1, WeeksPeriod: -1, DaysRepeatPeriod: -1}),
			[]string{"schedule.completion_interval must not be negative", "schedule.repeat_period_in_day must not be negative",
				"schedule.repeat_pause must not be negative", "schedule.weeks_period must not be negative", "schedule.days_repeat_period must not be negative"}},
		"the grid and the print of a route map": {validateRouteMapLook(&RouteMapLook{GridHorizontalStep: -1, GridVerticalStep: -2,
			Print: &RoutePrint{TopMargin: -1, LeftMargin: -1, BottomMargin: -1, RightMargin: -1}}),
			[]string{"route.look.grid_horizontal_step must not be negative", "route.look.grid_vertical_step must not be negative",
				"route.look.print.top_margin must not be negative", "route.look.print.left_margin must not be negative",
				"route.look.print.bottom_margin must not be negative", "route.look.print.right_margin must not be negative"}},
		"the ports of a transition": {validateRouteTransitionLook("route.transitions[0]", RouteTransition{FromPort: -1, ToPort: -1}, configuration),
			[]string{"route.transitions[0].from_port must not be negative", "route.transitions[0].to_port must not be negative"}},
	}
	for name, test := range cases {
		for _, want := range test.want {
			if !slices.Contains(test.issues, want) {
				t.Errorf("%s: %q is not among %q", name, want, test.issues)
			}
		}
		var negative []string
		for _, issue := range test.issues {
			if slices.Contains(test.want, issue) {
				negative = append(negative, issue)
			}
		}
		if !slices.Equal(negative, test.want) {
			t.Errorf("%s: refused in the order %q, want %q", name, negative, test.want)
		}
	}
	// A decorative line ends at two ports, either of which may be left out.
	decorations := validateRouteDecorations([]RouteDecoration{{Name: "Линия", Line: []RouteVertex{{}, {X: 10}},
		From: &RouteEnd{Port: -1}, To: &RouteEnd{Port: -1}}, {Name: "Вторая", Line: []RouteVertex{{}, {X: 10}}, To: &RouteEnd{Port: -1}}}, configuration)
	for _, want := range []string{"route.decorations[0].from.port must not be negative", "route.decorations[0].to.port must not be negative",
		"route.decorations[1].to.port must not be negative"} {
		if !slices.Contains(decorations, want) {
			t.Errorf("a decorative line: %q is not among %q", want, decorations)
		}
	}
	for _, issue := range decorations {
		if issue == "route.decorations[1].from.port must not be negative" {
			t.Error("a line with no start is refused for the port of its start")
		}
	}
}
