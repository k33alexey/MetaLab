package metadata

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"sort"
	"strings"
	"sync"

	"github.com/jackc/pgx/v5"

	"github.com/k33alexey/MetaLab/internal/bsl/bytecode"
	"github.com/k33alexey/MetaLab/internal/uuid"
)

const (
	dataLockExclusive = "exclusive"
	dataLockShared    = "shared"
)

type dataLock struct {
	mu      sync.RWMutex
	runtime *Runtime
	entries []*dataLockElement
}

type dataLockElement struct {
	mu                     sync.RWMutex
	runtime                *Runtime
	kind                   TypeKind
	metadataID             uuid.UUID
	objectID               uuid.UUID
	register               *InformationRegisterDefinition
	filter                 InformationRegisterFilter
	accumulationRegister   *AccumulationRegisterDefinition
	accumulationDimensions map[uuid.UUID]Value
	space                  string
	mode                   string
}

func (*dataLock) RuntimeTypeName() string { return "DataLock" }
func (lock *dataLock) RuntimeEqual(other bytecode.RuntimeObject) bool {
	candidate, ok := other.(*dataLock)
	return ok && candidate == lock
}
func (lock *dataLock) RuntimeDynamicMemory(limit uint64) (uint64, bool) {
	lock.mu.RLock()
	defer lock.mu.RUnlock()
	size := uint64(256)
	if size > limit {
		return limit, false
	}
	for _, entry := range lock.entries {
		entry.mu.RLock()
		addition := uint64(256 + len(entry.space) + len(entry.mode))
		for _, value := range entry.filter.Dimensions {
			addition += uint64(len(value.Data) + 96)
		}
		for _, value := range entry.accumulationDimensions {
			addition += uint64(len(value.Data) + 96)
		}
		entry.mu.RUnlock()
		if addition > limit-size {
			return limit, false
		}
		size += addition
	}
	return size, size <= limit
}

func (*dataLockElement) RuntimeTypeName() string { return "DataLockElement" }
func (element *dataLockElement) RuntimeEqual(other bytecode.RuntimeObject) bool {
	candidate, ok := other.(*dataLockElement)
	return ok && candidate == element
}
func (element *dataLockElement) RuntimeDynamicMemory(limit uint64) (uint64, bool) {
	element.mu.RLock()
	defer element.mu.RUnlock()
	size := uint64(256 + len(element.space) + len(element.mode))
	for _, value := range element.filter.Dimensions {
		size += uint64(len(value.Data) + 96)
	}
	for _, value := range element.accumulationDimensions {
		size += uint64(len(value.Data) + 96)
	}
	return size, size <= limit
}

// ConstructRuntimeObject creates server-only platform objects used by BSL.
func (runtime *Runtime) ConstructRuntimeObject(_ context.Context, name string, arguments []bytecode.Value) (bytecode.Value, bool, error) {
	if propertyName(name, "Запрос", "Query") {
		value, err := runtime.constructQuery(arguments)
		return value, true, err
	}
	if !propertyName(name, "БлокировкаДанных", "DataLock") {
		return bytecode.Undefined(), false, nil
	}
	if len(arguments) != 0 {
		return bytecode.Undefined(), true, fmt.Errorf("%s expects no arguments", name)
	}
	value, err := bytecode.Object(&dataLock{runtime: runtime})
	return value, true, err
}

func lockObjectForWrite(ctx context.Context, transaction pgx.Tx, kind TypeKind, metadataID, objectID uuid.UUID) error {
	if transaction == nil || metadataID.IsZero() || objectID.IsZero() {
		return fmt.Errorf("invalid automatic object lock")
	}
	_, err := transaction.Exec(ctx, "SELECT pg_advisory_xact_lock($1)", objectLockKey(kind, metadataID, objectID))
	if err != nil {
		return fmt.Errorf("lock %s object: %w", kind, err)
	}
	return nil
}

func objectLockKey(kind TypeKind, metadataID, objectID uuid.UUID) int64 {
	sum := sha256.Sum256([]byte(string(kind) + ":" + metadataID.String() + ":" + objectID.String()))
	return int64(binary.BigEndian.Uint64(sum[:8]))
}

func (runtime *Runtime) newDataLockElement(space string) (*dataLockElement, error) {
	parts := strings.Split(space, ".")
	if len(parts) != 2 || strings.TrimSpace(parts[1]) == "" {
		return nil, fmt.Errorf("unsupported data lock space %q", space)
	}
	name := strings.TrimSpace(parts[1])
	switch {
	case propertyName(strings.TrimSpace(parts[0]), "Справочник", "Catalog"):
		definition, ok := runtime.catalog.CatalogDefinition(name)
		if !ok {
			return nil, fmt.Errorf("unknown catalog %q", name)
		}
		return &dataLockElement{runtime: runtime, kind: CatalogType, metadataID: definition.ID, space: "Catalog." + definition.Name, mode: dataLockExclusive}, nil
	case propertyName(strings.TrimSpace(parts[0]), "Документ", "Document"):
		definition, ok := runtime.catalog.DocumentDefinition(name)
		if !ok {
			return nil, fmt.Errorf("unknown document %q", name)
		}
		return &dataLockElement{runtime: runtime, kind: DocumentType, metadataID: definition.ID, space: "Document." + definition.Name, mode: dataLockExclusive}, nil
	case propertyName(strings.TrimSpace(parts[0]), "РегистрСведений", "InformationRegister"):
		definition, ok := runtime.catalog.InformationRegisterDefinition(name)
		if !ok {
			return nil, fmt.Errorf("unknown information register %q", name)
		}
		return &dataLockElement{
			runtime: runtime, kind: TypeKind("information-register"), metadataID: definition.ID,
			register: &definition, filter: InformationRegisterFilter{Dimensions: map[uuid.UUID]Value{}},
			space: "InformationRegister." + definition.Name, mode: dataLockExclusive,
		}, nil
	case propertyName(strings.TrimSpace(parts[0]), "РегистрНакопления", "AccumulationRegister"):
		definition, ok := runtime.catalog.AccumulationRegisterDefinition(name)
		if !ok {
			return nil, fmt.Errorf("unknown accumulation register %q", name)
		}
		return &dataLockElement{
			runtime: runtime, kind: TypeKind("accumulation-register"), metadataID: definition.ID,
			accumulationRegister: &definition, accumulationDimensions: map[uuid.UUID]Value{},
			space: "AccumulationRegister." + definition.Name, mode: dataLockExclusive,
		}, nil
	default:
		return nil, fmt.Errorf("unsupported data lock space %q", space)
	}
}

func (runtime *Runtime) setDataLockValue(element *dataLockElement, field string, value bytecode.Value) error {
	element.mu.RLock()
	register := element.register
	accumulationRegister := element.accumulationRegister
	element.mu.RUnlock()
	if register != nil {
		return runtime.setInformationRegisterDataLockValue(element, field, value)
	}
	if accumulationRegister != nil {
		return runtime.setAccumulationRegisterDataLockValue(element, field, value)
	}
	if !propertyName(field, "Ссылка", "Ref") {
		return fmt.Errorf("data lock field %q is not supported", field)
	}
	object, ok := value.AsRuntimeObject()
	if !ok {
		return fmt.Errorf("data lock Ref value must be an object reference")
	}
	var objectID uuid.UUID
	switch reference := object.(type) {
	case *catalogReferenceObject:
		if reference.runtime != runtime || element.kind != CatalogType || reference.reference.CatalogID != element.metadataID {
			return fmt.Errorf("data lock reference does not belong to %s", element.space)
		}
		objectID = reference.reference.ObjectID
	case *documentReferenceObject:
		if reference.runtime != runtime || element.kind != DocumentType || reference.reference.DocumentID != element.metadataID {
			return fmt.Errorf("data lock reference does not belong to %s", element.space)
		}
		objectID = reference.reference.ObjectID
	default:
		return fmt.Errorf("data lock Ref value must be an object reference")
	}
	if objectID.IsZero() {
		return fmt.Errorf("data lock Ref value must not be empty")
	}
	element.mu.Lock()
	element.objectID = objectID
	element.mu.Unlock()
	return nil
}

func (runtime *Runtime) setAccumulationRegisterDataLockValue(element *dataLockElement, field string, value bytecode.Value) error {
	element.mu.RLock()
	definition := cloneAccumulationRegisterDefinition(*element.accumulationRegister)
	element.mu.RUnlock()
	dimension, ok := findCatalogAttribute(definition.Dimensions, field)
	if !ok {
		return fmt.Errorf("accumulation register data lock field %q is not supported", field)
	}
	stored, err := runtime.applicationValueFromBSL(dimension.Types, value, "accumulation register data lock dimension "+dimension.Name)
	if err != nil {
		return err
	}
	element.mu.Lock()
	element.accumulationDimensions[dimension.ID] = stored
	element.mu.Unlock()
	return nil
}

func (runtime *Runtime) setInformationRegisterDataLockValue(element *dataLockElement, field string, value bytecode.Value) error {
	element.mu.RLock()
	definition := cloneInformationRegisterDefinition(*element.register)
	element.mu.RUnlock()
	switch {
	case propertyName(field, "Период", "Period") && definition.Periodicity != InformationRegisterPeriodNone:
		period, ok := value.AsDate()
		if !ok {
			return fmt.Errorf("information register data lock period must be a date")
		}
		period, err := normalizeInformationRegisterPeriod(definition.Periodicity, period)
		if err != nil {
			return err
		}
		element.mu.Lock()
		element.filter.Period = &period
		element.mu.Unlock()
		return nil
	case propertyName(field, "Регистратор", "Recorder") && definition.WriteMode == InformationRegisterRecorder:
		opaque, ok := value.AsRuntimeObject()
		reference, valid := opaque.(*documentReferenceObject)
		if !ok || !valid || reference.runtime != runtime || !allowedInformationRegisterRecorder(definition, reference.reference) {
			return fmt.Errorf("information register data lock recorder is invalid")
		}
		stored := reference.reference
		element.mu.Lock()
		element.filter.Recorder = &stored
		element.mu.Unlock()
		return nil
	}
	dimension, ok := findCatalogAttribute(definition.Dimensions, field)
	if !ok {
		return fmt.Errorf("information register data lock field %q is not supported", field)
	}
	stored, err := runtime.applicationValueFromBSL(dimension.Types, value, "information register data lock dimension "+dimension.Name)
	if err != nil {
		return err
	}
	element.mu.Lock()
	element.filter.Dimensions[dimension.ID] = stored
	element.mu.Unlock()
	return nil
}

type resolvedDataLock struct {
	key  int64
	mode string
	tier int
}

func (runtime *Runtime) acquireDataLocks(ctx context.Context, lock *dataLock) error {
	pool, err := runtime.databasePool()
	if err != nil {
		return err
	}
	scope, ok := scopeFromContext(ctx, pool)
	if !ok || scope.tx == nil || scope.depth < 1 {
		return ErrTransactionNotActive
	}
	if scope.rollbackOnly {
		return ErrTransactionDoomed
	}
	lock.mu.RLock()
	entries := append([]*dataLockElement(nil), lock.entries...)
	lock.mu.RUnlock()
	resolved := make(map[int64]resolvedDataLock, len(entries)*2)
	merge := func(key int64, mode string, tier int) {
		current, exists := resolved[key]
		if !exists || current.mode == dataLockShared && mode == dataLockExclusive {
			resolved[key] = resolvedDataLock{key: key, mode: mode, tier: tier}
		}
	}
	for _, entry := range entries {
		entry.mu.RLock()
		kind, metadataID, objectID, mode := entry.kind, entry.metadataID, entry.objectID, entry.mode
		register, filter := entry.register, cloneInformationRegisterFilter(entry.filter)
		accumulationRegister, accumulationDimensions := entry.accumulationRegister, mapsCloneValues(entry.accumulationDimensions)
		entry.mu.RUnlock()
		if register != nil {
			tableKey := objectLockKey(TypeKind("information-register"), metadataID, metadataID)
			exact := len(filter.Dimensions) == len(register.Dimensions)
			if register.Periodicity != InformationRegisterPeriodNone {
				exact = exact && filter.Period != nil
			}
			if register.WriteMode == InformationRegisterRecorder {
				exact = filter.Recorder != nil
			}
			if exact {
				merge(tableKey, dataLockShared, 0)
				merge(informationRegisterFilterLockKey(*register, filter), mode, 1)
			} else {
				// A partial register range cannot be represented by one advisory key.
				// Use the exclusive hierarchy gate so both shared and exclusive
				// range locks reliably block overlapping writes.
				merge(tableKey, dataLockExclusive, 0)
			}
			continue
		}
		if accumulationRegister != nil {
			tableKey := objectLockKey(TypeKind("accumulation-register"), metadataID, metadataID)
			if len(accumulationDimensions) == len(accumulationRegister.Dimensions) {
				merge(tableKey, dataLockShared, 0)
				merge(accumulationDimensionLockKey(*accumulationRegister, accumulationDimensions), mode, 1)
			} else {
				merge(tableKey, dataLockExclusive, 0)
			}
			continue
		}
		key := objectLockKey(kind, metadataID, objectID)
		if objectID.IsZero() {
			return fmt.Errorf("data lock value is not set")
		}
		merge(key, mode, 1)
	}
	ordered := make([]resolvedDataLock, 0, len(resolved))
	for _, item := range resolved {
		ordered = append(ordered, item)
	}
	sort.Slice(ordered, func(left, right int) bool {
		if ordered[left].tier != ordered[right].tier {
			return ordered[left].tier < ordered[right].tier
		}
		return ordered[left].key < ordered[right].key
	})
	for _, item := range ordered {
		function := "pg_advisory_xact_lock"
		if item.mode == dataLockShared {
			function += "_shared"
		}
		if _, err := scope.tx.Exec(ctx, "SELECT "+function+"($1)", item.key); err != nil {
			scope.recordFailure(err)
			return classifyTransactionError(fmt.Errorf("acquire data lock: %w", err))
		}
	}
	return nil
}

func (runtime *Runtime) getDataLockProperty(value bytecode.RuntimeObject, name string) (bytecode.Value, bool, error) {
	element, ok := value.(*dataLockElement)
	if !ok {
		return bytecode.Undefined(), false, nil
	}
	if element.runtime != runtime {
		return bytecode.Undefined(), true, fmt.Errorf("data lock element belongs to another metadata runtime")
	}
	element.mu.RLock()
	defer element.mu.RUnlock()
	switch {
	case propertyName(name, "Режим", "Mode"):
		return bytecode.String(element.mode), true, nil
	case propertyName(name, "ПространствоБлокировки", "LockSpace"):
		return bytecode.String(element.space), true, nil
	default:
		return bytecode.Undefined(), true, fmt.Errorf("%s has no property %s", element.RuntimeTypeName(), name)
	}
}

func (runtime *Runtime) setDataLockProperty(value bytecode.RuntimeObject, name string, assigned bytecode.Value) (bool, error) {
	element, ok := value.(*dataLockElement)
	if !ok {
		if lock, isLock := value.(*dataLock); isLock {
			return true, fmt.Errorf("%s property %s is not writable", lock.RuntimeTypeName(), name)
		}
		return false, nil
	}
	if element.runtime != runtime {
		return true, fmt.Errorf("data lock element belongs to another metadata runtime")
	}
	if !propertyName(name, "Режим", "Mode") {
		return true, fmt.Errorf("%s property %s is not writable", element.RuntimeTypeName(), name)
	}
	mode, ok := assigned.AsString()
	if !ok || mode != dataLockExclusive && mode != dataLockShared {
		return true, fmt.Errorf("data lock mode must be DataLockMode.Exclusive or DataLockMode.Shared")
	}
	element.mu.Lock()
	element.mode = mode
	element.mu.Unlock()
	return true, nil
}

func (runtime *Runtime) callDataLockMethod(ctx context.Context, value bytecode.RuntimeObject, name string, arguments []bytecode.Value) (bytecode.Value, bool, error) {
	switch object := value.(type) {
	case *dataLock:
		if object.runtime != runtime {
			return bytecode.Undefined(), true, fmt.Errorf("data lock belongs to another metadata runtime")
		}
		switch {
		case propertyName(name, "Добавить", "Add"):
			if len(arguments) != 1 {
				return bytecode.Undefined(), true, fmt.Errorf("%s expects one lock space", name)
			}
			space, ok := arguments[0].AsString()
			if !ok {
				return bytecode.Undefined(), true, fmt.Errorf("%s expects one lock space string", name)
			}
			element, err := runtime.newDataLockElement(space)
			if err != nil {
				return bytecode.Undefined(), true, err
			}
			object.mu.Lock()
			object.entries = append(object.entries, element)
			object.mu.Unlock()
			result, err := bytecode.Object(element)
			return result, true, err
		case propertyName(name, "Заблокировать", "Lock"):
			if len(arguments) != 0 {
				return bytecode.Undefined(), true, fmt.Errorf("%s expects no arguments", name)
			}
			return bytecode.Undefined(), true, runtime.acquireDataLocks(ctx, object)
		default:
			return bytecode.Undefined(), true, fmt.Errorf("%s has no method %s", object.RuntimeTypeName(), name)
		}
	case *dataLockElement:
		if object.runtime != runtime {
			return bytecode.Undefined(), true, fmt.Errorf("data lock element belongs to another metadata runtime")
		}
		if !propertyName(name, "УстановитьЗначение", "SetValue") {
			return bytecode.Undefined(), true, fmt.Errorf("%s has no method %s", object.RuntimeTypeName(), name)
		}
		if len(arguments) != 2 {
			return bytecode.Undefined(), true, fmt.Errorf("%s expects a field and value", name)
		}
		field, ok := arguments[0].AsString()
		if !ok {
			return bytecode.Undefined(), true, fmt.Errorf("%s field must be a string", name)
		}
		return bytecode.Undefined(), true, runtime.setDataLockValue(object, field, arguments[1])
	default:
		return bytecode.Undefined(), false, nil
	}
}
