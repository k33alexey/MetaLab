package metadata

import (
	"fmt"
	"io"
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
	StartPoint          RoutePointKind = "start"
	ActivityPoint       RoutePointKind = "activity"
	ConditionPoint      RoutePointKind = "condition"
	VariantChoicePoint  RoutePointKind = "variant-choice"
	ProcessingPoint     RoutePointKind = "processing"
	SplitPoint          RoutePointKind = "split"
	MergePoint          RoutePointKind = "merge"
	NestedProcessPoint  RoutePointKind = "nested-process"
	CompletionPoint     RoutePointKind = "completion"
	maxRoutePointsPerBP                = 512
	maxRouteDecorations                = 512
	maxRouteVertices                   = 64
)

// createsTasks says whether a point of this kind creates tasks, and so whether
// the settings of those tasks belong on it.
func (kind RoutePointKind) createsTasks() bool {
	return kind == ActivityPoint || kind == NestedProcessPoint
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
type RouteDecoration struct {
	Name     string        `yaml:"name" json:"name"`
	Title    LocalizedText `yaml:"title,omitempty" json:"title,omitempty"`
	Shape    string        `yaml:"shape,omitempty" json:"shape,omitempty"`
	Location *RouteArea    `yaml:"location,omitempty" json:"location,omitempty"`
	Line     []RouteVertex `yaml:"line,omitempty" json:"line,omitempty"`
}

// RoutePoint is one step of a route.
//
// The name is not a caption: it is the value a reference to a route point
// carries, it is what the handler of that point is named after, and it is what
// a task stores to say where it stands.
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
	Variants []string `yaml:"variants,omitempty" json:"variants,omitempty"`
	// Addressing is what the tasks created here are addressed by.
	Addressing []AddressingValue `yaml:"addressing,omitempty" json:"addressing,omitempty"`
	// Location is where the point is drawn.
	Location *RouteArea `yaml:"location,omitempty" json:"location,omitempty"`
}

// RouteTransition is one line of the map, from a point to a point.
type RouteTransition struct {
	From string `yaml:"from" json:"from"`
	To   string `yaml:"to" json:"to"`
	// Branch is empty for a plain line, true or false out of a condition, and
	// the variant name out of a variant choice.
	Branch string `yaml:"branch,omitempty" json:"branch,omitempty"`
	// Vertices are the corners the line is bent around.
	Vertices []RouteVertex `yaml:"vertices,omitempty" json:"vertices,omitempty"`
}

// RouteMap is the points, the lines between them and what is drawn beside
// them. The drawing travels whole: a developer opening a transferred process
// must see the map they drew, not one a machine laid out afresh.
type RouteMap struct {
	Points      []RoutePoint      `yaml:"points,omitempty" json:"points,omitempty"`
	Transitions []RouteTransition `yaml:"transitions,omitempty" json:"transitions,omitempty"`
	Decorations []RouteDecoration `yaml:"decorations,omitempty" json:"decorations,omitempty"`
}

// BusinessProcessDefinition describes a sequence of steps and the moves between
// them. Tasks are created by the kind of task it names; the route itself is
// modelled here and executed later.
type BusinessProcessDefinition struct {
	Format int            `yaml:"format" json:"format"`
	ID     uuid.UUID      `yaml:"id" json:"id"`
	Name   string         `yaml:"name" json:"name"`
	Title  LocalizedText  `yaml:"title" json:"title"`
	Number DocumentNumber `yaml:"number" json:"number"`
	// Task is the kind of task this process creates at its points.
	Task *uuid.UUID `yaml:"task,omitempty" json:"task,omitempty"`
	// CreateTasksPrivileged creates tasks past the rights of whoever moved the
	// process: the step is the application's decision, not the user's.
	CreateTasksPrivileged bool             `yaml:"create_tasks_privileged,omitempty" json:"createTasksPrivileged,omitempty"`
	Route                 RouteMap         `yaml:"route,omitempty" json:"route,omitempty"`
	Attributes            []Attribute      `yaml:"attributes,omitempty" json:"attributes,omitempty"`
	TableParts            []TablePart      `yaml:"table_parts,omitempty" json:"tableParts,omitempty"`
	Forms                 ObjectForms      `yaml:"forms,omitempty" json:"forms,omitempty"`
	Commands              []ObjectCommand  `yaml:"commands,omitempty" json:"commands,omitempty"`
	Templates             []ObjectTemplate `yaml:"templates,omitempty" json:"templates,omitempty"`
	List                  ListSettings     `yaml:"list,omitempty" json:"list,omitempty"`
}

// DecodeBusinessProcess reads and validates one business process.
func DecodeBusinessProcess(source string, reader io.Reader, manifest project.Project) (BusinessProcessDefinition, error) {
	var value BusinessProcessDefinition
	if err := decodeStrict(source, reader, &value); err != nil {
		return BusinessProcessDefinition{}, err
	}
	issues := validateBase(value.Format, value.ID, value.Name, value.Title, manifest)
	issues = append(issues, validateNumberedObjectShape(numberedObjectShape{
		number:       value.Number,
		attributes:   value.Attributes,
		tableParts:   value.TableParts,
		forms:        value.Forms,
		list:         value.List,
		reservedName: reservedBusinessProcessName,
	}, manifest)...)
	if value.Task != nil && value.Task.IsZero() {
		issues = append(issues, "task must be a non-zero UUID")
	}
	issues = append(issues, validateRouteMap(value.Route, manifest)...)
	issues = append(issues, validateObjectCommands(value.Commands, value.ID, manifest)...)
	issues = append(issues, validateObjectTemplates(value.Templates, manifest)...)
	if err := issuesError(source, value.Format, issues); err != nil {
		return BusinessProcessDefinition{}, err
	}
	return value, nil
}

func validateRouteMap(route RouteMap, manifest project.Project) []string {
	if len(route.Points) == 0 && len(route.Transitions) == 0 {
		return nil
	}
	if len(route.Points) > maxRoutePointsPerBP {
		return []string{fmt.Sprintf("route.points must not contain more than %d points", maxRoutePointsPerBP)}
	}
	var issues []string
	names, ids := map[string]bool{}, map[uuid.UUID]bool{}
	variants := map[string]map[string]bool{}
	kinds := map[string]RoutePointKind{}
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
		if !validIdentifier(point.Name) || utf8.RuneCountInString(point.Name) > 128 {
			issues = append(issues, prefix+".name must be a valid identifier of at most 128 characters")
		}
		folded := strings.ToLower(point.Name)
		if names[folded] {
			issues = append(issues, prefix+".name must be unique: a reference to a route point carries this name")
		}
		names[folded] = true
		kinds[folded] = point.Kind
		// A caption is what the drawing shows; a point without one is drawn by
		// its name, and the platform's own maps leave the start uncaptioned.
		if len(point.Title) > 0 {
			issues = append(issues, validateTitle(prefix+".title", point.Title, manifest)...)
		}
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
		// waited for, and nowhere else.
		if (point.TaskDescription != "" || point.Group || len(point.Addressing) > 0) && !point.Kind.createsTasks() {
			issues = append(issues, prefix+" carries task settings, which belong to a point that creates tasks")
		}
		issues = append(issues, validateAddressingValues(prefix, point.Addressing)...)
		seen := map[string]bool{}
		for _, variant := range point.Variants {
			if seen[strings.ToLower(variant)] {
				issues = append(issues, prefix+".variants repeat "+variant)
			}
			seen[strings.ToLower(variant)] = true
		}
		variants[folded] = seen
	}
	if starts != 1 {
		issues = append(issues, "a route needs exactly one start")
	}
	if completions == 0 {
		issues = append(issues, "a route needs a completion: a process that cannot finish never lets go of its tasks")
	}
	for index, transition := range route.Transitions {
		prefix := fmt.Sprintf("route.transitions[%d]", index)
		if len(transition.Vertices) > maxRouteVertices {
			issues = append(issues, fmt.Sprintf("%s.vertices must not contain more than %d corners", prefix, maxRouteVertices))
		}
		from, to := strings.ToLower(transition.From), strings.ToLower(transition.To)
		if !names[from] {
			issues = append(issues, prefix+".from names "+transition.From+", which is not a point of this route")
			continue
		}
		if !names[to] {
			issues = append(issues, prefix+".to names "+transition.To+", which is not a point of this route")
			continue
		}
		switch kinds[from] {
		case ConditionPoint:
			if transition.Branch != TrueBranch && transition.Branch != FalseBranch {
				issues = append(issues, prefix+" out of a condition must say which branch it is: true or false")
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
	issues = append(issues, validateRouteDecorations(route.Decorations, manifest)...)
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

func validateRouteDecorations(decorations []RouteDecoration, manifest project.Project) []string {
	if len(decorations) > maxRouteDecorations {
		return []string{fmt.Sprintf("route.decorations must not contain more than %d items", maxRouteDecorations)}
	}
	var issues []string
	names := map[string]bool{}
	for index, decoration := range decorations {
		prefix := fmt.Sprintf("route.decorations[%d]", index)
		if !validIdentifier(decoration.Name) || utf8.RuneCountInString(decoration.Name) > 128 {
			issues = append(issues, prefix+".name must be a valid identifier of at most 128 characters")
		}
		folded := strings.ToLower(decoration.Name)
		if names[folded] {
			issues = append(issues, prefix+".name must be unique")
		}
		names[folded] = true
		if len(decoration.Title) > 0 {
			issues = append(issues, validateTitle(prefix+".title", decoration.Title, manifest)...)
		}
		issues = append(issues, validateRouteArea(prefix+".location", decoration.Location)...)
		if len(decoration.Line) > maxRouteVertices {
			issues = append(issues, fmt.Sprintf("%s.line must not contain more than %d corners", prefix, maxRouteVertices))
		}
		// A decoration that is neither placed nor drawn is nothing on the map.
		if decoration.Location == nil && len(decoration.Line) < 2 {
			issues = append(issues, prefix+" is neither placed on the map nor drawn as a line")
		}
		if decoration.Shape != "" && !validIdentifier(decoration.Shape) {
			issues = append(issues, prefix+".shape must be a valid identifier")
		}
	}
	return issues
}

// reservedBusinessProcessName keeps the standard attributes of a business
// process: its number and date, whether it is started and completed, and the
// task that heads it.
func reservedBusinessProcessName(name string) bool {
	switch strings.ToLower(name) {
	case "ссылка", "ref", "номер", "number", "дата", "date", "пометкаудаления", "deletionmark",
		"стартован", "started", "завершен", "завершён", "completed", "главнаязадача", "headtask", "версия", "version":
		return true
	default:
		return false
	}
}

func cloneRouteMap(route RouteMap) RouteMap {
	route.Points = slices.Clone(route.Points)
	for index := range route.Points {
		point := &route.Points[index]
		point.Title = cloneTitle(point.Title)
		point.Variants = slices.Clone(point.Variants)
		if point.NestedProcess != nil {
			id := *point.NestedProcess
			point.NestedProcess = &id
		}
		if point.Location != nil {
			area := *point.Location
			point.Location = &area
		}
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
		route.Transitions[index].Vertices = slices.Clone(route.Transitions[index].Vertices)
	}
	route.Decorations = slices.Clone(route.Decorations)
	for index := range route.Decorations {
		decoration := &route.Decorations[index]
		decoration.Title = cloneTitle(decoration.Title)
		decoration.Line = slices.Clone(decoration.Line)
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
	value.TableParts = slices.Clone(value.TableParts)
	for index := range value.TableParts {
		value.TableParts[index].Title = cloneTitle(value.TableParts[index].Title)
		value.TableParts[index].Attributes = cloneAttributes(value.TableParts[index].Attributes)
	}
	value.Route = cloneRouteMap(value.Route)
	for _, module := range []**uuid.UUID{&value.Task} {
		if *module != nil {
			id := **module
			*module = &id
		}
	}
	value.Forms = cloneObjectForms(value.Forms)
	value.List.SearchFields = slices.Clone(value.List.SearchFields)
	value.Commands = cloneObjectCommands(value.Commands)
	value.Templates = cloneObjectTemplates(value.Templates)
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
			{Name: "number", Type: documentNumberSQLType(definition.Number), Nullable: false},
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
	if definition.Number.Unique {
		table.Constraints = append(table.Constraints, schemadiff.Constraint{Name: physicalObjectName("uq", definition.ID), Type: "unique", Definition: "UNIQUE (number)"})
	} else {
		table.Indexes = append(table.Indexes, schemadiff.Index{Name: physicalObjectName("ic", definition.ID), Method: "btree", Keys: []string{"number"}})
	}
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
