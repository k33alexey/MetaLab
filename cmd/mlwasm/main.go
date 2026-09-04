//go:build js && wasm

// Command mlwasm exposes the MetaLab client VM to a browser.
package main

import (
	"fmt"
	"math"
	"syscall/js"
	"time"

	"github.com/k33alexey/MetaLab/internal/bsl/bytecode"
	"github.com/k33alexey/MetaLab/internal/bsl/clientvm"
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
	if arguments[1].Type() != js.TypeString || !js.Global().Get("Array").Call("isArray", arguments[2]).Bool() {
		return failure("call expects a string routine name and an argument array")
	}
	argumentCount := arguments[2].Length()
	if argumentCount > int(^uint16(0)) {
		return failure("too many routine arguments")
	}
	var inline [32]bytecode.Value
	values := inline[:]
	if argumentCount <= len(inline) {
		values = values[:argumentCount]
	} else {
		values = make([]bytecode.Value, argumentCount)
	}
	for index := range values {
		values[index], err = readValue(arguments[2].Index(index))
		if err != nil {
			return failure(fmt.Sprintf("argument %d: %v", index, err))
		}
	}
	value, err := registry.Call(handle, arguments[1].String(), values...)
	if err != nil {
		return failure(err.Error())
	}
	return valueResult(value)
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
		if !js.Global().Get("Array").Call("isArray", value).Bool() {
			break
		}
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
		length, _ := value.ArrayLength()
		array := js.Global().Get("Array").New(length)
		for index := 0; index < length; index++ {
			element, _ := value.ArrayElement(index)
			converted := valueResultAtDepth(element, depth+1)
			if !converted.Get("ok").Bool() {
				return failure(fmt.Sprintf("array element %d: %s", index, converted.Get("error").String()))
			}
			array.SetIndex(index, converted.Get("value"))
		}
		result.Set("kind", "array")
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
