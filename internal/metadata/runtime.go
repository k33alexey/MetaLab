package metadata

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/k33alexey/MetaLab/internal/bsl/bytecode"
	"github.com/k33alexey/MetaLab/internal/uuid"
)

// Runtime implements the BSL metadata boundary backed by PostgreSQL constants.
type Runtime struct {
	repository         *ConstantRepository
	catalogRepository  *CatalogRepository
	documentRepository *DocumentRepository
	catalog            *Catalog
	actor              *uuid.UUID
	eventsMu           sync.RWMutex
	events             map[uuid.UUID]CatalogEventHandler
	documentEvents     map[uuid.UUID]DocumentEventHandler
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
	return &Runtime{
		repository: repository, catalog: catalog, actor: actor,
		events: make(map[uuid.UUID]CatalogEventHandler), documentEvents: make(map[uuid.UUID]DocumentEventHandler),
	}, nil
}

// NewRuntimeWithCatalogs creates a metadata runtime with constants and catalog objects.
func NewRuntimeWithCatalogs(constants *ConstantRepository, catalogs *CatalogRepository, catalog *Catalog, actor *uuid.UUID) (*Runtime, error) {
	return NewRuntimeWithObjects(constants, catalogs, nil, catalog, actor)
}

// NewRuntimeWithObjects creates a metadata runtime with all currently supported application objects.
func NewRuntimeWithObjects(constants *ConstantRepository, catalogs *CatalogRepository, documents *DocumentRepository, catalog *Catalog, actor *uuid.UUID) (*Runtime, error) {
	if catalog == nil || constants == nil && catalogs == nil && documents == nil {
		return nil, fmt.Errorf("metadata runtime requires a catalog and at least one repository")
	}
	if constants != nil && constants.catalog != catalog || catalogs != nil && catalogs.catalog != catalog || documents != nil && documents.catalog != catalog {
		return nil, fmt.Errorf("metadata runtime catalog does not match its repositories")
	}
	runtime := &Runtime{
		repository: constants, catalogRepository: catalogs, documentRepository: documents, catalog: catalog,
		events: make(map[uuid.UUID]CatalogEventHandler), documentEvents: make(map[uuid.UUID]DocumentEventHandler),
	}
	if actor != nil {
		if actor.IsZero() {
			return nil, fmt.Errorf("metadata runtime actor UUID must not be zero")
		}
		copy := *actor
		runtime.actor = &copy
	}
	return runtime, nil
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
			if item.Kind == EnumerationType {
				if normalized, valid, _ := runtime.catalog.normalizeAs(Value{Kind: EnumerationType, Data: text}, item); valid {
					return normalized, nil
				}
			}
		}
		for _, item := range types {
			if item.Kind == UUIDType {
				if normalized, valid, _ := runtime.catalog.normalizeAs(Value{Kind: UUIDType, Data: text}, item); valid {
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
	case StringType, UUIDType, EnumerationType:
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
