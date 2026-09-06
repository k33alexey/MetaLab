// Package metadata defines validated application metadata loaded from an ML Project.
package metadata

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"slices"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/k33alexey/MetaLab/internal/project"
	"github.com/k33alexey/MetaLab/internal/uuid"
	"go.yaml.in/yaml/v3"
)

const (
	CurrentFormat      = 1
	maxObjectsPerKind  = 100_000
	maxEnumerationVals = 1 << 20
)

type Kind string

const (
	ConstantKind    Kind = "constants"
	EnumerationKind Kind = "enumerations"
	DefinedTypeKind Kind = "defined-types"
)

type TypeKind string

const (
	StringType      TypeKind = "string"
	NumberType      TypeKind = "number"
	BooleanType     TypeKind = "boolean"
	DateType        TypeKind = "date"
	UUIDType        TypeKind = "uuid"
	EnumerationType TypeKind = "enumeration"
	DefinedType     TypeKind = "defined-type"
)

var (
	ErrUnsupportedFormat = errors.New("unsupported metadata format")
	ErrDuplicateName     = errors.New("duplicate metadata name")
	ErrDuplicateID       = errors.New("duplicate metadata UUID")
)

// LocalizedText stores translations by configured language code.
type LocalizedText map[string]string

// Resolve returns the requested translation or the first configured non-empty fallback.
func (text LocalizedText) Resolve(language string, configured []project.Language) string {
	if value := strings.TrimSpace(text[language]); value != "" {
		return value
	}
	for _, item := range configured {
		if value := strings.TrimSpace(text[item.Code]); value != "" {
			return value
		}
	}
	keys := make([]string, 0, len(text))
	for key := range text {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		if value := strings.TrimSpace(text[key]); value != "" {
			return value
		}
	}
	return ""
}

// Type describes one allowed scalar value. References use stable metadata UUIDs.
type Type struct {
	Kind      TypeKind   `yaml:"kind"`
	Reference *uuid.UUID `yaml:"reference,omitempty"`
	Length    int        `yaml:"length,omitempty"`
	Precision int        `yaml:"precision,omitempty"`
	Scale     int        `yaml:"scale,omitempty"`
}

type Constant struct {
	Format  int           `yaml:"format"`
	ID      uuid.UUID     `yaml:"id"`
	Name    string        `yaml:"name"`
	Title   LocalizedText `yaml:"title"`
	Types   []Type        `yaml:"types"`
	Default *Value        `yaml:"default,omitempty"`
}

type EnumerationValue struct {
	ID    uuid.UUID     `yaml:"id"`
	Name  string        `yaml:"name"`
	Title LocalizedText `yaml:"title"`
}

type Enumeration struct {
	Format int                `yaml:"format"`
	ID     uuid.UUID          `yaml:"id"`
	Name   string             `yaml:"name"`
	Title  LocalizedText      `yaml:"title"`
	Values []EnumerationValue `yaml:"values"`
}

type DefinedTypeObject struct {
	Format int           `yaml:"format"`
	ID     uuid.UUID     `yaml:"id"`
	Name   string        `yaml:"name"`
	Title  LocalizedText `yaml:"title"`
	Types  []Type        `yaml:"types"`
}

// Catalog is an immutable-by-convention snapshot of the supported metadata kinds.
type Catalog struct {
	Project           project.Project
	Constants         []Constant
	Enumerations      []Enumeration
	DefinedTypes      []DefinedTypeObject
	constantByName    map[string]int
	constantByID      map[uuid.UUID]int
	enumerationByName map[string]int
	definedTypeByName map[string]int
	enumerationByID   map[uuid.UUID]int
	definedTypeByID   map[uuid.UUID]int
}

func (catalog *Catalog) ConstantByID(id uuid.UUID) (Constant, bool) {
	index, ok := catalog.constantByID[id]
	if !ok {
		return Constant{}, false
	}
	return cloneConstant(catalog.Constants[index]), true
}

func (catalog *Catalog) ConstantIDs() []uuid.UUID {
	if len(catalog.Constants) == 0 {
		return nil
	}
	result := make([]uuid.UUID, len(catalog.Constants))
	for index := range catalog.Constants {
		result[index] = catalog.Constants[index].ID
	}
	return result
}

func (catalog *Catalog) Constant(name string) (Constant, bool) {
	index, ok := catalog.constantByName[strings.ToLower(name)]
	if !ok {
		return Constant{}, false
	}
	return cloneConstant(catalog.Constants[index]), true
}

func (catalog *Catalog) Enumeration(name string) (Enumeration, bool) {
	index, ok := catalog.enumerationByName[strings.ToLower(name)]
	if !ok {
		return Enumeration{}, false
	}
	return cloneEnumeration(catalog.Enumerations[index]), true
}

func (catalog *Catalog) EnumerationValue(enumeration, value string) (EnumerationValue, bool) {
	item, ok := catalog.Enumeration(enumeration)
	if !ok {
		return EnumerationValue{}, false
	}
	for _, candidate := range item.Values {
		if strings.EqualFold(candidate.Name, value) {
			candidate.Title = cloneTitle(candidate.Title)
			return candidate, true
		}
	}
	return EnumerationValue{}, false
}

// ResolvedTypes expands nested defined types into concrete allowed types.
func (catalog *Catalog) ResolvedTypes(name string) ([]Type, bool) {
	item, ok := catalog.DefinedType(name)
	if !ok {
		return nil, false
	}
	types, err := catalog.expandTypes(item.Types, nil)
	return cloneTypes(types), err == nil
}

func (catalog *Catalog) DefinedType(name string) (DefinedTypeObject, bool) {
	index, ok := catalog.definedTypeByName[strings.ToLower(name)]
	if !ok {
		return DefinedTypeObject{}, false
	}
	return cloneDefinedType(catalog.DefinedTypes[index]), true
}

func DecodeConstant(source string, reader io.Reader, manifest project.Project) (Constant, error) {
	var value Constant
	if err := decodeStrict(source, reader, &value); err != nil {
		return Constant{}, err
	}
	issues := validateBase(value.Format, value.ID, value.Name, value.Title, manifest)
	issues = append(issues, validateTypes("types", value.Types, uuid.UUID{})...)
	if err := issuesError(source, value.Format, issues); err != nil {
		return Constant{}, err
	}
	return value, nil
}

func DecodeEnumeration(source string, reader io.Reader, manifest project.Project) (Enumeration, error) {
	var value Enumeration
	if err := decodeStrict(source, reader, &value); err != nil {
		return Enumeration{}, err
	}
	issues := validateBase(value.Format, value.ID, value.Name, value.Title, manifest)
	if len(value.Values) == 0 || len(value.Values) > maxEnumerationVals {
		issues = append(issues, "values must contain 1..1048576 items")
	}
	names, ids := map[string]bool{}, map[uuid.UUID]bool{}
	for index, item := range value.Values {
		prefix := fmt.Sprintf("values[%d]", index)
		if item.ID.IsZero() {
			issues = append(issues, prefix+".id must be a non-zero UUID")
		}
		if ids[item.ID] {
			issues = append(issues, prefix+".id must be unique")
		}
		ids[item.ID] = true
		if !validIdentifier(item.Name) {
			issues = append(issues, prefix+".name must be a valid identifier")
		}
		folded := strings.ToLower(item.Name)
		if names[folded] {
			issues = append(issues, prefix+".name must be unique")
		}
		names[folded] = true
		issues = append(issues, validateTitle(prefix+".title", item.Title, manifest)...)
	}
	if err := issuesError(source, value.Format, issues); err != nil {
		return Enumeration{}, err
	}
	return value, nil
}

func DecodeDefinedType(source string, reader io.Reader, manifest project.Project) (DefinedTypeObject, error) {
	var value DefinedTypeObject
	if err := decodeStrict(source, reader, &value); err != nil {
		return DefinedTypeObject{}, err
	}
	issues := validateBase(value.Format, value.ID, value.Name, value.Title, manifest)
	issues = append(issues, validateTypes("types", value.Types, value.ID)...)
	if err := issuesError(source, value.Format, issues); err != nil {
		return DefinedTypeObject{}, err
	}
	return value, nil
}

func Encode(writer io.Writer, value any) error {
	var content bytes.Buffer
	encoder := yaml.NewEncoder(&content)
	encoder.SetIndent(2)
	if err := encoder.Encode(value); err != nil {
		return fmt.Errorf("encode metadata: %w", err)
	}
	if err := encoder.Close(); err != nil {
		return fmt.Errorf("close metadata encoder: %w", err)
	}
	if content.Len() > project.MaxYAMLDocumentBytes {
		return project.ErrYAMLDocumentTooLarge
	}
	_, err := io.Copy(writer, &content)
	return err
}

func decodeStrict(source string, reader io.Reader, target any) error {
	content, err := io.ReadAll(io.LimitReader(reader, project.MaxYAMLDocumentBytes+1))
	if err != nil {
		return fmt.Errorf("read %s: %w", source, err)
	}
	if len(content) > project.MaxYAMLDocumentBytes {
		return fmt.Errorf("decode %s: %w", source, project.ErrYAMLDocumentTooLarge)
	}
	decoder := yaml.NewDecoder(bytes.NewReader(content))
	decoder.KnownFields(true)
	if err := decoder.Decode(target); err != nil {
		return fmt.Errorf("decode %s: %w", source, err)
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		if err != nil {
			return fmt.Errorf("decode trailing YAML in %s: %w", source, err)
		}
		return fmt.Errorf("decode %s: multiple YAML documents are not allowed", source)
	}
	return nil
}

func validateBase(format int, id uuid.UUID, name string, title LocalizedText, manifest project.Project) []string {
	var issues []string
	if format != CurrentFormat {
		issues = append(issues, fmt.Sprintf("format must be %d", CurrentFormat))
	}
	if id.IsZero() {
		issues = append(issues, "id must be a non-zero UUID")
	}
	if !validIdentifier(name) {
		issues = append(issues, "name must start with a letter and contain only letters or digits")
	}
	if utf8.RuneCountInString(name) > 128 {
		issues = append(issues, "name must not exceed 128 characters")
	}
	return append(issues, validateTitle("title", title, manifest)...)
}

func validateTitle(path string, title LocalizedText, manifest project.Project) []string {
	var issues []string
	configured := make(map[string]bool, len(manifest.Languages))
	for _, language := range manifest.Languages {
		configured[language.Code] = true
	}
	if len(title) == 0 {
		return []string{path + " must contain at least one translation"}
	}
	for language, value := range title {
		if !configured[language] {
			issues = append(issues, path+"."+language+" uses an unconfigured language")
		}
		if !validText(value, 512) {
			issues = append(issues, path+"."+language+" must contain 1..512 printable characters")
		}
	}
	return issues
}

func validateTypes(path string, types []Type, self uuid.UUID) []string {
	if len(types) == 0 || len(types) > 32 {
		return []string{path + " must contain 1..32 types"}
	}
	var issues []string
	seen := map[string]bool{}
	for index, item := range types {
		prefix := fmt.Sprintf("%s[%d]", path, index)
		key := string(item.Kind)
		if item.Reference != nil {
			key += ":" + item.Reference.String()
		}
		if seen[key] {
			issues = append(issues, prefix+" duplicates an allowed type")
		}
		seen[key] = true
		referenced := item.Kind == EnumerationType || item.Kind == DefinedType
		if referenced && (item.Reference == nil || item.Reference.IsZero()) {
			issues = append(issues, prefix+".reference is required")
		}
		if !referenced && item.Reference != nil {
			issues = append(issues, prefix+".reference is not allowed")
		}
		if item.Kind == DefinedType && item.Reference != nil && *item.Reference == self {
			issues = append(issues, prefix+" cannot reference itself")
		}
		switch item.Kind {
		case StringType:
			if item.Length < 0 || item.Length > 1_048_576 {
				issues = append(issues, prefix+".length must be 0..1048576")
			}
			if item.Precision != 0 || item.Scale != 0 {
				issues = append(issues, prefix+" has invalid numeric qualifiers")
			}
		case NumberType:
			if item.Precision < 1 || item.Precision > 38 {
				issues = append(issues, prefix+".precision must be 1..38")
			}
			if item.Scale < 0 || item.Scale > item.Precision {
				issues = append(issues, prefix+".scale must be 0..precision")
			}
			if item.Length != 0 {
				issues = append(issues, prefix+".length is not allowed")
			}
		case BooleanType, DateType, UUIDType, EnumerationType, DefinedType:
			if item.Length != 0 || item.Precision != 0 || item.Scale != 0 {
				issues = append(issues, prefix+" has unsupported qualifiers")
			}
		default:
			issues = append(issues, prefix+".kind is unsupported")
		}
	}
	return issues
}

func issuesError(source string, format int, issues []string) error {
	if len(issues) == 0 {
		return nil
	}
	err := fmt.Errorf("validate %s: %s", source, strings.Join(issues, "; "))
	if format != CurrentFormat {
		return fmt.Errorf("%w: %w", ErrUnsupportedFormat, err)
	}
	return err
}

func validIdentifier(value string) bool {
	for index, symbol := range []rune(value) {
		if index == 0 && !unicode.IsLetter(symbol) {
			return false
		}
		if !unicode.IsLetter(symbol) && !unicode.IsDigit(symbol) {
			return false
		}
	}
	return value != ""
}

func validText(value string, maximum int) bool {
	if !utf8.ValidString(value) || strings.TrimSpace(value) == "" || utf8.RuneCountInString(value) > maximum {
		return false
	}
	for _, symbol := range value {
		if unicode.IsControl(symbol) {
			return false
		}
	}
	return true
}

func cloneTypes(items []Type) []Type {
	result := slices.Clone(items)
	for index := range result {
		if result[index].Reference != nil {
			reference := *result[index].Reference
			result[index].Reference = &reference
		}
	}
	return result
}
func cloneTitle(value LocalizedText) LocalizedText {
	result := make(LocalizedText, len(value))
	for key, text := range value {
		result[key] = text
	}
	return result
}
func cloneConstant(value Constant) Constant {
	value.Title, value.Types = cloneTitle(value.Title), cloneTypes(value.Types)
	if value.Default != nil {
		defaultValue := *value.Default
		value.Default = &defaultValue
	}
	return value
}
func cloneEnumeration(value Enumeration) Enumeration {
	value.Title = cloneTitle(value.Title)
	value.Values = slices.Clone(value.Values)
	for index := range value.Values {
		value.Values[index].Title = cloneTitle(value.Values[index].Title)
	}
	return value
}
func cloneDefinedType(value DefinedTypeObject) DefinedTypeObject {
	value.Title, value.Types = cloneTitle(value.Title), cloneTypes(value.Types)
	return value
}
