// Package vm executes validated MetaLab BSL bytecode.
package vm

import (
	"fmt"
	"math"

	"github.com/k33alexey/MetaLab/internal/bsl/bytecode"
	"github.com/k33alexey/MetaLab/internal/bsl/syntax"
)

const (
	inlineStackSize = 64
	inlineLocalSize = 16
)

// Machine is an immutable, concurrency-safe bytecode runtime.
type Machine struct {
	program *bytecode.Program
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
	clone := &bytecode.Program{Version: program.Version, Functions: make([]bytecode.Function, len(program.Functions))}
	for index := range program.Functions {
		clone.Functions[index] = program.Functions[index]
		clone.Functions[index].Constants = append([]bytecode.Value(nil), program.Functions[index].Constants...)
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

// Call executes a named BSL routine with positional arguments.
func (machine *Machine) Call(name string, arguments ...bytecode.Value) (bytecode.Value, error) {
	function, ok := machine.program.Lookup(name)
	if !ok {
		return bytecode.Undefined(), fmt.Errorf("routine %q not found", name)
	}
	if len(arguments) != int(function.Arity) {
		return bytecode.Undefined(), fmt.Errorf(
			"routine %q expects %d arguments, got %d", function.Name, function.Arity, len(arguments),
		)
	}
	return execute(function, arguments)
}

func execute(function *bytecode.Function, arguments []bytecode.Value) (bytecode.Value, error) {
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
		case bytecode.OpNegate:
			value := pop()
			number, ok := value.AsNumber()
			if !ok {
				return bytecode.Undefined(), runtimeFailure(function, instruction, "unary minus requires a number")
			}
			push(bytecode.Number(-number))
		case bytecode.OpNot:
			value := pop()
			boolean, ok := value.AsBoolean()
			if !ok {
				return bytecode.Undefined(), runtimeFailure(function, instruction, "Not requires a boolean")
			}
			push(bytecode.Boolean(!boolean))
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
			index, ok := indexValue.AsNumber()
			if !ok || index < 0 || index != math.Trunc(index) || index > float64(maxInt()) {
				return bytecode.Undefined(), runtimeFailure(function, instruction, "array index must be a non-negative integer")
			}
			element, ok := array.ArrayElement(int(index))
			if !ok {
				return bytecode.Undefined(), runtimeFailure(function, instruction, "array index is out of range")
			}
			push(element)
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
		case bytecode.OpReturn:
			return pop(), nil
		default:
			return bytecode.Undefined(), runtimeFailure(function, instruction, "unknown opcode")
		}
	}
	return bytecode.Undefined(), fmt.Errorf("routine %q ended without return", function.Name)
}

func binary(opcode bytecode.Opcode, left, right bytecode.Value) (bytecode.Value, error) {
	if opcode == bytecode.OpEqual || opcode == bytecode.OpNotEqual {
		equal := valuesEqual(left, right)
		if opcode == bytecode.OpNotEqual {
			equal = !equal
		}
		return bytecode.Boolean(equal), nil
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
		leftString, leftIsString := left.AsString()
		rightString, rightIsString := right.AsString()
		if leftIsString && rightIsString {
			return bytecode.String(leftString + rightString), nil
		}
	}
	leftNumber, leftIsNumber := left.AsNumber()
	rightNumber, rightIsNumber := right.AsNumber()
	if !leftIsNumber || !rightIsNumber {
		return bytecode.Undefined(), fmt.Errorf("%s requires two numbers", opcode)
	}
	switch opcode {
	case bytecode.OpAdd:
		return bytecode.Number(leftNumber + rightNumber), nil
	case bytecode.OpSubtract:
		return bytecode.Number(leftNumber - rightNumber), nil
	case bytecode.OpMultiply:
		return bytecode.Number(leftNumber * rightNumber), nil
	case bytecode.OpDivide:
		if rightNumber == 0 {
			return bytecode.Undefined(), fmt.Errorf("division by zero")
		}
		return bytecode.Number(leftNumber / rightNumber), nil
	case bytecode.OpModulo:
		if rightNumber == 0 {
			return bytecode.Undefined(), fmt.Errorf("division by zero")
		}
		return bytecode.Number(math.Mod(leftNumber, rightNumber)), nil
	case bytecode.OpLess:
		return bytecode.Boolean(leftNumber < rightNumber), nil
	case bytecode.OpLessEqual:
		return bytecode.Boolean(leftNumber <= rightNumber), nil
	case bytecode.OpGreater:
		return bytecode.Boolean(leftNumber > rightNumber), nil
	case bytecode.OpGreaterEqual:
		return bytecode.Boolean(leftNumber >= rightNumber), nil
	default:
		return bytecode.Undefined(), fmt.Errorf("unsupported binary opcode %s", opcode)
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
		leftNumber, _ := left.AsNumber()
		rightNumber, _ := right.AsNumber()
		return leftNumber == rightNumber
	case bytecode.StringKind:
		leftString, _ := left.AsString()
		rightString, _ := right.AsString()
		return leftString == rightString
	case bytecode.BooleanKind:
		leftBoolean, _ := left.AsBoolean()
		rightBoolean, _ := right.AsBoolean()
		return leftBoolean == rightBoolean
	default:
		return false
	}
}

func runtimeFailure(function *bytecode.Function, instruction bytecode.Instruction, message string) error {
	return &RuntimeError{Function: function.Name, Span: instruction.Span, Message: message}
}

func maxInt() int { return int(^uint(0) >> 1) }
