// Package bytecode defines the versioned instruction format executed by ML.
package bytecode

import (
	"fmt"
	"strings"

	"github.com/k33alexey/MetaLab/internal/bsl/syntax"
)

const maxIndexedItems = 1 << 16

// Version changes whenever the bytecode contract becomes incompatible.
const Version uint16 = 6

// ExecutionContext is the runtime placement of one compiled routine.
type ExecutionContext uint8

const (
	ContextShared ExecutionContext = iota
	ContextClient
	ContextServer
	ContextServerNoContext
	ContextClientServer
	ContextClientServerNoContext
)

// AllowsClient reports whether the routine has a client implementation.
func (context ExecutionContext) AllowsClient() bool {
	return context == ContextShared || context == ContextClient ||
		context == ContextClientServer || context == ContextClientServerNoContext
}

// AllowsServer reports whether the routine has a server implementation.
func (context ExecutionContext) AllowsServer() bool {
	return context == ContextShared || context == ContextServer || context == ContextServerNoContext ||
		context == ContextClientServer || context == ContextClientServerNoContext
}

// ServerWithoutContext reports that a remote server call must not receive client context.
func (context ExecutionContext) ServerWithoutContext() bool {
	return context == ContextServerNoContext || context == ContextClientServerNoContext
}

// CallRoute tells a client VM whether a call crosses the server boundary.
type CallRoute uint8

const (
	CallLocal CallRoute = iota
	CallServer
	CallServerNoContext
)

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
	OpPop
	OpJumpIfTrueKeep
	OpJumpIfFalseKeep
	OpPositive
	OpBoolean
	OpLoadModule
	OpStoreModule
	OpCall
	OpRaise
	OpReraise
	OpErrorDescription
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
	OpPop: "pop", OpJumpIfTrueKeep: "jump_if_true_keep", OpJumpIfFalseKeep: "jump_if_false_keep",
	OpPositive:   "positive",
	OpBoolean:    "boolean",
	OpLoadModule: "load_module", OpStoreModule: "store_module", OpCall: "call",
	OpRaise: "raise", OpReraise: "reraise",
	OpErrorDescription: "error_description",
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

// Parameter defines the call contract stored with a compiled routine.
type Parameter struct {
	ByValue    bool
	HasDefault bool
	Default    Value
}

// ReferenceKind identifies a writable argument source for pass-by-reference.
type ReferenceKind uint8

const (
	NoReference ReferenceKind = iota
	LocalReference
	ModuleReference
)

// VariableReference locates a local or module variable.
type VariableReference struct {
	Kind     ReferenceKind
	Module   uint16
	Variable uint16
}

// CallSite binds one call instruction to its target and writable arguments.
type CallSite struct {
	Target     uint16
	Route      CallRoute
	References []VariableReference
}

// ModuleVariable is one named module storage slot.
type ModuleVariable struct {
	Name   string
	Export bool
}

// Module defines an independently scoped BSL source module.
type Module struct {
	Name      string
	Source    string
	Variables []ModuleVariable
}

// ExceptionHandler maps a protected instruction range to its Except block.
type ExceptionHandler struct {
	Start  uint16
	End    uint16
	Target uint16
}

// Function is one compiled BSL procedure or function.
type Function struct {
	Name       string
	Module     uint16
	IsFunction bool
	Export     bool
	Context    ExecutionContext
	Arity      uint16
	Parameters []Parameter
	LocalCount uint16
	MaxStack   uint16
	Constants  []Value
	CallSites  []CallSite
	ModuleVars []VariableReference
	Exceptions []ExceptionHandler
	Code       []Instruction
}

// Program is one deterministic collection of compiled routines.
type Program struct {
	Version   uint16
	Modules   []Module
	Functions []Function
}

// ClientProgram returns a validated client artifact. Server-only routine bodies
// are replaced by inert stubs while their signatures remain available for RPC.
func (program *Program) ClientProgram() (*Program, error) {
	if program == nil {
		return nil, fmt.Errorf("bytecode program is nil")
	}
	if err := program.Validate(); err != nil {
		return nil, err
	}
	client := &Program{
		Version: program.Version, Modules: make([]Module, len(program.Modules)),
		Functions: make([]Function, len(program.Functions)),
	}
	for index := range program.Modules {
		client.Modules[index] = program.Modules[index]
		client.Modules[index].Variables = append([]ModuleVariable(nil), program.Modules[index].Variables...)
	}
	for index := range program.Functions {
		source := &program.Functions[index]
		target := *source
		target.Parameters = append([]Parameter(nil), source.Parameters...)
		if !source.Context.AllowsClient() {
			target.Constants = []Value{Undefined()}
			target.CallSites = nil
			target.ModuleVars = nil
			target.Exceptions = nil
			target.MaxStack = 1
			target.Code = []Instruction{{Opcode: OpConstant}, {Opcode: OpReturn}}
		} else {
			target.Constants = append([]Value(nil), source.Constants...)
			target.ModuleVars = append([]VariableReference(nil), source.ModuleVars...)
			target.Exceptions = append([]ExceptionHandler(nil), source.Exceptions...)
			target.CallSites = make([]CallSite, len(source.CallSites))
			for callIndex := range source.CallSites {
				target.CallSites[callIndex] = source.CallSites[callIndex]
				target.CallSites[callIndex].References = append([]VariableReference(nil), source.CallSites[callIndex].References...)
			}
			target.Code = append([]Instruction(nil), source.Code...)
		}
		client.Functions[index] = target
	}
	if err := client.Validate(); err != nil {
		return nil, fmt.Errorf("validate client bytecode: %w", err)
	}
	return client, nil
}

// Lookup finds a routine using BSL case-insensitive name matching.
func (program *Program) Lookup(name string) (*Function, bool) {
	var match *Function
	for index := range program.Functions {
		function := &program.Functions[index]
		qualified := function.Name
		if int(function.Module) < len(program.Modules) {
			qualified = program.Modules[function.Module].Name + "." + function.Name
		}
		if strings.EqualFold(qualified, name) {
			return function, true
		}
		if strings.EqualFold(function.Name, name) {
			if match != nil {
				return nil, false
			}
			match = function
		}
	}
	return match, match != nil
}

// LookupInModule finds a routine in one exact module scope.
func (program *Program) LookupInModule(module uint16, name string) (*Function, uint16, bool) {
	for index := range program.Functions {
		if program.Functions[index].Module == module && strings.EqualFold(program.Functions[index].Name, name) {
			return &program.Functions[index], uint16(index), true
		}
	}
	return nil, 0, false
}

// Validate rejects malformed or incompatible bytecode before execution.
func (program *Program) Validate() error {
	if program.Version != Version {
		return fmt.Errorf("unsupported bytecode version %d", program.Version)
	}
	if len(program.Modules) > maxIndexedItems || len(program.Functions) > maxIndexedItems {
		return fmt.Errorf("program exceeds 16-bit module or function index space")
	}
	moduleNames := make(map[string]bool, len(program.Modules))
	for moduleIndex := range program.Modules {
		module := &program.Modules[moduleIndex]
		if len(module.Variables) > maxIndexedItems {
			return fmt.Errorf("module %q exceeds 16-bit variable index space", module.Name)
		}
		name := strings.ToLower(module.Name)
		if name == "" {
			return fmt.Errorf("module %d has empty name", moduleIndex)
		}
		if moduleNames[name] {
			return fmt.Errorf("duplicate module %q", module.Name)
		}
		moduleNames[name] = true
		variables := make(map[string]bool, len(module.Variables))
		for _, variable := range module.Variables {
			variableName := strings.ToLower(variable.Name)
			if variableName == "" || variables[variableName] {
				return fmt.Errorf("module %q has empty or duplicate variable %q", module.Name, variable.Name)
			}
			variables[variableName] = true
		}
	}
	names := make(map[string]bool, len(program.Functions))
	for functionIndex := range program.Functions {
		function := &program.Functions[functionIndex]
		if function.Name == "" {
			return fmt.Errorf("function %d has empty name", functionIndex)
		}
		if function.Context > ContextClientServerNoContext {
			return fmt.Errorf("function %q has unknown execution context %d", function.Name, function.Context)
		}
		if len(program.Modules) != 0 && int(function.Module) >= len(program.Modules) {
			return fmt.Errorf("function %q references module %d", function.Name, function.Module)
		}
		name := fmt.Sprintf("%d:%s", function.Module, strings.ToLower(function.Name))
		if names[name] {
			return fmt.Errorf("duplicate function %q", function.Name)
		}
		names[name] = true
		if err := validateFunction(program, function); err != nil {
			return fmt.Errorf("function %q: %w", function.Name, err)
		}
	}
	return nil
}

func validateFunction(program *Program, function *Function) error {
	if len(function.Code) == 0 || function.Code[len(function.Code)-1].Opcode != OpReturn {
		return fmt.Errorf("code must end with return")
	}
	if function.LocalCount < function.Arity {
		return fmt.Errorf("local count %d is smaller than arity %d", function.LocalCount, function.Arity)
	}
	if len(function.Constants) > maxIndexedItems || len(function.CallSites) > maxIndexedItems ||
		len(function.ModuleVars) > maxIndexedItems || len(function.Exceptions) > maxIndexedItems {
		return fmt.Errorf("routine exceeds 16-bit constant, call, module-access, or exception index space")
	}
	if len(function.Parameters) != 0 && len(function.Parameters) != int(function.Arity) {
		return fmt.Errorf("parameter metadata count %d differs from arity %d", len(function.Parameters), function.Arity)
	}
	optional := false
	for index, parameter := range function.Parameters {
		if parameter.HasDefault {
			optional = true
			switch parameter.Default.Kind() {
			case UndefinedKind, NumberKind, StringKind, BooleanKind, NullKind, DateKind:
			default:
				return fmt.Errorf("parameter %d has unsupported default kind %s", index, parameter.Default.Kind())
			}
		} else if optional {
			return fmt.Errorf("required parameter %d follows an optional parameter", index)
		}
	}
	for index, call := range function.CallSites {
		if int(call.Target) >= len(program.Functions) {
			return fmt.Errorf("call site %d references function %d", index, call.Target)
		}
		target := &program.Functions[call.Target]
		expectedRoute := CallLocal
		if !target.Context.AllowsClient() {
			if target.Context.ServerWithoutContext() {
				expectedRoute = CallServerNoContext
			} else {
				expectedRoute = CallServer
			}
		}
		if call.Route != expectedRoute {
			return fmt.Errorf("call site %d has route %d, expected %d", index, call.Route, expectedRoute)
		}
		if target.Module != function.Module && !target.Export {
			return fmt.Errorf("call site %d accesses non-exported routine %q", index, target.Name)
		}
		if len(call.References) != int(target.Arity) {
			return fmt.Errorf("call site %d has %d references for arity %d", index, len(call.References), target.Arity)
		}
		for referenceIndex, reference := range call.References {
			if len(target.Parameters) != 0 && target.Parameters[referenceIndex].ByValue && reference.Kind != NoReference {
				return fmt.Errorf("call site %d passes reference to by-value parameter %d", index, referenceIndex)
			}
			if err := validateReference(program, function, reference); err != nil {
				return fmt.Errorf("call site %d: %w", index, err)
			}
		}
	}
	for index, reference := range function.ModuleVars {
		if reference.Kind != ModuleReference {
			return fmt.Errorf("module variable access %d is not a module reference", index)
		}
		if err := validateReference(program, function, reference); err != nil {
			return fmt.Errorf("module variable access %d: %w", index, err)
		}
	}
	for index, handler := range function.Exceptions {
		if handler.Start > handler.End || int(handler.End) > len(function.Code) {
			return fmt.Errorf("exception handler %d has invalid protected range %d..%d", index, handler.Start, handler.End)
		}
		if int(handler.Target) >= len(function.Code) || handler.Target < handler.End {
			return fmt.Errorf("exception handler %d has invalid target %d", index, handler.Target)
		}
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
		case OpLoadModule, OpStoreModule:
			if int(instruction.Operand) >= len(function.ModuleVars) {
				return fmt.Errorf("instruction %d references module variable access %d", index, instruction.Operand)
			}
		case OpCall:
			if int(instruction.Operand) >= len(function.CallSites) {
				return fmt.Errorf("instruction %d references call site %d", index, instruction.Operand)
			}
		case OpJump, OpJumpIfFalse, OpJumpIfTrueKeep, OpJumpIfFalseKeep:
			if int(instruction.Operand) >= len(function.Code) {
				return fmt.Errorf("instruction %d jumps outside code to %d", index, instruction.Operand)
			}
		case OpReraise:
			if int(instruction.Operand) >= len(function.Exceptions) {
				return fmt.Errorf("instruction %d references exception handler %d", index, instruction.Operand)
			}
		case OpErrorDescription:
			if int(instruction.Operand) >= len(function.Exceptions) {
				return fmt.Errorf("instruction %d references exception handler %d", index, instruction.Operand)
			}
		case OpNegate, OpNot, OpArrayLength, OpPositive, OpBoolean, OpPop, OpRaise, OpAdd, OpSubtract, OpMultiply, OpDivide, OpModulo,
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
			required, delta := stackEffect(program, function, instruction)
			if depth < required {
				return fmt.Errorf("instruction %d underflows stack", index)
			}
			if depth > maximum {
				maximum = depth
			}
			nextDepth := depth + delta
			if instruction.Opcode == OpReturn || instruction.Opcode == OpRaise || instruction.Opcode == OpReraise {
				if nextDepth != 0 {
					return fmt.Errorf("instruction %d returns with stack depth %d", index, nextDepth)
				}
				continue
			}

			successors := [2]int{index + 1, -1}
			if instruction.Opcode == OpJump {
				successors[0] = int(instruction.Operand)
			} else if instruction.Opcode == OpJumpIfFalse || instruction.Opcode == OpJumpIfTrueKeep || instruction.Opcode == OpJumpIfFalseKeep {
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

func stackEffect(program *Program, function *Function, instruction Instruction) (required, delta int) {
	switch instruction.Opcode {
	case OpConstant, OpLoadLocal, OpErrorDescription:
		return 0, 1
	case OpLoadModule:
		return 0, 1
	case OpStoreLocal, OpStoreModule, OpJumpIfFalse, OpPop, OpReturn, OpRaise:
		return 1, -1
	case OpNegate, OpNot, OpArrayLength, OpPositive, OpBoolean, OpJumpIfTrueKeep, OpJumpIfFalseKeep:
		return 1, 0
	case OpJump, OpReraise:
		return 0, 0
	case OpCall:
		call := function.CallSites[instruction.Operand]
		arity := int(program.Functions[call.Target].Arity)
		return arity, 1 - arity
	case OpAdd, OpSubtract, OpMultiply, OpDivide, OpModulo,
		OpEqual, OpNotEqual, OpLess, OpLessEqual, OpGreater, OpGreaterEqual,
		OpAnd, OpOr, OpArrayElement:
		return 2, -1
	default:
		return 0, 0
	}
}

func validateReference(program *Program, function *Function, reference VariableReference) error {
	switch reference.Kind {
	case NoReference:
		return nil
	case LocalReference:
		if reference.Variable >= function.LocalCount {
			return fmt.Errorf("reference uses local %d", reference.Variable)
		}
		return nil
	case ModuleReference:
		if int(reference.Module) >= len(program.Modules) {
			return fmt.Errorf("reference uses module %d", reference.Module)
		}
		if int(reference.Variable) >= len(program.Modules[reference.Module].Variables) {
			return fmt.Errorf("reference uses variable %d of module %d", reference.Variable, reference.Module)
		}
		if reference.Module != function.Module && !program.Modules[reference.Module].Variables[reference.Variable].Export {
			return fmt.Errorf("reference uses non-exported variable %q", program.Modules[reference.Module].Variables[reference.Variable].Name)
		}
		return nil
	default:
		return fmt.Errorf("reference has unknown kind %d", reference.Kind)
	}
}
