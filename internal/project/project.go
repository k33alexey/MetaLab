// Package project defines the source model of an ML Project.
package project

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/k33alexey/MetaLab/internal/uuid"
	"go.yaml.in/yaml/v3"
)

const (
	// CurrentFormat is the latest supported ML Project manifest format.
	CurrentFormat = 1
	// MaxYAMLDocumentBytes bounds individual source documents before parsing.
	MaxYAMLDocumentBytes = 4 << 20
)

var (
	// ErrUnsupportedFormat identifies a manifest written in an unsupported format version.
	ErrUnsupportedFormat = errors.New("unsupported ML Project format")
	// ErrYAMLDocumentTooLarge prevents unbounded memory use while parsing project sources.
	ErrYAMLDocumentTooLarge = errors.New("ML Project YAML document is too large")
)

// ValidationIssue points to one invalid manifest field.
type ValidationIssue struct {
	Path    string
	Message string
}

// ValidationError contains all independently detectable manifest problems.
type ValidationError struct {
	Issues            []ValidationIssue
	unsupportedFormat bool
}

func (validation *ValidationError) Error() string {
	problems := make([]string, 0, len(validation.Issues))
	for _, issue := range validation.Issues {
		problems = append(problems, issue.Path+" "+issue.Message)
	}
	return "invalid ML Project: " + strings.Join(problems, "; ")
}

// Is makes future or obsolete manifest versions programmatically distinguishable.
func (validation *ValidationError) Is(target error) bool {
	return target == ErrUnsupportedFormat && validation.unsupportedFormat
}

// Project is the root manifest stored in mlproject.yaml.
type Project struct {
	Format          int        `yaml:"format"`
	ID              uuid.UUID  `yaml:"id"`
	Name            string     `yaml:"name"`
	Title           string     `yaml:"title"`
	DefaultLanguage string     `yaml:"default_language"`
	Languages       []Language `yaml:"languages"`
}

// Language defines an interface language available in an ML Project.
type Language struct {
	Name  string `yaml:"name"`
	Title string `yaml:"title"`
	Code  string `yaml:"code"`
}

// Decode reads one strict YAML document and validates it.
func Decode(reader io.Reader) (Project, error) {
	return DecodeSource("ML Project", reader)
}

// DecodeSource reads one named strict YAML document so diagnostics identify its source.
func DecodeSource(source string, reader io.Reader) (Project, error) {
	content, err := io.ReadAll(io.LimitReader(reader, MaxYAMLDocumentBytes+1))
	if err != nil {
		return Project{}, fmt.Errorf("read %s: %w", source, err)
	}
	if len(content) > MaxYAMLDocumentBytes {
		return Project{}, fmt.Errorf("decode %s: %w (maximum %d bytes)", source, ErrYAMLDocumentTooLarge, MaxYAMLDocumentBytes)
	}
	decoder := yaml.NewDecoder(bytes.NewReader(content))
	decoder.KnownFields(true)

	var result Project
	if err := decoder.Decode(&result); err != nil {
		return Project{}, fmt.Errorf("decode %s: %w", source, err)
	}

	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		if err != nil {
			return Project{}, fmt.Errorf("decode trailing YAML in %s: %w", source, err)
		}
		return Project{}, fmt.Errorf("decode %s: multiple YAML documents are not allowed", source)
	}

	if err := result.Validate(); err != nil {
		return Project{}, fmt.Errorf("validate %s: %w", source, err)
	}

	return result, nil
}

// Encode validates and writes a stable YAML representation.
func Encode(writer io.Writer, value Project) error {
	if err := value.Validate(); err != nil {
		return err
	}

	var content bytes.Buffer
	encoder := yaml.NewEncoder(&content)
	encoder.SetIndent(2)

	if err := encoder.Encode(value); err != nil {
		return fmt.Errorf("encode ML Project: %w", err)
	}
	if err := encoder.Close(); err != nil {
		return fmt.Errorf("close ML Project encoder: %w", err)
	}
	if content.Len() > MaxYAMLDocumentBytes {
		return ErrYAMLDocumentTooLarge
	}
	if _, err := io.Copy(writer, &content); err != nil {
		return fmt.Errorf("write ML Project: %w", err)
	}

	return nil
}

// Validate checks the project invariants used by all readers and writers.
func (p Project) Validate() error {
	var issues []ValidationIssue
	add := func(path, message string) { issues = append(issues, ValidationIssue{Path: path, Message: message}) }

	if p.Format != CurrentFormat {
		add("format", fmt.Sprintf("must be %d", CurrentFormat))
	}
	if p.ID.IsZero() {
		add("id", "must be a non-zero UUID")
	}
	if !isIdentifier(p.Name) {
		add("name", "must start with a letter and contain only letters or digits")
	} else if utf8.RuneCountInString(p.Name) > 128 {
		add("name", "must not exceed 128 characters")
	}
	if !isDisplayText(p.Title, 512) {
		add("title", "must contain 1 to 512 printable characters")
	}
	if len(p.Languages) == 0 {
		add("languages", "must contain at least one language")
	} else if len(p.Languages) > 100 {
		add("languages", "must not contain more than 100 languages")
	}

	seenNames := make(map[string]struct{}, len(p.Languages))
	seenCodes := make(map[string]struct{}, len(p.Languages))
	for index, language := range p.Languages {
		prefix := fmt.Sprintf("languages[%d]", index)
		if !isIdentifier(language.Name) {
			add(prefix+".name", "must start with a letter and contain only letters or digits")
		} else if utf8.RuneCountInString(language.Name) > 128 {
			add(prefix+".name", "must not exceed 128 characters")
		}
		if !isDisplayText(language.Title, 512) {
			add(prefix+".title", "must contain 1 to 512 printable characters")
		}
		if !isLocaleCode(language.Code) {
			add(prefix+".code", "must be a lowercase language code with optional region")
		}

		normalizedName := strings.ToLower(language.Name)
		if _, exists := seenNames[normalizedName]; exists {
			add(prefix+".name", "must be unique")
		}
		seenNames[normalizedName] = struct{}{}

		normalizedCode := strings.ToLower(language.Code)
		if _, exists := seenCodes[normalizedCode]; exists {
			add(prefix+".code", "must be unique")
		}
		seenCodes[normalizedCode] = struct{}{}
	}

	if !slices.ContainsFunc(p.Languages, func(language Language) bool {
		return strings.EqualFold(language.Code, p.DefaultLanguage)
	}) {
		add("default_language", "must reference a configured language code")
	}

	if len(issues) > 0 {
		return &ValidationError{Issues: issues, unsupportedFormat: p.Format > 0 && p.Format != CurrentFormat}
	}

	return nil
}

func isIdentifier(value string) bool {
	for index, current := range []rune(value) {
		if index == 0 && !unicode.IsLetter(current) {
			return false
		}
		if !unicode.IsLetter(current) && !unicode.IsDigit(current) {
			return false
		}
	}

	return value != ""
}

func isLocaleCode(value string) bool {
	parts := strings.Split(value, "-")
	if len(parts) < 1 || len(parts) > 2 || len(parts[0]) < 2 || len(parts[0]) > 3 {
		return false
	}
	for _, current := range parts[0] {
		if current < 'a' || current > 'z' {
			return false
		}
	}
	if len(parts) == 1 {
		return true
	}
	if len(parts[1]) != 2 {
		return false
	}
	for _, current := range parts[1] {
		if current < 'A' || current > 'Z' {
			return false
		}

	}
	return true
}

func isDisplayText(value string, maximum int) bool {
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
