package metadata

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/k33alexey/MetaLab/internal/bsl/bytecode"
	"github.com/k33alexey/MetaLab/internal/uuid"
)

const maxInformationRegisterRuntimeMemory = 64 << 20

type informationRegisterRecordSetObject struct {
	mu              sync.RWMutex
	definition      InformationRegisterDefinition
	set             *InformationRegisterRecordSet
	runtime         *Runtime
	readOnly        bool
	writeAtEnd      bool
	defaultRecorder *DocumentReference
	defaultPeriod   time.Time
}

type informationRegisterRecordObject struct {
	owner  *informationRegisterRecordSetObject
	record *InformationRegisterRecord
}

type informationRegisterFilterObject struct {
	owner *informationRegisterRecordSetObject
}

type informationRegisterFilterItemObject struct {
	owner     *informationRegisterRecordSetObject
	dimension *Attribute
	system    string
}

func (object *informationRegisterRecordSetObject) RuntimeTypeName() string {
	return "InformationRegisterRecordSet." + object.definition.Name
}
func (object *informationRegisterRecordSetObject) RuntimeEqual(other bytecode.RuntimeObject) bool {
	candidate, ok := other.(*informationRegisterRecordSetObject)
	return ok && candidate == object
}
func (object *informationRegisterRecordSetObject) RuntimeDynamicMemory(limit uint64) (uint64, bool) {
	object.mu.RLock()
	defer object.mu.RUnlock()
	return informationRegisterSetMemory(object.set, limit)
}

func informationRegisterSetMemory(set *InformationRegisterRecordSet, limit uint64) (uint64, bool) {
	size := uint64(512)
	for _, record := range set.Records {
		if record == nil || size > limit {
			return limit, false
		}
		size += informationRegisterRecordMemory(record)
		if size > limit {
			return limit, false
		}
	}
	for _, value := range set.Filter.Dimensions {
		size += uint64(len(value.Data) + 96)
		if size > limit {
			return limit, false
		}
	}
	return size, true
}
func (object *informationRegisterRecordSetObject) RuntimeCollectionLength() int {
	object.mu.RLock()
	defer object.mu.RUnlock()
	return len(object.set.Records)
}
func (object *informationRegisterRecordSetObject) RuntimeCollectionElement(index int) (bytecode.Value, bool) {
	object.mu.RLock()
	defer object.mu.RUnlock()
	if index < 0 || index >= len(object.set.Records) {
		return bytecode.Undefined(), false
	}
	value, err := bytecode.Object(&informationRegisterRecordObject{owner: object, record: object.set.Records[index]})
	return value, err == nil
}
func (object *informationRegisterRecordSetObject) RuntimeCollectionIndex(key bytecode.Value) (bytecode.Value, error) {
	index, ok := key.NumberInteger()
	if !ok || index < 0 || uint64(index) > uint64(^uint(0)>>1) {
		return bytecode.Undefined(), fmt.Errorf("information register record index must be a non-negative integer")
	}
	value, found := object.RuntimeCollectionElement(int(index))
	if !found {
		return bytecode.Undefined(), fmt.Errorf("information register record index is out of range")
	}
	return value, nil
}

func (object *informationRegisterRecordObject) RuntimeTypeName() string {
	return "InformationRegisterRecord." + object.owner.definition.Name
}
func (object *informationRegisterRecordObject) RuntimeEqual(other bytecode.RuntimeObject) bool {
	candidate, ok := other.(*informationRegisterRecordObject)
	return ok && candidate.owner == object.owner && candidate.record == object.record
}
func (object *informationRegisterRecordObject) RuntimeDynamicMemory(limit uint64) (uint64, bool) {
	return informationRegisterChildMemory(object.owner, limit, 256)
}

func (*informationRegisterFilterObject) RuntimeTypeName() string { return "InformationRegisterFilter" }
func (object *informationRegisterFilterObject) RuntimeEqual(other bytecode.RuntimeObject) bool {
	candidate, ok := other.(*informationRegisterFilterObject)
	return ok && candidate.owner == object.owner
}
func (object *informationRegisterFilterObject) RuntimeDynamicMemory(limit uint64) (uint64, bool) {
	return informationRegisterChildMemory(object.owner, limit, 192)
}

func (*informationRegisterFilterItemObject) RuntimeTypeName() string {
	return "InformationRegisterFilterItem"
}
func (object *informationRegisterFilterItemObject) RuntimeEqual(other bytecode.RuntimeObject) bool {
	candidate, ok := other.(*informationRegisterFilterItemObject)
	return ok && candidate.owner == object.owner && candidate.system == object.system &&
		(candidate.dimension == nil && object.dimension == nil || candidate.dimension != nil && object.dimension != nil && candidate.dimension.ID == object.dimension.ID)
}
func (object *informationRegisterFilterItemObject) RuntimeDynamicMemory(limit uint64) (uint64, bool) {
	return informationRegisterChildMemory(object.owner, limit, 192)
}

func informationRegisterChildMemory(owner *informationRegisterRecordSetObject, limit, own uint64) (uint64, bool) {
	if owner == nil || own > limit {
		return limit, false
	}
	owner.mu.RLock()
	defer owner.mu.RUnlock()
	retained, ok := informationRegisterSetMemory(owner.set, limit-own)
	if !ok {
		return limit, false
	}
	return own + retained, true
}

func informationRegisterRecordMemory(record *InformationRegisterRecord) uint64 {
	size := uint64(512)
	for _, group := range []map[uuid.UUID]Value{record.Dimensions, record.Resources, record.Attributes} {
		for _, value := range group {
			size += uint64(len(value.Data) + 96)
		}
	}
	return size
}

func (runtime *Runtime) CreateInformationRegisterRecordSet(_ context.Context, name string) (bytecode.Value, error) {
	if runtime.informationRegisterRepository == nil {
		return bytecode.Undefined(), fmt.Errorf("information register repository is not configured")
	}
	set, err := runtime.informationRegisterRepository.NewRecordSet(name)
	if err != nil {
		return bytecode.Undefined(), err
	}
	definition, _ := runtime.catalog.InformationRegisterDefinition(name)
	return runtime.wrapInformationRegisterRecordSet(definition, set)
}

func (runtime *Runtime) InformationRegisterSliceLast(ctx context.Context, name string, period, filter bytecode.Value) (bytecode.Value, error) {
	return runtime.informationRegisterSlice(ctx, name, period, filter, false)
}

func (runtime *Runtime) InformationRegisterSliceFirst(ctx context.Context, name string, period, filter bytecode.Value) (bytecode.Value, error) {
	return runtime.informationRegisterSlice(ctx, name, period, filter, true)
}

func (runtime *Runtime) informationRegisterSlice(ctx context.Context, name string, periodValue, filterValue bytecode.Value, first bool) (bytecode.Value, error) {
	if runtime.informationRegisterRepository == nil {
		return bytecode.Undefined(), fmt.Errorf("information register repository is not configured")
	}
	period, ok := periodValue.AsDate()
	if !ok {
		return bytecode.Undefined(), fmt.Errorf("information register slice period must be a date")
	}
	definition, ok := runtime.catalog.InformationRegisterDefinition(name)
	if !ok {
		return bytecode.Undefined(), fmt.Errorf("unknown information register %q", name)
	}
	dimensions, err := runtime.informationRegisterDimensionsFromBSL(definition, filterValue)
	if err != nil {
		return bytecode.Undefined(), err
	}
	var records []*InformationRegisterRecord
	if first {
		records, err = runtime.informationRegisterRepository.SliceFirst(ctx, name, period, dimensions)
	} else {
		records, err = runtime.informationRegisterRepository.SliceLast(ctx, name, period, dimensions)
	}
	if err != nil {
		return bytecode.Undefined(), err
	}
	set := &InformationRegisterRecordSet{RegisterID: definition.ID, Filter: InformationRegisterFilter{Dimensions: dimensions}, Records: records}
	return runtime.wrapInformationRegisterReadOnlySet(definition, set)
}

func (runtime *Runtime) informationRegisterDimensionsFromBSL(definition InformationRegisterDefinition, value bytecode.Value) (map[uuid.UUID]Value, error) {
	result := map[uuid.UUID]Value{}
	if value.Kind() == bytecode.UndefinedKind {
		return result, nil
	}
	names, ok := bytecode.CollectionPropertyNames(value)
	if !ok || value.Kind() != bytecode.StructureKind {
		return nil, fmt.Errorf("information register slice filter must be a Structure")
	}
	for _, name := range names {
		dimension, ok := findCatalogAttribute(definition.Dimensions, name)
		if !ok {
			return nil, fmt.Errorf("information register %s has no dimension %s", definition.Name, name)
		}
		item, err := bytecode.CollectionProperty(value, name)
		if err != nil {
			return nil, err
		}
		stored, err := runtime.applicationValueFromBSL(dimension.Types, item, "information register dimension "+dimension.Name)
		if err != nil {
			return nil, err
		}
		result[dimension.ID] = stored
	}
	return result, nil
}

func (runtime *Runtime) wrapInformationRegisterRecordSet(definition InformationRegisterDefinition, set *InformationRegisterRecordSet) (bytecode.Value, error) {
	return bytecode.Object(&informationRegisterRecordSetObject{definition: definition, set: cloneInformationRegisterRecordSet(set), runtime: runtime})
}

func (runtime *Runtime) wrapInformationRegisterReadOnlySet(definition InformationRegisterDefinition, set *InformationRegisterRecordSet) (bytecode.Value, error) {
	return bytecode.Object(&informationRegisterRecordSetObject{definition: definition, set: cloneInformationRegisterRecordSet(set), runtime: runtime, readOnly: true})
}

func (runtime *Runtime) getInformationRegisterProperty(value bytecode.RuntimeObject, name string) (bytecode.Value, bool, error) {
	switch object := value.(type) {
	case *informationRegisterRecordSetObject:
		if object.runtime != runtime {
			return bytecode.Undefined(), true, fmt.Errorf("information register record set belongs to another metadata runtime")
		}
		object.mu.RLock()
		defer object.mu.RUnlock()
		switch {
		case propertyName(name, "Отбор", "Filter"):
			result, err := bytecode.Object(&informationRegisterFilterObject{owner: object})
			return result, true, err
		case propertyName(name, "Количество", "Count"):
			return bytecode.Number(float64(len(object.set.Records))), true, nil
		case propertyName(name, "Записывать", "Write"):
			return bytecode.Boolean(object.writeAtEnd), true, nil
		}
		return bytecode.Undefined(), true, fmt.Errorf("%s has no property %s", object.RuntimeTypeName(), name)
	case *informationRegisterFilterObject:
		if object.owner.runtime != runtime {
			return bytecode.Undefined(), true, fmt.Errorf("information register filter belongs to another metadata runtime")
		}
		item := &informationRegisterFilterItemObject{owner: object.owner}
		switch {
		case propertyName(name, "Период", "Period") && object.owner.definition.Periodicity != InformationRegisterPeriodNone:
			item.system = "period"
		case propertyName(name, "Регистратор", "Recorder") && object.owner.definition.WriteMode == InformationRegisterRecorder:
			item.system = "recorder"
		default:
			dimension, ok := findCatalogAttribute(object.owner.definition.Dimensions, name)
			if !ok {
				return bytecode.Undefined(), true, fmt.Errorf("information register %s has no filter field %s", object.owner.definition.Name, name)
			}
			item.dimension = &dimension
		}
		result, err := bytecode.Object(item)
		return result, true, err
	case *informationRegisterFilterItemObject:
		if object.owner.runtime != runtime {
			return bytecode.Undefined(), true, fmt.Errorf("information register filter item belongs to another metadata runtime")
		}
		object.owner.mu.RLock()
		defer object.owner.mu.RUnlock()
		stored, used := informationRegisterFilterItemValue(object)
		switch {
		case propertyName(name, "Использование", "Use"):
			return bytecode.Boolean(used), true, nil
		case propertyName(name, "Значение", "Value"):
			if !used {
				return bytecode.Undefined(), true, nil
			}
			result, err := runtime.informationRegisterFilterValueToBSL(object, stored)
			return result, true, err
		default:
			return bytecode.Undefined(), true, fmt.Errorf("%s has no property %s", object.RuntimeTypeName(), name)
		}
	case *informationRegisterRecordObject:
		if object.owner.runtime != runtime {
			return bytecode.Undefined(), true, fmt.Errorf("information register record belongs to another metadata runtime")
		}
		object.owner.mu.RLock()
		defer object.owner.mu.RUnlock()
		result, err := runtime.informationRegisterRecordProperty(object, name)
		return result, true, err
	default:
		return bytecode.Undefined(), false, nil
	}

}

func (runtime *Runtime) informationRegisterRecordProperty(object *informationRegisterRecordObject, name string) (bytecode.Value, error) {
	definition, record := object.owner.definition, object.record
	switch {
	case propertyName(name, "Период", "Period") && definition.Periodicity != InformationRegisterPeriodNone:
		return bytecode.Date(record.Period)
	case propertyName(name, "Регистратор", "Recorder") && definition.WriteMode == InformationRegisterRecorder:
		document, ok := runtime.catalog.DocumentByID(record.Recorder.DocumentID)
		if !ok {
			return bytecode.Undefined(), fmt.Errorf("unknown recorder document %s", record.Recorder.DocumentID)
		}
		return runtime.wrapDocumentReference(document, record.Recorder)
	case propertyName(name, "НомерСтроки", "LineNumber") && definition.WriteMode == InformationRegisterRecorder:
		return bytecode.Number(float64(record.LineNumber)), nil
	case propertyName(name, "Активность", "Active") && definition.WriteMode == InformationRegisterRecorder:
		return bytecode.Boolean(record.Active), nil
	}
	field, values, ok := informationRegisterRecordField(definition, record, name)
	if !ok {
		return bytecode.Undefined(), fmt.Errorf("%s has no property %s", object.RuntimeTypeName(), name)
	}
	stored, present := values[field.ID]
	if !present {
		return bytecode.Undefined(), nil
	}
	return runtime.applicationValueToBSL(field.Types, stored)
}

func (runtime *Runtime) setInformationRegisterProperty(value bytecode.RuntimeObject, name string, assigned bytecode.Value) (bool, error) {
	switch object := value.(type) {
	case *informationRegisterRecordSetObject:
		if object.runtime != runtime {
			return true, fmt.Errorf("information register record set belongs to another metadata runtime")
		}
		if !propertyName(name, "Записывать", "Write") || object.readOnly {
			return true, fmt.Errorf("%s property %s is not writable", object.RuntimeTypeName(), name)
		}
		flag, ok := assigned.AsBoolean()
		if !ok {
			return true, fmt.Errorf("Write must be a boolean")
		}
		object.mu.Lock()
		object.writeAtEnd = flag
		object.mu.Unlock()
		return true, nil
	case *informationRegisterFilterItemObject:
		if object.owner.runtime != runtime {
			return true, fmt.Errorf("information register filter item belongs to another metadata runtime")
		}
		switch {
		case propertyName(name, "Значение", "Value"):
			return true, runtime.setInformationRegisterFilterItem(object, assigned)
		case propertyName(name, "Использование", "Use"):
			use, ok := assigned.AsBoolean()
			if !ok {
				return true, fmt.Errorf("filter Use must be a boolean")
			}
			if use {
				object.owner.mu.RLock()
				_, present := informationRegisterFilterItemValueLocked(object)
				object.owner.mu.RUnlock()
				if !present {
					return true, fmt.Errorf("filter value must be set before enabling it")
				}
				return true, nil
			}
			return true, runtime.clearInformationRegisterFilterItem(object)
		default:
			return true, fmt.Errorf("%s property %s is not writable", object.RuntimeTypeName(), name)
		}
	case *informationRegisterRecordObject:
		if object.owner.runtime != runtime {
			return true, fmt.Errorf("information register record belongs to another metadata runtime")
		}
		return true, runtime.setInformationRegisterRecordProperty(object, name, assigned)
	case *informationRegisterFilterObject:
		return true, fmt.Errorf("%s property %s is not writable", value.RuntimeTypeName(), name)
	default:
		return false, nil
	}
}

func (runtime *Runtime) setInformationRegisterRecordProperty(object *informationRegisterRecordObject, name string, assigned bytecode.Value) error {
	object.owner.mu.Lock()
	defer object.owner.mu.Unlock()
	if object.owner.readOnly {
		return fmt.Errorf("information register slice is read-only")
	}
	definition, record := object.owner.definition, object.record
	switch {
	case propertyName(name, "Период", "Period") && definition.Periodicity != InformationRegisterPeriodNone:
		date, ok := assigned.AsDate()
		if !ok {
			return fmt.Errorf("information register period must be a date")
		}
		record.Period = date
		return nil
	case propertyName(name, "Регистратор", "Recorder") && definition.WriteMode == InformationRegisterRecorder:
		opaque, ok := assigned.AsRuntimeObject()
		reference, okReference := opaque.(*documentReferenceObject)
		if !ok || !okReference || reference.runtime != runtime || !allowedInformationRegisterRecorder(definition, reference.reference) {
			return fmt.Errorf("information register recorder is invalid")
		}
		record.Recorder = reference.reference
		return nil
	case propertyName(name, "НомерСтроки", "LineNumber") && definition.WriteMode == InformationRegisterRecorder:
		line, ok := assigned.NumberInteger()
		if !ok || line < 0 || line > maxInformationRegisterLineNumber {
			return fmt.Errorf("information register line number must be 0..%d", maxInformationRegisterLineNumber)
		}
		record.LineNumber = int(line)
		return nil
	case propertyName(name, "Активность", "Active") && definition.WriteMode == InformationRegisterRecorder:
		active, ok := assigned.AsBoolean()
		if !ok {
			return fmt.Errorf("information register Active must be a boolean")
		}
		record.Active = active
		return nil
	}
	field, values, ok := informationRegisterRecordField(definition, record, name)
	if !ok {
		return fmt.Errorf("%s has no property %s", object.RuntimeTypeName(), name)
	}
	if assigned.Kind() == bytecode.UndefinedKind {
		if field.Required {
			return fmt.Errorf("information register field %s is required", field.Name)
		}
		delete(values, field.ID)
		return nil
	}
	stored, err := runtime.applicationValueFromBSL(field.Types, assigned, "information register field "+field.Name)
	if err != nil {
		return err
	}
	values[field.ID] = stored
	return nil
}

func informationRegisterRecordField(definition InformationRegisterDefinition, record *InformationRegisterRecord, name string) (Attribute, map[uuid.UUID]Value, bool) {
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

func (runtime *Runtime) callInformationRegisterMethod(ctx context.Context, value bytecode.RuntimeObject, name string, arguments []bytecode.Value) (bytecode.Value, bool, error) {
	switch object := value.(type) {
	case *informationRegisterRecordSetObject:
		if object.runtime != runtime {
			return bytecode.Undefined(), true, fmt.Errorf("information register record set belongs to another metadata runtime")
		}
		object.mu.Lock()
		defer object.mu.Unlock()
		switch {
		case propertyName(name, "Добавить", "Add"):
			if object.readOnly {
				return bytecode.Undefined(), true, fmt.Errorf("information register slice is read-only")
			}
			if len(arguments) != 0 {
				return bytecode.Undefined(), true, fmt.Errorf("%s expects no arguments", name)
			}
			record, err := object.set.Add()
			if err != nil {
				return bytecode.Undefined(), true, err
			}
			if object.defaultRecorder != nil {
				record.Recorder = *object.defaultRecorder
				record.Active = true
				if object.definition.Periodicity != InformationRegisterPeriodNone {
					record.Period = object.defaultPeriod
				}
			}
			result, err := bytecode.Object(&informationRegisterRecordObject{owner: object, record: record})
			return result, true, err
		case propertyName(name, "Очистить", "Clear"):
			if object.readOnly {
				return bytecode.Undefined(), true, fmt.Errorf("information register slice is read-only")
			}
			if len(arguments) != 0 {
				return bytecode.Undefined(), true, fmt.Errorf("%s expects no arguments", name)
			}
			object.set.Records = []*InformationRegisterRecord{}
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
			if !ok || index < 0 || uint64(index) >= uint64(len(object.set.Records)) {
				return bytecode.Undefined(), true, fmt.Errorf("information register record index is out of range")
			}
			result, err := bytecode.Object(&informationRegisterRecordObject{owner: object, record: object.set.Records[index]})
			return result, true, err
		case propertyName(name, "Удалить", "Delete"):
			if object.readOnly {
				return bytecode.Undefined(), true, fmt.Errorf("information register slice is read-only")
			}
			if len(arguments) != 1 {
				return bytecode.Undefined(), true, fmt.Errorf("%s expects one record", name)
			}
			row, ok := arguments[0].AsRuntimeObject()
			record, okRecord := row.(*informationRegisterRecordObject)
			if !ok || !okRecord || record.owner != object {
				return bytecode.Undefined(), true, fmt.Errorf("record does not belong to this record set")
			}
			for index, candidate := range object.set.Records {
				if candidate == record.record {
					copy(object.set.Records[index:], object.set.Records[index+1:])
					object.set.Records[len(object.set.Records)-1] = nil
					object.set.Records = object.set.Records[:len(object.set.Records)-1]
					return bytecode.Undefined(), true, nil
				}
			}
			return bytecode.Undefined(), true, fmt.Errorf("record is no longer in this record set")
		case propertyName(name, "Прочитать", "Read"):
			if object.readOnly {
				return bytecode.Undefined(), true, fmt.Errorf("information register slice is read-only")
			}
			if len(arguments) != 0 {
				return bytecode.Undefined(), true, fmt.Errorf("%s expects no arguments", name)
			}
			original := cloneInformationRegisterRecordSet(object.set)
			if err := runtime.informationRegisterRepository.Read(ctx, object.set); err != nil {
				return bytecode.Undefined(), true, err
			}
			if size, ok := informationRegisterSetMemory(object.set, maxInformationRegisterRuntimeMemory); !ok || size > maxInformationRegisterRuntimeMemory {
				object.set = original
				return bytecode.Undefined(), true, fmt.Errorf("information register record set exceeds runtime memory limit")
			}
			return bytecode.Undefined(), true, nil
		case propertyName(name, "Записать", "Write"):
			if object.readOnly {
				return bytecode.Undefined(), true, fmt.Errorf("information register slice is read-only")
			}
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
			if err := runtime.informationRegisterRepository.WriteWithHandler(ctx, object.set, replace, runtime.informationRegisterEventHandler(object.definition.ID)); err != nil {
				return bytecode.Undefined(), true, err
			}
			return bytecode.Undefined(), true, nil
		default:
			return bytecode.Undefined(), true, fmt.Errorf("%s has no method %s", object.RuntimeTypeName(), name)
		}
	case *informationRegisterFilterItemObject:
		if object.owner.runtime != runtime {
			return bytecode.Undefined(), true, fmt.Errorf("information register filter item belongs to another metadata runtime")
		}
		switch {
		case propertyName(name, "Установить", "Set"):
			if len(arguments) != 1 {
				return bytecode.Undefined(), true, fmt.Errorf("%s expects one argument", name)
			}
			return bytecode.Undefined(), true, runtime.setInformationRegisterFilterItem(object, arguments[0])
		case propertyName(name, "Снять", "Clear"):
			if len(arguments) != 0 {
				return bytecode.Undefined(), true, fmt.Errorf("%s expects no arguments", name)
			}
			return bytecode.Undefined(), true, runtime.clearInformationRegisterFilterItem(object)
		default:
			return bytecode.Undefined(), true, fmt.Errorf("%s has no method %s", object.RuntimeTypeName(), name)
		}
	case *informationRegisterFilterObject, *informationRegisterRecordObject:
		return bytecode.Undefined(), true, fmt.Errorf("%s has no method %s", value.RuntimeTypeName(), name)
	default:
		return bytecode.Undefined(), false, nil
	}
}

func (runtime *Runtime) setInformationRegisterFilterItem(item *informationRegisterFilterItemObject, assigned bytecode.Value) error {
	item.owner.mu.Lock()
	defer item.owner.mu.Unlock()
	if item.owner.readOnly {
		return fmt.Errorf("information register slice is read-only")
	}
	filter := &item.owner.set.Filter
	switch item.system {
	case "period":
		value, ok := assigned.AsDate()
		if !ok {
			return fmt.Errorf("information register period filter must be a date")
		}
		filter.Period = &value
		return nil
	case "recorder":
		opaque, ok := assigned.AsRuntimeObject()
		reference, okReference := opaque.(*documentReferenceObject)
		if !ok || !okReference || reference.runtime != runtime || !allowedInformationRegisterRecorder(item.owner.definition, reference.reference) {
			return fmt.Errorf("information register recorder filter is invalid")
		}
		value := reference.reference
		filter.Recorder = &value
		return nil
	default:
		if item.dimension == nil {
			return fmt.Errorf("information register filter field is invalid")
		}
		value, err := runtime.applicationValueFromBSL(item.dimension.Types, assigned, "information register dimension "+item.dimension.Name)
		if err != nil {
			return err
		}
		filter.Dimensions[item.dimension.ID] = value
		return nil
	}
}

func (runtime *Runtime) clearInformationRegisterFilterItem(item *informationRegisterFilterItemObject) error {
	item.owner.mu.Lock()
	defer item.owner.mu.Unlock()
	if item.owner.readOnly {
		return fmt.Errorf("information register slice is read-only")
	}
	switch item.system {
	case "period":
		item.owner.set.Filter.Period = nil
	case "recorder":
		item.owner.set.Filter.Recorder = nil
	default:
		if item.dimension == nil {
			return fmt.Errorf("information register filter field is invalid")
		}
		delete(item.owner.set.Filter.Dimensions, item.dimension.ID)
	}
	return nil
}

func informationRegisterFilterItemValue(item *informationRegisterFilterItemObject) (Value, bool) {
	return informationRegisterFilterItemValueLocked(item)
}

func informationRegisterFilterItemValueLocked(item *informationRegisterFilterItemObject) (Value, bool) {
	switch item.system {
	case "period":
		if item.owner.set.Filter.Period == nil {
			return Value{}, false
		}
		return Value{Kind: DateType, Data: item.owner.set.Filter.Period.Format(time.RFC3339Nano)}, true
	case "recorder":
		if item.owner.set.Filter.Recorder == nil {
			return Value{}, false
		}
		return Value{Kind: DocumentType, Data: item.owner.set.Filter.Recorder.ObjectID.String()}, true
	default:
		if item.dimension == nil {
			return Value{}, false
		}
		value, ok := item.owner.set.Filter.Dimensions[item.dimension.ID]
		return value, ok
	}
}

func (runtime *Runtime) informationRegisterFilterValueToBSL(item *informationRegisterFilterItemObject, value Value) (bytecode.Value, error) {
	switch item.system {
	case "period":
		date, err := time.Parse(time.RFC3339Nano, value.Data)
		if err != nil {
			return bytecode.Undefined(), err
		}
		return bytecode.Date(date)
	case "recorder":
		reference := *item.owner.set.Filter.Recorder
		definition, ok := runtime.catalog.DocumentByID(reference.DocumentID)
		if !ok {
			return bytecode.Undefined(), fmt.Errorf("unknown recorder document %s", reference.DocumentID)
		}
		return runtime.wrapDocumentReference(definition, reference)
	default:
		return runtime.applicationValueToBSL(item.dimension.Types, value)
	}
}
