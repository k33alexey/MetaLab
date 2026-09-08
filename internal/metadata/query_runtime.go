package metadata

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"unicode/utf8"

	"github.com/k33alexey/MetaLab/internal/bsl/bytecode"
)

type queryObject struct {
	mu         sync.RWMutex
	runtime    *Runtime
	text       string
	parameters map[string]bytecode.Value
}

type queryResultObject struct {
	runtime *Runtime
	columns []string
	rows    [][]bytecode.Value
}

type querySelectionObject struct {
	mu      sync.RWMutex
	result  *queryResultObject
	current int
}

func (*queryObject) RuntimeTypeName() string { return "Query" }
func (object *queryObject) RuntimeEqual(other bytecode.RuntimeObject) bool {
	candidate, ok := other.(*queryObject)
	return ok && candidate == object
}
func (object *queryObject) RuntimeDynamicMemory(limit uint64) (uint64, bool) {
	object.mu.RLock()
	defer object.mu.RUnlock()
	size := uint64(256 + len(object.text))
	if size > limit {
		return limit, false
	}
	for name, value := range object.parameters {
		memory, ok := value.DynamicMemory(limit - min(size, limit))
		if !ok || memory > limit-size {
			return limit, false
		}
		size += uint64(len(name)+32) + memory
		if size > limit {
			return limit, false
		}
	}
	return size, true
}

func (*queryResultObject) RuntimeTypeName() string { return "QueryResult" }
func (object *queryResultObject) RuntimeEqual(other bytecode.RuntimeObject) bool {
	candidate, ok := other.(*queryResultObject)
	return ok && candidate == object
}
func (object *queryResultObject) RuntimeDynamicMemory(limit uint64) (uint64, bool) {
	size := uint64(256)
	for _, name := range object.columns {
		size += uint64(len(name) + 32)
	}
	if size > limit {
		return limit, false
	}
	for _, row := range object.rows {
		size += uint64(len(row) * 16)
		if size > limit {
			return limit, false
		}
		for _, value := range row {
			memory, ok := value.DynamicMemory(limit - size)
			if !ok || memory > limit-size {
				return limit, false
			}
			size += memory
			if size > limit {
				return limit, false
			}
		}
	}
	return size, size <= limit
}

func (*querySelectionObject) RuntimeTypeName() string { return "QueryResultSelection" }
func (object *querySelectionObject) RuntimeEqual(other bytecode.RuntimeObject) bool {
	candidate, ok := other.(*querySelectionObject)
	return ok && candidate == object
}
func (object *querySelectionObject) RuntimeDynamicMemory(limit uint64) (uint64, bool) {
	object.mu.RLock()
	defer object.mu.RUnlock()
	if limit < 128 {
		return limit, false
	}
	memory, ok := object.result.RuntimeDynamicMemory(limit - 128)
	if !ok {
		return limit, false
	}
	return memory + 128, memory+128 <= limit
}

func (runtime *Runtime) constructQuery(arguments []bytecode.Value) (bytecode.Value, error) {
	if len(arguments) > 1 {
		return bytecode.Undefined(), fmt.Errorf("Query expects zero or one text argument")
	}
	text := ""
	if len(arguments) == 1 {
		var ok bool
		text, ok = arguments[0].AsString()
		if !ok {
			return bytecode.Undefined(), fmt.Errorf("Query text must be a string")
		}
		if !validQueryText(text, true) {
			return bytecode.Undefined(), fmt.Errorf("query text must not exceed %d UTF-8 bytes", maxQueryTextBytes)
		}
	}
	return bytecode.Object(&queryObject{runtime: runtime, text: text, parameters: make(map[string]bytecode.Value)})
}

func (runtime *Runtime) getQueryProperty(value bytecode.RuntimeObject, name string) (bytecode.Value, bool, error) {
	switch object := value.(type) {
	case *queryObject:
		if object.runtime != runtime {
			return bytecode.Undefined(), true, fmt.Errorf("query belongs to another metadata runtime")
		}
		if !propertyName(name, "Текст", "Text") {
			return bytecode.Undefined(), true, fmt.Errorf("Query has no property %s", name)
		}
		object.mu.RLock()
		defer object.mu.RUnlock()
		return bytecode.String(object.text), true, nil
	case *queryResultObject:
		if object.runtime != runtime {
			return bytecode.Undefined(), true, fmt.Errorf("query result belongs to another metadata runtime")
		}
		return bytecode.Undefined(), true, fmt.Errorf("QueryResult has no property %s", name)
	case *querySelectionObject:
		if object.result.runtime != runtime {
			return bytecode.Undefined(), true, fmt.Errorf("query selection belongs to another metadata runtime")
		}
		object.mu.RLock()
		defer object.mu.RUnlock()
		if object.current < 0 || object.current >= len(object.result.rows) {
			return bytecode.Undefined(), true, fmt.Errorf("query selection is not positioned on a row")
		}
		for index, column := range object.result.columns {
			if strings.EqualFold(column, name) {
				return object.result.rows[object.current][index], true, nil
			}
		}
		return bytecode.Undefined(), true, fmt.Errorf("QueryResultSelection has no field %s", name)
	default:
		return bytecode.Undefined(), false, nil
	}
}

func (runtime *Runtime) setQueryProperty(value bytecode.RuntimeObject, name string, assigned bytecode.Value) (bool, error) {
	object, ok := value.(*queryObject)
	if !ok {
		if _, queryResult := value.(*queryResultObject); queryResult {
			return true, fmt.Errorf("QueryResult property %s is not writable", name)
		}
		if _, selection := value.(*querySelectionObject); selection {
			return true, fmt.Errorf("QueryResultSelection property %s is not writable", name)
		}
		return false, nil
	}
	if object.runtime != runtime {
		return true, fmt.Errorf("query belongs to another metadata runtime")
	}
	if !propertyName(name, "Текст", "Text") {
		return true, fmt.Errorf("Query property %s is not writable", name)
	}
	text, ok := assigned.AsString()
	if !ok || !validQueryText(text, false) {
		return true, fmt.Errorf("query text must contain 1..%d UTF-8 bytes", maxQueryTextBytes)
	}
	object.mu.Lock()
	object.text = text
	object.mu.Unlock()
	return true, nil
}

func (runtime *Runtime) callQueryMethod(ctx context.Context, value bytecode.RuntimeObject, name string, arguments []bytecode.Value) (bytecode.Value, bool, error) {
	switch object := value.(type) {
	case *queryObject:
		if object.runtime != runtime {
			return bytecode.Undefined(), true, fmt.Errorf("query belongs to another metadata runtime")
		}
		switch {
		case propertyName(name, "УстановитьПараметр", "SetParameter"):
			if len(arguments) != 2 {
				return bytecode.Undefined(), true, fmt.Errorf("%s expects a name and value", name)
			}
			parameter, ok := arguments[0].AsString()
			if !ok || !queryParameterName(parameter) {
				return bytecode.Undefined(), true, fmt.Errorf("query parameter name is invalid")
			}
			object.mu.Lock()
			defer object.mu.Unlock()
			folded := strings.ToLower(parameter)
			if _, exists := object.parameters[folded]; !exists && len(object.parameters) == maxQueryParameters {
				return bytecode.Undefined(), true, fmt.Errorf("query cannot contain more than %d parameters", maxQueryParameters)
			}
			object.parameters[folded] = arguments[1]
			return bytecode.Undefined(), true, nil
		case propertyName(name, "Выполнить", "Execute"):
			if len(arguments) != 0 {
				return bytecode.Undefined(), true, fmt.Errorf("%s expects no arguments", name)
			}
			object.mu.RLock()
			text := object.text
			parameters := make(map[string]bytecode.Value, len(object.parameters))
			for key, parameter := range object.parameters {
				parameters[key] = parameter
			}
			object.mu.RUnlock()
			result, err := runtime.executeQuery(ctx, text, parameters)
			if err != nil {
				return bytecode.Undefined(), true, err
			}
			wrapped, err := bytecode.Object(result)
			return wrapped, true, err
		default:
			return bytecode.Undefined(), true, fmt.Errorf("Query has no method %s", name)
		}
	case *queryResultObject:
		if object.runtime != runtime {
			return bytecode.Undefined(), true, fmt.Errorf("query result belongs to another metadata runtime")
		}
		switch {
		case propertyName(name, "Пустой", "IsEmpty"):
			if len(arguments) != 0 {
				return bytecode.Undefined(), true, fmt.Errorf("%s expects no arguments", name)
			}
			return bytecode.Boolean(len(object.rows) == 0), true, nil
		case propertyName(name, "Выбрать", "Select"):
			if len(arguments) != 0 {
				return bytecode.Undefined(), true, fmt.Errorf("basic %s expects no arguments", name)
			}
			selection, err := bytecode.Object(&querySelectionObject{result: object, current: -1})
			return selection, true, err
		case propertyName(name, "Выгрузить", "Unload"):
			if len(arguments) != 0 {
				return bytecode.Undefined(), true, fmt.Errorf("basic %s expects no arguments", name)
			}
			table, err := queryResultValueTable(object)
			return table, true, err
		default:
			return bytecode.Undefined(), true, fmt.Errorf("QueryResult has no method %s", name)
		}
	case *querySelectionObject:
		if object.result.runtime != runtime {
			return bytecode.Undefined(), true, fmt.Errorf("query selection belongs to another metadata runtime")
		}
		switch {
		case propertyName(name, "Следующий", "Next"):
			if len(arguments) != 0 {
				return bytecode.Undefined(), true, fmt.Errorf("%s expects no arguments", name)
			}
			object.mu.Lock()
			defer object.mu.Unlock()
			if object.current < len(object.result.rows) {
				object.current++
			}
			return bytecode.Boolean(object.current < len(object.result.rows)), true, nil
		case propertyName(name, "Сбросить", "Reset"):
			if len(arguments) != 0 {
				return bytecode.Undefined(), true, fmt.Errorf("%s expects no arguments", name)
			}
			object.mu.Lock()
			object.current = -1
			object.mu.Unlock()
			return bytecode.Undefined(), true, nil
		default:
			return bytecode.Undefined(), true, fmt.Errorf("QueryResultSelection has no method %s", name)
		}
	default:
		return bytecode.Undefined(), false, nil
	}
}

func queryResultValueTable(result *queryResultObject) (bytecode.Value, error) {
	table, err := bytecode.ConstructCollection("ТаблицаЗначений", nil)
	if err != nil {
		return bytecode.Undefined(), err
	}
	columns, err := bytecode.CollectionProperty(table, "Колонки")
	if err != nil {
		return bytecode.Undefined(), err
	}
	for _, name := range result.columns {
		if _, err := bytecode.CollectionMethod(columns, "Добавить", []bytecode.Value{bytecode.String(name)}); err != nil {
			return bytecode.Undefined(), err
		}
	}
	for _, source := range result.rows {
		row, err := bytecode.CollectionMethod(table, "Добавить", nil)
		if err != nil {
			return bytecode.Undefined(), err
		}
		for index, value := range source {
			if err := bytecode.SetCollectionProperty(row, result.columns[index], value); err != nil {
				return bytecode.Undefined(), err
			}
		}
	}
	return table, nil
}

func validQueryText(value string, empty bool) bool {
	return utf8.ValidString(value) && len(value) <= maxQueryTextBytes && (empty || strings.TrimSpace(value) != "")
}
