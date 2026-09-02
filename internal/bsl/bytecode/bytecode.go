// Package bytecode defines the versioned instruction format executed by ML.
package bytecode

import (
	"fmt"
	"strings"

	"github.com/k33alexey/MetaLab/internal/bsl/syntax"
)

// Version changes whenever the bytecode contract becomes incompatible.
const Version uint16 = 1

// Opcode identifies one stack-machine instruction.
type Opcode uint8

const (
	OpConstant Opcode = iota
	OpLoadLocal
	OpNegate
	OpAdd
	OpSubtract
	OpMultiply
	OpDivide
	OpReturn
)

var opcodeNames = [...]string{
	OpConstant: "constant", OpLoadLocal: "load_local", OpNegate: "negate",
	OpAdd: "add", OpSubtract: "subtract", OpMultiply: "multiply",
	OpDivide: "divide", OpReturn: "return",
}

func (opcode Opcode) String() string {
	if int(opcode) >= len(opcodeNames) || opcodeNames[opcode] == "" {
		return fmt.Sprintf("opcode(%d)", opcode)
	}
	return opcodeNames[opcode]
}

// Instruction is one operation and its optional integer operand.
type Instruction struct {
	Opcode  Opcode
	Operand uint16
	Span    syntax.Span
}

// Function is one compiled BSL procedure or function.
type Function struct {
	Name       string
	IsFunction bool
	Export     bool
	Arity      uint16
	MaxStack   uint16
	Constants  []Value
	Code       []Instruction
}

// Program is one deterministic collection of compiled routines.
type Program struct {
	Version   uint16
	Functions []Function
}

// Lookup finds a routine using BSL case-insensitive name matching.
func (program *Program) Lookup(name string) (*Function, bool) {
	for index := range program.Functions {
		if strings.EqualFold(program.Functions[index].Name, name) {
			return &program.Functions[index], true
		}
	}
	return nil, false
}

// Validate rejects malformed or incompatible bytecode before execution.
func (program *Program) Validate() error {
	if program.Version != Version {
		return fmt.Errorf("unsupported bytecode version %d", program.Version)
	}
	names := make(map[string]bool, len(program.Functions))
	for functionIndex := range program.Functions {
		function := &program.Functions[functionIndex]
		name := strings.ToLower(function.Name)
		if name == "" {
			return fmt.Errorf("function %d has empty name", functionIndex)
		}
		if names[name] {
			return fmt.Errorf("duplicate function %q", function.Name)
		}
		names[name] = true
		if err := validateFunction(function); err != nil {
			return fmt.Errorf("function %q: %w", function.Name, err)
		}
	}
	return nil
}

func validateFunction(function *Function) error {
	if len(function.Code) == 0 || function.Code[len(function.Code)-1].Opcode != OpReturn {
		return fmt.Errorf("code must end with return")
	}
	depth := 0
	maximum := 0
	for index, instruction := range function.Code {
		switch instruction.Opcode {
		case OpConstant:
			if int(instruction.Operand) >= len(function.Constants) {
				return fmt.Errorf("instruction %d references constant %d", index, instruction.Operand)
			}
			depth++
		case OpLoadLocal:
			if instruction.Operand >= function.Arity {
				return fmt.Errorf("instruction %d references local %d", index, instruction.Operand)
			}
			depth++
		case OpNegate:
			if depth < 1 {
				return fmt.Errorf("instruction %d underflows stack", index)
			}
		case OpAdd, OpSubtract, OpMultiply, OpDivide:
			if depth < 2 {
				return fmt.Errorf("instruction %d underflows stack", index)
			}
			depth--
		case OpReturn:
			if depth < 1 {
				return fmt.Errorf("instruction %d underflows stack", index)
			}
			depth--
		default:
			return fmt.Errorf("instruction %d has unknown opcode %d", index, instruction.Opcode)
		}
		if depth > maximum {
			maximum = depth
		}
	}
	if depth != 0 {
		return fmt.Errorf("stack depth after code is %d", depth)
	}
	if maximum != int(function.MaxStack) {
		return fmt.Errorf("max stack is %d, declared %d", maximum, function.MaxStack)
	}
	return nil
}
