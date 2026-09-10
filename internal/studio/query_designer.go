package studio

import (
	"fmt"
	"sort"
	"strings"
	"unicode"

	"github.com/k33alexey/MetaLab/internal/metadata"
	"github.com/k33alexey/MetaLab/internal/project"
	"github.com/k33alexey/MetaLab/internal/querylang"
)

const maxQueryDesignerSources = 16

type QueryDesignerSchema struct {
	Sources []QueryDesignerSource `json:"sources"`
}

type QueryDesignerSource struct {
	Path   string               `json:"path"`
	Name   string               `json:"name"`
	Title  string               `json:"title"`
	Kind   string               `json:"kind"`
	Fields []QueryDesignerField `json:"fields"`
}

type QueryDesignerField struct {
	Name  string `json:"name"`
	Title string `json:"title"`
	Type  string `json:"type"`
}

type QueryDesign struct {
	Distinct bool                `json:"distinct"`
	Top      int                 `json:"top,omitempty"`
	Sources  []QueryDesignSource `json:"sources"`
	Fields   []QueryDesignField  `json:"fields"`
	Where    string              `json:"where,omitempty"`
}

type QueryDesignSource struct {
	Path       string `json:"path"`
	Alias      string `json:"alias"`
	Join       string `json:"join,omitempty"`
	LeftAlias  string `json:"leftAlias,omitempty"`
	LeftField  string `json:"leftField,omitempty"`
	RightField string `json:"rightField,omitempty"`
}

type QueryDesignField struct {
	Source    string `json:"source"`
	Field     string `json:"field"`
	Alias     string `json:"alias,omitempty"`
	Aggregate string `json:"aggregate,omitempty"`
	Group     bool   `json:"group,omitempty"`
	Order     string `json:"order,omitempty"`
}

type QueryDesignResult struct {
	Query string `json:"query"`
}

func (workspace *Workspace) QueryDesignerSchema() (QueryDesignerSchema, error) {
	workspace.mu.Lock()
	defer workspace.mu.Unlock()
	schema, err := workspace.queryDesignerSchemaLocked()
	if err != nil {
		return QueryDesignerSchema{}, err
	}
	return cloneQueryDesignerSchema(*schema), nil
}

func (workspace *Workspace) queryDesignerSchemaLocked() (*QueryDesignerSchema, error) {
	if workspace.querySchema != nil {
		return workspace.querySchema, nil
	}
	catalog, err := metadata.Load(workspace.root)
	if err != nil {
		return nil, err
	}
	language := catalog.Project.DefaultLanguage
	configured := catalog.Project.Languages
	var result QueryDesignerSchema
	appendSource := func(kind, name string, title metadata.LocalizedText, fields []QueryDesignerField) {
		resolved := title.Resolve(language, configured)
		if resolved == "" {
			resolved = name
		}
		result.Sources = append(result.Sources, QueryDesignerSource{Path: kind + "." + name, Name: name, Title: resolved, Kind: kind, Fields: fields})
	}
	for _, object := range catalog.Catalogs {
		fields := []QueryDesignerField{{Name: "Ссылка", Title: "Ссылка", Type: "catalog"}, {Name: "Версия", Title: "Версия", Type: "number"}, {Name: "Код", Title: "Код", Type: string(object.Code.Type)}, {Name: "Наименование", Title: "Наименование", Type: "string"}, {Name: "ПометкаУдаления", Title: "Пометка удаления", Type: "boolean"}, {Name: "ИмяПредопределенныхДанных", Title: "Имя предопределённых данных", Type: "string"}}
		fields = append(fields, queryDesignerAttributes(object.Attributes, language, configured)...)
		appendSource("Справочник", object.Name, object.Title, fields)
	}
	for _, object := range catalog.Documents {
		fields := []QueryDesignerField{{Name: "Ссылка", Title: "Ссылка", Type: "document"}, {Name: "Версия", Title: "Версия", Type: "number"}, {Name: "Номер", Title: "Номер", Type: string(object.Number.Type)}, {Name: "Дата", Title: "Дата", Type: "date"}, {Name: "Проведен", Title: "Проведён", Type: "boolean"}, {Name: "ПометкаУдаления", Title: "Пометка удаления", Type: "boolean"}}
		fields = append(fields, queryDesignerAttributes(object.Attributes, language, configured)...)
		appendSource("Документ", object.Name, object.Title, fields)
	}
	for _, object := range catalog.InformationRegisters {
		fields := []QueryDesignerField{{Name: "ИдентификаторЗаписи", Title: "Идентификатор записи", Type: "uuid"}}
		if object.Periodicity != metadata.InformationRegisterPeriodNone {
			fields = append(fields, QueryDesignerField{Name: "Период", Title: "Период", Type: "date"})
		}
		if object.WriteMode == metadata.InformationRegisterRecorder {
			fields = append(fields, recorderQueryDesignerFields()...)
		}
		fields = append(fields, queryDesignerAttributes(append(append(append([]metadata.Attribute{}, object.Dimensions...), object.Resources...), object.Attributes...), language, configured)...)
		appendSource("РегистрСведений", object.Name, object.Title, fields)
	}
	for _, object := range catalog.AccumulationRegisters {
		fields := []QueryDesignerField{{Name: "ИдентификаторЗаписи", Title: "Идентификатор записи", Type: "uuid"}, {Name: "Период", Title: "Период", Type: "date"}}
		fields = append(fields, recorderQueryDesignerFields()...)
		if object.Kind == metadata.AccumulationRegisterBalance {
			fields = append(fields, QueryDesignerField{Name: "ВидДвижения", Title: "Вид движения", Type: "movement-kind"})
		}
		fields = append(fields, queryDesignerAttributes(append(append(append([]metadata.Attribute{}, object.Dimensions...), object.Resources...), object.Attributes...), language, configured)...)
		appendSource("РегистрНакопления", object.Name, object.Title, fields)
	}
	sort.Slice(result.Sources, func(i, j int) bool {
		return strings.ToLower(result.Sources[i].Path) < strings.ToLower(result.Sources[j].Path)
	})
	workspace.querySchema = &result
	return workspace.querySchema, nil
}

func cloneQueryDesignerSchema(source QueryDesignerSchema) QueryDesignerSchema {
	result := QueryDesignerSchema{Sources: append([]QueryDesignerSource(nil), source.Sources...)}
	for index := range result.Sources {
		result.Sources[index].Fields = append([]QueryDesignerField(nil), source.Sources[index].Fields...)
	}
	return result
}

func queryDesignerAttributes(attributes []metadata.Attribute, language string, configured []project.Language) []QueryDesignerField {
	result := make([]QueryDesignerField, len(attributes))
	for index, attribute := range attributes {
		title := attribute.Title.Resolve(language, configured)
		if title == "" {
			title = attribute.Name
		}
		typeName := "composite"
		if len(attribute.Types) == 1 {
			typeName = string(attribute.Types[0].Kind)
		}
		result[index] = QueryDesignerField{Name: attribute.Name, Title: title, Type: typeName}
	}
	return result
}

func recorderQueryDesignerFields() []QueryDesignerField {
	return []QueryDesignerField{{Name: "Регистратор", Title: "Регистратор", Type: "recorder"}, {Name: "НомерСтроки", Title: "Номер строки", Type: "number"}, {Name: "Активность", Title: "Активность", Type: "boolean"}}
}

func (workspace *Workspace) BuildDesignedQuery(design QueryDesign) (QueryDesignResult, error) {
	workspace.mu.Lock()
	defer workspace.mu.Unlock()
	schema, err := workspace.queryDesignerSchemaLocked()
	if err != nil {
		return QueryDesignResult{}, err
	}
	query, err := buildDesignedQuery(*schema, design)
	if err != nil {
		return QueryDesignResult{}, err
	}
	return QueryDesignResult{Query: query}, nil
}

func buildDesignedQuery(schema QueryDesignerSchema, design QueryDesign) (string, error) {
	var err error
	design, err = normalizeDesignedRightJoin(design)
	if err != nil {
		return "", err
	}
	if len(design.Sources) == 0 || len(design.Sources) > maxQueryDesignerSources {
		return "", fmt.Errorf("query design must contain 1..%d sources", maxQueryDesignerSources)
	}
	if len(design.Fields) == 0 || len(design.Fields) > querylang.MaxResultFields {
		return "", fmt.Errorf("query design must contain 1..%d result fields", querylang.MaxResultFields)
	}
	if design.Top < 0 || design.Top > querylang.MaxTop {
		return "", fmt.Errorf("query TOP must be 0..%d", querylang.MaxTop)
	}
	sourceCatalog := make(map[string]QueryDesignerSource, len(schema.Sources))
	for _, source := range schema.Sources {
		sourceCatalog[strings.ToLower(source.Path)] = source
	}
	aliases := make(map[string]QueryDesignerSource, len(design.Sources))
	aliasPositions := make(map[string]int, len(design.Sources))
	for index, source := range design.Sources {
		definition, ok := sourceCatalog[strings.ToLower(source.Path)]
		if !ok {
			return "", fmt.Errorf("unknown query source %q", source.Path)
		}
		if !validQueryDesignerIdentifier(source.Alias) {
			return "", fmt.Errorf("source alias %q is invalid", source.Alias)
		}
		folded := strings.ToLower(source.Alias)
		if _, exists := aliases[folded]; exists {
			return "", fmt.Errorf("source alias %q is duplicated", source.Alias)
		}
		aliases[folded] = definition
		aliasPositions[folded] = index
		if index == 0 && source.Join != "" || index > 0 && source.Join == "" {
			return "", fmt.Errorf("join kind is inconsistent for source %q", source.Alias)
		}
	}
	fieldDefinition := func(alias, name string) (QueryDesignerField, bool) {
		source, ok := aliases[strings.ToLower(alias)]
		if !ok {
			return QueryDesignerField{}, false
		}
		for _, field := range source.Fields {
			if strings.EqualFold(field.Name, name) {
				return field, true
			}
		}
		return QueryDesignerField{}, false
	}
	fieldExists := func(alias, name string) bool {
		_, ok := fieldDefinition(alias, name)
		return ok
	}
	fieldExpression := func(field QueryDesignField) (string, error) {
		definition, exists := fieldDefinition(field.Source, field.Field)
		if !exists {
			return "", fmt.Errorf("unknown query field %s.%s", field.Source, field.Field)
		}
		expression := field.Source + "." + field.Field
		aggregates := map[string]string{"": "", "count": "КОЛИЧЕСТВО", "sum": "СУММА", "min": "МИНИМУМ", "max": "МАКСИМУМ", "avg": "СРЕДНЕЕ"}
		function, ok := aggregates[strings.ToLower(field.Aggregate)]
		if !ok {
			return "", fmt.Errorf("aggregate %q is unsupported", field.Aggregate)
		}
		if (function == "СУММА" || function == "СРЕДНЕЕ") && definition.Type != "number" && definition.Type != "defined-type" {
			return "", fmt.Errorf("aggregate %s requires a numeric field", function)
		}
		if (function == "МИНИМУМ" || function == "МАКСИМУМ") && definition.Type != "number" && definition.Type != "string" && definition.Type != "date" && definition.Type != "defined-type" {
			return "", fmt.Errorf("aggregate %s does not support field type %s", function, definition.Type)
		}
		if function != "" {
			expression = function + "(" + expression + ")"
		}
		if field.Alias != "" {
			if !validQueryDesignerIdentifier(field.Alias) {
				return "", fmt.Errorf("field alias %q is invalid", field.Alias)
			}
			expression += " КАК " + field.Alias
		}
		return expression, nil
	}
	selected := make([]string, len(design.Fields))
	groups, orders := make([]string, 0), make([]string, 0)
	resultAliases := make(map[string]bool, len(design.Fields))
	grouped := false
	for _, field := range design.Fields {
		grouped = grouped || field.Group || field.Aggregate != ""
	}
	for index, field := range design.Fields {
		expression, err := fieldExpression(field)
		if err != nil {
			return "", err
		}
		selected[index] = "    " + expression
		plain := field.Source + "." + field.Field
		if grouped && field.Aggregate == "" && !field.Group {
			return "", fmt.Errorf("non-aggregate field %s must be grouped", plain)
		}
		if field.Alias != "" {
			folded := strings.ToLower(field.Alias)
			if resultAliases[folded] {
				return "", fmt.Errorf("field alias %q is duplicated", field.Alias)
			}
			resultAliases[folded] = true
		}
		if field.Group {
			if field.Aggregate != "" {
				return "", fmt.Errorf("aggregate field %s cannot be grouped", plain)
			}
			groups = append(groups, "    "+plain)
		}
		orderExpression := plain
		if field.Alias != "" {
			orderExpression = field.Alias
		} else if field.Aggregate != "" {
			orderExpression = strings.TrimSuffix(expression, " КАК "+field.Alias)
		}
		switch strings.ToLower(field.Order) {
		case "":
		case "asc":
			orders = append(orders, "    "+orderExpression)
		case "desc":
			orders = append(orders, "    "+orderExpression+" УБЫВ")
		default:
			return "", fmt.Errorf("field order %q is unsupported", field.Order)
		}
	}
	header := "ВЫБРАТЬ"
	if design.Distinct {
		header += " РАЗЛИЧНЫЕ"
	}
	if design.Top > 0 {
		header += fmt.Sprintf(" ПЕРВЫЕ %d", design.Top)
	}
	var builder strings.Builder
	builder.WriteString(header + "\n" + strings.Join(selected, ",\n") + "\nИЗ\n")
	first := design.Sources[0]
	builder.WriteString("    " + first.Path + " КАК " + first.Alias)
	for index := 1; index < len(design.Sources); index++ {
		source := design.Sources[index]
		joinKind := strings.ToLower(source.Join)
		joins := map[string]string{"inner": "ВНУТРЕННЕЕ СОЕДИНЕНИЕ", "left": "ЛЕВОЕ СОЕДИНЕНИЕ", "full": "ПОЛНОЕ СОЕДИНЕНИЕ"}
		if joinKind == "cross" {
			builder.WriteString(",\n    " + source.Path + " КАК " + source.Alias)
			continue
		}
		join, ok := joins[joinKind]
		if !ok {
			return "", fmt.Errorf("join kind %q is unsupported", source.Join)
		}
		builder.WriteString("\n" + join + " " + source.Path + " КАК " + source.Alias)
		if !fieldExists(source.LeftAlias, source.LeftField) || !fieldExists(source.Alias, source.RightField) {
			return "", fmt.Errorf("join fields for source %q are invalid", source.Alias)
		}
		if aliasPositions[strings.ToLower(source.LeftAlias)] >= index {
			return "", fmt.Errorf("join source %q must reference an earlier source", source.Alias)
		}
		builder.WriteString("\nПО " + source.LeftAlias + "." + source.LeftField + " = " + source.Alias + "." + source.RightField)
	}
	if strings.TrimSpace(design.Where) != "" {
		builder.WriteString("\nГДЕ " + strings.TrimSpace(design.Where))
	}
	if len(groups) != 0 {
		builder.WriteString("\nСГРУППИРОВАТЬ ПО\n" + strings.Join(groups, ",\n"))
	}
	if len(orders) != 0 {
		builder.WriteString("\nУПОРЯДОЧИТЬ ПО\n" + strings.Join(orders, ",\n"))
	}
	query := builder.String()
	if _, err := querylang.Parse(query); err != nil {
		return "", err
	}
	return query, nil
}

func normalizeDesignedRightJoin(design QueryDesign) (QueryDesign, error) {
	position := -1
	for index, source := range design.Sources {
		if strings.EqualFold(source.Join, "right") {
			if position >= 0 || index != 1 {
				return QueryDesign{}, fmt.Errorf("right join can only be normalized as the first join")
			}
			position = index
		}
	}
	if position < 0 {
		return design, nil
	}
	design.Sources = append([]QueryDesignSource(nil), design.Sources...)
	left, right := design.Sources[0], design.Sources[1]
	design.Sources[0] = right
	design.Sources[0].Join = ""
	design.Sources[0].LeftAlias = ""
	design.Sources[0].LeftField = ""
	design.Sources[0].RightField = ""
	design.Sources[1] = left
	design.Sources[1].Join = "left"
	design.Sources[1].LeftAlias = right.Alias
	design.Sources[1].LeftField = right.RightField
	design.Sources[1].RightField = right.LeftField
	return design, nil
}

func validQueryDesignerIdentifier(value string) bool {
	runes := []rune(value)
	if len(runes) == 0 || len(runes) > 128 || !unicode.IsLetter(runes[0]) {
		return false
	}
	for _, symbol := range runes[1:] {
		if !unicode.IsLetter(symbol) && !unicode.IsDigit(symbol) {
			return false
		}
	}
	return true
}
