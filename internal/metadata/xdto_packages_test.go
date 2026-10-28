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

// xdtoBody is the body of a package as the prototype writes it - the byte
// order mark, the root declaring the model of XDTO, «xs» and «xsi», lines
// ended by CR LF, the last without one - holding nodes, each line of them
// ended by CR LF; none makes the root an empty element.
func xdtoBody(namespace, nodes string) string {
	root := "\uFEFF<package xmlns=\"http://v8.1c.ru/8.1/xdto\" xmlns:xs=\"http://www.w3.org/2001/XMLSchema\" " +
		"xmlns:xsi=\"http://www.w3.org/2001/XMLSchema-instance\" targetNamespace=\"" + namespace + "\""
	if nodes == "" {
		return root + "/>"
	}
	return root + ">\r\n" + nodes + "</package>"
}

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
		xdtoBody("http://example.org/goods/1.0", ""))
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

// Studio names a namespace of the platform by its address without
// http://v8.1c.ru/ and the version, any other whole (ML-STUDIO.md,
// «Пространства платформы — коротким именем»). The defects caught: a
// namespace of the project or another one under v8.1c.ru cut as if it were
// the platform's, a version left in the short name, and two namespaces of
// the platform under one short name, which typing it could not tell apart.
func TestXDTONamespaceShownShortensThePlatformsAlone(t *testing.T) {
	t.Parallel()
	for namespace, want := range map[string]string{
		"http://v8.1c.ru/8.1/data/core":                      "data/core",
		"http://v8.1c.ru/8.1/data/enterprise/current-config": "data/enterprise/current-config",
		"http://v8.1c.ru/8.1/data-composition-system/schema": "data-composition-system/schema",
		"http://v8.1c.ru/8.2/bsl":                            "bsl",
		"http://v8.1c.ru/8.3/xcf/enums":                      "xcf/enums",
		"http://v8.1c.ru/edi/edi_stnd/EnterpriseData/1.8":    "http://v8.1c.ru/edi/edi_stnd/EnterpriseData/1.8",
		"http://v8.1c.ru/8.1/data/core/extra":                "http://v8.1c.ru/8.1/data/core/extra",
		"http://www.sample-package.org":                      "http://www.sample-package.org",
		"http://www.w3.org/2001/XMLSchema":                   "http://www.w3.org/2001/XMLSchema",
		"":                                                   "",
	} {
		if got := XDTONamespaceShown(namespace); got != want {
			t.Errorf("XDTONamespaceShown(%q) = %q, want %q", namespace, got, want)
		}
	}
	names := PlatformXDTONamespaceNames()
	if len(names) != 66 {
		t.Fatalf("short names of %d namespaces of the platform, want 66", len(names))
	}
	seen := map[string]string{}
	for namespace, short := range names {
		if strings.Contains(short, "v8.1c.ru") || strings.HasPrefix(short, "8.") {
			t.Errorf("%s is shown as %q, the platform's prefix left", namespace, short)
		}
		if other, taken := seen[short]; taken {
			t.Errorf("%s and %s share the short name %q", namespace, other, short)
		}
		seen[short] = namespace
	}
}
