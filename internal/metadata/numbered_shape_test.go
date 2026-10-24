package metadata

import (
	"reflect"
	"testing"
)

// TestNumberedShapeCarriesEveryField catches a field of the shape a numbered
// object repeats that is not read from the description of one of the three
// kinds - checked by nothing then on that kind - and a field the shape gains
// with no line here. Which field of the shape comes from which field of the
// description is written out below, not read from numberedShapeOf.
func TestNumberedShapeCarriesEveryField(t *testing.T) {
	fields := map[string]string{
		"number":             "Number",
		"attributes":         "Attributes",
		"tableParts":         "TableParts",
		"forms":              "Forms",
		"standardAttributes": "StandardAttributes",
		"presentation":       "Presentations",
		"input":              "ObjectInput",
		"basedOn":            "BasedOn",
		"dataLock":           "DataLock",
		"dataLockFields":     "DataLockFields",
		"fullTextSearch":     "FullTextSearch",
		"dataHistory":        "DataHistorySettings",
		"additionalIndexes":  "AdditionalIndexes",
	}
	// Set by the decoder of a kind, not read from the shared fields.
	own := map[string]bool{"reservedName": true, "kind": true, "numberFromElsewhere": true, "extraFields": true}
	shapeType := reflect.TypeOf(numberedObjectShape{})
	for index := range shapeType.NumField() {
		name := shapeType.Field(index).Name
		if _, ok := fields[name]; !ok && !own[name] {
			t.Errorf("the shape has %s, which this test does not know: say where it is read from", name)
		}
	}
	for _, numbered := range []struct {
		kind        Kind
		description any
	}{
		{DocumentKind, &DocumentDefinition{}},
		{BusinessProcessKind, &BusinessProcessDefinition{}},
		{TaskKind, &TaskDefinition{}},
	} {
		description := reflect.ValueOf(numbered.description).Elem()
		for _, field := range fields {
			fillSample(description.FieldByName(field))
		}
		read := numberedShapeOf(description.Interface(), numbered.kind, func(string) bool { return false })
		shape := reflect.ValueOf(&read).Elem()
		for shapeField, field := range fields {
			want := description.FieldByName(field).Interface()
			if got := shape.FieldByName(shapeField); !reflect.DeepEqual(reflect.NewAt(got.Type(), got.Addr().UnsafePointer()).Elem().Interface(), want) {
				t.Errorf("%s: shape.%s does not carry %s", numbered.kind, shapeField, field)
			}
		}
		if shape.FieldByName("kind").String() != string(numbered.kind) {
			t.Errorf("%s: shape.kind = %q", numbered.kind, shape.FieldByName("kind").String())
		}
	}
}

// fillSample sets a value no zero value equals: a non-empty string, a one, an
// element in a list, every exported field of a structure.
func fillSample(value reflect.Value) {
	switch value.Kind() {
	case reflect.String:
		value.SetString("sample")
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		value.SetInt(1)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		value.SetUint(1)
	case reflect.Bool:
		value.SetBool(true)
	case reflect.Array:
		fillSample(value.Index(0))
	case reflect.Slice:
		list := reflect.MakeSlice(value.Type(), 1, 1)
		fillSample(list.Index(0))
		value.Set(list)
	case reflect.Map:
		entries := reflect.MakeMap(value.Type())
		key, item := reflect.New(value.Type().Key()).Elem(), reflect.New(value.Type().Elem()).Elem()
		fillSample(key)
		fillSample(item)
		entries.SetMapIndex(key, item)
		value.Set(entries)
	case reflect.Pointer:
		item := reflect.New(value.Type().Elem())
		fillSample(item.Elem())
		value.Set(item)
	case reflect.Struct:
		for index := range value.NumField() {
			if value.Type().Field(index).IsExported() {
				fillSample(value.Field(index))
			}
		}
	}
}
