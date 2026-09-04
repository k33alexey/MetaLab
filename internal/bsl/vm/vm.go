// Package vm executes validated MetaLab BSL bytecode.
package vm

import (
	"fmt"
	"strings"
	"sync"

	"github.com/k33alexey/MetaLab/internal/bsl/bytecode"
	"github.com/k33alexey/MetaLab/internal/bsl/syntax"
)

const (
	inlineStackSize = 64
	inlineLocalSize = 16
	maxCallDepth    = 256
)

var callArgumentPool = sync.Pool{New: func() any { return new([inlineLocalSize]callArgument) }}

// Machine is an immutable, concurrency-safe bytecode runtime.
type Machine struct {
	program *bytecode.Program
}

// Context owns module variables for one isolated BSL session.
type Context struct {
	machine *Machine
	mutex   sync.Mutex
	modules [][]bytecode.Value
}

// New validates a program once before it can be executed.
func New(program *bytecode.Program) (*Machine, error) {
	if program == nil {
		return nil, fmt.Errorf("bytecode program is nil")
	}
	if err := program.Validate(); err != nil {
		return nil, fmt.Errorf("validate bytecode: %w", err)
	}
	return &Machine{program: cloneProgram(program)}, nil
}

func cloneProgram(program *bytecode.Program) *bytecode.Program {
	clone := &bytecode.Program{
		Version: program.Version, Modules: make([]bytecode.Module, len(program.Modules)),
		Functions: make([]bytecode.Function, len(program.Functions)),
	}
	for index := range program.Modules {
		clone.Modules[index] = program.Modules[index]
		clone.Modules[index].Variables = append([]bytecode.ModuleVariable(nil), program.Modules[index].Variables...)
	}
	for index := range program.Functions {
		clone.Functions[index] = program.Functions[index]
		clone.Functions[index].Parameters = append([]bytecode.Parameter(nil), program.Functions[index].Parameters...)
		clone.Functions[index].Constants = append([]bytecode.Value(nil), program.Functions[index].Constants...)
		clone.Functions[index].ModuleVars = append([]bytecode.VariableReference(nil), program.Functions[index].ModuleVars...)
		clone.Functions[index].CallSites = make([]bytecode.CallSite, len(program.Functions[index].CallSites))
		for callIndex := range program.Functions[index].CallSites {
			clone.Functions[index].CallSites[callIndex] = program.Functions[index].CallSites[callIndex]
			clone.Functions[index].CallSites[callIndex].References = append(
				[]bytecode.VariableReference(nil), program.Functions[index].CallSites[callIndex].References...,
			)
		}
		clone.Functions[index].Code = append([]bytecode.Instruction(nil), program.Functions[index].Code...)
	}
	return clone
}

// RuntimeError identifies the routine and source position of an execution failure.
type RuntimeError struct {
	Function string
	Span     syntax.Span
	Message  string
}

func (runtimeError *RuntimeError) Error() string {
	return fmt.Sprintf(
		"%s:%d:%d: %s",
		runtimeError.Function,
		runtimeError.Span.Start.Line,
		runtimeError.Span.Start.Column,
		runtimeError.Message,
	)
}

// Call executes a named BSL routine in an isolated transient context. Use
// NewContext when module variables must persist between calls.
func (machine *Machine) Call(name string, arguments ...bytecode.Value) (bytecode.Value, error) {
	function, ok := machine.program.Lookup(name)
	if !ok {
		return bytecode.Undefined(), fmt.Errorf("routine %q not found", name)
	}
	arguments, err := completeArguments(function, arguments)
	if err != nil {
		return bytecode.Undefined(), err
	}
	if !requiresContext(function) {
		return executeFast(function, arguments)
	}
	return executeWithValues(machine.program, function, arguments, makeModuleValues(machine.program))
}

// NewContext creates isolated persistent module state for one user session.
func (machine *Machine) NewContext() *Context {
	return &Context{machine: machine, modules: makeModuleValues(machine.program)}
}

// Call executes a routine while preserving this context's module variables.
func (context *Context) Call(name string, arguments ...bytecode.Value) (bytecode.Value, error) {
	function, ok := context.machine.program.Lookup(name)
	if !ok {
		return bytecode.Undefined(), fmt.Errorf("routine %q not found", name)
	}
	completed, err := completeArguments(function, arguments)
	if err != nil {
		return bytecode.Undefined(), err
	}
	if !requiresContext(function) {
		return executeFast(function, completed)
	}
	context.mutex.Lock()
	defer context.mutex.Unlock()
	return executeWithValues(context.machine.program, function, completed, context.modules)
}

// CallExported invokes only a routine explicitly exported by a named module.
func (context *Context) CallExported(module, name string, arguments ...bytecode.Value) (bytecode.Value, error) {
	qualified := module + "." + name
	function, ok := context.machine.program.Lookup(qualified)
	if !ok || !function.Export {
		return bytecode.Undefined(), fmt.Errorf("exported routine %q not found", qualified)
	}
	return context.Call(qualified, arguments...)
}

func completeArguments(function *bytecode.Function, arguments []bytecode.Value) ([]bytecode.Value, error) {
	required := int(function.Arity)
	if len(function.Parameters) != 0 {
		for required > 0 && function.Parameters[required-1].HasDefault {
			required--
		}
	}
	if len(arguments) < required || len(arguments) > int(function.Arity) {
		return nil, fmt.Errorf(
			"routine %q expects %d..%d arguments, got %d", function.Name, required, function.Arity, len(arguments),
		)
	}
	if len(arguments) == int(function.Arity) {
		return arguments, nil
	}
	completed := make([]bytecode.Value, function.Arity)
	copy(completed, arguments)
	for index := len(arguments); index < len(completed); index++ {
		completed[index] = function.Parameters[index].Default
	}
	return completed, nil
}

func makeModuleValues(program *bytecode.Program) [][]bytecode.Value {
	hasVariables := false
	for index := range program.Modules {
		hasVariables = hasVariables || len(program.Modules[index].Variables) != 0
	}
	if !hasVariables {
		return nil
	}
	modules := make([][]bytecode.Value, len(program.Modules))
	for index := range program.Modules {
		if len(program.Modules[index].Variables) != 0 {
			modules[index] = make([]bytecode.Value, len(program.Modules[index].Variables))
		}
	}
	return modules
}

func requiresContext(function *bytecode.Function) bool {
	return len(function.CallSites) != 0 || len(function.ModuleVars) != 0
}

func executeFast(function *bytecode.Function, arguments []bytecode.Value) (bytecode.Value, error) {
	var inlineLocals [inlineLocalSize]bytecode.Value
	locals := inlineLocals[:]
	if int(function.LocalCount) > len(locals) {
		locals = make([]bytecode.Value, function.LocalCount)
	} else {
		locals = locals[:function.LocalCount]
	}
	copy(locals, arguments)
	var inline [inlineStackSize]bytecode.Value
	stack := inline[:]
	if int(function.MaxStack) > len(stack) {
		stack = make([]bytecode.Value, function.MaxStack)
	}
	depth := 0
	push := func(value bytecode.Value) {
		stack[depth] = value
		depth++
	}
	pop := func() bytecode.Value {
		depth--
		return stack[depth]
	}

	for instructionPointer := 0; instructionPointer < len(function.Code); instructionPointer++ {
		instruction := function.Code[instructionPointer]
		switch instruction.Opcode {
		case bytecode.OpConstant:
			push(function.Constants[instruction.Operand])
		case bytecode.OpLoadLocal:
			push(locals[instruction.Operand])
		case bytecode.OpStoreLocal:
			locals[instruction.Operand] = pop()
		case bytecode.OpPositive, bytecode.OpNegate:
			value := pop()
			if value.Kind() != bytecode.NumberKind {
				return bytecode.Undefined(), runtimeFailure(function, instruction, "unary sign requires a number")
			}
			if instruction.Opcode == bytecode.OpPositive {
				push(value)
				break
			}
			negated, err := bytecode.NegateNumber(value)
			if err != nil {
				return bytecode.Undefined(), runtimeFailure(function, instruction, err.Error())
			}
			push(negated)
		case bytecode.OpNot:
			value := pop()
			boolean, ok := value.AsBoolean()
			if !ok {
				return bytecode.Undefined(), runtimeFailure(function, instruction, "Not requires a boolean")
			}
			push(bytecode.Boolean(!boolean))
		case bytecode.OpBoolean:
			value := pop()
			if _, ok := value.AsBoolean(); !ok {
				return bytecode.Undefined(), runtimeFailure(function, instruction, "logical operation requires a boolean")
			}
			push(value)
		case bytecode.OpArrayLength:
			value := pop()
			length, ok := value.ArrayLength()
			if !ok {
				return bytecode.Undefined(), runtimeFailure(function, instruction, "For Each requires an array")
			}
			push(bytecode.Number(float64(length)))
		case bytecode.OpArrayElement:
			indexValue := pop()
			array := pop()
			index, ok := indexValue.NumberInteger()
			if !ok || index < 0 || uint64(index) > uint64(maxInt()) {
				return bytecode.Undefined(), runtimeFailure(function, instruction, "array index must be a non-negative integer")
			}
			element, ok := array.ArrayElement(int(index))
			if !ok {
				return bytecode.Undefined(), runtimeFailure(function, instruction, "array index is out of range")
			}
			push(element)
		case bytecode.OpPop:
			pop()
		case bytecode.OpAdd, bytecode.OpSubtract, bytecode.OpMultiply, bytecode.OpDivide,
			bytecode.OpModulo, bytecode.OpEqual, bytecode.OpNotEqual, bytecode.OpLess,
			bytecode.OpLessEqual, bytecode.OpGreater, bytecode.OpGreaterEqual,
			bytecode.OpAnd, bytecode.OpOr:
			right := pop()
			left := pop()
			result, err := binary(instruction.Opcode, left, right)
			if err != nil {
				return bytecode.Undefined(), runtimeFailure(function, instruction, err.Error())
			}
			push(result)
		case bytecode.OpJump:
			instructionPointer = int(instruction.Operand) - 1
		case bytecode.OpJumpIfFalse:
			condition := pop()
			boolean, ok := condition.AsBoolean()
			if !ok {
				return bytecode.Undefined(), runtimeFailure(function, instruction, "condition requires a boolean")
			}
			if !boolean {
				instructionPointer = int(instruction.Operand) - 1
			}
		case bytecode.OpJumpIfTrueKeep, bytecode.OpJumpIfFalseKeep:
			condition := stack[depth-1]
			boolean, ok := condition.AsBoolean()
			if !ok {
				return bytecode.Undefined(), runtimeFailure(function, instruction, "logical operation requires a boolean")
			}
			if instruction.Opcode == bytecode.OpJumpIfTrueKeep && boolean || instruction.Opcode == bytecode.OpJumpIfFalseKeep && !boolean {
				instructionPointer = int(instruction.Operand) - 1
			}
		case bytecode.OpReturn:
			return pop(), nil
		default:
			return bytecode.Undefined(), runtimeFailure(function, instruction, "unknown opcode")
		}
	}
	return bytecode.Undefined(), fmt.Errorf("routine %q ended without return", function.Name)
}

type callArgument struct {
	value    bytecode.Value
	identity uint64
}

func executeWithValues(
	program *bytecode.Program,
	function *bytecode.Function,
	values []bytecode.Value,
	modules [][]bytecode.Value,
) (bytecode.Value, error) {
	return executeAdvanced(program, function, nil, values, modules, 0)
}

func executeAdvanced(
	program *bytecode.Program,
	function *bytecode.Function,
	arguments []callArgument,
	rootValues []bytecode.Value,
	modules [][]bytecode.Value,
	callDepth int,
) (bytecode.Value, error) {
	if callDepth >= maxCallDepth {
		return bytecode.Undefined(), fmt.Errorf("maximum BSL call depth %d exceeded", maxCallDepth)
	}
	var inlineLocals [inlineLocalSize]bytecode.Value
	locals := inlineLocals[:]
	if int(function.LocalCount) > len(locals) {
		locals = make([]bytecode.Value, function.LocalCount)
	} else {
		locals = locals[:function.LocalCount]
	}
	var inlineIdentities [inlineLocalSize]uint64
	identities := inlineIdentities[:]
	if int(function.LocalCount) > len(identities) {
		identities = make([]uint64, function.LocalCount)
	} else {
		identities = identities[:function.LocalCount]
	}
	for index := 0; index < int(function.Arity); index++ {
		var argument callArgument
		if arguments != nil {
			argument = arguments[index]
		} else {
			argument.value = rootValues[index]
		}
		byValue := true
		if len(function.Parameters) != 0 {
			byValue = function.Parameters[index].ByValue
		}
		locals[index] = argument.value
		if !byValue {
			identities[index] = argument.identity
		}
	}
	moduleIdentity := func(reference bytecode.VariableReference) uint64 {
		return uint64(1)<<63 | uint64(reference.Module)<<32 | uint64(reference.Variable) + 1
	}
	moduleReference := func(identity uint64) bytecode.VariableReference {
		return bytecode.VariableReference{
			Kind: bytecode.ModuleReference, Module: uint16(identity >> 32),
			Variable: uint16((identity & uint64(^uint32(0))) - 1),
		}
	}
	setModule := func(reference bytecode.VariableReference, value bytecode.Value) {
		modules[reference.Module][reference.Variable] = value
		identity := moduleIdentity(reference)
		for index := range identities {
			if identities[index] == identity {
				locals[index] = value
			}
		}
	}
	setLocal := func(index uint16, value bytecode.Value) {
		identity := identities[index]
		if identity == 0 {
			locals[index] = value
			return
		}
		for localIndex := range identities {
			if identities[localIndex] == identity {
				locals[localIndex] = value
			}
		}
		if identity>>63 != 0 {
			reference := moduleReference(identity)
			modules[reference.Module][reference.Variable] = value
		}
	}
	localIdentity := func(index uint16) uint64 {
		if identities[index] != 0 {
			return identities[index]
		}
		return uint64(callDepth+1)<<32 | uint64(index) + 1
	}

	var inlineStack [inlineStackSize]bytecode.Value
	stack := inlineStack[:]
	if int(function.MaxStack) > len(stack) {
		stack = make([]bytecode.Value, function.MaxStack)
	}
	depth := 0
	push := func(value bytecode.Value) {
		stack[depth] = value
		depth++
	}
	pop := func() bytecode.Value {
		depth--
		return stack[depth]
	}

	for instructionPointer := 0; instructionPointer < len(function.Code); instructionPointer++ {
		instruction := function.Code[instructionPointer]
		switch instruction.Opcode {
		case bytecode.OpConstant:
			push(function.Constants[instruction.Operand])
		case bytecode.OpLoadLocal:
			push(locals[instruction.Operand])
		case bytecode.OpStoreLocal:
			setLocal(instruction.Operand, pop())
		case bytecode.OpLoadModule:
			reference := function.ModuleVars[instruction.Operand]
			push(modules[reference.Module][reference.Variable])
		case bytecode.OpStoreModule:
			setModule(function.ModuleVars[instruction.Operand], pop())
		case bytecode.OpPositive, bytecode.OpNegate:
			value := pop()
			if value.Kind() != bytecode.NumberKind {
				return bytecode.Undefined(), runtimeFailure(function, instruction, "unary sign requires a number")
			}
			if instruction.Opcode == bytecode.OpPositive {
				push(value)
				break
			}
			negated, err := bytecode.NegateNumber(value)
			if err != nil {
				return bytecode.Undefined(), runtimeFailure(function, instruction, err.Error())
			}
			push(negated)
		case bytecode.OpNot:
			value := pop()
			boolean, ok := value.AsBoolean()
			if !ok {
				return bytecode.Undefined(), runtimeFailure(function, instruction, "Not requires a boolean")
			}
			push(bytecode.Boolean(!boolean))
		case bytecode.OpBoolean:
			value := pop()
			if _, ok := value.AsBoolean(); !ok {
				return bytecode.Undefined(), runtimeFailure(function, instruction, "logical operation requires a boolean")
			}
			push(value)
		case bytecode.OpArrayLength:
			value := pop()
			length, ok := value.ArrayLength()
			if !ok {
				return bytecode.Undefined(), runtimeFailure(function, instruction, "For Each requires an array")
			}
			push(bytecode.Number(float64(length)))
		case bytecode.OpArrayElement:
			indexValue := pop()
			array := pop()
			index, ok := indexValue.NumberInteger()
			if !ok || index < 0 || uint64(index) > uint64(maxInt()) {
				return bytecode.Undefined(), runtimeFailure(function, instruction, "array index must be a non-negative integer")
			}
			element, ok := array.ArrayElement(int(index))
			if !ok {
				return bytecode.Undefined(), runtimeFailure(function, instruction, "array index is out of range")
			}
			push(element)
		case bytecode.OpPop:
			pop()
		case bytecode.OpAdd, bytecode.OpSubtract, bytecode.OpMultiply, bytecode.OpDivide,
			bytecode.OpModulo, bytecode.OpEqual, bytecode.OpNotEqual, bytecode.OpLess,
			bytecode.OpLessEqual, bytecode.OpGreater, bytecode.OpGreaterEqual,
			bytecode.OpAnd, bytecode.OpOr:
			right := pop()
			left := pop()
			result, err := binary(instruction.Opcode, left, right)
			if err != nil {
				return bytecode.Undefined(), runtimeFailure(function, instruction, err.Error())
			}
			push(result)
		case bytecode.OpJump:
			instructionPointer = int(instruction.Operand) - 1
		case bytecode.OpJumpIfFalse:
			condition := pop()
			boolean, ok := condition.AsBoolean()
			if !ok {
				return bytecode.Undefined(), runtimeFailure(function, instruction, "condition requires a boolean")
			}
			if !boolean {
				instructionPointer = int(instruction.Operand) - 1
			}
		case bytecode.OpJumpIfTrueKeep, bytecode.OpJumpIfFalseKeep:
			condition := stack[depth-1]
			boolean, ok := condition.AsBoolean()
			if !ok {
				return bytecode.Undefined(), runtimeFailure(function, instruction, "logical operation requires a boolean")
			}
			if instruction.Opcode == bytecode.OpJumpIfTrueKeep && boolean || instruction.Opcode == bytecode.OpJumpIfFalseKeep && !boolean {
				instructionPointer = int(instruction.Operand) - 1
			}
		case bytecode.OpCall:
			call := function.CallSites[instruction.Operand]
			target := &program.Functions[call.Target]
			arity := int(target.Arity)
			var pooledArguments *[inlineLocalSize]callArgument
			var callArguments []callArgument
			if arity <= inlineLocalSize {
				pooledArguments = callArgumentPool.Get().(*[inlineLocalSize]callArgument)
				callArguments = pooledArguments[:arity]
			} else {
				callArguments = make([]callArgument, arity)
			}
			for index := arity - 1; index >= 0; index-- {
				callArguments[index].value = pop()
				reference := call.References[index]
				switch reference.Kind {
				case bytecode.LocalReference:
					callArguments[index].identity = localIdentity(reference.Variable)
				case bytecode.ModuleReference:
					callArguments[index].identity = moduleIdentity(reference)
				}
			}
			result, err := executeAdvanced(program, target, callArguments, nil, modules, callDepth+1)
			if err != nil {
				releaseCallArguments(pooledArguments, callArguments)
				return bytecode.Undefined(), err
			}
			for index, reference := range call.References {
				switch reference.Kind {
				case bytecode.LocalReference:
					setLocal(reference.Variable, callArguments[index].value)
				case bytecode.ModuleReference:
					setModule(reference, callArguments[index].value)
				}
			}
			for index, identity := range identities {
				if identity>>63 != 0 {
					reference := moduleReference(identity)
					locals[index] = modules[reference.Module][reference.Variable]
				}
			}
			releaseCallArguments(pooledArguments, callArguments)
			push(result)
		case bytecode.OpReturn:
			if arguments != nil {
				for index := range arguments {
					if len(function.Parameters) != 0 && !function.Parameters[index].ByValue {
						arguments[index].value = locals[index]
					}
				}
			}
			return pop(), nil
		default:
			return bytecode.Undefined(), runtimeFailure(function, instruction, "unknown opcode")
		}
	}
	return bytecode.Undefined(), fmt.Errorf("routine %q ended without return", function.Name)
}

func releaseCallArguments(pooled *[inlineLocalSize]callArgument, arguments []callArgument) {
	if pooled == nil {
		return
	}
	for index := range arguments {
		arguments[index] = callArgument{}
	}
	callArgumentPool.Put(pooled)
}

func binary(opcode bytecode.Opcode, left, right bytecode.Value) (bytecode.Value, error) {
	if opcode == bytecode.OpEqual || opcode == bytecode.OpNotEqual {
		equal := valuesEqual(left, right)
		if opcode == bytecode.OpNotEqual {
			equal = !equal
		}
		return bytecode.Boolean(equal), nil
	}
	if opcode == bytecode.OpLess || opcode == bytecode.OpLessEqual || opcode == bytecode.OpGreater || opcode == bytecode.OpGreaterEqual {
		comparison, err := compareValues(left, right)
		if err != nil {
			return bytecode.Undefined(), err
		}
		switch opcode {
		case bytecode.OpLess:
			return bytecode.Boolean(comparison < 0), nil
		case bytecode.OpLessEqual:
			return bytecode.Boolean(comparison <= 0), nil
		case bytecode.OpGreater:
			return bytecode.Boolean(comparison > 0), nil
		default:
			return bytecode.Boolean(comparison >= 0), nil
		}
	}
	if opcode == bytecode.OpAnd || opcode == bytecode.OpOr {
		leftBoolean, leftOK := left.AsBoolean()
		rightBoolean, rightOK := right.AsBoolean()
		if !leftOK || !rightOK {
			return bytecode.Undefined(), fmt.Errorf("%s requires two booleans", opcode)
		}
		if opcode == bytecode.OpAnd {
			return bytecode.Boolean(leftBoolean && rightBoolean), nil
		}
		return bytecode.Boolean(leftBoolean || rightBoolean), nil
	}
	if opcode == bytecode.OpAdd {
		if leftString, leftIsString := left.AsString(); leftIsString {
			return bytecode.String(leftString + right.String()), nil
		}
	}
	if left.Kind() == bytecode.DateKind {
		switch opcode {
		case bytecode.OpAdd:
			if right.Kind() != bytecode.NumberKind {
				return bytecode.Undefined(), fmt.Errorf("date addition requires a number of seconds")
			}
			return bytecode.AddDateSeconds(left, right)
		case bytecode.OpSubtract:
			if right.Kind() == bytecode.NumberKind {
				negative, err := bytecode.NegateNumber(right)
				if err != nil {
					return bytecode.Undefined(), err
				}
				return bytecode.AddDateSeconds(left, negative)
			}
			if difference, ok := bytecode.DateDifferenceSeconds(left, right); ok {
				return difference, nil
			}
			return bytecode.Undefined(), fmt.Errorf("date subtraction requires a number or date")
		}
	}
	if left.Kind() != bytecode.NumberKind || right.Kind() != bytecode.NumberKind {
		return bytecode.Undefined(), fmt.Errorf("%s requires two numbers", opcode)
	}
	var (
		result bytecode.Value
		err    error
	)
	switch opcode {
	case bytecode.OpAdd:
		result, err = bytecode.AddNumbers(left, right)
	case bytecode.OpSubtract:
		result, err = bytecode.SubtractNumbers(left, right)
	case bytecode.OpMultiply:
		result, err = bytecode.MultiplyNumbers(left, right)
	case bytecode.OpDivide:
		result, err = bytecode.DivideNumbers(left, right)
	case bytecode.OpModulo:
		result, err = bytecode.RemainderNumbers(left, right)
	default:
		return bytecode.Undefined(), fmt.Errorf("unsupported binary opcode %s", opcode)
	}
	if err != nil {
		return bytecode.Undefined(), err
	}
	return result, nil
}

func compareValues(left, right bytecode.Value) (int, error) {
	if left.Kind() != right.Kind() {
		leftRank, leftOK := comparisonRank(left.Kind())
		rightRank, rightOK := comparisonRank(right.Kind())
		if !leftOK || !rightOK {
			return 0, fmt.Errorf("values of kind %s and %s cannot be ordered", left.Kind(), right.Kind())
		}
		if leftRank < rightRank {
			return -1, nil
		}
		return 1, nil
	}
	switch left.Kind() {
	case bytecode.UndefinedKind, bytecode.NullKind:
		return 0, nil
	case bytecode.BooleanKind:
		leftValue, _ := left.AsBoolean()
		rightValue, _ := right.AsBoolean()
		if leftValue == rightValue {
			return 0, nil
		}
		if !leftValue {
			return -1, nil
		}
		return 1, nil
	case bytecode.NumberKind:
		comparison, _ := bytecode.CompareNumbers(left, right)
		return comparison, nil
	case bytecode.DateKind:
		leftTicks, _ := left.DateTicks()
		rightTicks, _ := right.DateTicks()
		if leftTicks < rightTicks {
			return -1, nil
		}
		if leftTicks > rightTicks {
			return 1, nil
		}
		return 0, nil
	case bytecode.StringKind:
		leftValue, _ := left.AsString()
		rightValue, _ := right.AsString()
		return strings.Compare(leftValue, rightValue), nil
	default:
		return 0, fmt.Errorf("values of kind %s cannot be ordered", left.Kind())
	}
}

func comparisonRank(kind bytecode.ValueKind) (int, bool) {
	switch kind {
	case bytecode.NullKind:
		return 0, true
	case bytecode.UndefinedKind:
		return 1, true
	case bytecode.BooleanKind:
		return 2, true
	case bytecode.NumberKind:
		return 3, true
	case bytecode.DateKind:
		return 4, true
	case bytecode.StringKind:
		return 5, true
	default:
		return 0, false
	}
}

func valuesEqual(left, right bytecode.Value) bool {
	if left.Kind() != right.Kind() {
		return false
	}
	switch left.Kind() {
	case bytecode.UndefinedKind:
		return true
	case bytecode.NumberKind:
		comparison, _ := bytecode.CompareNumbers(left, right)
		return comparison == 0
	case bytecode.StringKind:
		leftString, _ := left.AsString()
		rightString, _ := right.AsString()
		return leftString == rightString
	case bytecode.BooleanKind:
		leftBoolean, _ := left.AsBoolean()
		rightBoolean, _ := right.AsBoolean()
		return leftBoolean == rightBoolean
	case bytecode.NullKind:
		return true
	case bytecode.DateKind:
		leftTicks, _ := left.DateTicks()
		rightTicks, _ := right.DateTicks()
		return leftTicks == rightTicks
	default:
		return false
	}
}

func runtimeFailure(function *bytecode.Function, instruction bytecode.Instruction, message string) error {
	return &RuntimeError{Function: function.Name, Span: instruction.Span, Message: message}
}

func maxInt() int { return int(^uint(0) >> 1) }
