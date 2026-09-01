package project

import (
	"bytes"
	"errors"
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
		"title must not be empty",
		"languages[0].title must not be empty",
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
