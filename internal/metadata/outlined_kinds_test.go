package metadata

import (
	"strings"
	"testing"
)

// An object of a kind described by its branch alone is read, named and counted.
// Until now the branch stood in the tree and the file in it was passed over in
// silence - the one outcome the conformance report calls unacceptable.
func TestOutlinedObjectsAreReadAndNotPassedOver(t *testing.T) {
	t.Parallel()
	root := metadataProject(t)
	identifiers := map[Kind]string{
		BotKind:                "f5000000-0000-4000-8000-000000000001",
		WSReferenceKind:        "f5000000-0000-4000-8000-000000000002",
		WebSocketClientKind:    "f5000000-0000-4000-8000-000000000003",
		IntegrationServiceKind: "f5000000-0000-4000-8000-000000000004",
		ExternalDataSourceKind: "f5000000-0000-4000-8000-000000000005",
	}
	for kind, id := range identifiers {
		writeMetadata(t, root, kind, id, `format: 1
id: `+id+`
name: Объект`+strings.ReplaceAll(string(kind), "-", "")+`
title: {ru: Объект}
comment: Перенесён, состав свойств не развёрнут
`)
	}

	catalog, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(catalog.OutlinedObjects) != len(identifiers) {
		t.Fatalf("an object was passed over: %+v", catalog.OutlinedObjects)
	}
	for kind := range identifiers {
		objects := catalog.OutlinedObjectsOf(kind)
		if len(objects) != 1 || objects[0].Comment == "" {
			t.Fatalf("the object of kind %s came back as %+v", kind, objects)
		}
	}
}

// Reading is strict, and that is the point: a property we have never seen stops
// the import with its name in the message. Loud and wrong beats quiet and lost.
func TestAnUnknownPropertyOfAnOutlinedObjectIsReportedNotIgnored(t *testing.T) {
	t.Parallel()
	root := metadataProject(t)
	const id = "f5000000-0000-4000-8000-000000000010"
	writeMetadata(t, root, BotKind, id, `format: 1
id: `+id+`
name: Помощник
title: {ru: Помощник}
channel: telegram
`)
	_, err := Load(root)
	if err == nil {
		t.Fatal("a property nobody described was accepted")
	}
	if !strings.Contains(err.Error(), "channel") {
		t.Fatalf("the error does not name the property: %v", err)
	}
}

// Two objects of one kind cannot share a name, the same as everywhere else.
func TestTwoOutlinedObjectsOfOneKindCannotShareAName(t *testing.T) {
	t.Parallel()
	root := metadataProject(t)
	for _, id := range []string{"f5000000-0000-4000-8000-000000000020", "f5000000-0000-4000-8000-000000000021"} {
		writeMetadata(t, root, IntegrationServiceKind, id, `format: 1
id: `+id+`
name: Обмен
title: {ru: Обмен}
`)
	}
	_, err := Load(root)
	if err == nil {
		t.Fatal("two objects of one kind sharing a name were accepted")
	}
	if !strings.Contains(err.Error(), "share a name") {
		t.Fatalf("the error does not say what is wrong: %v", err)
	}
}
