package bytecode

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"math"

	"github.com/k33alexey/MetaLab/internal/bsl/syntax"
)

const (
	wireMagic            = "MLBC"
	maxWireSize          = 64 << 20
	maxWireStringSize    = 16 << 20
	maxWireCollectionLen = 1 << 20
)

// MarshalBinary encodes a validated program for transfer to a MetaLab VM.
func MarshalBinary(program *Program) ([]byte, error) {
	if program == nil {
		return nil, fmt.Errorf("bytecode program is nil")
	}
	if err := program.Validate(); err != nil {
		return nil, fmt.Errorf("validate bytecode: %w", err)
	}
	if len(program.Functions) > maxWireCollectionLen {
		return nil, fmt.Errorf("too many functions: %d", len(program.Functions))
	}

	var output bytes.Buffer
	output.WriteString(wireMagic)
	writeUint16(&output, program.Version)
	writeUint32(&output, uint32(len(program.Modules)))
	for moduleIndex := range program.Modules {
		module := &program.Modules[moduleIndex]
		if err := writeString(&output, module.Name); err != nil {
			return nil, fmt.Errorf("module %d name: %w", moduleIndex, err)
		}
		writeUint32(&output, uint32(len(module.Variables)))
		for variableIndex, variable := range module.Variables {
			if err := writeString(&output, variable.Name); err != nil {
				return nil, fmt.Errorf("module %q variable %d: %w", module.Name, variableIndex, err)
			}
			if variable.Export {
				output.WriteByte(1)
			} else {
				output.WriteByte(0)
			}
		}
	}
	writeUint32(&output, uint32(len(program.Functions)))
	for index := range program.Functions {
		function := &program.Functions[index]
		if err := writeString(&output, function.Name); err != nil {
			return nil, fmt.Errorf("function %d name: %w", index, err)
		}
		writeUint16(&output, function.Module)
		var flags byte
		if function.IsFunction {
			flags |= 1
		}
		if function.Export {
			flags |= 2
		}
		output.WriteByte(flags)
		writeUint16(&output, function.Arity)
		for parameterIndex := 0; parameterIndex < int(function.Arity); parameterIndex++ {
			parameter := Parameter{ByValue: true}
			if len(function.Parameters) != 0 {
				parameter = function.Parameters[parameterIndex]
			}
			var parameterFlags byte
			if parameter.ByValue {
				parameterFlags |= 1
			}
			if parameter.HasDefault {
				parameterFlags |= 2
			}
			output.WriteByte(parameterFlags)
			if parameter.HasDefault {
				if err := writeValue(&output, parameter.Default); err != nil {
					return nil, fmt.Errorf("function %q parameter %d default: %w", function.Name, parameterIndex, err)
				}
			}
		}
		writeUint16(&output, function.LocalCount)
		writeUint16(&output, function.MaxStack)

		if len(function.Constants) > maxWireCollectionLen {
			return nil, fmt.Errorf("function %q has too many constants", function.Name)
		}
		writeUint32(&output, uint32(len(function.Constants)))
		for constantIndex, value := range function.Constants {
			if err := writeValue(&output, value); err != nil {
				return nil, fmt.Errorf("function %q constant %d: %w", function.Name, constantIndex, err)
			}
		}

		writeUint32(&output, uint32(len(function.ModuleVars)))
		for _, reference := range function.ModuleVars {
			writeReference(&output, reference)
		}
		writeUint32(&output, uint32(len(function.CallSites)))
		for _, call := range function.CallSites {
			writeUint16(&output, call.Target)
			writeUint32(&output, uint32(len(call.References)))
			for _, reference := range call.References {
				writeReference(&output, reference)
			}
		}

		if len(function.Code) > maxWireCollectionLen {
			return nil, fmt.Errorf("function %q has too many instructions", function.Name)
		}
		writeUint32(&output, uint32(len(function.Code)))
		for instructionIndex, instruction := range function.Code {
			output.WriteByte(byte(instruction.Opcode))
			writeUint16(&output, instruction.Operand)
			if err := writeSpan(&output, instruction.Span); err != nil {
				return nil, fmt.Errorf("function %q instruction %d: %w", function.Name, instructionIndex, err)
			}
		}
	}
	if output.Len() > maxWireSize {
		return nil, fmt.Errorf("encoded bytecode is too large: %d bytes", output.Len())
	}
	return output.Bytes(), nil
}

// UnmarshalBinary decodes and validates bytecode received by a MetaLab VM.
func UnmarshalBinary(data []byte) (*Program, error) {
	if len(data) > maxWireSize {
		return nil, fmt.Errorf("encoded bytecode is too large: %d bytes", len(data))
	}
	decoder := wireDecoder{data: data}
	magic, err := decoder.readBytes(len(wireMagic))
	if err != nil || string(magic) != wireMagic {
		return nil, fmt.Errorf("invalid bytecode header")
	}
	version, err := decoder.readUint16()
	if err != nil {
		return nil, err
	}
	moduleCount, err := decoder.readCount("modules", 8)
	if err != nil {
		return nil, err
	}
	program := &Program{Version: version, Modules: make([]Module, moduleCount)}
	for index := range program.Modules {
		module, readErr := decoder.readModule()
		if readErr != nil {
			return nil, fmt.Errorf("module %d: %w", index, readErr)
		}
		program.Modules[index] = module
	}
	functionCount, err := decoder.readCount("functions", 21)
	if err != nil {
		return nil, err
	}
	program.Functions = make([]Function, functionCount)
	for index := range program.Functions {
		function, err := decoder.readFunction()
		if err != nil {
			return nil, fmt.Errorf("function %d: %w", index, err)
		}
		program.Functions[index] = function
	}
	if decoder.remaining() != 0 {
		return nil, fmt.Errorf("bytecode has %d trailing bytes", decoder.remaining())
	}
	if err := program.Validate(); err != nil {
		return nil, fmt.Errorf("validate bytecode: %w", err)
	}
	return program, nil
}

func writeValue(output *bytes.Buffer, value Value) error {
	output.WriteByte(byte(value.Kind()))
	switch value.Kind() {
	case UndefinedKind:
		return nil
	case NumberKind:
		number, _ := value.NumberText()
		return writeString(output, number)
	case StringKind:
		text, _ := value.AsString()
		return writeString(output, text)
	case BooleanKind:
		boolean, _ := value.AsBoolean()
		if boolean {
			output.WriteByte(1)
		} else {
			output.WriteByte(0)
		}
		return nil
	case NullKind:
		return nil
	case DateKind:
		ticks, _ := value.DateTicks()
		writeUint64(output, uint64(ticks))
		return nil
	default:
		return fmt.Errorf("unsupported value kind %d", value.Kind())
	}
}

func writeString(output *bytes.Buffer, value string) error {
	if len(value) > maxWireStringSize {
		return fmt.Errorf("string is too large: %d bytes", len(value))
	}
	writeUint32(output, uint32(len(value)))
	output.WriteString(value)
	return nil
}

func writeSpan(output *bytes.Buffer, span syntax.Span) error {
	positions := [...]int{
		span.Start.Offset, span.Start.Line, span.Start.Column,
		span.End.Offset, span.End.Line, span.End.Column,
	}
	for _, value := range positions {
		if value < 0 || uint64(value) > math.MaxUint32 {
			return fmt.Errorf("source position %d is outside uint32", value)
		}
		writeUint32(output, uint32(value))
	}
	return nil
}

func writeUint16(output *bytes.Buffer, value uint16) {
	var data [2]byte
	binary.LittleEndian.PutUint16(data[:], value)
	output.Write(data[:])
}

func writeUint32(output *bytes.Buffer, value uint32) {
	var data [4]byte
	binary.LittleEndian.PutUint32(data[:], value)
	output.Write(data[:])
}

func writeUint64(output *bytes.Buffer, value uint64) {
	var data [8]byte
	binary.LittleEndian.PutUint64(data[:], value)
	output.Write(data[:])
}

func writeReference(output *bytes.Buffer, reference VariableReference) {
	output.WriteByte(byte(reference.Kind))
	writeUint16(output, reference.Module)
	writeUint16(output, reference.Variable)
}

type wireDecoder struct {
	data   []byte
	offset int
}

func (decoder *wireDecoder) readModule() (Module, error) {
	name, err := decoder.readString()
	if err != nil {
		return Module{}, fmt.Errorf("name: %w", err)
	}
	variableCount, err := decoder.readCount("variables", 5)
	if err != nil {
		return Module{}, err
	}
	variables := make([]ModuleVariable, variableCount)
	for index := range variables {
		variableName, readErr := decoder.readString()
		if readErr != nil {
			return Module{}, fmt.Errorf("variable %d name: %w", index, readErr)
		}
		exported, readErr := decoder.readByte()
		if readErr != nil {
			return Module{}, fmt.Errorf("variable %d export: %w", index, readErr)
		}
		if exported > 1 {
			return Module{}, fmt.Errorf("variable %d has invalid export flag %d", index, exported)
		}
		variables[index] = ModuleVariable{Name: variableName, Export: exported == 1}
	}
	return Module{Name: name, Variables: variables}, nil
}

func (decoder *wireDecoder) readFunction() (Function, error) {
	name, err := decoder.readString()
	if err != nil {
		return Function{}, fmt.Errorf("name: %w", err)
	}
	module, err := decoder.readUint16()
	if err != nil {
		return Function{}, err
	}
	flags, err := decoder.readByte()
	if err != nil {
		return Function{}, err
	}
	if flags&^byte(3) != 0 {
		return Function{}, fmt.Errorf("unknown function flags %d", flags)
	}
	arity, err := decoder.readUint16()
	if err != nil {
		return Function{}, err
	}
	parameters := make([]Parameter, arity)
	for index := range parameters {
		parameterFlags, readErr := decoder.readByte()
		if readErr != nil {
			return Function{}, fmt.Errorf("parameter %d: %w", index, readErr)
		}
		if parameterFlags&^byte(3) != 0 {
			return Function{}, fmt.Errorf("parameter %d has unknown flags %d", index, parameterFlags)
		}
		parameters[index] = Parameter{ByValue: parameterFlags&1 != 0, HasDefault: parameterFlags&2 != 0}
		if parameters[index].HasDefault {
			parameters[index].Default, readErr = decoder.readValue()
			if readErr != nil {
				return Function{}, fmt.Errorf("parameter %d default: %w", index, readErr)
			}
		}
	}
	localCount, err := decoder.readUint16()
	if err != nil {
		return Function{}, err
	}
	maxStack, err := decoder.readUint16()
	if err != nil {
		return Function{}, err
	}
	constantCount, err := decoder.readCount("constants", 1)
	if err != nil {
		return Function{}, err
	}
	constants := make([]Value, constantCount)
	for index := range constants {
		constants[index], err = decoder.readValue()
		if err != nil {
			return Function{}, fmt.Errorf("constant %d: %w", index, err)
		}
	}
	moduleVariableCount, err := decoder.readCount("module variable accesses", 5)
	if err != nil {
		return Function{}, err
	}
	moduleVariables := make([]VariableReference, moduleVariableCount)
	for index := range moduleVariables {
		moduleVariables[index], err = decoder.readReference()
		if err != nil {
			return Function{}, fmt.Errorf("module variable access %d: %w", index, err)
		}
	}
	callCount, err := decoder.readCount("call sites", 6)
	if err != nil {
		return Function{}, err
	}
	calls := make([]CallSite, callCount)
	for index := range calls {
		target, readErr := decoder.readUint16()
		if readErr != nil {
			return Function{}, fmt.Errorf("call site %d target: %w", index, readErr)
		}
		referenceCount, readErr := decoder.readCount("call references", 5)
		if readErr != nil {
			return Function{}, fmt.Errorf("call site %d: %w", index, readErr)
		}
		references := make([]VariableReference, referenceCount)
		for referenceIndex := range references {
			references[referenceIndex], readErr = decoder.readReference()
			if readErr != nil {
				return Function{}, fmt.Errorf("call site %d reference %d: %w", index, referenceIndex, readErr)
			}
		}
		calls[index] = CallSite{Target: target, References: references}
	}
	instructionCount, err := decoder.readCount("instructions", 27)
	if err != nil {
		return Function{}, err
	}
	code := make([]Instruction, instructionCount)
	for index := range code {
		opcode, readErr := decoder.readByte()
		if readErr != nil {
			return Function{}, fmt.Errorf("instruction %d: %w", index, readErr)
		}
		operand, readErr := decoder.readUint16()
		if readErr != nil {
			return Function{}, fmt.Errorf("instruction %d: %w", index, readErr)
		}
		span, readErr := decoder.readSpan()
		if readErr != nil {
			return Function{}, fmt.Errorf("instruction %d: %w", index, readErr)
		}
		code[index] = Instruction{Opcode: Opcode(opcode), Operand: operand, Span: span}
	}
	return Function{
		Name: name, Module: module, IsFunction: flags&1 != 0, Export: flags&2 != 0,
		Arity: arity, Parameters: parameters, LocalCount: localCount, MaxStack: maxStack,
		Constants: constants, ModuleVars: moduleVariables, CallSites: calls, Code: code,
	}, nil
}

func (decoder *wireDecoder) readReference() (VariableReference, error) {
	kind, err := decoder.readByte()
	if err != nil {
		return VariableReference{}, err
	}
	module, err := decoder.readUint16()
	if err != nil {
		return VariableReference{}, err
	}
	variable, err := decoder.readUint16()
	if err != nil {
		return VariableReference{}, err
	}
	if ReferenceKind(kind) > ModuleReference {
		return VariableReference{}, fmt.Errorf("unknown reference kind %d", kind)
	}
	return VariableReference{Kind: ReferenceKind(kind), Module: module, Variable: variable}, nil
}

func (decoder *wireDecoder) readValue() (Value, error) {
	kind, err := decoder.readByte()
	if err != nil {
		return Value{}, err
	}
	switch ValueKind(kind) {
	case UndefinedKind:
		return Undefined(), nil
	case NumberKind:
		text, readErr := decoder.readString()
		if readErr != nil {
			return Value{}, readErr
		}
		return ParseNumber(text)
	case StringKind:
		text, readErr := decoder.readString()
		if readErr != nil {
			return Value{}, readErr
		}
		return String(text), nil
	case BooleanKind:
		encoded, readErr := decoder.readByte()
		if readErr != nil {
			return Value{}, readErr
		}
		if encoded > 1 {
			return Value{}, fmt.Errorf("invalid boolean value %d", encoded)
		}
		return Boolean(encoded == 1), nil
	case NullKind:
		return Null(), nil
	case DateKind:
		ticks, readErr := decoder.readUint64()
		if readErr != nil {
			return Value{}, readErr
		}
		return DateFromTicks(int64(ticks))
	default:
		return Value{}, fmt.Errorf("unsupported value kind %d", kind)
	}
}

func (decoder *wireDecoder) readSpan() (syntax.Span, error) {
	values := [6]int{}
	for index := range values {
		value, err := decoder.readUint32()
		if err != nil {
			return syntax.Span{}, err
		}
		if uint64(value) > uint64(maxInt()) {
			return syntax.Span{}, fmt.Errorf("source position overflows int")
		}
		values[index] = int(value)
	}
	return syntax.Span{
		Start: syntax.Position{Offset: values[0], Line: values[1], Column: values[2]},
		End:   syntax.Position{Offset: values[3], Line: values[4], Column: values[5]},
	}, nil
}

func (decoder *wireDecoder) readString() (string, error) {
	length, err := decoder.readUint32()
	if err != nil {
		return "", err
	}
	if length > maxWireStringSize {
		return "", fmt.Errorf("string is too large: %d bytes", length)
	}
	data, err := decoder.readBytes(int(length))
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func (decoder *wireDecoder) readCount(label string, minimumEntrySize int) (int, error) {
	count, err := decoder.readUint32()
	if err != nil {
		return 0, err
	}
	if count > maxWireCollectionLen {
		return 0, fmt.Errorf("too many %s: %d", label, count)
	}
	if uint64(count)*uint64(minimumEntrySize) > uint64(decoder.remaining()) {
		return 0, fmt.Errorf("declared %s exceed remaining bytecode", label)
	}
	return int(count), nil
}

func (decoder *wireDecoder) readByte() (byte, error) {
	data, err := decoder.readBytes(1)
	if err != nil {
		return 0, err
	}
	return data[0], nil
}

func (decoder *wireDecoder) readUint16() (uint16, error) {
	data, err := decoder.readBytes(2)
	if err != nil {
		return 0, err
	}
	return binary.LittleEndian.Uint16(data), nil
}

func (decoder *wireDecoder) readUint32() (uint32, error) {
	data, err := decoder.readBytes(4)
	if err != nil {
		return 0, err
	}
	return binary.LittleEndian.Uint32(data), nil
}

func (decoder *wireDecoder) readUint64() (uint64, error) {
	data, err := decoder.readBytes(8)
	if err != nil {
		return 0, err
	}
	return binary.LittleEndian.Uint64(data), nil
}

func (decoder *wireDecoder) readBytes(length int) ([]byte, error) {
	if length < 0 || length > decoder.remaining() {
		return nil, fmt.Errorf("unexpected end of bytecode at offset %d", decoder.offset)
	}
	result := decoder.data[decoder.offset : decoder.offset+length]
	decoder.offset += length
	return result, nil
}

func (decoder *wireDecoder) remaining() int { return len(decoder.data) - decoder.offset }

func maxInt() int { return int(^uint(0) >> 1) }
