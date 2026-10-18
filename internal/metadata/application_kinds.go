package metadata

import (
	"reflect"
	"strings"

	"github.com/k33alexey/MetaLab/internal/uuid"
)

// ObjectHead names an object of the configuration in a list of objects of
// several kinds: its identifier, its name and its synonym.
type ObjectHead struct {
	ID    uuid.UUID
	Name  string
	Title LocalizedText
}

// ApplicationKinds are the kinds whose objects ML App opens on a screen
// (kindInfo.application), in the order the model reads the kinds. The
// navigation lists them, and a request for any other kind is refused.
func ApplicationKinds() []Kind {
	var result []Kind
	for _, item := range kindInfos {
		if item.application {
			result = append(result, item.kind)
		}
	}
	return result
}

// OpensInApplication says ML App opens objects of the kind.
func OpensInApplication(kind Kind) bool {
	item, ok := kindInfoOf(kind)
	return ok && item.application
}

// ObjectHeads lists the objects of a kind in the order the catalog keeps
// them; none for a kind the catalog keeps no list of.
func (catalog *Catalog) ObjectHeads(kind Kind) []ObjectHead {
	list, ok := catalog.objectsOf(kind)
	if !ok {
		return nil
	}
	result := make([]ObjectHead, 0, list.Len())
	for index := range list.Len() {
		result = append(result, objectHead(list.Index(index)))
	}
	return result
}

// ObjectHead finds an object of a kind by its name, in any case, as the
// definitions of a kind are found by name (CatalogDefinition).
func (catalog *Catalog) ObjectHead(kind Kind, name string) (ObjectHead, bool) {
	list, ok := catalog.objectsOf(kind)
	if !ok {
		return ObjectHead{}, false
	}
	wanted := strings.ToLower(name)
	for index := range list.Len() {
		if head := objectHead(list.Index(index)); strings.ToLower(head.Name) == wanted {
			return head, true
		}
	}
	return ObjectHead{}, false
}

// objectHead reads the identifier, the name and the synonym every
// description of an object has (kindInfo.described: a test requires ID and
// Name of each).
func objectHead(object reflect.Value) ObjectHead {
	head := ObjectHead{Name: nameOf(object)}
	if id, ok := object.FieldByName("ID").Interface().(uuid.UUID); ok {
		head.ID = id
	}
	if title := object.FieldByName("Title"); title.IsValid() {
		head.Title, _ = title.Interface().(LocalizedText)
	}
	return head
}
