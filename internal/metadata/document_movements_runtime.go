package metadata

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/k33alexey/MetaLab/internal/bsl/bytecode"
)

// documentMovementsObject exposes recorder-owned register sets as
// ThisObject.Movements.<Register> / ЭтотОбъект.Движения.<Регистр>.
type documentMovementsObject struct {
	runtime      *Runtime
	recorder     DocumentReference
	period       time.Time
	information  map[string]*informationRegisterRecordSetObject
	accumulation map[string]*accumulationRegisterRecordSetObject
}

func (*documentMovementsObject) RuntimeTypeName() string { return "DocumentMovements" }
func (object *documentMovementsObject) RuntimeEqual(other bytecode.RuntimeObject) bool {
	candidate, ok := other.(*documentMovementsObject)
	return ok && candidate == object
}
func (object *documentMovementsObject) RuntimeDynamicMemory(limit uint64) (uint64, bool) {
	size := uint64(512)
	for _, set := range object.information {
		if size > limit {
			return limit, false
		}
		current, ok := set.RuntimeDynamicMemory(limit - size)
		if !ok {
			return limit, false
		}
		size += current
	}
	for _, set := range object.accumulation {
		if size > limit {
			return limit, false
		}
		current, ok := set.RuntimeDynamicMemory(limit - size)
		if !ok {
			return limit, false
		}
		size += current
	}
	return size, size <= limit
}

func (runtime *Runtime) newDocumentMovements(definition DocumentDefinition, record *DocumentRecord) (*documentMovementsObject, error) {
	if record == nil || record.Reference.DocumentID != definition.ID || record.Reference.ObjectID.IsZero() {
		return nil, fmt.Errorf("document movements require a valid recorder")
	}
	result := &documentMovementsObject{
		runtime: runtime, recorder: record.Reference, period: record.Date,
		information: map[string]*informationRegisterRecordSetObject{}, accumulation: map[string]*accumulationRegisterRecordSetObject{},
	}
	for _, register := range runtime.catalog.InformationRegisters {
		if register.WriteMode != InformationRegisterRecorder || !allowedInformationRegisterRecorder(register, record.Reference) {
			continue
		}
		if runtime.informationRegisterRepository == nil {
			return nil, fmt.Errorf("information register repository is not configured for document movements")
		}
		set, err := runtime.informationRegisterRepository.NewRecordSet(register.Name)
		if err != nil {
			return nil, err
		}
		recorder := record.Reference
		set.Filter.Recorder = &recorder
		result.information[strings.ToLower(register.Name)] = &informationRegisterRecordSetObject{
			definition: register, set: set, runtime: runtime, defaultRecorder: &recorder, defaultPeriod: record.Date,
		}
	}
	for _, register := range runtime.catalog.AccumulationRegisters {
		if !allowedAccumulationRegisterRecorder(register, record.Reference) {
			continue
		}
		if runtime.accumulationRegisterRepository == nil {
			return nil, fmt.Errorf("accumulation register repository is not configured for document movements")
		}
		set, err := runtime.accumulationRegisterRepository.NewRecordSet(register.Name)
		if err != nil {
			return nil, err
		}
		recorder := record.Reference
		set.Filter.Recorder = &recorder
		result.accumulation[strings.ToLower(register.Name)] = &accumulationRegisterRecordSetObject{
			definition: register, set: set, runtime: runtime, defaultRecorder: &recorder, defaultPeriod: record.Date,
		}
	}
	return result, nil
}

func (object *documentMovementsObject) setDocument(recorder DocumentReference, period time.Time) {
	object.recorder, object.period = recorder, period
	for _, set := range object.information {
		set.mu.Lock()
		copy := recorder
		set.defaultRecorder, set.defaultPeriod, set.set.Filter.Recorder = &copy, period, &copy
		set.mu.Unlock()
	}
	for _, set := range object.accumulation {
		set.mu.Lock()
		copy := recorder
		set.defaultRecorder, set.defaultPeriod, set.set.Filter.Recorder = &copy, period, &copy
		set.mu.Unlock()
	}
}

func (runtime *Runtime) getDocumentMovementsProperty(value bytecode.RuntimeObject, name string) (bytecode.Value, bool, error) {
	object, ok := value.(*documentMovementsObject)
	if !ok {
		return bytecode.Undefined(), false, nil
	}
	if object.runtime != runtime {
		return bytecode.Undefined(), true, fmt.Errorf("document movements belong to another metadata runtime")
	}
	folded := strings.ToLower(name)
	if set, ok := object.information[folded]; ok {
		result, err := bytecode.Object(set)
		return result, true, err
	}
	if set, ok := object.accumulation[folded]; ok {
		result, err := bytecode.Object(set)
		return result, true, err
	}
	return bytecode.Undefined(), true, fmt.Errorf("document movements have no register %s", name)
}

func (runtime *Runtime) setDocumentMovementsProperty(value bytecode.RuntimeObject, name string, _ bytecode.Value) (bool, error) {
	object, ok := value.(*documentMovementsObject)
	if !ok {
		return false, nil
	}
	if object.runtime != runtime {
		return true, fmt.Errorf("document movements belong to another metadata runtime")
	}
	return true, fmt.Errorf("document movement register %s is read-only", name)
}

func (runtime *Runtime) callDocumentMovementsMethod(value bytecode.RuntimeObject, name string, _ []bytecode.Value) (bytecode.Value, bool, error) {
	object, ok := value.(*documentMovementsObject)
	if !ok {
		return bytecode.Undefined(), false, nil
	}
	if object.runtime != runtime {
		return bytecode.Undefined(), true, fmt.Errorf("document movements belong to another metadata runtime")
	}
	return bytecode.Undefined(), true, fmt.Errorf("DocumentMovements has no method %s", name)
}

func (runtime *Runtime) writeDocumentMovements(ctx context.Context, object *documentMovementsObject) error {
	if object == nil {
		return nil
	}
	if object.runtime != runtime {
		return fmt.Errorf("document movements belong to another metadata runtime")
	}
	for _, definition := range runtime.catalog.InformationRegisters {
		set := object.information[strings.ToLower(definition.Name)]
		if set == nil {
			continue
		}
		set.mu.RLock()
		write, working := set.writeAtEnd, cloneInformationRegisterRecordSet(set.set)
		set.mu.RUnlock()
		if !write {
			continue
		}
		if err := runtime.informationRegisterRepository.WriteWithHandler(ctx, working, true, runtime.informationRegisterEventHandler(definition.ID)); err != nil {
			return fmt.Errorf("write document movements to information register %s: %w", definition.Name, err)
		}
		set.mu.Lock()
		set.set = working
		set.mu.Unlock()
	}
	for _, definition := range runtime.catalog.AccumulationRegisters {
		set := object.accumulation[strings.ToLower(definition.Name)]
		if set == nil {
			continue
		}
		set.mu.RLock()
		write, working := set.writeAtEnd, cloneAccumulationRegisterRecordSet(set.set)
		set.mu.RUnlock()
		if !write {
			continue
		}
		if err := runtime.accumulationRegisterRepository.WriteWithHandler(ctx, working, true, runtime.accumulationRegisterEventHandler(definition.ID)); err != nil {
			return fmt.Errorf("write document movements to accumulation register %s: %w", definition.Name, err)
		}
		set.mu.Lock()
		set.set = working
		set.mu.Unlock()
	}
	return nil
}

var _ bytecode.RuntimeObject = (*documentMovementsObject)(nil)
