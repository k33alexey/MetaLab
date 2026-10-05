package metadata

import (
	"fmt"
	"io"
	"maps"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/k33alexey/MetaLab/internal/project"
	"github.com/k33alexey/MetaLab/internal/schemadiff"
	"github.com/k33alexey/MetaLab/internal/uuid"
)

// RoutePointKind is what a point of a route does. The list is the platform's
// own and closed: an application draws a map out of these, it does not invent
// kinds of point.
type RoutePointKind string

const (
	StartPoint         RoutePointKind = "start"
	ActivityPoint      RoutePointKind = "activity"
	ConditionPoint     RoutePointKind = "condition"
	VariantChoicePoint RoutePointKind = "variant-choice"
	ProcessingPoint    RoutePointKind = "processing"
	SplitPoint         RoutePointKind = "split"
	MergePoint         RoutePointKind = "merge"
	NestedProcessPoint RoutePointKind = "nested-process"
	CompletionPoint    RoutePointKind = "completion"
)

// createsTasks says whether a point of this kind creates tasks, and so whether
// it describes them. Only an activity addresses them, groups them and explains
// itself: a nested process creates the tasks its processes are led by, and the
// prototype gives that point neither addressing nor a group nor an
// explanation - not in the help, not in what the designer saves.
func (kind RoutePointKind) createsTasks() bool {
	return kind == ActivityPoint || kind == NestedProcessPoint
}

// RouteEvent is one event of a route point a procedure of the object module
// may handle. The names are the prototype's own, written the way this model
// writes names.
type RouteEvent string

const (
	BeforeStartEvent                     RouteEvent = "before-start"
	InteractiveActivationProcessingEvent RouteEvent = "interactive-activation-processing"
	BeforeCreateTasksEvent               RouteEvent = "before-create-tasks"
	OnCreateTaskEvent                    RouteEvent = "on-create-task"
	OnExecuteEvent                       RouteEvent = "on-execute"
	CheckExecutionProcessingEvent        RouteEvent = "check-execution-processing"
	BeforeExecuteEvent                   RouteEvent = "before-execute"
	BeforeExecuteInteractivelyEvent      RouteEvent = "before-execute-interactively"
	ConditionCheckEvent                  RouteEvent = "condition-check"
	SwitchProcessingEvent                RouteEvent = "switch-processing"
	ProcessingEvent                      RouteEvent = "processing"
	BeforeCreateSubBusinessProcesses     RouteEvent = "before-create-sub-business-processes"
	OnCreateSubBusinessProcesses         RouteEvent = "on-create-sub-business-processes"
	OnCompleteEvent                      RouteEvent = "on-complete"
)

// routeEventsOfKind is what a point of each kind may be handled on. The help
// lists fourteen events of a route point and says which kind each is for; the
// designer saves the same sets. A split and a merge have none.
var routeEventsOfKind = map[RoutePointKind][]RouteEvent{
	StartPoint: {BeforeStartEvent},
	ActivityPoint: {InteractiveActivationProcessingEvent, BeforeCreateTasksEvent, OnCreateTaskEvent,
		OnExecuteEvent, CheckExecutionProcessingEvent, BeforeExecuteEvent, BeforeExecuteInteractivelyEvent},
	ConditionPoint:     {ConditionCheckEvent},
	VariantChoicePoint: {SwitchProcessingEvent},
	ProcessingPoint:    {ProcessingEvent},
	NestedProcessPoint: {BeforeCreateTasksEvent, OnCreateTaskEvent, BeforeCreateSubBusinessProcesses,
		OnCreateSubBusinessProcesses, OnExecuteEvent, BeforeExecuteEvent},
	CompletionPoint: {OnCompleteEvent},
}

// Branch says which way out of a point a transition goes. A condition has two
// ways out and a variant choice has one per variant; without the branch a map
// would keep its lines and lose what decides between them.
const (
	TrueBranch  = "true"
	FalseBranch = "false"
)

// RouteArea is where a point or a decoration sits on the drawing, in the
// coordinates the map is drawn in.
type RouteArea struct {
	Top    int `yaml:"top" json:"top"`
	Left   int `yaml:"left" json:"left"`
	Bottom int `yaml:"bottom" json:"bottom"`
	Right  int `yaml:"right" json:"right"`
}

// RouteVertex is one corner of a line: a transition bent around a point of the
// drawing, or a decorative line drawn by hand.
type RouteVertex struct {
	X int `yaml:"x" json:"x"`
	Y int `yaml:"y" json:"y"`
}

// AddressingValue fixes what a point addresses the tasks it creates by. The
// attribute is an addressing attribute of the kind of task the process
// creates, and an empty value leaves the attribute to the handler of the
// point.
//
// This is the whole of addressing that needs no application code: a point says
// "this step is for the role of accountant", and the addressing register turns
// that into the people who hold the role.
type AddressingValue struct {
	Attribute string `yaml:"attribute" json:"attribute"`
	Value     *Value `yaml:"value,omitempty" json:"value,omitempty"`
}

// RouteDecoration is a caption or a line drawn on the map. It changes nothing
// about where the process goes, and it is kept for exactly that reason: a map
// transferred without its decorations is not the map that was drawn.
//
// A decoration is one of two things: a shape placed on the map, or a
// decorative line drawn through its corners. A decorative line may be attached
// to a point or a shape at either end, as erp attaches captions to the steps
// they explain, and it then follows them when they move.
type RouteDecoration struct {
	Name     string        `yaml:"name" json:"name"`
	Title    LocalizedText `yaml:"title,omitempty" json:"title,omitempty"`
	Shape    string        `yaml:"shape,omitempty" json:"shape,omitempty"`
	Location *RouteArea    `yaml:"location,omitempty" json:"location,omitempty"`
	Line     []RouteVertex `yaml:"line,omitempty" json:"line,omitempty"`
	Look     *RouteLook    `yaml:"look,omitempty" json:"look,omitempty"`
	// FlipMode and Angle turn a shape: mirrored by the designer's flip code,
	// rotated by degrees.
	FlipMode int     `yaml:"flip_mode,omitempty" json:"flipMode,omitempty"`
	Angle    float64 `yaml:"angle,omitempty" json:"angle,omitempty"`
	// From, To and LineLook belong to a decorative line.
	From     *RouteEnd      `yaml:"from,omitempty" json:"from,omitempty"`
	To       *RouteEnd      `yaml:"to,omitempty" json:"to,omitempty"`
	LineLook *RouteLineLook `yaml:"line_look,omitempty" json:"lineLook,omitempty"`
}

// RouteVariant is one way out of a variant choice. The name is what the
// handler returns and what a line out of the point is marked with; the caption
// and the colour are how the variant is drawn.
type RouteVariant struct {
	Name      string        `yaml:"name" json:"name"`
	Title     LocalizedText `yaml:"title,omitempty" json:"title,omitempty"`
	BackColor *ColorValue   `yaml:"back_color,omitempty" json:"backColor,omitempty"`
}

// RoutePoint is one step of a route.
//
// The name is not a caption: it is the value a reference to a route point
// carries, and it is what a task stores to say where it stands. It is not what
// a handler is found by: the handlers are bound, event by event, to procedures
// of the object module, and in erp 26 of 96 bound procedures are not named
// after their point at all, some serving several points.
type RoutePoint struct {
	ID   uuid.UUID      `yaml:"id" json:"id"`
	Name string         `yaml:"name" json:"name"`
	Kind RoutePointKind `yaml:"kind" json:"kind"`
	// TaskDescription is what the tasks created at this point are called, and
	// Explanation is the hint shown beside the point.
	TaskDescription string        `yaml:"task_description,omitempty" json:"taskDescription,omitempty"`
	Explanation     string        `yaml:"explanation,omitempty" json:"explanation,omitempty"`
	Title           LocalizedText `yaml:"title,omitempty" json:"title,omitempty"`
	// Group makes one task for the whole group of addressees instead of one
	// task each.
	Group bool `yaml:"group,omitempty" json:"group,omitempty"`
	// NestedProcess is the business process started at a nested-process point.
	NestedProcess *uuid.UUID `yaml:"nested_process,omitempty" json:"nestedProcess,omitempty"`
	// Variants are the ways out of a variant-choice point.
	Variants []RouteVariant `yaml:"variants,omitempty" json:"variants,omitempty"`
	// TruePort and FalsePort are the ports of a condition its two branches
	// leave by - which side of the drawn box is "yes". The help draws the
	// true branch on the right by default; erp has one condition the other
	// way round.
	TruePort  int `yaml:"true_port,omitempty" json:"truePort,omitempty"`
	FalsePort int `yaml:"false_port,omitempty" json:"falsePort,omitempty"`
	// Addressing is what the tasks created here are addressed by.
	Addressing []AddressingValue `yaml:"addressing,omitempty" json:"addressing,omitempty"`
	// Location is where the point is drawn.
	Location *RouteArea `yaml:"location,omitempty" json:"location,omitempty"`
	// Handlers binds events of this point to procedures of the object module.
	// An event left out is not handled.
	Handlers map[RouteEvent]string `yaml:"handlers,omitempty" json:"handlers,omitempty"`
	Look     *RouteLook            `yaml:"look,omitempty" json:"look,omitempty"`
}

// RouteTransition is one line of the map, from a point to a point.
//
// Its name and caption are the drawn line's: "Линия1" and the "Да" written
// beside it. The ports say at which side of each point the line is attached.
type RouteTransition struct {
	Name     string        `yaml:"name,omitempty" json:"name,omitempty"`
	Title    LocalizedText `yaml:"title,omitempty" json:"title,omitempty"`
	From     string        `yaml:"from" json:"from"`
	To       string        `yaml:"to" json:"to"`
	FromPort int           `yaml:"from_port,omitempty" json:"fromPort,omitempty"`
	ToPort   int           `yaml:"to_port,omitempty" json:"toPort,omitempty"`
	// Branch is empty for a plain line, true or false out of a condition, and
	// the variant name out of a variant choice.
	Branch string `yaml:"branch,omitempty" json:"branch,omitempty"`
	// Vertices are the corners the line is bent around.
	Vertices []RouteVertex  `yaml:"vertices,omitempty" json:"vertices,omitempty"`
	Look     *RouteLook     `yaml:"look,omitempty" json:"look,omitempty"`
	LineLook *RouteLineLook `yaml:"line_look,omitempty" json:"lineLook,omitempty"`
}

// RouteMap is the points, the lines between them and what is drawn beside
// them. The drawing travels whole: a developer opening a transferred process
// must see the map they drew, not one a machine laid out afresh.
type RouteMap struct {
	Points      []RoutePoint      `yaml:"points,omitempty" json:"points,omitempty"`
	Transitions []RouteTransition `yaml:"transitions,omitempty" json:"transitions,omitempty"`
	Decorations []RouteDecoration `yaml:"decorations,omitempty" json:"decorations,omitempty"`
	// Look is the background, the grid and the print settings of the map.
	Look *RouteMapLook `yaml:"look,omitempty" json:"look,omitempty"`
}

// BusinessProcessDefinition describes a sequence of steps and the moves between
// them. Tasks are created by the kind of task it names; the route itself is
// modelled here and executed later.
type BusinessProcessDefinition struct {
	Format int           `yaml:"format" json:"format"`
	ID     uuid.UUID     `yaml:"id" json:"id"`
	Name   string        `yaml:"name" json:"name"`
	Title  LocalizedText `yaml:"title" json:"title"`
	// Presentations is how this object is named to the person using it -
	// see object_presentation.go.
	Presentations `yaml:",inline" json:",inline"` // A business process is edited and picked, but stands for nothing in one
	// line: the prototype gives it no presentation - see object_choice.go.
	EditType    EditType `yaml:"edit_type,omitempty" json:"editType,omitempty"`
	ObjectInput `yaml:",inline" json:",inline"`
	// BasedOn are the objects one of these may be made out of, the list the
	// command to make it offers - see based_on.go.
	BasedOn []uuid.UUID `yaml:"based_on,omitempty" json:"basedOn,omitempty"`
	// DataLock is how the platform locks a row of this object while it is
	// written, and DataLockFields are the fields it may be locked by - see
	// data_lock_settings.go.
	DataLock       project.DataLockControlMode `yaml:"data_lock,omitempty" json:"dataLock,omitempty"`
	DataLockFields []ObjectField               `yaml:"data_lock_fields,omitempty" json:"dataLockFields,omitempty"`
	// FullTextSearch is whether this object is in the full-text index at all -
	// see full_text_search.go.
	FullTextSearch FullTextSearchMode `yaml:"full_text_search,omitempty" json:"fullTextSearch,omitempty"`
	// DataHistorySettings is whether this object takes part in data history
	// and the two flags that go with it - see data_history.go.
	DataHistorySettings `yaml:",inline" json:",inline"`
	// AdditionalIndexes are the indexes this object asks the database for
	// beside the ones the platform builds - see additional_indexes.go.
	AdditionalIndexes []AdditionalIndex `yaml:"additional_indexes,omitempty" json:"additionalIndexes,omitempty"`

	Number DocumentNumber `yaml:"number" json:"number"`
	// Task is the kind of task this process creates at its points.
	Task *uuid.UUID `yaml:"task,omitempty" json:"task,omitempty"`
	// CreateTasksPrivileged creates tasks past the rights of whoever moved the
	// process: the step is the application's decision, not the user's.
	CreateTasksPrivileged bool                   `yaml:"create_tasks_privileged,omitempty" json:"createTasksPrivileged,omitempty"`
	Route                 RouteMap               `yaml:"route,omitempty" json:"route,omitempty"`
	Attributes            []Attribute            `yaml:"attributes,omitempty" json:"attributes,omitempty"`
	TableParts            []TablePart            `yaml:"table_parts,omitempty" json:"tableParts,omitempty"`
	Characteristics       []ObjectCharacteristic `yaml:"characteristics,omitempty" json:"characteristics,omitempty"`
	StandardAttributes    []StandardAttribute    `yaml:"standard_attributes,omitempty" json:"standardAttributes,omitempty"`
	Forms                 ObjectForms            `yaml:"forms,omitempty" json:"forms,omitempty"`
	Commands              []ObjectCommand        `yaml:"commands,omitempty" json:"commands,omitempty"`
	Templates             []ObjectTemplate       `yaml:"templates,omitempty" json:"templates,omitempty"`
	List                  ListSettings           `yaml:"list,omitempty" json:"list,omitempty"`
}

// DecodeBusinessProcess reads and validates one business process.
func DecodeBusinessProcess(source string, reader io.Reader, configuration project.Project) (BusinessProcessDefinition, error) {
	var value BusinessProcessDefinition
	if err := decodeStrict(source, reader, &value); err != nil {
		return BusinessProcessDefinition{}, err
	}
	issues := validateBase(value.Format, value.ID, value.Name, value.Title, configuration)
	issues = append(issues, validateNumberedObjectShape(numberedObjectShape{
		number:             value.Number,
		attributes:         value.Attributes,
		tableParts:         value.TableParts,
		forms:              value.Forms,
		list:               value.List,
		reservedName:       reservedBusinessProcessName,
		kind:               BusinessProcessKind,
		standardAttributes: value.StandardAttributes,
		presentation:       value.Presentations,
		input:              value.ObjectInput,
		basedOn:            value.BasedOn,
		dataLock:           value.DataLock,
		dataLockFields:     value.DataLockFields,
		fullTextSearch:     value.FullTextSearch,
		dataHistory:        value.DataHistorySettings,
		additionalIndexes:  value.AdditionalIndexes,
	}, configuration)...)
	if value.Task != nil && value.Task.IsZero() {
		issues = append(issues, "task must be a non-zero UUID")
	}
	if !validEditType(value.EditType) {
		issues = append(issues, "edit_type must be in-dialog, in-list or both-ways")
	}
	issues = append(issues, validateRouteMap(value.Route, configuration)...)
	issues = append(issues, validateObjectCommands(value.Commands, value.ID, configuration)...)
	issues = append(issues, validateObjectTemplates(value.Templates, configuration)...)
	issues = append(issues, validateObjectCharacteristics(value.Characteristics)...)
	if err := issuesError(source, value.Format, issues); err != nil {
		return BusinessProcessDefinition{}, err
	}
	return value, nil
}

func validateRouteMap(route RouteMap, configuration project.Project) []string {
	if len(route.Points) == 0 && len(route.Transitions) == 0 {
		return nil
	}
	var issues []string
	names, ids := map[string]bool{}, map[uuid.UUID]bool{}
	variants := map[string]map[string]bool{}
	kinds := map[string]RoutePointKind{}
	ports := map[string][2]int{}
	starts, completions := 0, 0
	for index, point := range route.Points {
		prefix := fmt.Sprintf("route.points[%d]", index)
		if point.ID.IsZero() {
			issues = append(issues, prefix+".id must be a non-zero UUID")
		}
		if ids[point.ID] {
			issues = append(issues, prefix+".id must be unique")
		}
		ids[point.ID] = true
		if !validIdentifier(point.Name) || utf8.RuneCountInString(point.Name) > maxNameLength {
			issues = append(issues, prefix+".name must be a valid identifier of at most 255 characters")
		}
		folded := strings.ToLower(point.Name)
		if names[folded] {
			issues = append(issues, prefix+".name must be unique: a reference to a route point carries this name")
		}
		names[folded] = true
		kinds[folded] = point.Kind
		// A caption is what the drawing shows; a point without one is drawn by
		// its name, and the platform's own maps leave the start uncaptioned.
		issues = append(issues, validateTitle(prefix+".title", point.Title, configuration)...)
		issues = append(issues, validateRouteArea(prefix+".location", point.Location)...)
		switch point.Kind {
		case StartPoint:
			starts++
		case CompletionPoint:
			completions++
		case ActivityPoint, ConditionPoint, ProcessingPoint, SplitPoint, MergePoint:
		case VariantChoicePoint:
			if len(point.Variants) < 2 {
				issues = append(issues, prefix+" of a variant choice needs at least two variants")
			}
		case NestedProcessPoint:
			if point.NestedProcess == nil || point.NestedProcess.IsZero() {
				issues = append(issues, prefix+" of a nested process needs the process it starts")
			}
		default:
			issues = append(issues, prefix+".kind is unsupported")
		}
		if point.NestedProcess != nil && point.Kind != NestedProcessPoint {
			issues = append(issues, prefix+".nested_process is allowed for a nested-process point only")
		}
		if len(point.Variants) > 0 && point.Kind != VariantChoicePoint {
			issues = append(issues, prefix+".variants are allowed for a variant choice only")
		}
		// Tasks are created where work is done and where a nested process is
		// waited for, and nowhere else; only work is addressed, grouped and
		// explained.
		if point.TaskDescription != "" && !point.Kind.createsTasks() {
			issues = append(issues, prefix+" carries task settings, which belong to a point that creates tasks")
		}
		if (point.Group || point.Explanation != "" || len(point.Addressing) > 0) && point.Kind != ActivityPoint {
			issues = append(issues, prefix+" carries a group, an explanation or addressing, which only an activity has")
		}
		issues = append(issues, validateRouteHandlers(prefix, point)...)
		issues = append(issues, validateAddressingValues(prefix, point.Addressing)...)
		seen := map[string]bool{}
		for number, variant := range point.Variants {
			path := fmt.Sprintf("%s.variants[%d]", prefix, number)
			if !validIdentifier(variant.Name) {
				issues = append(issues, path+".name must be a valid identifier: the handler returns the variant by it")
			}
			if seen[strings.ToLower(variant.Name)] {
				issues = append(issues, prefix+".variants repeat "+variant.Name)
			}
			seen[strings.ToLower(variant.Name)] = true
			issues = append(issues, validateTitle(path+".title", variant.Title, configuration)...)
			if variant.BackColor != nil {
				issues = append(issues, validateColorValue(path+".back_color", *variant.BackColor)...)
			}
		}
		variants[folded] = seen
		if (point.TruePort != 0 || point.FalsePort != 0) && point.Kind != ConditionPoint {
			issues = append(issues, prefix+" gives ports to branches, which only a condition has")
		}
		if point.TruePort < 0 || point.FalsePort < 0 || (point.TruePort != 0 && point.TruePort == point.FalsePort) {
			issues = append(issues, prefix+" must leave by two different ports for its two branches")
		}
		ports[folded] = [2]int{point.TruePort, point.FalsePort}
		issues = append(issues, validateRouteLook(prefix+".look", point.Look, true, configuration)...)
	}
	// A route may have several starts - the method that starts a process is
	// given the one to start from - but it needs one.
	if starts == 0 {
		issues = append(issues, "a route needs a start")
	}
	if completions == 0 {
		issues = append(issues, "a route needs a completion: a process that cannot finish never lets go of its tasks")
	}
	for index, transition := range route.Transitions {
		prefix := fmt.Sprintf("route.transitions[%d]", index)
		from, to := strings.ToLower(transition.From), strings.ToLower(transition.To)
		if !names[from] {
			issues = append(issues, prefix+".from names "+transition.From+", which is not a point of this route")
			continue
		}
		if !names[to] {
			issues = append(issues, prefix+".to names "+transition.To+", which is not a point of this route")
			continue
		}
		issues = append(issues, validateRouteTransitionLook(prefix, transition, configuration)...)
		switch kinds[from] {
		case ConditionPoint:
			if transition.Branch != TrueBranch && transition.Branch != FalseBranch {
				issues = append(issues, prefix+" out of a condition must say which branch it is: true or false")
			}
			// The branch is said twice in the prototype - by the line's port
			// and by the condition's port of that branch - and the two have
			// to agree, or the map shows one way and the process takes the
			// other.
			if pair := ports[from]; transition.FromPort != 0 && pair != [2]int{} {
				want := pair[0]
				if transition.Branch == FalseBranch {
					want = pair[1]
				}
				if transition.FromPort != want {
					issues = append(issues, prefix+" leaves the condition by the port of the other branch")
				}
			}
		case VariantChoicePoint:
			if !variants[from][strings.ToLower(transition.Branch)] {
				issues = append(issues, prefix+" out of a variant choice must name one of its variants")
			}
		case CompletionPoint:
			issues = append(issues, prefix+" leaves a completion, which is where a route ends")
		default:
			if transition.Branch != "" {
				issues = append(issues, prefix+".branch is meaningless out of this point")
			}
		}
	}
	issues = append(issues, validateRouteDecorations(route.Decorations, configuration)...)
	issues = append(issues, validateRouteNames(route)...)
	issues = append(issues, validateRouteMapLook(route.Look)...)
	// A point nobody can reach is a step that never runs, and a map drawn with
	// one is a map that lies about what the process does.
	reachable := map[string]bool{}
	for _, point := range route.Points {
		if point.Kind == StartPoint {
			reachable[strings.ToLower(point.Name)] = true
		}
	}
	for changed := true; changed; {
		changed = false
		for _, transition := range route.Transitions {
			from, to := strings.ToLower(transition.From), strings.ToLower(transition.To)
			if reachable[from] && !reachable[to] {
				reachable[to] = true
				changed = true
			}
		}
	}
	for index, point := range route.Points {
		if !reachable[strings.ToLower(point.Name)] {
			issues = append(issues, fmt.Sprintf("route.points[%d] %s is not reachable from the start", index, point.Name))
		}
	}
	return issues
}

// validateRouteHandlers keeps each handler on an event its point has and names
// a procedure by what a procedure may be called.
func validateRouteHandlers(prefix string, point RoutePoint) []string {
	var issues []string
	for event, procedure := range point.Handlers {
		if !slices.Contains(routeEventsOfKind[point.Kind], event) {
			issues = append(issues, fmt.Sprintf("%s.handlers.%s is not an event of a %s point", prefix, event, point.Kind))
		}
		if !validIdentifier(procedure) {
			issues = append(issues, fmt.Sprintf("%s.handlers.%s must name a procedure", prefix, event))
		}
	}
	return issues
}

// validateRouteArea keeps a drawn box from being drawn inside out: a point
// whose right edge is left of its left edge is not drawn at all.
func validateRouteArea(path string, area *RouteArea) []string {
	if area == nil {
		return nil
	}
	if area.Right < area.Left || area.Bottom < area.Top {
		return []string{path + " is drawn inside out"}
	}
	return nil
}

func validateAddressingValues(prefix string, values []AddressingValue) []string {
	var issues []string
	seen := map[string]bool{}
	for index, value := range values {
		path := fmt.Sprintf("%s.addressing[%d]", prefix, index)
		folded := strings.ToLower(value.Attribute)
		if !validIdentifier(value.Attribute) {
			issues = append(issues, path+".attribute must be a valid identifier")
			continue
		}
		if seen[folded] {
			issues = append(issues, path+".attribute repeats "+value.Attribute)
		}
		seen[folded] = true
	}
	return issues
}

func validateRouteDecorations(decorations []RouteDecoration, configuration project.Project) []string {
	var issues []string
	names := map[string]bool{}
	for index, decoration := range decorations {
		prefix := fmt.Sprintf("route.decorations[%d]", index)
		if !validIdentifier(decoration.Name) || utf8.RuneCountInString(decoration.Name) > maxNameLength {
			issues = append(issues, prefix+".name must be a valid identifier of at most 255 characters")
		}
		folded := strings.ToLower(decoration.Name)
		if names[folded] {
			issues = append(issues, prefix+".name must be unique")
		}
		names[folded] = true
		issues = append(issues, validateTitle(prefix+".title", decoration.Title, configuration)...)
		issues = append(issues, validateRouteArea(prefix+".location", decoration.Location)...)
		// A decoration that is neither placed nor drawn is nothing on the map.
		if decoration.Location == nil && len(decoration.Line) < 2 {
			issues = append(issues, prefix+" is neither placed on the map nor drawn as a line")
		}
		if decoration.Shape != "" && !slices.Contains(routeShapes, decoration.Shape) {
			issues = append(issues, fmt.Sprintf("%s.shape must be one of %v", prefix, routeShapes))
		}
		line := len(decoration.Line) > 0 || decoration.From != nil || decoration.To != nil || decoration.LineLook != nil
		if line && (decoration.Location != nil || decoration.Shape != "" || decoration.FlipMode != 0 || decoration.Angle != 0) {
			issues = append(issues, prefix+" is both a shape and a decorative line")
		}
		issues = append(issues, validateRouteLook(prefix+".look", decoration.Look, !line, configuration)...)
		if !line && decoration.Look != nil && decoration.Look.Border != nil {
			issues = append(issues, prefix+".look.border is not allowed: the outline of a shape is the shape")
		}
		if decoration.LineLook != nil {
			issues = append(issues, validateRouteLineLook(prefix+".line_look", *decoration.LineLook)...)
		}
		for end, attached := range map[string]*RouteEnd{"from": decoration.From, "to": decoration.To} {
			if attached != nil && attached.Port < 0 {
				issues = append(issues, prefix+"."+end+".port must not be negative")
			}
		}
	}
	return issues
}

// validateRouteTransitionLook checks what a transition carries beside where it
// goes: its name and caption, its ports and how it is drawn.
func validateRouteTransitionLook(prefix string, transition RouteTransition, configuration project.Project) []string {
	var issues []string
	if transition.Name != "" && (!validIdentifier(transition.Name) || utf8.RuneCountInString(transition.Name) > maxNameLength) {
		issues = append(issues, prefix+".name must be a valid identifier of at most 255 characters")
	}
	issues = append(issues, validateTitle(prefix+".title", transition.Title, configuration)...)
	if transition.FromPort < 0 || transition.ToPort < 0 {
		issues = append(issues, prefix+" ports must not be negative")
	}
	issues = append(issues, validateRouteLook(prefix+".look", transition.Look, false, configuration)...)
	if transition.LineLook != nil {
		issues = append(issues, validateRouteLineLook(prefix+".line_look", *transition.LineLook)...)
	}
	return issues
}

// validateRouteNames keeps one name for one item across the whole map, lines
// included: a line is attached to an item by its name, and the designer names
// every item of the map apart. A decorative line attached to an item has to
// find one - a point or a shape, not another line.
func validateRouteNames(route RouteMap) []string {
	var issues []string
	type owner struct {
		collection string
		index      int
	}
	owners := map[string]owner{}
	claim := func(name, collection string, index int) {
		if name == "" {
			return
		}
		folded := strings.ToLower(name)
		// Two points or two shapes of one name are refused where they are
		// checked; here only a name shared across kinds of item.
		if previous, ok := owners[folded]; ok && previous.collection != collection {
			issues = append(issues, fmt.Sprintf("route.%s[%d] and route.%s[%d] are both named %s: the items of a map are named apart",
				previous.collection, previous.index, collection, index, name))
			return
		}
		owners[folded] = owner{collection, index}
	}
	attachable := map[string]bool{}
	for index, point := range route.Points {
		claim(point.Name, "points", index)
		attachable[strings.ToLower(point.Name)] = true
	}
	lines := map[string]bool{}
	for index, transition := range route.Transitions {
		claim(transition.Name, "transitions", index)
		if transition.Name == "" {
			continue
		}
		if lines[strings.ToLower(transition.Name)] {
			issues = append(issues, fmt.Sprintf("route.transitions[%d].name must be unique", index))
		}
		lines[strings.ToLower(transition.Name)] = true
	}
	for index, decoration := range route.Decorations {
		claim(decoration.Name, "decorations", index)
		if len(decoration.Line) == 0 && decoration.From == nil && decoration.To == nil {
			attachable[strings.ToLower(decoration.Name)] = true
		}
	}
	for index, decoration := range route.Decorations {
		for end, attached := range map[string]*RouteEnd{"from": decoration.From, "to": decoration.To} {
			if attached == nil || attached.Item == "" {
				continue
			}
			if !attachable[strings.ToLower(attached.Item)] {
				issues = append(issues, fmt.Sprintf("route.decorations[%d].%s is attached to %s, which is not a point or a shape of this map", index, end, attached.Item))
			}
		}
	}
	return issues
}

// reservedBusinessProcessName keeps the standard attributes of a business
// process: its number and date, whether it is started and completed, and the
// task that heads it.
func reservedBusinessProcessName(name string) bool {
	return reservedStandardName(BusinessProcessKind, name) || reservedDataVersionName(name)
}

func cloneRouteMap(route RouteMap) RouteMap {
	route.Points = slices.Clone(route.Points)
	for index := range route.Points {
		point := &route.Points[index]
		point.Title = cloneTitle(point.Title)
		point.Variants = slices.Clone(point.Variants)
		for number := range point.Variants {
			point.Variants[number].Title = cloneTitle(point.Variants[number].Title)
			point.Variants[number].BackColor = clonePointer(point.Variants[number].BackColor)
		}
		point.Look = cloneRouteLook(point.Look)
		if point.NestedProcess != nil {
			id := *point.NestedProcess
			point.NestedProcess = &id
		}
		if point.Location != nil {
			area := *point.Location
			point.Location = &area
		}
		point.Handlers = maps.Clone(point.Handlers)
		point.Addressing = slices.Clone(point.Addressing)
		for value := range point.Addressing {
			if point.Addressing[value].Value != nil {
				held := *point.Addressing[value].Value
				point.Addressing[value].Value = &held
			}
		}
	}
	route.Transitions = slices.Clone(route.Transitions)
	for index := range route.Transitions {
		transition := &route.Transitions[index]
		transition.Vertices = slices.Clone(transition.Vertices)
		transition.Title = cloneTitle(transition.Title)
		transition.Look = cloneRouteLook(transition.Look)
		if transition.LineLook != nil {
			look := cloneRouteLineLook(*transition.LineLook)
			transition.LineLook = &look
		}
	}
	route.Look = cloneRouteMapLook(route.Look)
	route.Decorations = slices.Clone(route.Decorations)
	for index := range route.Decorations {
		decoration := &route.Decorations[index]
		decoration.Title = cloneTitle(decoration.Title)
		decoration.Line = slices.Clone(decoration.Line)
		decoration.Look = cloneRouteLook(decoration.Look)
		decoration.From = clonePointer(decoration.From)
		decoration.To = clonePointer(decoration.To)
		if decoration.LineLook != nil {
			look := cloneRouteLineLook(*decoration.LineLook)
			decoration.LineLook = &look
		}
		if decoration.Location != nil {
			area := *decoration.Location
			decoration.Location = &area
		}
	}
	return route
}

func cloneBusinessProcess(value BusinessProcessDefinition) BusinessProcessDefinition {
	value.Title = cloneTitle(value.Title)
	value.Attributes = cloneAttributes(value.Attributes)
	value.TableParts = cloneTableParts(value.TableParts)
	value.Route = cloneRouteMap(value.Route)
	if value.Task != nil {
		task := *value.Task
		value.Task = &task
	}
	value.Forms = cloneFormSet(value.Forms)
	value.List.SearchFields = slices.Clone(value.List.SearchFields)
	value.Commands = cloneObjectCommands(value.Commands)
	value.Templates = cloneObjectTemplates(value.Templates)
	value.Characteristics = cloneObjectCharacteristics(value.Characteristics)
	value.StandardAttributes = cloneStandardAttributes(value.StandardAttributes)
	value.Presentations = clonePresentations(value.Presentations)
	value.ObjectInput = cloneObjectInput(value.ObjectInput)
	value.BasedOn = slices.Clone(value.BasedOn)
	value.DataLockFields = cloneDataLockFields(value.DataLockFields)
	return value
}

// BusinessProcess returns one business process by name, folded case.
func (catalog *Catalog) BusinessProcess(name string) (BusinessProcessDefinition, bool) {
	index, ok := catalog.businessProcessByName[strings.ToLower(name)]
	if !ok {
		return BusinessProcessDefinition{}, false
	}
	return cloneBusinessProcess(catalog.BusinessProcesses[index]), true
}

// BusinessProcessByID returns one business process by its identifier.
func (catalog *Catalog) BusinessProcessByID(id uuid.UUID) (BusinessProcessDefinition, bool) {
	index, ok := catalog.businessProcessByID[id]
	if !ok {
		return BusinessProcessDefinition{}, false
	}
	return cloneBusinessProcess(catalog.BusinessProcesses[index]), true
}

func (catalog *Catalog) businessProcessTables(definition BusinessProcessDefinition) (schemadiff.Table, []schemadiff.Table, error) {
	tableName, err := PhysicalCatalogTable(definition.ID)
	if err != nil {
		return schemadiff.Table{}, nil, err
	}
	table := schemadiff.Table{
		Name: tableName,
		Columns: []schemadiff.Column{
			{Name: "ref", Type: "uuid", Nullable: false},
			{Name: "version", Type: "bigint", Nullable: false, Default: "1"},
			{Name: "date", Type: "timestamp with time zone", Nullable: false},
			// Started and completed are the two facts the platform itself
			// keeps about a process, and every list of processes filters by
			// them.
			{Name: "started", Type: "boolean", Nullable: false, Default: "false"},
			{Name: "completed", Type: "boolean", Nullable: false, Default: "false"},
			// The task a nested process was started from, empty for a process
			// nobody nested.
			{Name: "head_task", Type: "uuid", Nullable: true},
			{Name: "deletion_mark", Type: "boolean", Nullable: false, Default: "false"},
		},
		Constraints: []schemadiff.Constraint{
			{Name: physicalObjectName("pk", definition.ID), Type: "primary_key", Definition: "PRIMARY KEY (ref)"},
		},
		Indexes: []schemadiff.Index{
			{Name: physicalObjectName("im", definition.ID), Method: "btree", Keys: []string{"deletion_mark"}},
			{Name: physicalObjectName("id", definition.ID), Method: "btree", Keys: []string{"date"}},
		},
	}
	appendNumberColumn(&table, definition.ID, definition.Number)
	if definition.Task != nil {
		target, err := schemadiff.TableName(*definition.Task)
		if err != nil {
			return schemadiff.Table{}, nil, err
		}
		table.Constraints = append(table.Constraints, schemadiff.Constraint{
			Name: physicalObjectName("ft", definition.ID), Type: "foreign_key",
			Definition: "FOREIGN KEY (head_task) REFERENCES " + schemadiff.ApplicationSchema + "." + target + "(ref) DEFERRABLE INITIALLY DEFERRED",
		})
	}
	for _, attribute := range definition.Attributes {
		if err := catalog.appendAttributeSchema(&table, attribute); err != nil {
			return schemadiff.Table{}, nil, fmt.Errorf("business process %s attribute %s: %w", definition.Name, attribute.Name, err)
		}
	}
	appendListSearchIndexes(&table, definition.ID, definition.List, []string{"Number"}, definition.Attributes, map[string]listColumn{
		"number": {name: "number", kind: definition.Number.Type},
	})
	parts, err := catalog.tablePartTables("business process", definition.Name, definition.ID, definition.TableParts)
	if err != nil {
		return schemadiff.Table{}, nil, err
	}
	return table, parts, nil
}
