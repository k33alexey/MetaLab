package bytecode

import (
	"errors"
	"fmt"
	"strings"
	"sync"
	"unicode"
)

const maxCollectionItems = 1 << 20

// ErrCollectionMemoryLimit reports that a collection could not be created
// within the caller-provided memory budget.
var ErrCollectionMemoryLimit = errors.New("collection memory limit exceeded")

type collectionEntry struct {
	key   Value
	name  string
	value Value
}

type collectionObject struct {
	mu      sync.RWMutex
	kind    ValueKind
	values  []Value
	entries []collectionEntry
	table   *collectionObject
	columns *collectionObject
	name    string
	title   string
	nextID  int64
}

func objectValue(kind ValueKind, object *collectionObject) Value {
	return Value{kind: kind, object: object}
}

func newArrayObject(values []Value) *collectionObject {
	return &collectionObject{kind: ArrayKind, values: append([]Value(nil), values...)}
}

// ConstructCollection creates one supported BSL collection type.
func ConstructCollection(name string, arguments []Value) (Value, error) {
	return ConstructCollectionWithin(name, arguments, ^uint64(0))
}

// ConstructCollectionWithin creates a collection without exceeding its initial
// dynamic-memory budget.
func ConstructCollectionWithin(name string, arguments []Value, limit uint64) (Value, error) {
	if limit < 128 {
		return Undefined(), fmt.Errorf("%w: %s", ErrCollectionMemoryLimit, name)
	}
	switch foldCollectionName(name) {
	case "array":
		return constructArray(arguments, limit)
	case "structure":
		return constructStructure(arguments, limit)
	case "map":
		if len(arguments) != 0 {
			return Undefined(), constructorArity(name, len(arguments), 0, 0)
		}
		return objectValue(MapKind, &collectionObject{kind: MapKind}), nil
	case "valuelist":
		if len(arguments) != 0 {
			return Undefined(), constructorArity(name, len(arguments), 0, 0)
		}
		return objectValue(ValueListKind, &collectionObject{kind: ValueListKind, nextID: 1}), nil
	case "valuetable":
		if len(arguments) != 0 {
			return Undefined(), constructorArity(name, len(arguments), 0, 0)
		}
		if limit < 256 {
			return Undefined(), fmt.Errorf("%w: %s", ErrCollectionMemoryLimit, name)
		}
		table := &collectionObject{kind: ValueTableKind}
		table.columns = &collectionObject{kind: ValueTableColumnsKind, table: table}
		return objectValue(ValueTableKind, table), nil
	default:
		return Undefined(), fmt.Errorf("unknown collection type %q", name)
	}
}

func foldCollectionName(name string) string {
	switch {
	case strings.EqualFold(name, "Массив"), strings.EqualFold(name, "Array"):
		return "array"
	case strings.EqualFold(name, "Структура"), strings.EqualFold(name, "Structure"):
		return "structure"
	case strings.EqualFold(name, "Соответствие"), strings.EqualFold(name, "Map"):
		return "map"
	case strings.EqualFold(name, "СписокЗначений"), strings.EqualFold(name, "ValueList"):
		return "valuelist"
	case strings.EqualFold(name, "ТаблицаЗначений"), strings.EqualFold(name, "ValueTable"):
		return "valuetable"
	default:
		return ""
	}
}

func constructArray(arguments []Value, limit uint64) (Value, error) {
	if len(arguments) == 0 {
		return Array(), nil
	}
	if len(arguments) > 16 {
		return Undefined(), fmt.Errorf("Array supports at most 16 dimensions")
	}
	dimensions := make([]int, len(arguments))
	parents, slots := uint64(1), uint64(0)
	for index, argument := range arguments {
		dimension, err := collectionIndex(argument, true)
		if err != nil {
			return Undefined(), fmt.Errorf("Array dimension %d: %w", index+1, err)
		}
		if dimension != 0 && parents > uint64(maxCollectionItems)/uint64(dimension) {
			return Undefined(), fmt.Errorf("Array exceeds %d elements", maxCollectionItems)
		}
		level := parents * uint64(dimension)
		if level > uint64(maxCollectionItems)-slots {
			return Undefined(), fmt.Errorf("Array exceeds %d elements", maxCollectionItems)
		}
		slots += level
		parents = level
		dimensions[index] = dimension
	}
	if slots > (limit-128)/128 {
		return Undefined(), fmt.Errorf("%w: Array", ErrCollectionMemoryLimit)
	}
	return constructArrayDimensions(dimensions), nil
}

func constructArrayDimensions(dimensions []int) Value {
	values := make([]Value, dimensions[0])
	if len(dimensions) > 1 {
		for index := range values {
			values[index] = constructArrayDimensions(dimensions[1:])
		}
	}
	return objectValue(ArrayKind, newArrayObject(values))
}

func constructStructure(arguments []Value, limit uint64) (Value, error) {
	object := &collectionObject{kind: StructureKind}
	if len(arguments) == 0 {
		return objectValue(StructureKind, object), nil
	}
	names, ok := arguments[0].AsString()
	if !ok {
		return Undefined(), fmt.Errorf("Structure property names must be a string")
	}
	parts := strings.Split(names, ",")
	if names == "" {
		parts = nil
	}
	if len(parts) > maxCollectionItems || uint64(len(parts)) > (limit-128)/208 {
		return Undefined(), fmt.Errorf("%w: Structure", ErrCollectionMemoryLimit)
	}
	if len(arguments)-1 > len(parts) {
		return Undefined(), fmt.Errorf("Structure received more values than property names")
	}
	for index, part := range parts {
		name := strings.TrimSpace(part)
		if !validPropertyName(name) {
			return Undefined(), fmt.Errorf("invalid Structure property name %q", name)
		}
		value := Undefined()
		if index+1 < len(arguments) {
			value = arguments[index+1]
		}
		object.entries = append(object.entries, collectionEntry{name: name, value: value})
	}
	return objectValue(StructureKind, object), nil
}

func validPropertyName(name string) bool {
	for index, character := range []rune(name) {
		if index == 0 && unicode.IsDigit(character) || character != '_' && !unicode.IsLetter(character) && !unicode.IsDigit(character) {
			return false
		}
	}
	return name != ""
}

// CollectionMethod invokes a Russian or English collection method.
func CollectionMethod(receiver Value, name string, arguments []Value) (Value, error) {
	if receiver.object == nil {
		return Undefined(), fmt.Errorf("%s has no method %s", receiver.kind, name)
	}
	switch receiver.kind {
	case ArrayKind:
		return arrayMethod(receiver.object, name, arguments)
	case StructureKind:
		return structureMethod(receiver.object, name, arguments)
	case MapKind:
		return mapMethod(receiver.object, name, arguments)
	case ValueListKind:
		return valueListMethod(receiver.object, name, arguments)
	case ValueTableKind:
		return valueTableMethod(receiver.object, name, arguments)
	case ValueTableColumnsKind:
		return tableColumnsMethod(receiver.object.table, name, arguments)
	default:
		return Undefined(), fmt.Errorf("%s has no method %s", receiver.kind, name)
	}
}

// CollectionMutationEstimate returns a conservative allocation reservation for
// a mutating method. Reserving it before dispatch keeps resource-limit failures atomic.
func CollectionMutationEstimate(receiver Value, name string, arguments []Value) uint64 {
	if receiver.object == nil {
		return 0
	}
	root := receiver.object
	if receiver.kind == ValueTableColumnsKind {
		root = receiver.object.table
	}
	if root == nil {
		return 0
	}
	root.mu.RLock()
	defer root.mu.RUnlock()
	switch receiver.kind {
	case ArrayKind:
		if method(name, "Добавить", "Add") || method(name, "Вставить", "Insert") {
			return 192
		}
	case StructureKind:
		if method(name, "Вставить", "Insert") && len(arguments) != 0 {
			key, ok := arguments[0].AsString()
			if ok {
				if _, found := findNamedEntry(receiver.object.entries, key); !found {
					return 416 + uint64(len(key))
				}
			}
		}
	case MapKind:
		if method(name, "Вставить", "Insert") && len(arguments) != 0 && findMapEntry(receiver.object.entries, arguments[0]) < 0 {
			return 416
		}
	case ValueListKind:
		if method(name, "Добавить", "Add") || method(name, "Вставить", "Insert") {
			return 2_400
		}
		if method(name, "ЗагрузитьЗначения", "LoadValues") && len(arguments) == 1 && arguments[0].kind == ArrayKind {
			if count, ok := CollectionLength(arguments[0]); ok {
				return uint64(count) * 2_400
			}
		}
	case ValueTableKind:
		if method(name, "Добавить", "Add") || method(name, "Вставить", "Insert") {
			return 448 + uint64(len(root.entries))*192
		}
	case ValueTableColumnsKind:
		if method(name, "Добавить", "Add") || method(name, "Вставить", "Insert") {
			return 672 + uint64(len(root.values))*192
		}
	}
	return 0
}

// CollectionMethodReturnsDetachedValue reports methods whose result allocates
// storage that is not already owned by the receiver.
func CollectionMethodReturnsDetachedValue(receiver Value, name string) bool {
	if receiver.kind == ValueListKind {
		return method(name, "ВыгрузитьЗначения", "UnloadValues") || method(name, "Скопировать", "Copy")
	}
	if receiver.kind == ValueTableKind {
		return method(name, "НайтиСтроки", "FindRows") || method(name, "ВыгрузитьКолонку", "UnloadColumn") ||
			method(name, "Скопировать", "Copy") || method(name, "СкопироватьКолонки", "CopyColumns")
	}
	return false
}

// CollectionDetachedResultEstimate reserves a conservative amount for methods
// that build an independent result collection.
func CollectionDetachedResultEstimate(receiver Value, name string) uint64 {
	if receiver.object == nil || !CollectionMethodReturnsDetachedValue(receiver, name) {
		return 0
	}
	object := receiver.object
	object.mu.RLock()
	defer object.mu.RUnlock()
	if receiver.kind == ValueListKind {
		if method(name, "ВыгрузитьЗначения", "UnloadValues") {
			return 256 + uint64(len(object.values))*192
		}
		return 256 + uint64(len(object.values))*2_400
	}
	if method(name, "НайтиСтроки", "FindRows") || method(name, "ВыгрузитьКолонку", "UnloadColumn") {
		return 256 + uint64(len(object.values))*192
	}
	columns := uint64(len(object.entries))
	result := 256 + columns*672
	if method(name, "Скопировать", "Copy") {
		result += uint64(len(object.values)) * (448 + columns*192)
	}
	return result
}

// CollectionIndexMutationEstimate reserves storage for index insertion.
func CollectionIndexMutationEstimate(receiver, key Value) uint64 {
	if receiver.kind != MapKind || receiver.object == nil {
		return 0
	}
	receiver.object.mu.RLock()
	defer receiver.object.mu.RUnlock()
	if findMapEntry(receiver.object.entries, key) < 0 {
		return 416
	}
	return 0
}

func arrayMethod(object *collectionObject, name string, arguments []Value) (Value, error) {
	object.mu.Lock()
	defer object.mu.Unlock()
	switch {
	case method(name, "Количество", "Count"):
		if err := exactArguments(name, arguments, 0); err != nil {
			return Undefined(), err
		}
		return Number(float64(len(object.values))), nil
	case method(name, "ВГраница", "UBound"):
		if err := exactArguments(name, arguments, 0); err != nil {
			return Undefined(), err
		}
		return Number(float64(len(object.values) - 1)), nil
	case method(name, "Добавить", "Add"):
		if err := argumentRange(name, arguments, 0, 1); err != nil {
			return Undefined(), err
		}
		if len(object.values) >= maxCollectionItems {
			return Undefined(), fmt.Errorf("Array item limit reached")
		}
		value := Undefined()
		if len(arguments) == 1 {
			value = arguments[0]
		}
		object.values = append(object.values, value)
		return Undefined(), nil
	case method(name, "Вставить", "Insert"):
		if err := argumentRange(name, arguments, 1, 2); err != nil {
			return Undefined(), err
		}
		index, err := collectionIndex(arguments[0], true)
		if err != nil || index > len(object.values) {
			return Undefined(), invalidIndex(err)
		}
		if len(object.values) >= maxCollectionItems {
			return Undefined(), fmt.Errorf("Array item limit reached")
		}
		value := Undefined()
		if len(arguments) == 2 {
			value = arguments[1]
		}
		object.values = append(object.values, Undefined())
		copy(object.values[index+1:], object.values[index:])
		object.values[index] = value
		return Undefined(), nil
	case method(name, "Удалить", "Delete"):
		if err := exactArguments(name, arguments, 1); err != nil {
			return Undefined(), err
		}
		index, err := collectionIndex(arguments[0], false)
		if err != nil || index >= len(object.values) {
			return Undefined(), invalidIndex(err)
		}
		copy(object.values[index:], object.values[index+1:])
		object.values[len(object.values)-1] = Value{}
		object.values = object.values[:len(object.values)-1]
		return Undefined(), nil
	case method(name, "Получить", "Get"):
		if err := exactArguments(name, arguments, 1); err != nil {
			return Undefined(), err
		}
		index, err := collectionIndex(arguments[0], false)
		if err != nil || index >= len(object.values) {
			return Undefined(), invalidIndex(err)
		}
		return object.values[index], nil
	case method(name, "Установить", "Set"):
		if err := exactArguments(name, arguments, 2); err != nil {
			return Undefined(), err
		}
		index, err := collectionIndex(arguments[0], false)
		if err != nil || index >= len(object.values) {
			return Undefined(), invalidIndex(err)
		}
		object.values[index] = arguments[1]
		return Undefined(), nil
	case method(name, "Найти", "Find"):
		if err := exactArguments(name, arguments, 1); err != nil {
			return Undefined(), err
		}
		for index, value := range object.values {
			if ValuesEqual(value, arguments[0]) {
				return Number(float64(index)), nil
			}
		}
		return Undefined(), nil
	case method(name, "Очистить", "Clear"):
		if err := exactArguments(name, arguments, 0); err != nil {
			return Undefined(), err
		}
		object.values = nil
		return Undefined(), nil
	default:
		return Undefined(), fmt.Errorf("Array has no method %s", name)
	}
}

func structureMethod(object *collectionObject, name string, arguments []Value) (Value, error) {
	object.mu.Lock()
	defer object.mu.Unlock()
	switch {
	case method(name, "Количество", "Count"):
		if err := exactArguments(name, arguments, 0); err != nil {
			return Undefined(), err
		}
		return Number(float64(len(object.entries))), nil
	case method(name, "Вставить", "Insert"):
		if err := argumentRange(name, arguments, 1, 2); err != nil {
			return Undefined(), err
		}
		key, ok := arguments[0].AsString()
		if !ok || !validPropertyName(key) {
			return Undefined(), fmt.Errorf("Structure property name must be a valid identifier")
		}
		value := Undefined()
		if len(arguments) == 2 {
			value = arguments[1]
		}
		if _, found := findNamedEntry(object.entries, key); !found && len(object.entries) >= maxCollectionItems {
			return Undefined(), fmt.Errorf("Structure property limit reached")
		}
		setNamedEntry(object, key, value)
		return Undefined(), nil
	case method(name, "Удалить", "Delete"):
		if err := exactArguments(name, arguments, 1); err != nil {
			return Undefined(), err
		}
		key, ok := arguments[0].AsString()
		if !ok {
			return Undefined(), fmt.Errorf("Structure property name must be a string")
		}
		deleteNamedEntry(object, key)
		return Undefined(), nil
	case method(name, "Свойство", "Property"):
		if err := argumentRange(name, arguments, 1, 2); err != nil {
			return Undefined(), err
		}
		key, ok := arguments[0].AsString()
		if !ok {
			return Undefined(), fmt.Errorf("Structure property name must be a string")
		}
		index, found := findNamedEntry(object.entries, key)
		if len(arguments) == 2 {
			arguments[1] = Undefined()
			if found {
				arguments[1] = object.entries[index].value
			}
		}
		return Boolean(found), nil
	case method(name, "Получить", "Get"):
		if err := exactArguments(name, arguments, 1); err != nil {
			return Undefined(), err
		}
		key, ok := arguments[0].AsString()
		if !ok {
			return Undefined(), fmt.Errorf("Structure property name must be a string")
		}
		if index, found := findNamedEntry(object.entries, key); found {
			return object.entries[index].value, nil
		}
		return Undefined(), nil
	case method(name, "Очистить", "Clear"):
		if err := exactArguments(name, arguments, 0); err != nil {
			return Undefined(), err
		}
		object.entries = nil
		return Undefined(), nil
	default:
		return Undefined(), fmt.Errorf("Structure has no method %s", name)
	}
}

func mapMethod(object *collectionObject, name string, arguments []Value) (Value, error) {
	object.mu.Lock()
	defer object.mu.Unlock()
	switch {
	case method(name, "Количество", "Count"):
		if err := exactArguments(name, arguments, 0); err != nil {
			return Undefined(), err
		}
		return Number(float64(len(object.entries))), nil
	case method(name, "Вставить", "Insert"):
		if err := argumentRange(name, arguments, 1, 2); err != nil {
			return Undefined(), err
		}
		value := Undefined()
		if len(arguments) == 2 {
			value = arguments[1]
		}
		if findMapEntry(object.entries, arguments[0]) < 0 && len(object.entries) >= maxCollectionItems {
			return Undefined(), fmt.Errorf("Map item limit reached")
		}
		setMapEntry(object, arguments[0], value)
		return Undefined(), nil
	case method(name, "Получить", "Get"):
		if err := exactArguments(name, arguments, 1); err != nil {
			return Undefined(), err
		}
		if index := findMapEntry(object.entries, arguments[0]); index >= 0 {
			return object.entries[index].value, nil
		}
		return Undefined(), nil
	case method(name, "Удалить", "Delete"):
		if err := exactArguments(name, arguments, 1); err != nil {
			return Undefined(), err
		}
		deleteMapEntry(object, arguments[0])
		return Undefined(), nil
	case method(name, "Очистить", "Clear"):
		if err := exactArguments(name, arguments, 0); err != nil {
			return Undefined(), err
		}
		object.entries = nil
		return Undefined(), nil
	default:
		return Undefined(), fmt.Errorf("Map has no method %s", name)
	}
}

func valueListMethod(object *collectionObject, name string, arguments []Value) (Value, error) {
	object.mu.Lock()
	defer object.mu.Unlock()
	switch {
	case method(name, "Количество", "Count"):
		if err := exactArguments(name, arguments, 0); err != nil {
			return Undefined(), err
		}
		return Number(float64(len(object.values))), nil
	case method(name, "Добавить", "Add"):
		if err := argumentRange(name, arguments, 0, 4); err != nil {
			return Undefined(), err
		}
		if len(object.values) >= maxCollectionItems {
			return Undefined(), fmt.Errorf("ValueList item limit reached")
		}
		item, err := newListItem(object, arguments)
		if err != nil {
			return Undefined(), err
		}
		object.values = append(object.values, item)
		return item, nil
	case method(name, "Вставить", "Insert"):
		if err := argumentRange(name, arguments, 1, 5); err != nil {
			return Undefined(), err
		}
		if len(object.values) >= maxCollectionItems {
			return Undefined(), fmt.Errorf("ValueList item limit reached")
		}
		index, err := collectionIndex(arguments[0], true)
		if err != nil || index > len(object.values) {
			return Undefined(), invalidIndex(err)
		}
		item, err := newListItem(object, arguments[1:])
		if err != nil {
			return Undefined(), err
		}
		object.values = append(object.values, Undefined())
		copy(object.values[index+1:], object.values[index:])
		object.values[index] = item
		return item, nil
	case method(name, "Получить", "Get"):
		if err := exactArguments(name, arguments, 1); err != nil {
			return Undefined(), err
		}
		index, err := collectionIndex(arguments[0], false)
		if err != nil || index >= len(object.values) {
			return Undefined(), invalidIndex(err)
		}
		return object.values[index], nil
	case method(name, "Индекс", "IndexOf"):
		if err := exactArguments(name, arguments, 1); err != nil {
			return Undefined(), err
		}
		for index, item := range object.values {
			if SameReference(item, arguments[0]) {
				return Number(float64(index)), nil
			}
		}
		return Number(-1), nil
	case method(name, "НайтиПоЗначению", "FindByValue"):
		if err := exactArguments(name, arguments, 1); err != nil {
			return Undefined(), err
		}
		for _, item := range object.values {
			item.object.mu.RLock()
			value, _ := collectionPropertyLocked(item, "Value")
			item.object.mu.RUnlock()
			if ValuesEqual(value, arguments[0]) {
				return item, nil
			}
		}
		return Undefined(), nil
	case method(name, "НайтиПоИдентификатору", "FindByID"):
		if err := exactArguments(name, arguments, 1); err != nil {
			return Undefined(), err
		}
		for _, item := range object.values {
			item.object.mu.RLock()
			value, _ := collectionPropertyLocked(item, "ID")
			item.object.mu.RUnlock()
			if ValuesEqual(value, arguments[0]) {
				return item, nil
			}
		}
		return Undefined(), nil
	case method(name, "Удалить", "Delete"):
		if err := exactArguments(name, arguments, 1); err != nil {
			return Undefined(), err
		}
		index := listItemIndex(object.values, arguments[0])
		if index < 0 {
			return Undefined(), fmt.Errorf("ValueList item not found")
		}
		copy(object.values[index:], object.values[index+1:])
		object.values[len(object.values)-1] = Value{}
		object.values = object.values[:len(object.values)-1]
		return Undefined(), nil
	case method(name, "ЗагрузитьЗначения", "LoadValues"):
		if err := exactArguments(name, arguments, 1); err != nil {
			return Undefined(), err
		}
		if arguments[0].kind != ArrayKind {
			return Undefined(), fmt.Errorf("LoadValues expects an Array")
		}
		values, ok := CollectionSnapshot(arguments[0])
		if !ok {
			return Undefined(), fmt.Errorf("LoadValues expects an Array")
		}
		if len(values) > maxCollectionItems {
			return Undefined(), fmt.Errorf("ValueList item limit reached")
		}
		object.values = nil
		for _, value := range values {
			item, _ := newListItem(object, []Value{value})
			object.values = append(object.values, item)
		}
		return Undefined(), nil
	case method(name, "ВыгрузитьЗначения", "UnloadValues"):
		if err := exactArguments(name, arguments, 0); err != nil {
			return Undefined(), err
		}
		values := make([]Value, len(object.values))
		for index, item := range object.values {
			item.object.mu.RLock()
			values[index], _ = collectionPropertyLocked(item, "Value")
			item.object.mu.RUnlock()
		}
		return Array(values...), nil
	case method(name, "Сдвинуть", "Move"):
		if err := exactArguments(name, arguments, 2); err != nil {
			return Undefined(), err
		}
		index := listItemIndex(object.values, arguments[0])
		offset, ok := arguments[1].NumberInteger()
		if index < 0 || !ok || offset > int64(len(object.values)) || offset < -int64(len(object.values)) {
			return Undefined(), fmt.Errorf("Move expects a list item and integer offset")
		}
		target := index + int(offset)
		if target < 0 || target >= len(object.values) {
			return Undefined(), fmt.Errorf("Move target is out of range")
		}
		item := object.values[index]
		if target < index {
			copy(object.values[target+1:index+1], object.values[target:index])
		} else if target > index {
			copy(object.values[index:target], object.values[index+1:target+1])
		}
		object.values[target] = item
		return Undefined(), nil
	case method(name, "ЗаполнитьПометки", "FillChecks"):
		if err := exactArguments(name, arguments, 1); err != nil {
			return Undefined(), err
		}
		if _, ok := arguments[0].AsBoolean(); !ok {
			return Undefined(), fmt.Errorf("FillChecks expects a boolean")
		}
		for _, item := range object.values {
			item.object.mu.Lock()
			index, _ := findNamedEntry(item.object.entries, "Check")
			item.object.entries[index].value = arguments[0]
			item.object.mu.Unlock()
		}
		return Undefined(), nil
	case method(name, "Скопировать", "Copy"):
		if err := exactArguments(name, arguments, 0); err != nil {
			return Undefined(), err
		}
		copyObject := &collectionObject{kind: ValueListKind, nextID: object.nextID, values: make([]Value, len(object.values))}
		for index, item := range object.values {
			item.object.mu.RLock()
			entries := append([]collectionEntry(nil), item.object.entries...)
			item.object.mu.RUnlock()
			copyObject.values[index] = objectValue(ValueListItemKind, &collectionObject{kind: ValueListItemKind, entries: entries})
		}
		return objectValue(ValueListKind, copyObject), nil
	case method(name, "Очистить", "Clear"):
		if err := exactArguments(name, arguments, 0); err != nil {
			return Undefined(), err
		}
		object.values = nil
		return Undefined(), nil
	default:
		return Undefined(), fmt.Errorf("ValueList has no method %s", name)
	}
}

func newListItem(list *collectionObject, arguments []Value) (Value, error) {
	value, presentation, check, picture := Undefined(), String(""), Boolean(false), Undefined()
	if len(arguments) > 0 {
		value = arguments[0]
	}
	if len(arguments) > 1 {
		presentation = arguments[1]
	}
	if len(arguments) > 2 {
		if _, ok := arguments[2].AsBoolean(); !ok {
			return Undefined(), fmt.Errorf("ValueList item Check must be boolean")
		}
		check = arguments[2]
	}
	if len(arguments) > 3 {
		picture = arguments[3]
	}
	identifier := Number(float64(list.nextID))
	list.nextID++
	item := &collectionObject{kind: ValueListItemKind, entries: []collectionEntry{{name: "Value", value: value}, {name: "Presentation", value: presentation}, {name: "Check", value: check}, {name: "Picture", value: picture}, {name: "ID", value: identifier}}}
	return objectValue(ValueListItemKind, item), nil
}

func valueTableMethod(table *collectionObject, name string, arguments []Value) (Value, error) {
	table.mu.Lock()
	defer table.mu.Unlock()
	switch {
	case method(name, "Количество", "Count"):
		if err := exactArguments(name, arguments, 0); err != nil {
			return Undefined(), err
		}
		return Number(float64(len(table.values))), nil
	case method(name, "Добавить", "Add"):
		if err := exactArguments(name, arguments, 0); err != nil {
			return Undefined(), err
		}
		return addTableRow(table, len(table.values))
	case method(name, "Вставить", "Insert"):
		if err := exactArguments(name, arguments, 1); err != nil {
			return Undefined(), err
		}
		index, err := collectionIndex(arguments[0], true)
		if err != nil || index > len(table.values) {
			return Undefined(), invalidIndex(err)
		}
		return addTableRow(table, index)
	case method(name, "Получить", "Get"):
		if err := exactArguments(name, arguments, 1); err != nil {
			return Undefined(), err
		}
		index, err := collectionIndex(arguments[0], false)
		if err != nil || index >= len(table.values) {
			return Undefined(), invalidIndex(err)
		}
		return table.values[index], nil
	case method(name, "Индекс", "IndexOf"):
		if err := exactArguments(name, arguments, 1); err != nil {
			return Undefined(), err
		}
		for index, row := range table.values {
			if SameReference(row, arguments[0]) {
				return Number(float64(index)), nil
			}
		}
		return Number(-1), nil
	case method(name, "Удалить", "Delete"):
		if err := exactArguments(name, arguments, 1); err != nil {
			return Undefined(), err
		}
		index := listItemIndex(table.values, arguments[0])
		if index < 0 {
			return Undefined(), fmt.Errorf("ValueTable row not found")
		}
		copy(table.values[index:], table.values[index+1:])
		table.values[len(table.values)-1] = Value{}
		table.values = table.values[:len(table.values)-1]
		return Undefined(), nil
	case method(name, "Найти", "Find"):
		if err := argumentRange(name, arguments, 1, 2); err != nil {
			return Undefined(), err
		}
		columns, err := selectedColumns(table, arguments[1:])
		if err != nil {
			return Undefined(), err
		}
		for _, row := range table.values {
			for _, column := range columns {
				if ValuesEqual(row.object.values[column], arguments[0]) {
					return row, nil
				}
			}
		}
		return Undefined(), nil
	case method(name, "НайтиСтроки", "FindRows"):
		if err := exactArguments(name, arguments, 1); err != nil {
			return Undefined(), err
		}
		if arguments[0].kind != StructureKind || arguments[0].object == nil {
			return Undefined(), fmt.Errorf("FindRows expects a Structure filter")
		}
		arguments[0].object.mu.RLock()
		filter := append([]collectionEntry(nil), arguments[0].object.entries...)
		arguments[0].object.mu.RUnlock()
		columns := make([]int, len(filter))
		for index, condition := range filter {
			column, found := findNamedEntry(table.entries, condition.name)
			if !found {
				return Undefined(), fmt.Errorf("column %q not found", condition.name)
			}
			columns[index] = column
		}
		result := make([]Value, 0)
		for _, row := range table.values {
			matches := true
			for index, condition := range filter {
				if !ValuesEqual(row.object.values[columns[index]], condition.value) {
					matches = false
					break
				}
			}
			if matches {
				result = append(result, row)
			}
		}
		return Array(result...), nil
	case method(name, "ВыгрузитьКолонку", "UnloadColumn"):
		if err := exactArguments(name, arguments, 1); err != nil {
			return Undefined(), err
		}
		column, err := tableColumnIndex(table, arguments[0])
		if err != nil {
			return Undefined(), err
		}
		result := make([]Value, len(table.values))
		for index, row := range table.values {
			result[index] = row.object.values[column]
		}
		return Array(result...), nil
	case method(name, "ЗагрузитьКолонку", "LoadColumn"):
		if err := exactArguments(name, arguments, 2); err != nil {
			return Undefined(), err
		}
		if arguments[0].kind != ArrayKind {
			return Undefined(), fmt.Errorf("LoadColumn expects an Array")
		}
		values, ok := CollectionSnapshot(arguments[0])
		if !ok || len(values) != len(table.values) {
			return Undefined(), fmt.Errorf("LoadColumn value count must match the row count")
		}
		column, err := tableColumnIndex(table, arguments[1])
		if err != nil {
			return Undefined(), err
		}
		for index, row := range table.values {
			row.object.values[column] = values[index]
		}
		return Undefined(), nil
	case method(name, "ЗаполнитьЗначения", "FillValues"):
		if err := argumentRange(name, arguments, 1, 2); err != nil {
			return Undefined(), err
		}
		columns, err := selectedColumns(table, arguments[1:])
		if err != nil {
			return Undefined(), err
		}
		for _, row := range table.values {
			for _, column := range columns {
				row.object.values[column] = arguments[0]
			}
		}
		return Undefined(), nil
	case method(name, "Скопировать", "Copy"):
		if err := argumentRange(name, arguments, 0, 2); err != nil {
			return Undefined(), err
		}
		rows := table.values
		if len(arguments) > 0 && arguments[0].kind != UndefinedKind {
			if arguments[0].kind != ArrayKind {
				return Undefined(), fmt.Errorf("Copy rows must be an Array")
			}
			var ok bool
			rows, ok = CollectionSnapshot(arguments[0])
			if !ok {
				return Undefined(), fmt.Errorf("Copy rows must be an Array")
			}
		}
		columns, err := selectedColumns(table, arguments[1:])
		if err != nil {
			return Undefined(), err
		}
		return copyValueTable(table, rows, columns, true)
	case method(name, "СкопироватьКолонки", "CopyColumns"):
		if err := argumentRange(name, arguments, 0, 1); err != nil {
			return Undefined(), err
		}
		columns, err := selectedColumns(table, arguments)
		if err != nil {
			return Undefined(), err
		}
		return copyValueTable(table, nil, columns, false)
	case method(name, "Итог", "Total"):
		if err := exactArguments(name, arguments, 1); err != nil {
			return Undefined(), err
		}
		column, err := tableColumnIndex(table, arguments[0])
		if err != nil {
			return Undefined(), err
		}
		total := Number(0)
		for _, row := range table.values {
			total, err = AddNumbers(total, row.object.values[column])
			if err != nil {
				return Undefined(), fmt.Errorf("Total requires numeric values")
			}
		}
		return total, nil
	case method(name, "Очистить", "Clear"):
		if err := exactArguments(name, arguments, 0); err != nil {
			return Undefined(), err
		}
		table.values = nil
		return Undefined(), nil
	default:
		return Undefined(), fmt.Errorf("ValueTable has no method %s", name)
	}
}

func tableColumnsMethod(table *collectionObject, name string, arguments []Value) (Value, error) {
	if table == nil {
		return Undefined(), fmt.Errorf("detached ValueTable columns")
	}
	table.mu.Lock()
	defer table.mu.Unlock()
	switch {
	case method(name, "Количество", "Count"):
		if err := exactArguments(name, arguments, 0); err != nil {
			return Undefined(), err
		}
		return Number(float64(len(table.entries))), nil
	case method(name, "Добавить", "Add"):
		if err := argumentRange(name, arguments, 1, 4); err != nil {
			return Undefined(), err
		}
		return addTableColumn(table, len(table.entries), arguments)
	case method(name, "Вставить", "Insert"):
		if err := argumentRange(name, arguments, 2, 5); err != nil {
			return Undefined(), err
		}
		index, err := collectionIndex(arguments[0], true)
		if err != nil || index > len(table.entries) {
			return Undefined(), invalidIndex(err)
		}
		return addTableColumn(table, index, arguments[1:])
	case method(name, "Найти", "Find"):
		if err := exactArguments(name, arguments, 1); err != nil {
			return Undefined(), err
		}
		name, ok := arguments[0].AsString()
		if !ok {
			return Undefined(), fmt.Errorf("column name must be a string")
		}
		if index, found := findNamedEntry(table.entries, name); found {
			return table.entries[index].value, nil
		}
		return Undefined(), nil
	case method(name, "Получить", "Get"):
		if err := exactArguments(name, arguments, 1); err != nil {
			return Undefined(), err
		}
		index, err := collectionIndex(arguments[0], false)
		if err != nil || index >= len(table.entries) {
			return Undefined(), invalidIndex(err)
		}
		return table.entries[index].value, nil
	case method(name, "Индекс", "IndexOf"):
		if err := exactArguments(name, arguments, 1); err != nil {
			return Undefined(), err
		}
		for index, column := range table.entries {
			if SameReference(column.value, arguments[0]) {
				return Number(float64(index)), nil
			}
		}
		return Number(-1), nil
	case method(name, "Удалить", "Delete"):
		if err := exactArguments(name, arguments, 1); err != nil {
			return Undefined(), err
		}
		index, err := tableColumnIndex(table, arguments[0])
		if err != nil {
			return Undefined(), err
		}
		copy(table.entries[index:], table.entries[index+1:])
		table.entries[len(table.entries)-1] = collectionEntry{}
		table.entries = table.entries[:len(table.entries)-1]
		for _, row := range table.values {
			copy(row.object.values[index:], row.object.values[index+1:])
			row.object.values[len(row.object.values)-1] = Value{}
			row.object.values = row.object.values[:len(row.object.values)-1]
		}
		return Undefined(), nil
	case method(name, "Очистить", "Clear"):
		if err := exactArguments(name, arguments, 0); err != nil {
			return Undefined(), err
		}
		table.entries = nil
		for _, row := range table.values {
			row.object.values = nil
		}
		return Undefined(), nil
	default:
		return Undefined(), fmt.Errorf("ValueTableColumns has no method %s", name)
	}
}

func copyValueTable(source *collectionObject, rows []Value, columns []int, includeRows bool) (Value, error) {
	target := &collectionObject{kind: ValueTableKind}
	target.columns = &collectionObject{kind: ValueTableColumnsKind, table: target}
	for _, sourceIndex := range columns {
		sourceColumn := source.entries[sourceIndex].value.object
		column := objectValue(ValueTableColumnKind, &collectionObject{
			kind: ValueTableColumnKind, table: target, name: sourceColumn.name, title: sourceColumn.title,
		})
		target.entries = append(target.entries, collectionEntry{name: sourceColumn.name, value: column})
	}
	if !includeRows {
		return objectValue(ValueTableKind, target), nil
	}
	if len(rows) > maxCollectionItems {
		return Undefined(), fmt.Errorf("ValueTable row limit reached")
	}
	for _, sourceRow := range rows {
		if sourceRow.kind != ValueTableRowKind || sourceRow.object == nil || sourceRow.object.table != source {
			return Undefined(), fmt.Errorf("Copy contains a row from another ValueTable")
		}
		values := make([]Value, len(columns))
		for index, sourceIndex := range columns {
			values[index] = sourceRow.object.values[sourceIndex]
		}
		row := objectValue(ValueTableRowKind, &collectionObject{kind: ValueTableRowKind, table: target, values: values})
		target.values = append(target.values, row)
	}
	return objectValue(ValueTableKind, target), nil
}

func addTableRow(table *collectionObject, index int) (Value, error) {
	if len(table.values) >= maxCollectionItems {
		return Undefined(), fmt.Errorf("ValueTable row limit reached")
	}
	row := objectValue(ValueTableRowKind, &collectionObject{kind: ValueTableRowKind, table: table, values: make([]Value, len(table.entries))})
	table.values = append(table.values, Undefined())
	copy(table.values[index+1:], table.values[index:])
	table.values[index] = row
	return row, nil
}

func addTableColumn(table *collectionObject, index int, arguments []Value) (Value, error) {
	if len(table.entries) >= maxCollectionItems {
		return Undefined(), fmt.Errorf("ValueTable column limit reached")
	}
	name, ok := arguments[0].AsString()
	if !ok || !validPropertyName(name) {
		return Undefined(), fmt.Errorf("column name must be a valid identifier")
	}
	if _, found := findNamedEntry(table.entries, name); found {
		return Undefined(), fmt.Errorf("column %q already exists", name)
	}
	title := name
	if len(arguments) >= 3 {
		if text, ok := arguments[2].AsString(); ok {
			title = text
		}
	}
	column := objectValue(ValueTableColumnKind, &collectionObject{kind: ValueTableColumnKind, table: table, name: name, title: title})
	entry := collectionEntry{name: name, value: column}
	table.entries = append(table.entries, collectionEntry{})
	copy(table.entries[index+1:], table.entries[index:])
	table.entries[index] = entry
	for _, row := range table.values {
		row.object.values = append(row.object.values, Undefined())
		copy(row.object.values[index+1:], row.object.values[index:])
		row.object.values[index] = Undefined()
	}
	return column, nil
}

// CollectionProperty reads an object property using case-insensitive BSL names.
func CollectionProperty(receiver Value, name string) (Value, error) {
	if receiver.object == nil {
		return Undefined(), fmt.Errorf("%s has no property %s", receiver.kind, name)
	}
	root := receiver.object
	if receiver.kind == ValueTableRowKind || receiver.kind == ValueTableColumnKind {
		root = receiver.object.table
	}
	if root == nil {
		return Undefined(), fmt.Errorf("detached %s", receiver.kind)
	}
	root.mu.RLock()
	defer root.mu.RUnlock()
	return collectionPropertyLocked(receiver, name)
}

func collectionPropertyLocked(receiver Value, name string) (Value, error) {
	switch receiver.kind {
	case StructureKind:
		if index, ok := findNamedEntry(receiver.object.entries, name); ok {
			return receiver.object.entries[index].value, nil
		}
	case ValueListItemKind:
		if index, ok := findNamedEntry(receiver.object.entries, canonicalItemProperty(name)); ok {
			return receiver.object.entries[index].value, nil
		}
	case ValueTableKind:
		if method(name, "Колонки", "Columns") {
			return objectValue(ValueTableColumnsKind, receiver.object.columns), nil
		}
	case ValueTableRowKind:
		if index, ok := findNamedEntry(receiver.object.table.entries, name); ok {
			return receiver.object.values[index], nil
		}
	case ValueTableColumnKind:
		switch {
		case method(name, "Имя", "Name"):
			return String(receiver.object.name), nil
		case method(name, "Заголовок", "Title"):
			return String(receiver.object.title), nil
		}
	case KeyAndValueKind:
		if index, ok := findNamedEntry(receiver.object.entries, canonicalPairProperty(name)); ok {
			return receiver.object.entries[index].value, nil
		}
	}
	return Undefined(), fmt.Errorf("%s has no property %s", receiver.kind, name)
}

// SetCollectionProperty updates a writable collection property.
func SetCollectionProperty(receiver Value, name string, value Value) error {
	if receiver.object == nil {
		return fmt.Errorf("%s has no property %s", receiver.kind, name)
	}
	root := receiver.object
	if receiver.kind == ValueTableRowKind || receiver.kind == ValueTableColumnKind {
		root = receiver.object.table
	}
	if root == nil {
		return fmt.Errorf("detached %s", receiver.kind)
	}
	root.mu.Lock()
	defer root.mu.Unlock()
	switch receiver.kind {
	case StructureKind:
		if index, ok := findNamedEntry(receiver.object.entries, name); ok {
			receiver.object.entries[index].value = value
			return nil
		}
		return fmt.Errorf("Structure has no property %s", name)
	case ValueListItemKind:
		property := canonicalItemProperty(name)
		if strings.EqualFold(property, "ID") {
			return fmt.Errorf("ValueList item ID is read-only")
		}
		if strings.EqualFold(property, "Check") {
			if _, ok := value.AsBoolean(); !ok {
				return fmt.Errorf("ValueList item Check must be boolean")
			}
		}
		if index, ok := findNamedEntry(receiver.object.entries, property); ok {
			receiver.object.entries[index].value = value
			return nil
		}
	case ValueTableRowKind:
		if index, ok := findNamedEntry(receiver.object.table.entries, name); ok {
			receiver.object.values[index] = value
			return nil
		}
	case ValueTableColumnKind:
		if method(name, "Заголовок", "Title") {
			text, ok := value.AsString()
			if !ok {
				return fmt.Errorf("column Title must be a string")
			}
			receiver.object.title = text
			return nil
		}
	}
	return fmt.Errorf("%s property %s is not writable", receiver.kind, name)
}

// CollectionIndex reads an indexed collection element.
func CollectionIndex(receiver, key Value) (Value, error) {
	if receiver.object == nil {
		return Undefined(), fmt.Errorf("%s is not indexable", receiver.kind)
	}
	root := receiver.object
	if receiver.kind == ValueTableRowKind || receiver.kind == ValueTableColumnsKind {
		root = receiver.object.table
	}
	root.mu.RLock()
	defer root.mu.RUnlock()
	switch receiver.kind {
	case ArrayKind, ValueListKind, ValueTableKind:
		index, err := collectionIndex(key, false)
		if err != nil || index >= len(receiver.object.values) {
			return Undefined(), invalidIndex(err)
		}
		return receiver.object.values[index], nil
	case StructureKind:
		name, ok := key.AsString()
		if !ok {
			return Undefined(), fmt.Errorf("Structure index must be a string")
		}
		if index, found := findNamedEntry(receiver.object.entries, name); found {
			return receiver.object.entries[index].value, nil
		}
		return Undefined(), nil
	case MapKind:
		if index := findMapEntry(receiver.object.entries, key); index >= 0 {
			return receiver.object.entries[index].value, nil
		}
		return Undefined(), nil
	case ValueTableRowKind:
		index, err := tableColumnIndex(receiver.object.table, key)
		if err != nil {
			return Undefined(), err
		}
		return receiver.object.values[index], nil
	case ValueTableColumnsKind:
		index, err := tableColumnIndex(receiver.object.table, key)
		if err != nil {
			return Undefined(), err
		}
		return receiver.object.table.entries[index].value, nil
	default:
		return Undefined(), fmt.Errorf("%s is not indexable", receiver.kind)
	}
}

// SetCollectionIndex updates an indexed collection element.
func SetCollectionIndex(receiver, key, value Value) error {
	if receiver.object == nil {
		return fmt.Errorf("%s is not indexable", receiver.kind)
	}
	root := receiver.object
	if receiver.kind == ValueTableRowKind {
		root = receiver.object.table
	}
	root.mu.Lock()
	defer root.mu.Unlock()
	switch receiver.kind {
	case ArrayKind:
		index, err := collectionIndex(key, false)
		if err != nil || index >= len(receiver.object.values) {
			return invalidIndex(err)
		}
		receiver.object.values[index] = value
		return nil
	case StructureKind:
		name, ok := key.AsString()
		if !ok {
			return fmt.Errorf("Structure index must be a string")
		}
		if index, found := findNamedEntry(receiver.object.entries, name); found {
			receiver.object.entries[index].value = value
			return nil
		}
		return fmt.Errorf("Structure has no property %s", name)
	case MapKind:
		if findMapEntry(receiver.object.entries, key) < 0 && len(receiver.object.entries) >= maxCollectionItems {
			return fmt.Errorf("Map item limit reached")
		}
		setMapEntry(receiver.object, key, value)
		return nil
	case ValueTableRowKind:
		index, err := tableColumnIndex(receiver.object.table, key)
		if err != nil {
			return err
		}
		receiver.object.values[index] = value
		return nil
	default:
		return fmt.Errorf("%s index is not writable", receiver.kind)
	}
}

// CollectionSnapshot returns values in deterministic For Each order.
func CollectionSnapshot(value Value) ([]Value, bool) {
	if value.object == nil {
		return nil, false
	}
	root := value.object
	if value.kind == ValueTableColumnsKind {
		root = value.object.table
	}
	if root == nil {
		return nil, false
	}
	root.mu.RLock()
	defer root.mu.RUnlock()
	switch value.kind {
	case ArrayKind, ValueListKind, ValueTableKind:
		return append([]Value(nil), value.object.values...), true
	case ValueTableColumnsKind:
		result := make([]Value, len(root.entries))
		for index := range root.entries {
			result[index] = root.entries[index].value
		}
		return result, true
	case StructureKind, MapKind:
		result := make([]Value, len(value.object.entries))
		for index, entry := range value.object.entries {
			key := entry.key
			if value.kind == StructureKind {
				key = String(entry.name)
			}
			pair := &collectionObject{kind: KeyAndValueKind, entries: []collectionEntry{{name: "Key", value: key}, {name: "Value", value: entry.value}}}
			result[index] = objectValue(KeyAndValueKind, pair)
		}
		return result, true
	default:
		return nil, false
	}
}

// CollectionLength returns the number of values produced by For Each.
func CollectionLength(value Value) (int, bool) {
	if value.object == nil {
		return 0, false
	}
	root := value.object
	if value.kind == ValueTableColumnsKind {
		root = value.object.table
	}
	if root == nil {
		return 0, false
	}
	root.mu.RLock()
	defer root.mu.RUnlock()
	switch value.kind {
	case ArrayKind, ValueListKind, ValueTableKind:
		return len(value.object.values), true
	case StructureKind, MapKind:
		return len(value.object.entries), true
	case ValueTableColumnsKind:
		return len(root.entries), true
	default:
		return 0, false
	}
}

// CollectionElement returns one value in deterministic For Each order.
func CollectionElement(value Value, index int) (Value, bool) {
	if value.object == nil || index < 0 {
		return Undefined(), false
	}
	root := value.object
	if value.kind == ValueTableColumnsKind {
		root = value.object.table
	}
	if root == nil {
		return Undefined(), false
	}
	root.mu.RLock()
	defer root.mu.RUnlock()
	switch value.kind {
	case ArrayKind, ValueListKind, ValueTableKind:
		if index >= len(value.object.values) {
			return Undefined(), false
		}
		return value.object.values[index], true
	case ValueTableColumnsKind:
		if index >= len(root.entries) {
			return Undefined(), false
		}
		return root.entries[index].value, true
	case StructureKind, MapKind:
		if index >= len(value.object.entries) {
			return Undefined(), false
		}
		entry := value.object.entries[index]
		key := entry.key
		if value.kind == StructureKind {
			key = String(entry.name)
		}
		pair := &collectionObject{kind: KeyAndValueKind, entries: []collectionEntry{{name: "Key", value: key}, {name: "Value", value: entry.value}}}
		return objectValue(KeyAndValueKind, pair), true
	default:
		return Undefined(), false
	}
}

// CollectionPropertyNames returns deterministic public property names for
// object-shaped collection values used by RPC and WebAssembly bridges.
func CollectionPropertyNames(value Value) ([]string, bool) {
	if value.object == nil {
		return nil, false
	}
	root := value.object
	if value.kind == ValueTableRowKind || value.kind == ValueTableColumnKind {
		root = value.object.table
	}
	if root == nil {
		return nil, false
	}
	root.mu.RLock()
	defer root.mu.RUnlock()
	switch value.kind {
	case StructureKind:
		result := make([]string, len(value.object.entries))
		for index := range value.object.entries {
			result[index] = value.object.entries[index].name
		}
		return result, true
	case ValueListItemKind:
		return []string{"Value", "Presentation", "Check", "Picture", "ID"}, true
	case ValueTableRowKind:
		result := make([]string, len(root.entries))
		for index := range root.entries {
			result[index] = root.entries[index].name
		}
		return result, true
	case ValueTableColumnKind:
		return []string{"Name", "Title"}, true
	case KeyAndValueKind:
		return []string{"Key", "Value"}, true
	default:
		return nil, false
	}
}

// SameReference compares collection object identity.
func SameReference(left, right Value) bool {
	return left.kind == right.kind && left.object != nil && left.object == right.object
}

// ValuesEqual implements primitive equality and collection reference equality.
func ValuesEqual(left, right Value) bool {
	if left.kind != right.kind {
		return false
	}
	switch left.kind {
	case UndefinedKind, NullKind:
		return true
	case NumberKind:
		comparison, _ := CompareNumbers(left, right)
		return comparison == 0
	case StringKind:
		return left.text == right.text
	case BooleanKind:
		return left.boolean == right.boolean
	case DateKind:
		return left.dateTicks == right.dateTicks
	default:
		return left.object != nil && left.object == right.object
	}
}

func (object *collectionObject) length() int {
	object.mu.RLock()
	defer object.mu.RUnlock()
	if object.kind == StructureKind || object.kind == MapKind {
		return len(object.entries)
	}
	if object.kind == ValueTableColumnsKind && object.table != nil {
		return len(object.table.entries)
	}
	return len(object.values)
}
func (object *collectionObject) arrayElement(index int) (Value, bool) {
	object.mu.RLock()
	defer object.mu.RUnlock()
	if index < 0 || index >= len(object.values) {
		return Undefined(), false
	}
	return object.values[index], true
}

func (object *collectionObject) dynamicMemory(limit uint64, depth int, visited map[*collectionObject]struct{}) (uint64, bool) {
	if depth > 64 {
		return limit, false
	}
	if _, exists := visited[object]; exists {
		return 0, true
	}
	visited[object] = struct{}{}
	lock := object
	if (object.kind == ValueTableRowKind || object.kind == ValueTableColumnKind) && object.table != nil {
		lock = object.table
	}
	lock.mu.RLock()
	values := append([]Value(nil), object.values...)
	entries := append([]collectionEntry(nil), object.entries...)
	name, title := object.name, object.title
	lock.mu.RUnlock()

	size := uint64(128 + len(name) + len(title))
	if object.kind == ValueTableKind && object.columns != nil {
		size += 128
	}
	add := func(amount uint64) bool {
		if amount > limit-min(size, limit) {
			return false
		}
		size += amount
		return true
	}
	if !add(uint64(len(values))*96) || !add(uint64(len(entries))*208) {
		return limit, false
	}
	measure := func(value Value) bool {
		remaining := limit - min(size, limit)
		amount, ok := dynamicMemory(value, remaining, depth+1, visited)
		return ok && add(amount)
	}
	for _, value := range values {
		if !measure(value) {
			return limit, false
		}
	}
	for _, entry := range entries {
		if !add(uint64(len(entry.name))) || !measure(entry.key) || !measure(entry.value) {
			return limit, false
		}
	}
	return size, size <= limit
}

func method(name, russian, english string) bool {
	return strings.EqualFold(name, russian) || strings.EqualFold(name, english)
}
func exactArguments(name string, arguments []Value, count int) error {
	if len(arguments) != count {
		return fmt.Errorf("%s expects %d arguments, got %d", name, count, len(arguments))
	}
	return nil
}
func argumentRange(name string, arguments []Value, minimum, maximum int) error {
	if len(arguments) < minimum || len(arguments) > maximum {
		return fmt.Errorf("%s expects %d..%d arguments, got %d", name, minimum, maximum, len(arguments))
	}
	return nil
}
func constructorArity(name string, actual, minimum, maximum int) error {
	return fmt.Errorf("%s constructor expects %d..%d arguments, got %d", name, minimum, maximum, actual)
}
func collectionIndex(value Value, allowEnd bool) (int, error) {
	number, ok := value.NumberInteger()
	if !ok || number < 0 || uint64(number) > uint64(maxCollectionItems) {
		return 0, fmt.Errorf("collection index must be a non-negative integer")
	}
	if !allowEnd && number == maxCollectionItems {
		return 0, fmt.Errorf("collection index is out of range")
	}
	return int(number), nil
}
func invalidIndex(err error) error {
	if err != nil {
		return err
	}
	return fmt.Errorf("collection index is out of range")
}
func findNamedEntry(entries []collectionEntry, name string) (int, bool) {
	for index := range entries {
		if strings.EqualFold(entries[index].name, name) {
			return index, true
		}
	}
	return -1, false
}
func setNamedEntry(object *collectionObject, name string, value Value) {
	if index, ok := findNamedEntry(object.entries, name); ok {
		object.entries[index].value = value
		return
	}
	object.entries = append(object.entries, collectionEntry{name: name, value: value})
}
func deleteNamedEntry(object *collectionObject, name string) {
	if index, ok := findNamedEntry(object.entries, name); ok {
		copy(object.entries[index:], object.entries[index+1:])
		object.entries[len(object.entries)-1] = collectionEntry{}
		object.entries = object.entries[:len(object.entries)-1]
	}
}
func findMapEntry(entries []collectionEntry, key Value) int {
	for index := range entries {
		if ValuesEqual(entries[index].key, key) {
			return index
		}
	}
	return -1
}
func setMapEntry(object *collectionObject, key, value Value) {
	if index := findMapEntry(object.entries, key); index >= 0 {
		object.entries[index].value = value
		return
	}
	object.entries = append(object.entries, collectionEntry{key: key, value: value})
}
func deleteMapEntry(object *collectionObject, key Value) {
	if index := findMapEntry(object.entries, key); index >= 0 {
		copy(object.entries[index:], object.entries[index+1:])
		object.entries[len(object.entries)-1] = collectionEntry{}
		object.entries = object.entries[:len(object.entries)-1]
	}
}
func listItemIndex(values []Value, item Value) int {
	if index, err := collectionIndex(item, false); err == nil && index < len(values) {
		return index
	}
	for index, value := range values {
		if SameReference(value, item) {
			return index
		}
	}
	return -1
}
func canonicalItemProperty(name string) string {
	switch {
	case method(name, "Значение", "Value"):
		return "Value"
	case method(name, "Представление", "Presentation"):
		return "Presentation"
	case method(name, "Пометка", "Check"):
		return "Check"
	case method(name, "Картинка", "Picture"):
		return "Picture"
	case method(name, "Идентификатор", "ID"):
		return "ID"
	default:
		return name
	}
}
func canonicalPairProperty(name string) string {
	if method(name, "Ключ", "Key") {
		return "Key"
	}
	if method(name, "Значение", "Value") {
		return "Value"
	}
	return name
}
func tableColumnIndex(table *collectionObject, value Value) (int, error) {
	if index, err := collectionIndex(value, false); err == nil {
		if index < len(table.entries) {
			return index, nil
		}
		return 0, invalidIndex(nil)
	}
	if name, ok := value.AsString(); ok {
		if index, found := findNamedEntry(table.entries, name); found {
			return index, nil
		}
		return 0, fmt.Errorf("column %q not found", name)
	}
	for index, entry := range table.entries {
		if SameReference(entry.value, value) {
			return index, nil
		}
	}
	return 0, fmt.Errorf("ValueTable column not found")
}
func selectedColumns(table *collectionObject, arguments []Value) ([]int, error) {
	if len(arguments) == 0 || arguments[0].Kind() == UndefinedKind {
		result := make([]int, len(table.entries))
		for index := range result {
			result[index] = index
		}
		return result, nil
	}
	names, ok := arguments[0].AsString()
	if !ok {
		return nil, fmt.Errorf("column list must be a string")
	}
	parts := strings.Split(names, ",")
	result := make([]int, 0, len(parts))
	for _, part := range parts {
		index, found := findNamedEntry(table.entries, strings.TrimSpace(part))
		if !found {
			return nil, fmt.Errorf("column %q not found", strings.TrimSpace(part))
		}
		result = append(result, index)
	}
	return result, nil
}
