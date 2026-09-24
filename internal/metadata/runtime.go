package metadata

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/k33alexey/MetaLab/internal/bsl/bytecode"
	"github.com/k33alexey/MetaLab/internal/uuid"
)

// Runtime implements the BSL metadata boundary backed by PostgreSQL constants.
type Runtime struct {
	repository                     *ConstantRepository
	catalogRepository              *CatalogRepository
	documentRepository             *DocumentRepository
	informationRegisterRepository  *InformationRegisterRepository
	accumulationRegisterRepository *AccumulationRegisterRepository
	catalog                        *Catalog
	actor                          *uuid.UUID
	eventsMu                       sync.RWMutex
	events                         map[uuid.UUID]CatalogEventHandler
	documentEvents                 map[uuid.UUID]DocumentEventHandler
	informationRegisterEvents      map[uuid.UUID]InformationRegisterEventHandler
	accumulationRegisterEvents     map[uuid.UUID]AccumulationRegisterEventHandler
	dataLockWait                   atomic.Int64
	sessionParametersMu            sync.RWMutex
	sessionParameters              map[string]bytecode.Value
	sessionParameterLists          map[string][]Value
	sessionModule                  *SessionBSLEvents
	sessionModuleRunning           bool
}

func NewRuntime(repository *ConstantRepository, catalog *Catalog, actor *uuid.UUID) (*Runtime, error) {
	if repository == nil || catalog == nil {
		return nil, fmt.Errorf("metadata runtime requires a catalog and constant repository")
	}
	if repository.catalog != catalog {
		return nil, fmt.Errorf("metadata runtime catalog does not match its constant repository")
	}
	if actor != nil {
		if actor.IsZero() {
			return nil, fmt.Errorf("metadata runtime actor UUID must not be zero")
		}
		copy := *actor
		actor = &copy
	}
	runtime := &Runtime{
		repository: repository, catalog: catalog, actor: actor,
		events: make(map[uuid.UUID]CatalogEventHandler), documentEvents: make(map[uuid.UUID]DocumentEventHandler),
		informationRegisterEvents:  make(map[uuid.UUID]InformationRegisterEventHandler),
		accumulationRegisterEvents: make(map[uuid.UUID]AccumulationRegisterEventHandler),
		sessionParameters:          make(map[string]bytecode.Value),
		sessionParameterLists:      make(map[string][]Value),
	}
	if _, err := runtime.databasePool(); err != nil {
		return nil, err
	}
	return runtime, nil
}

// NewRuntimeWithCatalogs creates a metadata runtime with constants and catalog objects.
func NewRuntimeWithCatalogs(constants *ConstantRepository, catalogs *CatalogRepository, catalog *Catalog, actor *uuid.UUID) (*Runtime, error) {
	return NewRuntimeWithObjects(constants, catalogs, nil, catalog, actor)
}

// NewRuntimeWithObjects creates a metadata runtime with all currently supported application objects.
func NewRuntimeWithObjects(constants *ConstantRepository, catalogs *CatalogRepository, documents *DocumentRepository, catalog *Catalog, actor *uuid.UUID) (*Runtime, error) {
	return NewRuntimeWithAllObjects(constants, catalogs, documents, nil, catalog, actor)
}

// NewRuntimeWithAllObjects creates a metadata runtime including information registers.
func NewRuntimeWithAllObjects(constants *ConstantRepository, catalogs *CatalogRepository, documents *DocumentRepository, informationRegisters *InformationRegisterRepository, catalog *Catalog, actor *uuid.UUID) (*Runtime, error) {
	return NewRuntimeWithAllRegisters(constants, catalogs, documents, informationRegisters, nil, catalog, actor)
}

// NewRuntimeWithAllRegisters creates a runtime with every currently supported repository.
func NewRuntimeWithAllRegisters(constants *ConstantRepository, catalogs *CatalogRepository, documents *DocumentRepository, informationRegisters *InformationRegisterRepository, accumulationRegisters *AccumulationRegisterRepository, catalog *Catalog, actor *uuid.UUID) (*Runtime, error) {
	if catalog == nil || constants == nil && catalogs == nil && documents == nil && informationRegisters == nil && accumulationRegisters == nil {
		return nil, fmt.Errorf("metadata runtime requires a catalog and at least one repository")
	}
	if constants != nil && constants.catalog != catalog || catalogs != nil && catalogs.catalog != catalog || documents != nil && documents.catalog != catalog ||
		informationRegisters != nil && informationRegisters.catalog != catalog || accumulationRegisters != nil && accumulationRegisters.catalog != catalog {
		return nil, fmt.Errorf("metadata runtime catalog does not match its repositories")
	}
	runtime := &Runtime{
		repository: constants, catalogRepository: catalogs, documentRepository: documents, informationRegisterRepository: informationRegisters, accumulationRegisterRepository: accumulationRegisters, catalog: catalog,
		events: make(map[uuid.UUID]CatalogEventHandler), documentEvents: make(map[uuid.UUID]DocumentEventHandler),
		informationRegisterEvents:  make(map[uuid.UUID]InformationRegisterEventHandler),
		accumulationRegisterEvents: make(map[uuid.UUID]AccumulationRegisterEventHandler),
		sessionParameters:          make(map[string]bytecode.Value),
		sessionParameterLists:      make(map[string][]Value),
	}
	if actor != nil {
		if actor.IsZero() {
			return nil, fmt.Errorf("metadata runtime actor UUID must not be zero")
		}
		copy := *actor
		runtime.actor = &copy
	}
	if _, err := runtime.databasePool(); err != nil {
		return nil, err
	}
	return runtime, nil
}

func (runtime *Runtime) SetAccumulationRegisterEventHandler(name string, handler AccumulationRegisterEventHandler) error {
	definition, ok := runtime.catalog.AccumulationRegisterDefinition(name)
	if !ok {
		return fmt.Errorf("unknown accumulation register %q", name)
	}
	runtime.eventsMu.Lock()
	defer runtime.eventsMu.Unlock()
	if handler == nil {
		delete(runtime.accumulationRegisterEvents, definition.ID)
	} else {
		runtime.accumulationRegisterEvents[definition.ID] = handler
	}
	return nil
}

func (runtime *Runtime) accumulationRegisterEventHandler(id uuid.UUID) AccumulationRegisterEventHandler {
	runtime.eventsMu.RLock()
	defer runtime.eventsMu.RUnlock()
	return runtime.accumulationRegisterEvents[id]
}

func (runtime *Runtime) SetInformationRegisterEventHandler(name string, handler InformationRegisterEventHandler) error {
	definition, ok := runtime.catalog.InformationRegisterDefinition(name)
	if !ok {
		return fmt.Errorf("unknown information register %q", name)
	}
	runtime.eventsMu.Lock()
	defer runtime.eventsMu.Unlock()
	if handler == nil {
		delete(runtime.informationRegisterEvents, definition.ID)
	} else {
		runtime.informationRegisterEvents[definition.ID] = handler
	}
	return nil
}

func (runtime *Runtime) informationRegisterEventHandler(id uuid.UUID) InformationRegisterEventHandler {
	runtime.eventsMu.RLock()
	defer runtime.eventsMu.RUnlock()
	return runtime.informationRegisterEvents[id]
}

func (runtime *Runtime) SetDocumentEventHandler(name string, handler DocumentEventHandler) error {
	definition, ok := runtime.catalog.DocumentDefinition(name)
	if !ok {
		return fmt.Errorf("unknown document %q", name)
	}
	runtime.eventsMu.Lock()
	defer runtime.eventsMu.Unlock()
	if handler == nil {
		delete(runtime.documentEvents, definition.ID)
	} else {
		runtime.documentEvents[definition.ID] = handler
	}
	return nil
}

func (runtime *Runtime) documentEventHandler(id uuid.UUID) DocumentEventHandler {
	runtime.eventsMu.RLock()
	defer runtime.eventsMu.RUnlock()
	return runtime.documentEvents[id]
}

func (runtime *Runtime) SetCatalogEventHandler(name string, handler CatalogEventHandler) error {
	definition, ok := runtime.catalog.CatalogDefinition(name)
	if !ok {
		return fmt.Errorf("unknown catalog %q", name)
	}
	runtime.eventsMu.Lock()
	defer runtime.eventsMu.Unlock()
	if handler == nil {
		delete(runtime.events, definition.ID)
	} else {
		runtime.events[definition.ID] = handler
	}
	return nil
}

func (runtime *Runtime) catalogEventHandler(id uuid.UUID) CatalogEventHandler {
	runtime.eventsMu.RLock()
	defer runtime.eventsMu.RUnlock()
	return runtime.events[id]
}

func (runtime *Runtime) GetConstant(ctx context.Context, name string) (bytecode.Value, error) {
	if runtime.repository == nil {
		return bytecode.Undefined(), fmt.Errorf("constant repository is not configured")
	}
	stored, err := runtime.repository.Get(ctx, name)
	if errors.Is(err, ErrConstantValueNotFound) {
		constant, ok := runtime.catalog.Constant(name)
		if !ok {
			return bytecode.Undefined(), fmt.Errorf("unknown constant %q", name)
		}
		if constant.Default == nil {
			return bytecode.Undefined(), nil
		}
		return runtime.applicationValueToBSL(constant.Types, *constant.Default)
	}
	if err != nil {
		return bytecode.Undefined(), err
	}
	constant, _ := runtime.catalog.Constant(name)
	return runtime.applicationValueToBSL(constant.Types, stored.Value)
}

func (runtime *Runtime) SetConstant(ctx context.Context, name string, value bytecode.Value) error {
	if runtime.repository == nil {
		return fmt.Errorf("constant repository is not configured")
	}
	constant, ok := runtime.catalog.Constant(name)
	if !ok {
		return fmt.Errorf("unknown constant %q", name)
	}
	converted, err := runtime.valueFromBSL(constant, value)
	if err != nil {
		return err
	}
	_, err = runtime.repository.Set(ctx, name, converted, runtime.actor)
	return err
}

// GetSessionParameter reads a session parameter from server-process memory.
// Session parameters are never persisted to PostgreSQL: an unset parameter
// resolves to its declared default for the lifetime of this Runtime.
//
// Reading one the solution has not set yet is what triggers the session module:
// the handler is asked for this parameter by name, and the value is re-read
// afterwards. The handler is allowed to set a whole group at once, so the
// second read - not the handler's answer - decides what this call returns.
func (runtime *Runtime) GetSessionParameter(ctx context.Context, name string) (bytecode.Value, error) {
	parameter, ok := runtime.catalog.SessionParameter(name)
	if !ok {
		return bytecode.Undefined(), fmt.Errorf("unknown session parameter %q", name)
	}
	if stored, set := runtime.storedSessionParameter(name); set {
		return stored, nil
	}
	if _, err := runtime.runSessionModule(ctx, []string{parameter.Name}); err != nil {
		return bytecode.Undefined(), err
	}
	if stored, set := runtime.storedSessionParameter(name); set {
		return stored, nil
	}
	if parameter.Default == nil {
		return bytecode.Undefined(), nil
	}
	return runtime.applicationValueToBSL(parameter.Types, *parameter.Default)
}

func (runtime *Runtime) storedSessionParameter(name string) (bytecode.Value, bool) {
	runtime.sessionParametersMu.RLock()
	defer runtime.sessionParametersMu.RUnlock()
	stored, set := runtime.sessionParameters[strings.ToLower(name)]
	return stored, set
}

// MaxSessionParameterValues bounds one list-valued session parameter. The list
// becomes an IN(...) list inside every restricted read, so it is bounded for the
// same reason a policy's literal list is.
const MaxSessionParameterValues = 1000

// SetSessionParameter stores a session parameter in server-process memory
// for the lifetime of this Runtime.
//
// The value may be a LIST, not only a single value: "the warehouses this user
// may see" is the shape real restrictions are written against, and the
// declarative operators В/НЕ В exist precisely to consume it.
func (runtime *Runtime) SetSessionParameter(_ context.Context, name string, value bytecode.Value) error {
	parameter, ok := runtime.catalog.SessionParameter(name)
	if !ok {
		return fmt.Errorf("unknown session parameter %q", name)
	}
	converted, normalized, err := runtime.sessionParameterValues(parameter, value)
	if err != nil {
		return err
	}
	runtime.sessionParametersMu.Lock()
	// Both views are created on first write: a Runtime built as a literal (as
	// several tests do) has neither, and a panic there would say nothing about
	// the real problem.
	if runtime.sessionParameters == nil {
		runtime.sessionParameters = make(map[string]bytecode.Value)
	}
	if runtime.sessionParameterLists == nil {
		runtime.sessionParameterLists = make(map[string][]Value)
	}
	runtime.sessionParameters[strings.ToLower(name)] = normalized
	runtime.sessionParameterLists[strings.ToLower(name)] = converted
	runtime.sessionParametersMu.Unlock()
	return nil
}

// sessionParameterValues normalizes what BSL assigned, in both representations
// this runtime keeps: the application values a restriction compares against, and
// the BSL value the same parameter reads back as. They are produced here
// together so the two views cannot drift apart.
func (runtime *Runtime) sessionParameterValues(parameter SessionParameter, value bytecode.Value) ([]Value, bytecode.Value, error) {
	if length, ok := value.ArrayLength(); ok {
		if length > MaxSessionParameterValues {
			return nil, bytecode.Undefined(), fmt.Errorf("session parameter %s must not hold more than %d values", parameter.Name, MaxSessionParameterValues)
		}
		values := make([]Value, 0, length)
		elements := make([]bytecode.Value, 0, length)
		for index := 0; index < length; index++ {
			element, _ := value.ArrayElement(index)
			converted, err := runtime.valueFromBSLForSessionParameter(parameter, element)
			if err != nil {
				return nil, bytecode.Undefined(), err
			}
			normalized, err := valueToBSL(converted)
			if err != nil {
				return nil, bytecode.Undefined(), err
			}
			values = append(values, converted)
			elements = append(elements, normalized)
		}
		return values, bytecode.Array(elements...), nil
	}
	converted, err := runtime.valueFromBSLForSessionParameter(parameter, value)
	if err != nil {
		return nil, bytecode.Undefined(), err
	}
	normalized, err := valueToBSL(converted)
	if err != nil {
		return nil, bytecode.Undefined(), err
	}
	return []Value{converted}, normalized, nil
}

// SessionParameterValues resolves one session parameter into the application
// values a row restriction compares against, running the session module if the
// solution has not set it yet. A parameter with neither a value nor a default is
// reported as absent: a restriction that cannot be evaluated must refuse the
// read, never widen it.
func (runtime *Runtime) SessionParameterValues(ctx context.Context, name string) ([]Value, bool, error) {
	parameter, ok := runtime.catalog.SessionParameter(name)
	if !ok {
		return nil, false, fmt.Errorf("unknown session parameter %q", name)
	}
	if values, set := runtime.storedSessionParameterValues(name); set {
		return values, true, nil
	}
	if _, err := runtime.runSessionModule(ctx, []string{parameter.Name}); err != nil {
		return nil, false, err
	}
	if values, set := runtime.storedSessionParameterValues(name); set {
		return values, true, nil
	}
	if parameter.Default == nil {
		return nil, false, nil
	}
	return []Value{*parameter.Default}, true, nil
}

func (runtime *Runtime) storedSessionParameterValues(name string) ([]Value, bool) {
	runtime.sessionParametersMu.RLock()
	defer runtime.sessionParametersMu.RUnlock()
	values, set := runtime.sessionParameterLists[strings.ToLower(name)]
	return values, set
}

func (runtime *Runtime) valueFromBSLForSessionParameter(parameter SessionParameter, value bytecode.Value) (Value, error) {
	if _, ok := value.AsRuntimeObject(); ok {
		return runtime.applicationValueFromBSL(parameter.Types, value, "session parameter "+parameter.Name)
	}
	switch value.Kind() {
	case bytecode.StringKind:
		text, _ := value.AsString()
		types, _ := runtime.catalog.expandTypes(parameter.Types, nil)
		for _, item := range types {
			if item.Kind == EnumerationType && item.Reference != nil {
				if normalized, valid, _ := runtime.catalog.normalizeAs(Value{Kind: EnumerationType, Data: text, Object: *item.Reference}, item); valid {
					return normalized, nil
				}
			}
		}
		for _, item := range types {
			if item.Kind == ObjectUUIDType {
				if normalized, valid, _ := runtime.catalog.normalizeAs(Value{Kind: ObjectUUIDType, Data: text}, item); valid {
					return normalized, nil
				}
			}
		}
		return runtime.catalog.NormalizeSessionParameterValue(parameter, Value{Kind: StringType, Data: text})
	case bytecode.NumberKind:
		text, _ := value.NumberText()
		return runtime.catalog.NormalizeSessionParameterValue(parameter, Value{Kind: NumberType, Data: text})
	case bytecode.BooleanKind:
		boolean, _ := value.AsBoolean()
		if boolean {
			return runtime.catalog.NormalizeSessionParameterValue(parameter, Value{Kind: BooleanType, Data: "true"})
		}
		return runtime.catalog.NormalizeSessionParameterValue(parameter, Value{Kind: BooleanType, Data: "false"})
	case bytecode.DateKind:
		date, _ := value.AsDate()
		return runtime.catalog.NormalizeSessionParameterValue(parameter, Value{Kind: DateType, Data: date.Format("2006-01-02T15:04:05.999999999Z07:00")})
	default:
		return Value{}, fmt.Errorf("session parameter %s cannot store BSL value kind %s", parameter.Name, value.Kind())
	}
}

func (runtime *Runtime) GetEnumerationValue(_ context.Context, enumeration, value string) (bytecode.Value, error) {
	item, ok := runtime.catalog.EnumerationValue(enumeration, value)
	if !ok {
		return bytecode.Undefined(), fmt.Errorf("unknown enumeration value %s.%s", enumeration, value)
	}
	return bytecode.String(item.ID.String()), nil
}

func (runtime *Runtime) GetDefinedType(_ context.Context, name string) (bytecode.Value, error) {
	types, ok := runtime.catalog.ResolvedTypes(name)
	if !ok {
		return bytecode.Undefined(), fmt.Errorf("unknown defined type %q", name)
	}
	result := make([]bytecode.Value, 0, len(types))
	for _, item := range types {
		reference := ""
		if item.Reference != nil {
			reference = item.Reference.String()
		}
		descriptor, err := bytecode.ConstructCollection("Structure", []bytecode.Value{
			bytecode.String("Kind,Reference,Length,Precision,Scale,Тип,Ссылка,Длина,Точность,Масштаб"),
			bytecode.String(string(item.Kind)), bytecode.String(reference),
			bytecode.Number(float64(item.Length)), bytecode.Number(float64(item.Precision)), bytecode.Number(float64(item.Scale)),
			bytecode.String(string(item.Kind)), bytecode.String(reference),
			bytecode.Number(float64(item.Length)), bytecode.Number(float64(item.Precision)), bytecode.Number(float64(item.Scale)),
		})
		if err != nil {
			return bytecode.Undefined(), err
		}
		result = append(result, descriptor)
	}
	return bytecode.Array(result...), nil
}

func (runtime *Runtime) valueFromBSL(constant Constant, value bytecode.Value) (Value, error) {
	if _, ok := value.AsRuntimeObject(); ok {
		return runtime.applicationValueFromBSL(constant.Types, value, "constant "+constant.Name)
	}
	switch value.Kind() {
	case bytecode.StringKind:
		text, _ := value.AsString()
		types, _ := runtime.catalog.expandTypes(constant.Types, nil)
		for _, item := range types {
			if item.Kind == EnumerationType && item.Reference != nil {
				if normalized, valid, _ := runtime.catalog.normalizeAs(Value{Kind: EnumerationType, Data: text, Object: *item.Reference}, item); valid {
					return normalized, nil
				}
			}
		}
		for _, item := range types {
			if item.Kind == ObjectUUIDType {
				if normalized, valid, _ := runtime.catalog.normalizeAs(Value{Kind: ObjectUUIDType, Data: text}, item); valid {
					return normalized, nil
				}
			}
		}
		return runtime.catalog.NormalizeValue(constant, Value{Kind: StringType, Data: text})
	case bytecode.NumberKind:
		text, _ := value.NumberText()
		return runtime.catalog.NormalizeValue(constant, Value{Kind: NumberType, Data: text})
	case bytecode.BooleanKind:
		boolean, _ := value.AsBoolean()
		if boolean {
			return runtime.catalog.NormalizeValue(constant, Value{Kind: BooleanType, Data: "true"})
		}
		return runtime.catalog.NormalizeValue(constant, Value{Kind: BooleanType, Data: "false"})
	case bytecode.DateKind:
		date, _ := value.AsDate()
		return runtime.catalog.NormalizeValue(constant, Value{Kind: DateType, Data: date.Format("2006-01-02T15:04:05.999999999Z07:00")})
	default:
		return Value{}, fmt.Errorf("constant %s cannot store BSL value kind %s", constant.Name, value.Kind())
	}
}

func valueToBSL(value Value) (bytecode.Value, error) {
	switch value.Kind {
	case StringType, ObjectUUIDType, EnumerationType:
		return bytecode.String(value.Data), nil
	case NumberType:
		return bytecode.ParseNumber(value.Data)
	case BooleanType:
		return bytecode.Boolean(value.Data == "true"), nil
	case DateType:
		date, err := time.Parse(time.RFC3339Nano, value.Data)
		if err != nil {
			return bytecode.Undefined(), fmt.Errorf("decode constant date: %w", err)
		}
		return bytecode.Date(date)
	default:
		return bytecode.Undefined(), fmt.Errorf("unsupported constant value kind %s", value.Kind)
	}
}
