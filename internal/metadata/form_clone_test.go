package metadata

import (
	"fmt"
	"reflect"
	"testing"
)

// fillEverything sets every field reachable from the value to something that
// is not its zero: a pointer to a filled value, a list and a map of one
// filled entry. Every text, number and identifier is different, so a copy
// that takes one field for another differs from the original. The elements
// of a form hold elements, so a list is filled only a few levels deep.
func fillEverything(value reflect.Value, depth int, counter *int) {
	*counter++
	switch value.Kind() {
	case reflect.String:
		value.SetString(fmt.Sprint("x", *counter))
	case reflect.Bool:
		value.SetBool(true)
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		value.SetInt(int64(*counter % 100))
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		value.SetUint(uint64(*counter % 250))
	case reflect.Float32, reflect.Float64:
		value.SetFloat(float64(*counter))
	case reflect.Array:
		for index := range value.Len() {
			fillEverything(value.Index(index), depth, counter)
		}
	case reflect.Pointer:
		// A table holds its additions, which are elements, behind pointers;
		// they are filled as deep as the lists are.
		if value.Type().Elem() == reflect.TypeFor[ManagedFormElement]() {
			if depth > 3 {
				return
			}
			depth++
		}
		pointed := reflect.New(value.Type().Elem())
		fillEverything(pointed.Elem(), depth, counter)
		value.Set(pointed)
	case reflect.Slice:
		// The deepest lists are left empty rather than absent: the children
		// of an element are handed out as an empty list, never as none, as
		// they always were.
		if depth > 3 {
			value.Set(reflect.MakeSlice(value.Type(), 0, 0))
			return
		}
		list := reflect.MakeSlice(value.Type(), 1, 1)
		fillEverything(list.Index(0), depth+1, counter)
		value.Set(list)
	case reflect.Map:
		filled := reflect.MakeMap(value.Type())
		key, entry := reflect.New(value.Type().Key()).Elem(), reflect.New(value.Type().Elem()).Elem()
		fillEverything(key, depth, counter)
		fillEverything(entry, depth, counter)
		filled.SetMapIndex(key, entry)
		value.Set(filled)
	case reflect.Struct:
		for index := range value.NumField() {
			if value.Type().Field(index).IsExported() {
				fillEverything(value.Field(index), depth, counter)
			}
		}
	}
}

// sharedMemory reports the places where the copy holds the same memory as the
// original: a pointer to the same value, a list over the same array, the
// same map.
func sharedMemory(original, copied reflect.Value, path string) []string {
	var shared []string
	switch original.Kind() {
	case reflect.Pointer:
		if original.IsNil() || copied.IsNil() {
			return nil
		}
		if original.Pointer() == copied.Pointer() {
			shared = append(shared, path)
		}
		shared = append(shared, sharedMemory(original.Elem(), copied.Elem(), path)...)
	case reflect.Slice:
		if original.Len() > 0 && copied.Len() > 0 && original.Pointer() == copied.Pointer() {
			shared = append(shared, path)
		}
		for index := range min(original.Len(), copied.Len()) {
			shared = append(shared, sharedMemory(original.Index(index), copied.Index(index), path+"[]")...)
		}
	case reflect.Map:
		if !original.IsNil() && !copied.IsNil() && original.Pointer() == copied.Pointer() {
			shared = append(shared, path)
		}
	case reflect.Array:
		for index := range original.Len() {
			shared = append(shared, sharedMemory(original.Index(index), copied.Index(index), path)...)
		}
	case reflect.Struct:
		for index := range original.NumField() {
			if original.Type().Field(index).IsExported() {
				shared = append(shared, sharedMemory(original.Field(index), copied.Field(index), path+"."+original.Type().Field(index).Name)...)
			}
		}
	}
	return shared
}

// A form handed out by the runtime snapshot is a copy of the whole form:
// whatever its caller changes in it reaches neither the snapshot nor any
// other opening of the form. Every field of the form is filled by
// reflection, so a field the model gains later is checked without the test
// being told about it.
//
// Defect caught: the tooltip, the roles an element is shown to, its fonts,
// colours, border and pictures, the attributes of the form with their types,
// rights and columns, and its purposes shared with the snapshot (all of them
// were, until 2.219), so that one opening changing them changes them for
// every later one; a field added to the form and forgotten in the copy; the
// copy losing a value or taking one field for another.
func TestARuntimeFormSharesNothingWithTheSnapshot(t *testing.T) {
	t.Parallel()
	var form ManagedForm
	counter := 0
	fillEverything(reflect.ValueOf(&form).Elem(), 0, &counter)
	if len(form.Items) == 0 || len(form.Items[0].Children) == 0 || form.Items[0].FieldLook.Font == nil || form.Attributes[0].DynamicList == nil ||
		form.Items[0].SearchStringAddition == nil || form.Items[0].SearchStringAddition.Font == nil {
		t.Fatalf("the form is not filled: %+v", form)
	}
	copied := cloneRuntimeForm(form)
	if !reflect.DeepEqual(copied, form) {
		t.Fatalf("the copy differs from the form:\n%+v\n%+v", copied, form)
	}
	if shared := sharedMemory(reflect.ValueOf(form), reflect.ValueOf(copied), "form"); len(shared) > 0 {
		t.Fatalf("the copy shares with the snapshot: %v", shared)
	}
}
