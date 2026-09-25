package metadata

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const (
	firstXDTOPackage  = "f2000000-0000-4000-8000-000000000001"
	secondXDTOPackage = "f2000000-0000-4000-8000-000000000002"
)

// writeXDTOPackage writes one package: its description, and the schema beside
// it when the test asks for one.
func writeXDTOPackage(t *testing.T, root, id, name, namespace, schema string) {
	t.Helper()
	writeMetadata(t, root, XDTOPackageKind, id, `format: 1
id: `+id+`
name: `+name+`
title: {ru: `+name+`}
namespace: `+namespace+`
`)
	if schema == "" {
		return
	}
	path := filepath.Join(root, "metadata", string(XDTOPackageKind), name, XDTOPackageContentFile)
	if err := os.WriteFile(path, []byte(schema), 0o644); err != nil {
		t.Fatal(err)
	}
}

// A package is its namespace and its schema: the first is what the other side
// of an exchange names the types by, the second is the types themselves. Both
// survive being written and read back.
func TestXDTOPackageCarriesItsNamespaceAndSchema(t *testing.T) {
	t.Parallel()
	root := metadataProject(t)
	writeXDTOPackage(t, root, firstXDTOPackage, "ОбменТоварами", "http://example.org/goods/1.0",
		`<package targetNamespace="http://example.org/goods/1.0"/>`)
	writeXDTOPackage(t, root, secondXDTOPackage, "ОбменДокументами", "http://example.org/documents/1.0", "")

	catalog, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(catalog.XDTOPackages) != 2 {
		t.Fatalf("a package was lost: %+v", catalog.XDTOPackages)
	}
	item, ok := catalog.XDTOPackage("обментоварами")
	if !ok || item.Namespace != "http://example.org/goods/1.0" {
		t.Fatalf("the package is not found by name: %+v", item)
	}
	if _, ok := catalog.XDTOPackageByID(item.ID); !ok {
		t.Fatal("the package is not found by identifier")
	}
}

// What one package cannot check about itself: that no other defines the same
// namespace. Two packages on one namespace are two answers to "what is a type
// called X here", and the exchange would take whichever it read last.
func TestTwoXDTOPackagesCannotShareANamespace(t *testing.T) {
	t.Parallel()
	root := metadataProject(t)
	writeXDTOPackage(t, root, firstXDTOPackage, "ОбменТоварами", "http://example.org/goods/1.0", "")
	writeXDTOPackage(t, root, secondXDTOPackage, "ОбменТоварамиНовый", "http://example.org/goods/1.0", "")
	_, err := Load(root)
	if err == nil {
		t.Fatal("two packages on one namespace were accepted")
	}
	if !strings.Contains(err.Error(), "both define namespace") {
		t.Fatalf("the error does not say what is wrong: %v", err)
	}
}

// A namespace is checked for being a namespace and not for looking like an
// address we recognise: nothing dereferences it, and refusing an unfamiliar one
// would refuse a configuration that works.
func TestXDTOPackageNamespaceIsCheckedInFormOnly(t *testing.T) {
	t.Parallel()
	for name, test := range map[string]struct {
		namespace string
		refused   bool
	}{
		"не адрес вовсе":           {"ОбменТоварами", false},
		"адрес с пробелом":         {"http://example.org/goods 1.0", true},
		"пустое пространство":      {"", true},
		"адрес с переводом строки": {"http://example.org/\ngoods", true},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			root := metadataProject(t)
			writeXDTOPackage(t, root, firstXDTOPackage, "ОбменТоварами", test.namespace, "")
			_, err := Load(root)
			if test.refused && err == nil {
				t.Fatal("a namespace that names nothing was accepted")
			}
			if !test.refused && err != nil {
				t.Fatalf("a namespace that is not an address was refused: %v", err)
			}
		})
	}
}

// A package keeps its description and its schema. Anything else in the folder
// is a file nobody reads, and a schema stored under a name of its own would be
// a schema the platform never finds.
func TestXDTOPackageFolderKeepsOnlyItsOwnFiles(t *testing.T) {
	t.Parallel()
	root := metadataProject(t)
	writeXDTOPackage(t, root, firstXDTOPackage, "ОбменТоварами", "http://example.org/goods/1.0", "")
	stray := filepath.Join(root, "metadata", string(XDTOPackageKind), "ОбменТоварами", "схема.xml")
	if err := os.WriteFile(stray, []byte("<package/>"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(root); err == nil {
		t.Fatal("a package keeping a file nobody reads was accepted")
	}
}

// The folder is named after the package, like every other object folder.
func TestXDTOPackageLiesInAFolderNamedAfterIt(t *testing.T) {
	t.Parallel()
	root := metadataProject(t)
	writeXDTOPackage(t, root, firstXDTOPackage, "ОбменТоварами", "http://example.org/goods/1.0", "")
	directory := filepath.Join(root, "metadata", string(XDTOPackageKind))
	if err := os.Rename(filepath.Join(directory, "ОбменТоварами"), filepath.Join(directory, "ЧужаяПапка")); err != nil {
		t.Fatal(err)
	}
	_, err := Load(root)
	if err == nil {
		t.Fatal("a package lying in someone else's folder was accepted")
	}
	if !strings.Contains(err.Error(), "ЧужаяПапка") {
		t.Fatalf("the error does not say where the package lies: %v", err)
	}
}
