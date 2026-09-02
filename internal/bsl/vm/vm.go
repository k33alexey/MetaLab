// Package vm executes validated MetaLab BSL bytecode.
package vm

import (
	"fmt"

	"github.com/k33alexey/MetaLab/internal/bsl/bytecode"
	"github.com/k33alexey/MetaLab/internal/bsl/syntax"
)

const inlineStackSize = 64

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

	for _, instruction := range function.Code {
		switch instruction.Opcode {
		case bytecode.OpConstant:
			push(function.Constants[instruction.Operand])
		case bytecode.OpLoadLocal:
			push(arguments[instruction.Operand])
		case bytecode.OpNegate:
			value := pop()
			number, ok := value.AsNumber()
			if !ok {
				return bytecode.Undefined(), runtimeFailure(function, instruction, "unary minus requires a number")
			}
			push(bytecode.Number(-number))
		case bytecode.OpAdd, bytecode.OpSubtract, bytecode.OpMultiply, bytecode.OpDivide:
			right := pop()
			left := pop()
			result, err := binary(instruction.Opcode, left, right)
			if err != nil {
				return bytecode.Undefined(), runtimeFailure(function, instruction, err.Error())
			}
			push(result)
		case bytecode.OpReturn:
			return pop(), nil
		default:
			return bytecode.Undefined(), runtimeFailure(function, instruction, "unknown opcode")
		}
	}
	return bytecode.Undefined(), fmt.Errorf("routine %q ended without return", function.Name)
}

func binary(opcode bytecode.Opcode, left, right bytecode.Value) (bytecode.Value, error) {
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
	default:
		return bytecode.Undefined(), fmt.Errorf("unsupported binary opcode %s", opcode)
	}
}

func runtimeFailure(function *bytecode.Function, instruction bytecode.Instruction, message string) error {
	return &RuntimeError{Function: function.Name, Span: instruction.Span, Message: message}
}
