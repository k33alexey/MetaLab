package metadata

import (
	"fmt"
	"slices"

	"github.com/k33alexey/MetaLab/internal/uuid"
)

// Which registers a document writes into is said once, on the document. The
// prototype says it there and nowhere else: not one of the four kinds of
// register has a property that lists its documents, while eleven of the
// twenty-five documents of the demonstration configuration name their
// registers outright, across all four kinds.
//
// The question "who writes into this register" is still asked - by the record
// store checking a recorder, by the form offering a command, by the deletion
// check looking for references. It is answered by reading the documents rather
// than by keeping the answer twice.

// DocumentMovementTarget is one register a document writes into, resolved
// against the configuration.
type DocumentMovementTarget struct {
	Kind Kind
	ID   uuid.UUID
	Name string
}

// movementRegister finds a register by identifier among the four kinds that
// keep records of documents. The second result is false when no register of
// any of them has that identifier.
func (catalog *Catalog) movementRegister(id uuid.UUID) (DocumentMovementTarget, bool) {
	if index, ok := catalog.informationRegisterByID[id]; ok {
		return DocumentMovementTarget{Kind: InformationRegisterKind, ID: id, Name: catalog.InformationRegisters[index].Name}, true
	}
	if index, ok := catalog.accumulationRegisterByID[id]; ok {
		return DocumentMovementTarget{Kind: AccumulationRegisterKind, ID: id, Name: catalog.AccumulationRegisters[index].Name}, true
	}
	if index, ok := catalog.accountingRegisterByID[id]; ok {
		return DocumentMovementTarget{Kind: AccountingRegisterKind, ID: id, Name: catalog.AccountingRegisters[index].Name}, true
	}
	if index, ok := catalog.calculationRegisterByID[id]; ok {
		return DocumentMovementTarget{Kind: CalculationRegisterKind, ID: id, Name: catalog.CalculationRegisters[index].Name}, true
	}
	return DocumentMovementTarget{}, false
}

// DocumentMovements are the registers one document writes into, in the order
// the document names them. A register that is not in the configuration is left
// out rather than guessed at; loading refuses such a document, so this can
// only happen to a catalog assembled by hand.
func (catalog *Catalog) DocumentMovements(document uuid.UUID) []DocumentMovementTarget {
	movements := catalog.documentMovements(document)
	result := make([]DocumentMovementTarget, 0, len(movements))
	for _, id := range movements {
		if target, ok := catalog.movementRegister(id); ok {
			result = append(result, target)
		}
	}
	return result
}

// RegisterRecorders are the documents that write into one register. It reads
// the documents every time rather than keeping a second copy of the link: a
// copy is a thing that can disagree with the original, and the work here is a
// walk over a list that the caller is about to follow with a round trip to the
// database anyway.
func (catalog *Catalog) RegisterRecorders(register uuid.UUID) []uuid.UUID {
	var result []uuid.UUID
	for _, document := range catalog.Documents {
		if slices.Contains(document.Movements, register) {
			result = append(result, document.ID)
		}
	}
	return result
}

// writesInto is the same question about one document, which is what a record
// being written actually asks.
func (catalog *Catalog) writesInto(document, register uuid.UUID) bool {
	return slices.Contains(catalog.documentMovements(document), register)
}

// documentMovements finds one document's registers by walking the documents.
// It does not go through the identifier index, because a catalog assembled by
// hand - a test, the role editor - may carry documents without it, and an
// answer that silently becomes "writes nowhere" is worse than a walk.
func (catalog *Catalog) documentMovements(document uuid.UUID) []uuid.UUID {
	for _, item := range catalog.Documents {
		if item.ID == document {
			return item.Movements
		}
	}
	return nil
}

// validateDocumentMovements resolves what every document writes into. Two
// things are refused, and each of them is a document that would fail only at
// the moment it is posted: a register that is not in the configuration, and an
// information register written independently, which has no recorder column for
// a document's records to live in.
func (catalog *Catalog) validateDocumentMovements() error {
	for _, document := range catalog.Documents {
		for index, id := range document.Movements {
			target, ok := catalog.movementRegister(id)
			if !ok {
				return fmt.Errorf("document %s writes into %s, which is not a register of the configuration", document.Name, id)
			}
			if target.Kind != InformationRegisterKind {
				continue
			}
			register := catalog.InformationRegisters[catalog.informationRegisterByID[id]]
			if register.WriteMode != InformationRegisterRecorder {
				return fmt.Errorf("document %s writes into information register %s, which is written independently and keeps no recorder (movements[%d])",
					document.Name, register.Name, index)
			}
		}
	}
	return nil
}

// ValidateForDatabase checks what the prototype checks when the configuration
// is saved into the database, and not before: a register of accumulation,
// calculation or accounting that no document writes into has no recorder,
// and the designer refuses to save it (checked by the owner on the platform,
// 01.10.2026 - obligatory for all three). An information register is not
// among them: it may be written independently.
//
// It is not part of reading the project, on purpose. A developer creates the
// register first and the document after, and a project that refused to open
// in between could not be given the document that mends it. The prototype is
// the same: the register is created freely, and saving it into the base is
// what fails.
func (catalog *Catalog) ValidateForDatabase() error {
	written := map[uuid.UUID]bool{}
	for _, document := range catalog.Documents {
		for _, id := range document.Movements {
			written[id] = true
		}
	}
	check := func(kind string, id uuid.UUID, name string) error {
		if !written[id] {
			return fmt.Errorf("%s %s has no recorder: no document writes into it", kind, name)
		}
		return nil
	}
	for _, item := range catalog.AccumulationRegisters {
		if err := check("accumulation register", item.ID, item.Name); err != nil {
			return err
		}
	}
	for _, item := range catalog.CalculationRegisters {
		if err := check("calculation register", item.ID, item.Name); err != nil {
			return err
		}
	}
	for _, item := range catalog.AccountingRegisters {
		if err := check("accounting register", item.ID, item.Name); err != nil {
			return err
		}
	}
	return nil
}
