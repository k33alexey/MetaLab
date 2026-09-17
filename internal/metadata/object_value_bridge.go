package metadata

import (
	"context"
	"fmt"

	"github.com/k33alexey/MetaLab/internal/bsl/bytecode"
)

// ValueFromBSL converts a BSL value into ML's typed application Value using
// the exact rules SetObjectProperty already applies to an assigned attribute
// value (including catalog/document reference validation) - exposed so ML
// App write routes can decode request JSON into bytecode.Value without
// duplicating that type-matching logic.
func (runtime *Runtime) ValueFromBSL(types []Type, value bytecode.Value, owner string) (Value, error) {
	return runtime.applicationValueFromBSL(types, value, owner)
}

// ValueToBSL is the inverse of ValueFromBSL, matching what GetObjectProperty
// already returns for a stored attribute value.
func (runtime *Runtime) ValueToBSL(types []Type, value Value) (bytecode.Value, error) {
	return runtime.applicationValueToBSL(types, value)
}

// StandardValueFromBSL converts a scalar BSL value returned for a built-in
// object property (Код/Наименование/Номер/Дата/Проведён/ПометкаУдаления,
// none of which is declared through an attribute's []Type) into its typed
// application Value, inferring the Kind from the BSL value itself.
func StandardValueFromBSL(value bytecode.Value) Value {
	switch value.Kind() {
	case bytecode.StringKind:
		text, _ := value.AsString()
		return Value{Kind: StringType, Data: text}
	case bytecode.NumberKind:
		text, _ := value.NumberText()
		return Value{Kind: NumberType, Data: text}
	case bytecode.BooleanKind:
		boolean, _ := value.AsBoolean()
		if boolean {
			return Value{Kind: BooleanType, Data: "true"}
		}
		return Value{Kind: BooleanType, Data: "false"}
	case bytecode.DateKind:
		date, _ := value.AsDate()
		return Value{Kind: DateType, Data: date.Format("2006-01-02T15:04:05.999999999Z07:00")}
	default:
		return Value{}
	}
}

// StandardValueToBSL is the inverse of StandardValueFromBSL, for assigning a
// built-in object property from a typed application Value.
func StandardValueToBSL(value Value) (bytecode.Value, error) {
	return valueToBSL(value)
}

// SetObjectTableRows replaces one table part's current rows on a catalog or
// document object under construction. SetObjectProperty keeps table parts
// read-only as a whole, matching BSL, which only ever mutates them through
// the table part's own collection (one Добавить() at a time) - this is the
// Go-side equivalent HTTP write routes use to populate table parts from
// structured request data instead of running a BSL script cell by cell.
func (runtime *Runtime) SetObjectTableRows(ctx context.Context, value bytecode.RuntimeObject, tablePartName string, rows []ObjectRow) error {
	switch object := value.(type) {
	case *catalogObject:
		if object.runtime != runtime {
			return fmt.Errorf("catalog object belongs to another metadata runtime")
		}
		part, ok := findCatalogTablePart(object.definition.TableParts, tablePartName)
		if !ok {
			return fmt.Errorf("%s has no table part %s", object.RuntimeTypeName(), tablePartName)
		}
		operation := PermissionUpdate
		if object.record.Version == 0 {
			operation = PermissionCreate
		}
		if err := requireFields(ctx, object.definition.ID, operation, part.ID.String()); err != nil {
			return err
		}
		table, err := runtime.catalogTableToBSL(part, rows)
		if err != nil {
			return err
		}
		object.mu.Lock()
		object.tables[part.ID] = table
		object.mu.Unlock()
		return nil
	case *documentObject:
		if object.runtime != runtime {
			return fmt.Errorf("document object belongs to another metadata runtime")
		}
		part, ok := findCatalogTablePart(object.definition.TableParts, tablePartName)
		if !ok {
			return fmt.Errorf("%s has no table part %s", object.RuntimeTypeName(), tablePartName)
		}
		operation := PermissionUpdate
		if object.record.Version == 0 {
			operation = PermissionCreate
		}
		if err := requireFields(ctx, object.definition.ID, operation, part.ID.String()); err != nil {
			return err
		}
		table, err := runtime.catalogTableToBSL(part, rows)
		if err != nil {
			return err
		}
		object.mu.Lock()
		object.tables[part.ID] = table
		object.mu.Unlock()
		return nil
	default:
		return fmt.Errorf("%s has no table parts", value.RuntimeTypeName())
	}
}
