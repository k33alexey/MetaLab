package project

import (
	"bytes"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/k33alexey/MetaLab/internal/uuid"
)

const validYAML = `format: 1
id: 018f1f72-3b4c-7d6e-8f90-123456789abc
name: SalesDemo
title: Продажи и склад
default_language: ru
languages:
  - name: Русский
    title: Русский
    code: ru
  - name: Українська
    title: Українська
    code: uk
  - name: English
    title: English
    code: en
`

func TestDecodeValidProject(t *testing.T) {
	t.Parallel()

	value, err := Decode(strings.NewReader(validYAML))
	if err != nil {
		t.Fatalf("Decode() error = %v", err)
	}
	if value.Name != "SalesDemo" {
		t.Fatalf("Project.Name = %q, want SalesDemo", value.Name)
	}
	if value.ID.String() != "018f1f72-3b4c-7d6e-8f90-123456789abc" {
		t.Fatalf("Project.ID = %s", value.ID)
	}
	if len(value.Languages) != 3 {
		t.Fatalf("len(Project.Languages) = %d, want 3", len(value.Languages))
	}
}

func TestEncodeIsStable(t *testing.T) {
	t.Parallel()

	value, err := Decode(strings.NewReader(validYAML))
	if err != nil {
		t.Fatalf("Decode() error = %v", err)
	}

	var first bytes.Buffer
	if err := Encode(&first, value); err != nil {
		t.Fatalf("first Encode() error = %v", err)
	}
	firstYAML := first.String()

	restored, err := Decode(strings.NewReader(firstYAML))
	if err != nil {
		t.Fatalf("second Decode() error = %v", err)
	}

	var second bytes.Buffer
	if err := Encode(&second, restored); err != nil {
		t.Fatalf("second Encode() error = %v", err)
	}

	if firstYAML != second.String() {
		t.Fatalf("serialization is not stable:\nfirst:\n%s\nsecond:\n%s", firstYAML, second.String())
	}
	if firstYAML != validYAML {
		t.Fatalf("Encode() output differs from canonical YAML:\ngot:\n%s\nwant:\n%s", firstYAML, validYAML)
	}
}

func TestDecodeRejectsUnknownField(t *testing.T) {
	t.Parallel()

	input := validYAML + "unexpected: value\n"
	if _, err := Decode(strings.NewReader(input)); err == nil {
		t.Fatal("Decode() returned no error for unknown field")
	}
}

func TestDecodeRejectsMultipleDocuments(t *testing.T) {
	t.Parallel()

	input := validYAML + "---\n" + validYAML
	if _, err := Decode(strings.NewReader(input)); err == nil {
		t.Fatal("Decode() returned no error for multiple documents")
	}
}

func TestDecodeReportsNamedSourceAndLine(t *testing.T) {
	t.Parallel()

	_, err := DecodeSource("metadata/catalogs/example.yaml", strings.NewReader("format: 1\nunknown: true\n"))
	if err == nil || !strings.Contains(err.Error(), "metadata/catalogs/example.yaml") || !strings.Contains(err.Error(), "line 2") {
		t.Fatalf("DecodeSource() error = %v", err)
	}
}

func TestDecodeRejectsOversizedDocument(t *testing.T) {
	t.Parallel()

	reader := io.LimitReader(strings.NewReader(strings.Repeat("x", MaxYAMLDocumentBytes+1)), MaxYAMLDocumentBytes+1)
	if _, err := Decode(reader); !errors.Is(err, ErrYAMLDocumentTooLarge) {
		t.Fatalf("Decode() error = %v, want ErrYAMLDocumentTooLarge", err)
	}
}

func TestUnsupportedFormatIsMatchableAndStructured(t *testing.T) {
	t.Parallel()

	value, err := Decode(strings.NewReader(strings.Replace(validYAML, "format: 1", "format: 2", 1)))
	if !errors.Is(err, ErrUnsupportedFormat) {
		t.Fatalf("Decode() value=%+v error=%v, want ErrUnsupportedFormat", value, err)
	}
	var validation *ValidationError
	if !errors.As(err, &validation) || len(validation.Issues) != 1 || validation.Issues[0].Path != "format" {
		t.Fatalf("validation error = %#v", validation)
	}
}

func TestValidationDiagnosticIncludesNamedSource(t *testing.T) {
	t.Parallel()

	input := strings.Replace(validYAML, "title: Продажи и склад", "title: ''", 1)
	_, err := DecodeSource("mlproject.yaml", strings.NewReader(input))
	if err == nil || !strings.Contains(err.Error(), "validate mlproject.yaml") || !strings.Contains(err.Error(), "title") {
		t.Fatalf("DecodeSource() error = %v", err)
	}
}

func TestValidateReportsAllProblems(t *testing.T) {
	t.Parallel()

	value := Project{
		Format:          2,
		Name:            "1 invalid",
		DefaultLanguage: "de",
		Languages: []Language{
			{Name: "Русский", Title: "", Code: "RU"},
			{Name: "русский", Title: "Українська", Code: "RU"},
		},
	}

	err := value.Validate()
	if err == nil {
		t.Fatal("Validate() returned no error")
	}

	for _, expected := range []string{
		"format must be 1",
		"id must be a non-zero UUID",
		"name must start with a letter",
		"title must contain 1 to 512 printable characters",
		"languages[0].title must contain 1 to 512 printable characters",
		"languages[0].code",
		"languages[1].name must be unique",
		"languages[1].code must be unique",
		"default_language",
	} {
		if !strings.Contains(err.Error(), expected) {
			t.Errorf("Validate() error = %q, want substring %q", err, expected)
		}
	}
}

func TestValidateRejectsUnboundedOrControlText(t *testing.T) {
	t.Parallel()

	value := Project{
		Format: CurrentFormat, ID: uuid.MustNew(), Name: "A" + strings.Repeat("b", 128),
		Title: "Unsafe\nTitle", DefaultLanguage: "ru",
		Languages: []Language{{Name: "Русский", Title: "Русский", Code: "ru"}},
	}
	err := value.Validate()
	if err == nil || !strings.Contains(err.Error(), "name must not exceed 128") || !strings.Contains(err.Error(), "title must contain") {
		t.Fatalf("Validate() error = %v", err)
	}
}

func TestValidateAcceptsUnicodeIdentifiersAndRegion(t *testing.T) {
	t.Parallel()

	value := Project{
		Format:          CurrentFormat,
		ID:              uuid.MustNew(),
		Name:            "Торгівля2",
		Title:           "Торгівля",
		DefaultLanguage: "uk-UA",
		Languages: []Language{
			{Name: "Українська", Title: "Українська", Code: "uk-UA"},
		},
	}

	if err := value.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
}

func TestEncodeRejectsInvalidProject(t *testing.T) {
	t.Parallel()

	var output bytes.Buffer
	if err := Encode(&output, Project{}); err == nil {
		t.Fatal("Encode() returned no error for invalid project")
	}
}

func TestEncodeReportsWriterError(t *testing.T) {
	t.Parallel()

	value, err := Decode(strings.NewReader(validYAML))
	if err != nil {
		t.Fatalf("Decode() error = %v", err)
	}

	if err := Encode(failingWriter{}, value); err == nil {
		t.Fatal("Encode() returned no writer error")
	}
}

type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) {
	return 0, errors.New("write failed")
}
