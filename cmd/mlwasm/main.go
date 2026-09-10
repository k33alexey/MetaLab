//go:build js && wasm

// Command mlwasm exposes the MetaLab client VM to a browser.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"syscall/js"
	"time"

	"github.com/k33alexey/MetaLab/internal/bsl/bytecode"
	"github.com/k33alexey/MetaLab/internal/bsl/clientvm"
	"github.com/k33alexey/MetaLab/internal/bsl/vm"
)

const (
	maxInputSize      = 64 << 20
	maxCollectionSize = 1 << 20
	maxValueNesting   = 64
)

var (
	registry  = clientvm.NewRegistry()
	callbacks []js.Func
)

func main() {
	api := js.Global().Get("Object").New()
	export(api, "load", load)
	export(api, "call", call)
	export(api, "release", release)
	export(api, "debugStart", debugStart)
	export(api, "debugState", debugState)
	export(api, "debugCommand", debugCommand)
	export(api, "debugPause", debugPause)
	export(api, "debugSetBreakpoints", debugSetBreakpoints)
	export(api, "debugEvaluate", debugEvaluate)
	export(api, "debugStop", debugStop)
	js.Global().Set("MetaLabWasm", api)
	select {}
}

func export(api js.Value, name string, function func(js.Value, []js.Value) any) {
	callback := js.FuncOf(protect(function))
	callbacks = append(callbacks, callback)
	api.Set(name, callback)
}

func protect(function func(js.Value, []js.Value) any) func(js.Value, []js.Value) (result any) {
	return func(this js.Value, arguments []js.Value) (result any) {
		defer func() {
			if recovered := recover(); recovered != nil {
				result = failure(fmt.Sprintf("client VM panic: %v", recovered))
			}
		}()
		return function(this, arguments)
	}
}

func load(_ js.Value, arguments []js.Value) any {
	if len(arguments) != 1 || !arguments[0].InstanceOf(js.Global().Get("Uint8Array")) {
		return failure("load expects one Uint8Array")
	}
	length := arguments[0].Get("byteLength").Int()
	if length < 0 || length > maxInputSize {
		return failure("bytecode input is too large")
	}
	encoded := make([]byte, length)
	if copied := js.CopyBytesToGo(encoded, arguments[0]); copied != length {
		return failure("could not copy complete bytecode input")
	}
	handle, err := registry.Load(encoded)
	if err != nil {
		return failure(err.Error())
	}
	result := success()
	result.Set("handle", handle)
	return result
}

func call(_ js.Value, arguments []js.Value) any {
	if len(arguments) != 3 {
		return failure("call expects handle, routine name and argument array")
	}
	handle, err := readHandle(arguments[0])
	if err != nil {
		return failure(err.Error())
	}
	if arguments[1].Type() != js.TypeString {
		return failure("call expects a string routine name and an argument array")
	}
	var inline [32]bytecode.Value
	values, err := readArguments(arguments[2], inline[:])
	if err != nil {
		return failure(err.Error())
	}
	value, err := registry.Call(handle, arguments[1].String(), values...)
	if err != nil {
		return executionFailure(err)
	}
	return valueResult(value)
}

func debugStart(_ js.Value, arguments []js.Value) any {
	if len(arguments) != 4 {
		return failure("debugStart expects handle, routine name, argument array and breakpoints")
	}
	handle, err := readHandle(arguments[0])
	if err != nil {
		return failure(err.Error())
	}
	if arguments[1].Type() != js.TypeString {
		return failure("debug routine name must be a string")
	}
	var inline [32]bytecode.Value
	values, err := readArguments(arguments[2], inline[:])
	if err != nil {
		return failure(err.Error())
	}
	breakpoints, err := readBreakpoints(arguments[3])
	if err != nil {
		return failure(err.Error())
	}
	snapshot, err := registry.StartDebug(context.Background(), handle, arguments[1].String(), breakpoints, values...)
	return debugSnapshotResult(snapshot, err)
}

func debugState(_ js.Value, arguments []js.Value) any {
	if len(arguments) != 1 {
		return failure("debugState expects one handle")
	}
	handle, err := readHandle(arguments[0])
	if err != nil {
		return failure(err.Error())
	}
	snapshot, err := registry.DebugState(handle)
	return debugSnapshotResult(snapshot, err)
}

func debugCommand(_ js.Value, arguments []js.Value) any {
	if len(arguments) != 2 {
		return failure("debugCommand expects handle and action")
	}
	handle, err := readHandle(arguments[0])
	if err != nil {
		return failure(err.Error())
	}
	if arguments[1].Type() != js.TypeString {
		return failure("debug action must be a string")
	}
	snapshot, err := registry.DebugCommand(handle, vm.DebugAction(arguments[1].String()))
	return debugSnapshotResult(snapshot, err)
}

func debugPause(_ js.Value, arguments []js.Value) any {
	if len(arguments) != 1 {
		return failure("debugPause expects one handle")
	}
	handle, err := readHandle(arguments[0])
	if err != nil {
		return failure(err.Error())
	}
	snapshot, err := registry.PauseDebug(handle)
	return debugSnapshotResult(snapshot, err)
}

func debugSetBreakpoints(_ js.Value, arguments []js.Value) any {
	if len(arguments) != 2 {
		return failure("debugSetBreakpoints expects handle and breakpoints")
	}
	handle, err := readHandle(arguments[0])
	if err != nil {
		return failure(err.Error())
	}
	breakpoints, err := readBreakpoints(arguments[1])
	if err != nil {
		return failure(err.Error())
	}
	snapshot, err := registry.SetDebugBreakpoints(handle, breakpoints)
	return debugSnapshotResult(snapshot, err)
}

func debugEvaluate(_ js.Value, arguments []js.Value) any {
	if len(arguments) != 3 {
		return failure("debugEvaluate expects handle, frame and expression")
	}
	handle, err := readHandle(arguments[0])
	if err != nil {
		return failure(err.Error())
	}
	if arguments[1].Type() != js.TypeNumber || arguments[2].Type() != js.TypeString {
		return failure("debugEvaluate expects an integer frame and string expression")
	}
	frame := arguments[1].Float()
	if frame < 0 || frame > math.MaxInt32 || math.Trunc(frame) != frame {
		return failure("invalid debug frame")
	}
	value, err := registry.EvaluateDebug(handle, int(frame), arguments[2].String())
	if err != nil {
		return failure(err.Error())
	}
	result := success()
	encoded, err := jsonToJS(value)
	if err != nil {
		return failure(err.Error())
	}
	result.Set("value", encoded)
	return result
}

func debugStop(_ js.Value, arguments []js.Value) any {
	if len(arguments) != 1 {
		return failure("debugStop expects one handle")
	}
	handle, err := readHandle(arguments[0])
	if err != nil {
		return failure(err.Error())
	}
	snapshot, err := registry.StopDebug(handle)
	return debugSnapshotResult(snapshot, err)
}

func debugSnapshotResult(snapshot vm.DebugSnapshot, err error) js.Value {
	if err != nil {
		return failure(err.Error())
	}
	encoded, err := jsonToJS(snapshot)
	if err != nil {
		return failure(err.Error())
	}
	result := success()
	result.Set("snapshot", encoded)
	return result
}

func jsonToJS(value any) (js.Value, error) {
	encoded, err := json.Marshal(value)
	if err != nil {
		return js.Undefined(), fmt.Errorf("encode debugger result: %w", err)
	}
	return js.Global().Get("JSON").Call("parse", string(encoded)), nil
}

func readArguments(value js.Value, scratch []bytecode.Value) ([]bytecode.Value, error) {
	if !js.Global().Get("Array").Call("isArray", value).Bool() {
		return nil, fmt.Errorf("routine arguments must be an array")
	}
	argumentCount := value.Length()
	if argumentCount > int(^uint16(0)) {
		return nil, fmt.Errorf("too many routine arguments")
	}
	values := scratch
	if argumentCount <= len(values) {
		values = values[:argumentCount]
	} else {
		values = make([]bytecode.Value, argumentCount)
	}
	for index := range values {
		converted, err := readValue(value.Index(index))
		if err != nil {
			return nil, fmt.Errorf("argument %d: %w", index, err)
		}
		values[index] = converted
	}
	return values, nil
}

func readBreakpoints(value js.Value) ([]vm.DebugBreakpoint, error) {
	if !js.Global().Get("Array").Call("isArray", value).Bool() {
		return nil, fmt.Errorf("debug breakpoints must be an array")
	}
	if value.Length() > 10_000 {
		return nil, fmt.Errorf("too many debug breakpoints")
	}
	result := make([]vm.DebugBreakpoint, value.Length())
	for index := range result {
		item := value.Index(index)
		if item.Type() != js.TypeObject || item.Get("path").Type() != js.TypeString || item.Get("line").Type() != js.TypeNumber {
			return nil, fmt.Errorf("breakpoint %d must contain path and line", index)
		}
		line := item.Get("line").Float()
		if line < 1 || line > 10_000_000 || math.Trunc(line) != line {
			return nil, fmt.Errorf("breakpoint %d has invalid line", index)
		}
		result[index] = vm.DebugBreakpoint{Filename: item.Get("path").String(), Line: int(line)}
	}
	return result, nil
}

func release(_ js.Value, arguments []js.Value) any {
	if len(arguments) != 1 {
		return failure("release expects one handle")
	}
	handle, err := readHandle(arguments[0])
	if err != nil {
		return failure(err.Error())
	}
	if !registry.Release(handle) {
		return failure(fmt.Sprintf("client VM machine %d not found", handle))
	}
	return success()
}

func readHandle(value js.Value) (uint32, error) {
	if value.Type() != js.TypeNumber {
		return 0, fmt.Errorf("machine handle must be a number")
	}
	number := value.Float()
	if number <= 0 || number > float64(^uint32(0)) || math.Trunc(number) != number {
		return 0, fmt.Errorf("invalid machine handle")
	}
	return uint32(number), nil
}

func readValue(value js.Value) (bytecode.Value, error) {
	return readValueAtDepth(value, 0)
}

func readValueAtDepth(value js.Value, depth int) (bytecode.Value, error) {
	if depth > maxValueNesting {
		return bytecode.Undefined(), fmt.Errorf("JavaScript value nesting is too deep")
	}
	switch value.Type() {
	case js.TypeUndefined:
		return bytecode.Undefined(), nil
	case js.TypeNull:
		return bytecode.Null(), nil
	case js.TypeBoolean:
		return bytecode.Boolean(value.Bool()), nil
	case js.TypeNumber:
		return bytecode.NumberFromFloat64(value.Float())
	case js.TypeString:
		return bytecode.String(value.String()), nil
	case js.TypeObject:
		if value.InstanceOf(js.Global().Get("Date")) {
			milliseconds := value.Call("getTime").Float()
			if math.IsNaN(milliseconds) || math.IsInf(milliseconds, 0) || milliseconds > math.MaxInt64 || milliseconds < math.MinInt64 {
				return bytecode.Undefined(), fmt.Errorf("invalid JavaScript date")
			}
			return bytecode.Date(time.UnixMilli(int64(milliseconds)))
		}
		if kind := value.Get("kind"); kind.Type() == js.TypeString && kind.String() == "number" {
			text := value.Get("text")
			if text.Type() != js.TypeString {
				return bytecode.Undefined(), fmt.Errorf("exact number requires decimal text")
			}
			return bytecode.ParseNumber(text.String())
		}
		if js.Global().Get("Array").Call("isArray", value).Bool() {
			length := value.Length()
			if length < 0 || length > maxCollectionSize {
				return bytecode.Undefined(), fmt.Errorf("JavaScript array is too large")
			}
			elements := make([]bytecode.Value, length)
			for index := range elements {
				converted, err := readValueAtDepth(value.Index(index), depth+1)
				if err != nil {
					return bytecode.Undefined(), fmt.Errorf("array element %d: %w", index, err)
				}
				elements[index] = converted
			}
			return bytecode.Array(elements...), nil
		}
		keys := js.Global().Get("Object").Call("keys", value)
		if keys.Length() > maxCollectionSize {
			return bytecode.Undefined(), fmt.Errorf("JavaScript object has too many properties")
		}
		result, err := bytecode.ConstructCollection("Structure", nil)
		if err != nil {
			return bytecode.Undefined(), err
		}
		for index := 0; index < keys.Length(); index++ {
			name := keys.Index(index).String()
			converted, convertErr := readValueAtDepth(value.Get(name), depth+1)
			if convertErr != nil {
				return bytecode.Undefined(), fmt.Errorf("property %s: %w", name, convertErr)
			}
			if _, insertErr := bytecode.CollectionMethod(result, "Insert", []bytecode.Value{bytecode.String(name), converted}); insertErr != nil {
				return bytecode.Undefined(), insertErr
			}
		}
		return result, nil
	default:
	}
	return bytecode.Undefined(), fmt.Errorf("unsupported JavaScript value type %s", value.Type())
}

func valueResult(value bytecode.Value) js.Value {
	return valueResultAtDepth(value, 0)
}

func valueResultAtDepth(value bytecode.Value, depth int) js.Value {
	if depth > maxValueNesting {
		return failure("VM result nesting is too deep")
	}
	result := success()
	switch value.Kind() {
	case bytecode.UndefinedKind:
		result.Set("kind", "undefined")
		result.Set("value", js.Undefined())
	case bytecode.NumberKind:
		number, _ := value.AsNumber()
		exact, _ := value.NumberText()
		result.Set("kind", "number")
		result.Set("value", number)
		result.Set("text", exact)
	case bytecode.StringKind:
		text, _ := value.AsString()
		result.Set("kind", "string")
		result.Set("value", text)
	case bytecode.BooleanKind:
		boolean, _ := value.AsBoolean()
		result.Set("kind", "boolean")
		result.Set("value", boolean)
	case bytecode.ArrayKind:
		length, _ := bytecode.CollectionLength(value)
		array := js.Global().Get("Array").New(length)
		for index := 0; index < length; index++ {
			element, _ := bytecode.CollectionElement(value, index)
			converted := valueResultAtDepth(element, depth+1)
			if !converted.Get("ok").Bool() {
				return failure(fmt.Sprintf("array element %d: %s", index, converted.Get("error").String()))
			}
			array.SetIndex(index, converted.Get("value"))
		}
		result.Set("kind", "array")
		result.Set("value", array)
	case bytecode.StructureKind, bytecode.ValueListItemKind, bytecode.ValueTableRowKind,
		bytecode.ValueTableColumnKind, bytecode.KeyAndValueKind:
		object, err := collectionObjectResult(value, depth)
		if err != nil {
			return failure(err.Error())
		}
		result.Set("kind", value.Kind().String())
		result.Set("value", object)
	case bytecode.MapKind, bytecode.ValueListKind, bytecode.ValueTableKind, bytecode.ValueTableColumnsKind:
		values, ok := bytecode.CollectionSnapshot(value)
		if !ok {
			return failure("could not enumerate collection result")
		}
		array := js.Global().Get("Array").New(len(values))
		for index, item := range values {
			converted := valueResultAtDepth(item, depth+1)
			if !converted.Get("ok").Bool() {
				return failure(fmt.Sprintf("collection item %d: %s", index, converted.Get("error").String()))
			}
			array.SetIndex(index, converted.Get("value"))
		}
		result.Set("kind", value.Kind().String())
		result.Set("value", array)
	case bytecode.NullKind:
		result.Set("kind", "null")
		result.Set("value", js.Null())
	case bytecode.DateKind:
		date, _ := value.AsDate()
		ticks, _ := value.DateTicks()
		result.Set("kind", "date")
		result.Set("value", js.Global().Get("Date").New(date.UnixMilli()))
		result.Set("ticks100us", float64(ticks))
	default:
		return failure("unsupported VM result type")
	}
	return result
}

func collectionObjectResult(value bytecode.Value, depth int) (js.Value, error) {
	names, ok := bytecode.CollectionPropertyNames(value)
	if !ok {
		return js.Undefined(), fmt.Errorf("could not enumerate %s properties", value.Kind())
	}
	object := js.Global().Get("Object").New()
	for _, name := range names {
		property, err := bytecode.CollectionProperty(value, name)
		if err != nil {
			return js.Undefined(), err
		}
		converted := valueResultAtDepth(property, depth+1)
		if !converted.Get("ok").Bool() {
			return js.Undefined(), fmt.Errorf("property %s: %s", name, converted.Get("error").String())
		}
		object.Set(name, converted.Get("value"))
	}
	return object, nil
}

func success() js.Value {
	result := js.Global().Get("Object").New()
	result.Set("ok", true)
	return result
}

func failure(message string) js.Value {
	result := js.Global().Get("Object").New()
	result.Set("ok", false)
	result.Set("error", message)
	return result
}

func executionFailure(err error) js.Value {
	result := failure(err.Error())
	var runtimeError *vm.RuntimeError
	if !errors.As(err, &runtimeError) {
		return result
	}
	result.Set("message", runtimeError.Message)
	stack := js.Global().Get("Array").New(len(runtimeError.Stack))
	for index, frame := range runtimeError.Stack {
		encoded := js.Global().Get("Object").New()
		encoded.Set("module", frame.Module)
		encoded.Set("function", frame.Function)
		encoded.Set("filename", frame.Filename)
		encoded.Set("line", frame.Span.Start.Line)
		encoded.Set("column", frame.Span.Start.Column)
		encoded.Set("endLine", frame.Span.End.Line)
		encoded.Set("endColumn", frame.Span.End.Column)
		stack.SetIndex(index, encoded)
	}
	result.Set("stack", stack)
	return result
}
