// Package project defines the source model of an ML Project.
package project

import (
	"errors"
	"fmt"
	"io"
	"slices"
	"strings"
	"unicode"

	"github.com/k33alexey/MetaLab/internal/uuid"
	"go.yaml.in/yaml/v3"
)

const (
	// CurrentFormat is the latest supported ML Project manifest format.
	CurrentFormat = 1
)

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
	decoder := yaml.NewDecoder(reader)
	decoder.KnownFields(true)

	var result Project
	if err := decoder.Decode(&result); err != nil {
		return Project{}, fmt.Errorf("decode ML Project: %w", err)
	}

	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		if err != nil {
			return Project{}, fmt.Errorf("decode trailing YAML: %w", err)
		}
		return Project{}, errors.New("decode ML Project: multiple YAML documents are not allowed")
	}

	if err := result.Validate(); err != nil {
		return Project{}, err
	}

	return result, nil
}

// Encode validates and writes a stable YAML representation.
func Encode(writer io.Writer, value Project) error {
	if err := value.Validate(); err != nil {
		return err
	}

	encoder := yaml.NewEncoder(writer)
	encoder.SetIndent(2)

	if err := encoder.Encode(value); err != nil {
		return fmt.Errorf("encode ML Project: %w", err)
	}
	if err := encoder.Close(); err != nil {
		return fmt.Errorf("close ML Project encoder: %w", err)
	}

	return nil
}

// Validate checks the project invariants used by all readers and writers.
func (p Project) Validate() error {
	var problems []string

	if p.Format != CurrentFormat {
		problems = append(problems, fmt.Sprintf("format must be %d", CurrentFormat))
	}
	if p.ID.IsZero() {
		problems = append(problems, "id must be a non-zero UUID")
	}
	if !isIdentifier(p.Name) {
		problems = append(problems, "name must start with a letter and contain only letters or digits")
	}
	if strings.TrimSpace(p.Title) == "" {
		problems = append(problems, "title must not be empty")
	}
	if len(p.Languages) == 0 {
		problems = append(problems, "at least one language is required")
	}

	seenNames := make(map[string]struct{}, len(p.Languages))
	seenCodes := make(map[string]struct{}, len(p.Languages))
	for index, language := range p.Languages {
		prefix := fmt.Sprintf("languages[%d]", index)
		if !isIdentifier(language.Name) {
			problems = append(problems, prefix+".name must start with a letter and contain only letters or digits")
		}
		if strings.TrimSpace(language.Title) == "" {
			problems = append(problems, prefix+".title must not be empty")
		}
		if !isLocaleCode(language.Code) {
			problems = append(problems, prefix+".code must be a lowercase language code with optional region")
		}

		normalizedName := strings.ToLower(language.Name)
		if _, exists := seenNames[normalizedName]; exists {
			problems = append(problems, prefix+".name must be unique")
		}
		seenNames[normalizedName] = struct{}{}

		normalizedCode := strings.ToLower(language.Code)
		if _, exists := seenCodes[normalizedCode]; exists {
			problems = append(problems, prefix+".code must be unique")
		}
		seenCodes[normalizedCode] = struct{}{}
	}

	if !slices.ContainsFunc(p.Languages, func(language Language) bool {
		return strings.EqualFold(language.Code, p.DefaultLanguage)
	}) {
		problems = append(problems, "default_language must reference a configured language code")
	}

	if len(problems) > 0 {
		return errors.New("invalid ML Project: " + strings.Join(problems, "; "))
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
