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
	mu         sync.RWMutex
	runtime    *Runtime
	kind       TypeKind
	metadataID uuid.UUID
	objectID   uuid.UUID
	space      string
	mode       string
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
	return size, size <= limit
}

// ConstructRuntimeObject creates server-only platform objects used by BSL.
func (runtime *Runtime) ConstructRuntimeObject(_ context.Context, name string, arguments []bytecode.Value) (bytecode.Value, bool, error) {
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
	default:
		return nil, fmt.Errorf("unsupported data lock space %q", space)
	}
}

func (runtime *Runtime) setDataLockValue(element *dataLockElement, field string, value bytecode.Value) error {
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

type resolvedDataLock struct {
	key  int64
	mode string
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
	resolved := make(map[int64]string, len(entries))
	for _, entry := range entries {
		entry.mu.RLock()
		key, mode, objectID := objectLockKey(entry.kind, entry.metadataID, entry.objectID), entry.mode, entry.objectID
		entry.mu.RUnlock()
		if objectID.IsZero() {
			return fmt.Errorf("data lock value is not set")
		}
		if current := resolved[key]; current == dataLockExclusive || current == mode {
			continue
		}
		resolved[key] = mode
	}
	ordered := make([]resolvedDataLock, 0, len(resolved))
	for key, mode := range resolved {
		ordered = append(ordered, resolvedDataLock{key: key, mode: mode})
	}
	sort.Slice(ordered, func(left, right int) bool { return ordered[left].key < ordered[right].key })
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
