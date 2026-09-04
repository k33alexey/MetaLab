// Package bytecode defines the versioned instruction format executed by ML.
package bytecode

import (
	"fmt"
	"strings"

	"github.com/k33alexey/MetaLab/internal/bsl/syntax"
)

// Version changes whenever the bytecode contract becomes incompatible.
const Version uint16 = 2

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
	OpStoreLocal
	OpNot
	OpModulo
	OpEqual
	OpNotEqual
	OpLess
	OpLessEqual
	OpGreater
	OpGreaterEqual
	OpJump
	OpJumpIfFalse
	OpAnd
	OpOr
	OpArrayLength
	OpArrayElement
)

var opcodeNames = [...]string{
	OpConstant: "constant", OpLoadLocal: "load_local", OpNegate: "negate",
	OpAdd: "add", OpSubtract: "subtract", OpMultiply: "multiply",
	OpDivide: "divide", OpReturn: "return",
	OpStoreLocal: "store_local", OpNot: "not", OpModulo: "modulo",
	OpEqual: "equal", OpNotEqual: "not_equal", OpLess: "less",
	OpLessEqual: "less_equal", OpGreater: "greater", OpGreaterEqual: "greater_equal",
	OpJump: "jump", OpJumpIfFalse: "jump_if_false",
	OpAnd: "and", OpOr: "or",
	OpArrayLength: "array_length", OpArrayElement: "array_element",
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
	LocalCount uint16
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
	if function.LocalCount < function.Arity {
		return fmt.Errorf("local count %d is smaller than arity %d", function.LocalCount, function.Arity)
	}
	for index, instruction := range function.Code {
		switch instruction.Opcode {
		case OpConstant:
			if int(instruction.Operand) >= len(function.Constants) {
				return fmt.Errorf("instruction %d references constant %d", index, instruction.Operand)
			}
		case OpLoadLocal, OpStoreLocal:
			if instruction.Operand >= function.LocalCount {
				return fmt.Errorf("instruction %d references local %d", index, instruction.Operand)
			}
		case OpJump, OpJumpIfFalse:
			if int(instruction.Operand) >= len(function.Code) {
				return fmt.Errorf("instruction %d jumps outside code to %d", index, instruction.Operand)
			}
		case OpNegate, OpNot, OpArrayLength, OpAdd, OpSubtract, OpMultiply, OpDivide, OpModulo,
			OpEqual, OpNotEqual, OpLess, OpLessEqual, OpGreater, OpGreaterEqual,
			OpAnd, OpOr, OpArrayElement, OpReturn:
		default:
			return fmt.Errorf("instruction %d has unknown opcode %d", index, instruction.Opcode)
		}
	}

	depthAt := make([]int, len(function.Code))
	for index := range depthAt {
		depthAt[index] = -1
	}
	maximum := 0
	for seed := range function.Code {
		if depthAt[seed] >= 0 {
			continue
		}
		depthAt[seed] = 0
		work := []int{seed}
		for len(work) != 0 {
			index := work[len(work)-1]
			work = work[:len(work)-1]
			instruction := function.Code[index]
			depth := depthAt[index]
			required, delta := stackEffect(instruction.Opcode)
			if depth < required {
				return fmt.Errorf("instruction %d underflows stack", index)
			}
			if depth > maximum {
				maximum = depth
			}
			nextDepth := depth + delta
			if instruction.Opcode == OpReturn {
				if nextDepth != 0 {
					return fmt.Errorf("instruction %d returns with stack depth %d", index, nextDepth)
				}
				continue
			}

			successors := [2]int{index + 1, -1}
			if instruction.Opcode == OpJump {
				successors[0] = int(instruction.Operand)
			} else if instruction.Opcode == OpJumpIfFalse {
				successors[1] = int(instruction.Operand)
			}
			for _, successor := range successors {
				if successor < 0 {
					continue
				}
				if successor >= len(function.Code) {
					return fmt.Errorf("instruction %d falls outside code", index)
				}
				if depthAt[successor] == -1 {
					depthAt[successor] = nextDepth
					work = append(work, successor)
				} else if depthAt[successor] != nextDepth {
					return fmt.Errorf("instruction %d has conflicting stack depths %d and %d", successor, depthAt[successor], nextDepth)
				}
			}
		}
	}
	if maximum != int(function.MaxStack) {
		return fmt.Errorf("max stack is %d, declared %d", maximum, function.MaxStack)
	}
	return nil
}

func stackEffect(opcode Opcode) (required, delta int) {
	switch opcode {
	case OpConstant, OpLoadLocal:
		return 0, 1
	case OpStoreLocal, OpJumpIfFalse, OpReturn:
		return 1, -1
	case OpNegate, OpNot, OpArrayLength:
		return 1, 0
	case OpJump:
		return 0, 0
	case OpAdd, OpSubtract, OpMultiply, OpDivide, OpModulo,
		OpEqual, OpNotEqual, OpLess, OpLessEqual, OpGreater, OpGreaterEqual,
		OpAnd, OpOr, OpArrayElement:
		return 2, -1
	default:
		return 0, 0
	}
}
