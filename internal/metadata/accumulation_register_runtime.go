package metadata

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/k33alexey/MetaLab/internal/bsl/bytecode"
	"github.com/k33alexey/MetaLab/internal/uuid"
)

type accumulationRegisterRecordSetObject struct {
	mu         sync.RWMutex
	definition AccumulationRegisterDefinition
	set        *AccumulationRegisterRecordSet
	runtime    *Runtime
}

type accumulationRegisterRecordObject struct {
	owner  *accumulationRegisterRecordSetObject
	record *AccumulationRegisterRecord
}

type accumulationRegisterFilterObject struct {
	owner *accumulationRegisterRecordSetObject
}
type accumulationRegisterRecorderFilterObject struct {
	owner *accumulationRegisterRecordSetObject
}

type accumulationVirtualTableObject struct {
	definition AccumulationRegisterDefinition
	kind       string
	rows       []AccumulationRegisterAggregate
	runtime    *Runtime
}

type accumulationVirtualRowObject struct {
	owner *accumulationVirtualTableObject
	row   *AccumulationRegisterAggregate
}

func (object *accumulationRegisterRecordSetObject) RuntimeTypeName() string {
	return "AccumulationRegisterRecordSet." + object.definition.Name
}
func (object *accumulationRegisterRecordSetObject) RuntimeEqual(other bytecode.RuntimeObject) bool {
	candidate, ok := other.(*accumulationRegisterRecordSetObject)
	return ok && candidate == object
}
func (object *accumulationRegisterRecordSetObject) RuntimeDynamicMemory(limit uint64) (uint64, bool) {
	object.mu.RLock()
	defer object.mu.RUnlock()
	return accumulationRegisterSetMemory(object.set, limit)
}
func (object *accumulationRegisterRecordSetObject) RuntimeCollectionLength() int {
	object.mu.RLock()
	defer object.mu.RUnlock()
	return len(object.set.Records)
}
func (object *accumulationRegisterRecordSetObject) RuntimeCollectionElement(index int) (bytecode.Value, bool) {
	object.mu.RLock()
	defer object.mu.RUnlock()
	if index < 0 || index >= len(object.set.Records) {
		return bytecode.Undefined(), false
	}
	value, err := bytecode.Object(&accumulationRegisterRecordObject{owner: object, record: object.set.Records[index]})
	return value, err == nil
}
func (object *accumulationRegisterRecordSetObject) RuntimeCollectionIndex(key bytecode.Value) (bytecode.Value, error) {
	index, ok := key.NumberInteger()
	if !ok || index < 0 {
		return bytecode.Undefined(), fmt.Errorf("accumulation register record index must be a non-negative integer")
	}
	value, found := object.RuntimeCollectionElement(int(index))
	if !found {
		return bytecode.Undefined(), fmt.Errorf("accumulation register record index is out of range")
	}
	return value, nil
}

func (object *accumulationRegisterRecordObject) RuntimeTypeName() string {
	return "AccumulationRegisterRecord." + object.owner.definition.Name
}
func (object *accumulationRegisterRecordObject) RuntimeEqual(other bytecode.RuntimeObject) bool {
	candidate, ok := other.(*accumulationRegisterRecordObject)
	return ok && candidate.owner == object.owner && candidate.record == object.record
}
func (object *accumulationRegisterRecordObject) RuntimeDynamicMemory(limit uint64) (uint64, bool) {
	return accumulationRegisterChildMemory(object.owner, limit, 256)
}
func (*accumulationRegisterFilterObject) RuntimeTypeName() string {
	return "AccumulationRegisterFilter"
}
func (object *accumulationRegisterFilterObject) RuntimeEqual(other bytecode.RuntimeObject) bool {
	candidate, ok := other.(*accumulationRegisterFilterObject)
	return ok && candidate.owner == object.owner
}
func (object *accumulationRegisterFilterObject) RuntimeDynamicMemory(limit uint64) (uint64, bool) {
	return accumulationRegisterChildMemory(object.owner, limit, 192)
}
func (*accumulationRegisterRecorderFilterObject) RuntimeTypeName() string {
	return "AccumulationRegisterFilterItem"
}
func (object *accumulationRegisterRecorderFilterObject) RuntimeEqual(other bytecode.RuntimeObject) bool {
	candidate, ok := other.(*accumulationRegisterRecorderFilterObject)
	return ok && candidate.owner == object.owner
}
func (object *accumulationRegisterRecorderFilterObject) RuntimeDynamicMemory(limit uint64) (uint64, bool) {
	return accumulationRegisterChildMemory(object.owner, limit, 192)
}

func accumulationRegisterChildMemory(owner *accumulationRegisterRecordSetObject, limit, own uint64) (uint64, bool) {
	if owner == nil || own > limit {
		return limit, false
	}
	owner.mu.RLock()
	defer owner.mu.RUnlock()
	retained, ok := accumulationRegisterSetMemory(owner.set, limit-own)
	if !ok {
		return limit, false
	}
	return own + retained, true
}

func (object *accumulationVirtualTableObject) RuntimeTypeName() string {
	return "AccumulationRegisterVirtualTable." + object.definition.Name
}
func (object *accumulationVirtualTableObject) RuntimeEqual(other bytecode.RuntimeObject) bool {
	candidate, ok := other.(*accumulationVirtualTableObject)
	return ok && candidate == object
}
func (object *accumulationVirtualTableObject) RuntimeDynamicMemory(limit uint64) (uint64, bool) {
	size := uint64(512)
	for _, row := range object.rows {
		for _, group := range []map[uuid.UUID]Value{row.Dimensions, row.Opening, row.Receipt, row.Expense, row.Turnover, row.Closing} {
			for _, value := range group {
				size += uint64(len(value.Data) + 96)
				if size > limit {
					return limit, false
				}
			}
		}
	}
	return size, true
}
func (object *accumulationVirtualTableObject) RuntimeCollectionLength() int { return len(object.rows) }
func (object *accumulationVirtualTableObject) RuntimeCollectionElement(index int) (bytecode.Value, bool) {
	if index < 0 || index >= len(object.rows) {
		return bytecode.Undefined(), false
	}
	value, err := bytecode.Object(&accumulationVirtualRowObject{owner: object, row: &object.rows[index]})
	return value, err == nil
}
func (object *accumulationVirtualTableObject) RuntimeCollectionIndex(key bytecode.Value) (bytecode.Value, error) {
	index, ok := key.NumberInteger()
	if !ok || index < 0 {
		return bytecode.Undefined(), fmt.Errorf("virtual table row index must be a non-negative integer")
	}
	value, found := object.RuntimeCollectionElement(int(index))
	if !found {
		return bytecode.Undefined(), fmt.Errorf("virtual table row index is out of range")
	}
	return value, nil
}
func (object *accumulationVirtualRowObject) RuntimeTypeName() string {
	return "AccumulationRegisterVirtualRow." + object.owner.definition.Name
}
func (object *accumulationVirtualRowObject) RuntimeEqual(other bytecode.RuntimeObject) bool {
	candidate, ok := other.(*accumulationVirtualRowObject)
	return ok && candidate.owner == object.owner && candidate.row == object.row
}
func (object *accumulationVirtualRowObject) RuntimeDynamicMemory(limit uint64) (uint64, bool) {
	if limit < 256 {
		return limit, false
	}
	retained, ok := object.owner.RuntimeDynamicMemory(limit - 256)
	if !ok {
		return limit, false
	}
	return retained + 256, retained+256 <= limit
}

func (runtime *Runtime) CreateAccumulationRegisterRecordSet(_ context.Context, name string) (bytecode.Value, error) {
	if runtime.accumulationRegisterRepository == nil {
		return bytecode.Undefined(), fmt.Errorf("accumulation register repository is not configured")
	}
	set, err := runtime.accumulationRegisterRepository.NewRecordSet(name)
	if err != nil {
		return bytecode.Undefined(), err
	}
	definition, _ := runtime.catalog.AccumulationRegisterDefinition(name)
	return bytecode.Object(&accumulationRegisterRecordSetObject{definition: definition, set: set, runtime: runtime})
}

func (runtime *Runtime) AccumulationRegisterBalances(ctx context.Context, name string, period, filter bytecode.Value) (bytecode.Value, error) {
	definition, dimensions, err := runtime.accumulationManagerArguments(name, filter)
	if err != nil {
		return bytecode.Undefined(), err
	}
	date, ok := period.AsDate()
	if !ok {
		return bytecode.Undefined(), fmt.Errorf("accumulation register balance period must be a date")
	}
	rows, err := runtime.accumulationRegisterRepository.Balances(ctx, name, date, dimensions)
	if err != nil {
		return bytecode.Undefined(), err
	}
	return runtime.wrapAccumulationVirtualTable(definition, "balances", rows)
}

func (runtime *Runtime) AccumulationRegisterTurnovers(ctx context.Context, name string, begin, end, filter bytecode.Value) (bytecode.Value, error) {
	definition, dimensions, err := runtime.accumulationManagerArguments(name, filter)
	if err != nil {
		return bytecode.Undefined(), err
	}
	from, okFrom := begin.AsDate()
	to, okTo := end.AsDate()
	if !okFrom || !okTo {
		return bytecode.Undefined(), fmt.Errorf("accumulation register turnover periods must be dates")
	}
	rows, err := runtime.accumulationRegisterRepository.Turnovers(ctx, name, from, to, dimensions)
	if err != nil {
		return bytecode.Undefined(), err
	}
	return runtime.wrapAccumulationVirtualTable(definition, "turnovers", rows)
}

func (runtime *Runtime) AccumulationRegisterBalancesAndTurnovers(ctx context.Context, name string, begin, end, filter bytecode.Value) (bytecode.Value, error) {
	definition, dimensions, err := runtime.accumulationManagerArguments(name, filter)
	if err != nil {
		return bytecode.Undefined(), err
	}
	from, okFrom := begin.AsDate()
	to, okTo := end.AsDate()
	if !okFrom || !okTo {
		return bytecode.Undefined(), fmt.Errorf("accumulation register periods must be dates")
	}
	rows, err := runtime.accumulationRegisterRepository.BalancesAndTurnovers(ctx, name, from, to, dimensions)
	if err != nil {
		return bytecode.Undefined(), err
	}
	return runtime.wrapAccumulationVirtualTable(definition, "balances-and-turnovers", rows)
}

func (runtime *Runtime) accumulationManagerArguments(name string, filter bytecode.Value) (AccumulationRegisterDefinition, map[uuid.UUID]Value, error) {
	if runtime.accumulationRegisterRepository == nil {
		return AccumulationRegisterDefinition{}, nil, fmt.Errorf("accumulation register repository is not configured")
	}
	definition, ok := runtime.catalog.AccumulationRegisterDefinition(name)
	if !ok {
		return AccumulationRegisterDefinition{}, nil, fmt.Errorf("unknown accumulation register %q", name)
	}
	dimensions, err := runtime.accumulationDimensionsFromBSL(definition, filter)
	return definition, dimensions, err
}

func (runtime *Runtime) accumulationDimensionsFromBSL(definition AccumulationRegisterDefinition, value bytecode.Value) (map[uuid.UUID]Value, error) {
	result := map[uuid.UUID]Value{}
	if value.Kind() == bytecode.UndefinedKind {
		return result, nil
	}
	names, ok := bytecode.CollectionPropertyNames(value)
	if !ok || value.Kind() != bytecode.StructureKind {
		return nil, fmt.Errorf("accumulation register filter must be a Structure")
	}
	for _, name := range names {
		dimension, ok := findCatalogAttribute(definition.Dimensions, name)
		if !ok {
			return nil, fmt.Errorf("accumulation register %s has no dimension %s", definition.Name, name)
		}
		item, err := bytecode.CollectionProperty(value, name)
		if err != nil {
			return nil, err
		}
		stored, err := runtime.applicationValueFromBSL(dimension.Types, item, "accumulation register dimension "+dimension.Name)
		if err != nil {
			return nil, err
		}
		result[dimension.ID] = stored
	}
	return result, nil
}

func (runtime *Runtime) wrapAccumulationVirtualTable(definition AccumulationRegisterDefinition, kind string, rows []AccumulationRegisterAggregate) (bytecode.Value, error) {
	return bytecode.Object(&accumulationVirtualTableObject{definition: definition, kind: kind, rows: rows, runtime: runtime})
}

func (runtime *Runtime) getAccumulationRegisterProperty(value bytecode.RuntimeObject, name string) (bytecode.Value, bool, error) {
	switch object := value.(type) {
	case *accumulationRegisterRecordSetObject:
		if object.runtime != runtime {
			return bytecode.Undefined(), true, fmt.Errorf("accumulation register record set belongs to another runtime")
		}
		object.mu.RLock()
		defer object.mu.RUnlock()
		switch {
		case propertyName(name, "Отбор", "Filter"):
			result, err := bytecode.Object(&accumulationRegisterFilterObject{owner: object})
			return result, true, err
		case propertyName(name, "Количество", "Count"):
			return bytecode.Number(float64(len(object.set.Records))), true, nil
		case propertyName(name, "БлокироватьДляИзменения", "LockForUpdate"):
			return bytecode.Boolean(object.set.LockForUpdate), true, nil
		default:
			return bytecode.Undefined(), true, fmt.Errorf("%s has no property %s", object.RuntimeTypeName(), name)
		}
	case *accumulationRegisterFilterObject:
		if object.owner.runtime != runtime {
			return bytecode.Undefined(), true, fmt.Errorf("accumulation register filter belongs to another runtime")
		}
		if !propertyName(name, "Регистратор", "Recorder") {
			return bytecode.Undefined(), true, fmt.Errorf("%s has no property %s", object.RuntimeTypeName(), name)
		}
		result, err := bytecode.Object(&accumulationRegisterRecorderFilterObject{owner: object.owner})
		return result, true, err
	case *accumulationRegisterRecorderFilterObject:
		if object.owner.runtime != runtime {
			return bytecode.Undefined(), true, fmt.Errorf("accumulation register filter belongs to another runtime")
		}
		object.owner.mu.RLock()
		defer object.owner.mu.RUnlock()
		switch {
		case propertyName(name, "Использование", "Use"):
			return bytecode.Boolean(object.owner.set.Filter.Recorder != nil), true, nil
		case propertyName(name, "Значение", "Value"):
			if object.owner.set.Filter.Recorder == nil {
				return bytecode.Undefined(), true, nil
			}
			document, _ := runtime.catalog.DocumentByID(object.owner.set.Filter.Recorder.DocumentID)
			result, err := runtime.wrapDocumentReference(document, *object.owner.set.Filter.Recorder)
			return result, true, err
		default:
			return bytecode.Undefined(), true, fmt.Errorf("%s has no property %s", object.RuntimeTypeName(), name)
		}
	case *accumulationRegisterRecordObject:
		if object.owner.runtime != runtime {
			return bytecode.Undefined(), true, fmt.Errorf("accumulation register record belongs to another runtime")
		}
		object.owner.mu.RLock()
		defer object.owner.mu.RUnlock()
		result, err := runtime.accumulationRecordProperty(object, name)
		return result, true, err
	case *accumulationVirtualTableObject:
		if object.runtime != runtime {
			return bytecode.Undefined(), true, fmt.Errorf("accumulation virtual table belongs to another runtime")
		}
		if propertyName(name, "Количество", "Count") {
			return bytecode.Number(float64(len(object.rows))), true, nil
		}
		return bytecode.Undefined(), true, fmt.Errorf("%s has no property %s", object.RuntimeTypeName(), name)
	case *accumulationVirtualRowObject:
		if object.owner.runtime != runtime {
			return bytecode.Undefined(), true, fmt.Errorf("accumulation virtual row belongs to another runtime")
		}
		result, err := runtime.accumulationVirtualRowProperty(object, name)
		return result, true, err
	default:
		return bytecode.Undefined(), false, nil
	}
}

func (runtime *Runtime) accumulationRecordProperty(object *accumulationRegisterRecordObject, name string) (bytecode.Value, error) {
	definition, record := object.owner.definition, object.record
	switch {
	case propertyName(name, "Период", "Period"):
		return bytecode.Date(record.Period)
	case propertyName(name, "Регистратор", "Recorder"):
		document, _ := runtime.catalog.DocumentByID(record.Recorder.DocumentID)
		return runtime.wrapDocumentReference(document, record.Recorder)
	case propertyName(name, "НомерСтроки", "LineNumber"):
		return bytecode.Number(float64(record.LineNumber)), nil
	case propertyName(name, "Активность", "Active"):
		return bytecode.Boolean(record.Active), nil
	case propertyName(name, "ВидДвижения", "MovementKind") && definition.Kind == AccumulationRegisterBalance:
		if record.MovementKind == AccumulationMovementExpense {
			return bytecode.String("expense"), nil
		}
		return bytecode.String("receipt"), nil
	}
	field, values, ok := accumulationRecordField(definition, record, name)
	if !ok {
		return bytecode.Undefined(), fmt.Errorf("%s has no property %s", object.RuntimeTypeName(), name)
	}
	stored, present := values[field.ID]
	if !present {
		return bytecode.Undefined(), nil
	}
	return runtime.applicationValueToBSL(field.Types, stored)
}

func (runtime *Runtime) accumulationVirtualRowProperty(object *accumulationVirtualRowObject, name string) (bytecode.Value, error) {
	if dimension, ok := findCatalogAttribute(object.owner.definition.Dimensions, name); ok {
		value, present := object.row.Dimensions[dimension.ID]
		if !present {
			return bytecode.Undefined(), nil
		}
		return runtime.applicationValueToBSL(dimension.Types, value)
	}
	for _, resource := range object.owner.definition.Resources {
		var group map[uuid.UUID]Value
		switch object.owner.kind {
		case "balances":
			if propertyName(name, resource.Name+"Остаток", resource.Name+"Balance") {
				group = object.row.Turnover
			}
		case "turnovers":
			switch {
			case propertyName(name, resource.Name+"Оборот", resource.Name+"Turnover"):
				group = object.row.Turnover
			case object.owner.definition.Kind == AccumulationRegisterBalance && propertyName(name, resource.Name+"Приход", resource.Name+"Receipt"):
				group = object.row.Receipt
			case object.owner.definition.Kind == AccumulationRegisterBalance && propertyName(name, resource.Name+"Расход", resource.Name+"Expense"):
				group = object.row.Expense
			}
		case "balances-and-turnovers":
			switch {
			case propertyName(name, resource.Name+"НачальныйОстаток", resource.Name+"OpeningBalance"):
				group = object.row.Opening
			case propertyName(name, resource.Name+"Приход", resource.Name+"Receipt"):
				group = object.row.Receipt
			case propertyName(name, resource.Name+"Расход", resource.Name+"Expense"):
				group = object.row.Expense
			case propertyName(name, resource.Name+"Оборот", resource.Name+"Turnover"):
				group = object.row.Turnover
			case propertyName(name, resource.Name+"КонечныйОстаток", resource.Name+"ClosingBalance"):
				group = object.row.Closing
			}
		}
		if group != nil {
			value := group[resource.ID]
			return bytecode.ParseNumber(value.Data)
		}
	}
	return bytecode.Undefined(), fmt.Errorf("%s has no property %s", object.RuntimeTypeName(), name)
}

func (runtime *Runtime) setAccumulationRegisterProperty(value bytecode.RuntimeObject, name string, assigned bytecode.Value) (bool, error) {
	switch object := value.(type) {
	case *accumulationRegisterRecordSetObject:
		if object.runtime != runtime {
			return true, fmt.Errorf("accumulation register record set belongs to another runtime")
		}
		if !propertyName(name, "БлокироватьДляИзменения", "LockForUpdate") {
			return true, fmt.Errorf("%s property %s is not writable", object.RuntimeTypeName(), name)
		}
		flag, ok := assigned.AsBoolean()
		if !ok {
			return true, fmt.Errorf("LockForUpdate must be a boolean")
		}
		object.mu.Lock()
		object.set.LockForUpdate = flag
		object.mu.Unlock()
		return true, nil
	case *accumulationRegisterRecorderFilterObject:
		if !propertyName(name, "Значение", "Value") {
			return true, fmt.Errorf("%s property %s is not writable", object.RuntimeTypeName(), name)
		}
		return true, runtime.setAccumulationRecorderFilter(object, assigned)
	case *accumulationRegisterRecordObject:
		return true, runtime.setAccumulationRecordProperty(object, name, assigned)
	case *accumulationRegisterFilterObject, *accumulationVirtualTableObject, *accumulationVirtualRowObject:
		return true, fmt.Errorf("%s property %s is not writable", value.RuntimeTypeName(), name)
	default:
		return false, nil
	}
}

func (runtime *Runtime) setAccumulationRecorderFilter(object *accumulationRegisterRecorderFilterObject, assigned bytecode.Value) error {
	opaque, ok := assigned.AsRuntimeObject()
	reference, valid := opaque.(*documentReferenceObject)
	if !ok || !valid || reference.runtime != runtime || !allowedAccumulationRegisterRecorder(object.owner.definition, reference.reference) {
		return fmt.Errorf("accumulation register recorder filter is invalid")
	}
	value := reference.reference
	object.owner.mu.Lock()
	object.owner.set.Filter.Recorder = &value
	object.owner.mu.Unlock()
	return nil
}

func (runtime *Runtime) setAccumulationRecordProperty(object *accumulationRegisterRecordObject, name string, assigned bytecode.Value) error {
	if object.owner.runtime != runtime {
		return fmt.Errorf("accumulation register record belongs to another runtime")
	}
	object.owner.mu.Lock()
	defer object.owner.mu.Unlock()
	definition, record := object.owner.definition, object.record
	switch {
	case propertyName(name, "Период", "Period"):
		date, ok := assigned.AsDate()
		if !ok {
			return fmt.Errorf("accumulation register period must be a date")
		}
		record.Period = date
		return nil
	case propertyName(name, "Регистратор", "Recorder"):
		opaque, ok := assigned.AsRuntimeObject()
		reference, valid := opaque.(*documentReferenceObject)
		if !ok || !valid || reference.runtime != runtime || !allowedAccumulationRegisterRecorder(definition, reference.reference) {
			return fmt.Errorf("accumulation register recorder is invalid")
		}
		record.Recorder = reference.reference
		return nil
	case propertyName(name, "НомерСтроки", "LineNumber"):
		line, ok := assigned.NumberInteger()
		if !ok || line < 0 || line > maxInformationRegisterLineNumber {
			return fmt.Errorf("accumulation register line number must be 0..%d", maxInformationRegisterLineNumber)
		}
		record.LineNumber = int(line)
		return nil
	case propertyName(name, "Активность", "Active"):
		active, ok := assigned.AsBoolean()
		if !ok {
			return fmt.Errorf("accumulation register Active must be a boolean")
		}
		record.Active = active
		return nil
	case propertyName(name, "ВидДвижения", "MovementKind") && definition.Kind == AccumulationRegisterBalance:
		text, ok := assigned.AsString()
		if !ok {
			return fmt.Errorf("accumulation register movement kind is invalid")
		}
		switch {
		case strings.EqualFold(text, "receipt"), strings.EqualFold(text, "приход"):
			record.MovementKind = AccumulationMovementReceipt
		case strings.EqualFold(text, "expense"), strings.EqualFold(text, "расход"):
			record.MovementKind = AccumulationMovementExpense
		default:
			return fmt.Errorf("accumulation register movement kind is invalid")
		}
		return nil
	}
	field, values, ok := accumulationRecordField(definition, record, name)
	if !ok {
		return fmt.Errorf("%s has no property %s", object.RuntimeTypeName(), name)
	}
	if assigned.Kind() == bytecode.UndefinedKind {
		if field.Required || slicesContainsAttribute(definition.Resources, field.ID) {
			return fmt.Errorf("accumulation register field %s is required", field.Name)
		}
		delete(values, field.ID)
		return nil
	}
	stored, err := runtime.applicationValueFromBSL(field.Types, assigned, "accumulation register field "+field.Name)
	if err != nil {
		return err
	}
	values[field.ID] = stored
	return nil
}

func slicesContainsAttribute(fields []Attribute, id uuid.UUID) bool {
	for _, field := range fields {
		if field.ID == id {
			return true
		}
	}
	return false
}

func accumulationRecordField(definition AccumulationRegisterDefinition, record *AccumulationRegisterRecord, name string) (Attribute, map[uuid.UUID]Value, bool) {
	for _, group := range []struct {
		fields []Attribute
		values map[uuid.UUID]Value
	}{{definition.Dimensions, record.Dimensions}, {definition.Resources, record.Resources}, {definition.Attributes, record.Attributes}} {
		if field, ok := findCatalogAttribute(group.fields, name); ok {
			return field, group.values, true
		}
	}
	return Attribute{}, nil, false
}

func (runtime *Runtime) callAccumulationRegisterMethod(ctx context.Context, value bytecode.RuntimeObject, name string, arguments []bytecode.Value) (bytecode.Value, bool, error) {
	switch object := value.(type) {
	case *accumulationRegisterRecordSetObject:
		if object.runtime != runtime {
			return bytecode.Undefined(), true, fmt.Errorf("accumulation register record set belongs to another runtime")
		}
		object.mu.Lock()
		defer object.mu.Unlock()
		switch {
		case propertyName(name, "Добавить", "Add"):
			if len(arguments) != 0 {
				return bytecode.Undefined(), true, fmt.Errorf("%s expects no arguments", name)
			}
			record, err := object.set.Add()
			if err != nil {
				return bytecode.Undefined(), true, err
			}
			result, err := bytecode.Object(&accumulationRegisterRecordObject{owner: object, record: record})
			return result, true, err
		case propertyName(name, "Очистить", "Clear"):
			if len(arguments) != 0 {
				return bytecode.Undefined(), true, fmt.Errorf("%s expects no arguments", name)
			}
			object.set.Records = []*AccumulationRegisterRecord{}
			return bytecode.Undefined(), true, nil
		case propertyName(name, "Количество", "Count"):
			if len(arguments) != 0 {
				return bytecode.Undefined(), true, fmt.Errorf("%s expects no arguments", name)
			}
			return bytecode.Number(float64(len(object.set.Records))), true, nil
		case propertyName(name, "Получить", "Get"):
			if len(arguments) != 1 {
				return bytecode.Undefined(), true, fmt.Errorf("%s expects one index", name)
			}
			index, ok := arguments[0].NumberInteger()
			if !ok || index < 0 || int(index) >= len(object.set.Records) {
				return bytecode.Undefined(), true, fmt.Errorf("accumulation register record index is out of range")
			}
			result, err := bytecode.Object(&accumulationRegisterRecordObject{owner: object, record: object.set.Records[index]})
			return result, true, err
		case propertyName(name, "Удалить", "Delete"):
			if len(arguments) != 1 {
				return bytecode.Undefined(), true, fmt.Errorf("%s expects one record", name)
			}
			opaque, ok := arguments[0].AsRuntimeObject()
			record, valid := opaque.(*accumulationRegisterRecordObject)
			if !ok || !valid || record.owner != object {
				return bytecode.Undefined(), true, fmt.Errorf("record does not belong to this record set")
			}
			for index, candidate := range object.set.Records {
				if candidate == record.record {
					object.set.Records = append(object.set.Records[:index], object.set.Records[index+1:]...)
					return bytecode.Undefined(), true, nil
				}
			}
			return bytecode.Undefined(), true, fmt.Errorf("record is no longer in this record set")
		case propertyName(name, "Прочитать", "Read"):
			if len(arguments) != 0 {
				return bytecode.Undefined(), true, fmt.Errorf("%s expects no arguments", name)
			}
			original := cloneAccumulationRegisterRecordSet(object.set)
			if err := runtime.accumulationRegisterRepository.Read(ctx, object.set); err != nil {
				object.set = original
				return bytecode.Undefined(), true, err
			}
			return bytecode.Undefined(), true, nil
		case propertyName(name, "Записать", "Write"):
			if len(arguments) > 1 {
				return bytecode.Undefined(), true, fmt.Errorf("%s expects zero or one boolean argument", name)
			}
			replace := true
			if len(arguments) == 1 {
				var ok bool
				replace, ok = arguments[0].AsBoolean()
				if !ok {
					return bytecode.Undefined(), true, fmt.Errorf("%s expects a boolean argument", name)
				}
			}
			if err := runtime.accumulationRegisterRepository.WriteWithHandler(ctx, object.set, replace, runtime.accumulationRegisterEventHandler(object.definition.ID)); err != nil {
				return bytecode.Undefined(), true, err
			}
			return bytecode.Undefined(), true, nil
		default:
			return bytecode.Undefined(), true, fmt.Errorf("%s has no method %s", object.RuntimeTypeName(), name)
		}
	case *accumulationRegisterRecorderFilterObject:
		switch {
		case propertyName(name, "Установить", "Set"):
			if len(arguments) != 1 {
				return bytecode.Undefined(), true, fmt.Errorf("%s expects one argument", name)
			}
			return bytecode.Undefined(), true, runtime.setAccumulationRecorderFilter(object, arguments[0])
		case propertyName(name, "Снять", "Clear"):
			if len(arguments) != 0 {
				return bytecode.Undefined(), true, fmt.Errorf("%s expects no arguments", name)
			}
			object.owner.mu.Lock()
			object.owner.set.Filter.Recorder = nil
			object.owner.mu.Unlock()
			return bytecode.Undefined(), true, nil
		default:
			return bytecode.Undefined(), true, fmt.Errorf("%s has no method %s", object.RuntimeTypeName(), name)
		}
	case *accumulationVirtualTableObject:
		if propertyName(name, "Количество", "Count") && len(arguments) == 0 {
			return bytecode.Number(float64(len(object.rows))), true, nil
		}
		return bytecode.Undefined(), true, fmt.Errorf("%s has no method %s", object.RuntimeTypeName(), name)
	case *accumulationRegisterFilterObject, *accumulationRegisterRecordObject, *accumulationVirtualRowObject:
		return bytecode.Undefined(), true, fmt.Errorf("%s has no method %s", value.RuntimeTypeName(), name)
	default:
		return bytecode.Undefined(), false, nil
	}
}
