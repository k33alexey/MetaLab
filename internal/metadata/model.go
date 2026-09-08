// Package metadata defines validated application metadata loaded from an ML Project.
package metadata

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"maps"
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
	ConstantKind             Kind = "constants"
	EnumerationKind          Kind = "enumerations"
	DefinedTypeKind          Kind = "defined-types"
	CatalogKind              Kind = "catalogs"
	DocumentKind             Kind = "documents"
	InformationRegisterKind  Kind = "information-registers"
	AccumulationRegisterKind Kind = "accumulation-registers"
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
	CatalogType     TypeKind = "catalog"
	DocumentType    TypeKind = "document"
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

type CatalogCode struct {
	Type   TypeKind `yaml:"type"`
	Length int      `yaml:"length"`
	Auto   bool     `yaml:"auto"`
	Unique bool     `yaml:"unique"`
}

// PredefinedCatalogItem binds configuration identity to one stable catalog reference.
type PredefinedCatalogItem struct {
	ID          uuid.UUID        `yaml:"id"`
	Name        string           `yaml:"name"`
	Code        string           `yaml:"code,omitempty"`
	Description string           `yaml:"description,omitempty"`
	Attributes  map[string]Value `yaml:"attributes,omitempty"`
}

type Attribute struct {
	ID       uuid.UUID     `yaml:"id"`
	Name     string        `yaml:"name"`
	Title    LocalizedText `yaml:"title"`
	Types    []Type        `yaml:"types"`
	Required bool          `yaml:"required,omitempty"`
	Indexed  bool          `yaml:"indexed,omitempty"`
}

type TablePart struct {
	ID         uuid.UUID     `yaml:"id"`
	Name       string        `yaml:"name"`
	Title      LocalizedText `yaml:"title"`
	Attributes []Attribute   `yaml:"attributes"`
}

// CatalogDefinition describes one ML catalog and its persistent record shape.
type CatalogDefinition struct {
	Format            int                     `yaml:"format"`
	ID                uuid.UUID               `yaml:"id"`
	Name              string                  `yaml:"name"`
	Title             LocalizedText           `yaml:"title"`
	Code              CatalogCode             `yaml:"code"`
	DescriptionLength int                     `yaml:"description_length"`
	Attributes        []Attribute             `yaml:"attributes,omitempty"`
	TableParts        []TablePart             `yaml:"table_parts,omitempty"`
	ObjectModule      *uuid.UUID              `yaml:"object_module,omitempty"`
	ManagerModule     *uuid.UUID              `yaml:"manager_module,omitempty"`
	Forms             ObjectForms             `yaml:"forms,omitempty"`
	Predefined        []PredefinedCatalogItem `yaml:"predefined,omitempty"`
}

// Catalog is an immutable-by-convention snapshot of the supported metadata kinds.
type Catalog struct {
	Project                    project.Project
	Constants                  []Constant
	Enumerations               []Enumeration
	DefinedTypes               []DefinedTypeObject
	Catalogs                   []CatalogDefinition
	Documents                  []DocumentDefinition
	InformationRegisters       []InformationRegisterDefinition
	AccumulationRegisters      []AccumulationRegisterDefinition
	constantByName             map[string]int
	constantByID               map[uuid.UUID]int
	enumerationByName          map[string]int
	definedTypeByName          map[string]int
	enumerationByID            map[uuid.UUID]int
	definedTypeByID            map[uuid.UUID]int
	catalogByName              map[string]int
	catalogByID                map[uuid.UUID]int
	documentByName             map[string]int
	documentByID               map[uuid.UUID]int
	informationRegisterByName  map[string]int
	informationRegisterByID    map[uuid.UUID]int
	accumulationRegisterByName map[string]int
	accumulationRegisterByID   map[uuid.UUID]int
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

func (catalog *Catalog) CatalogDefinition(name string) (CatalogDefinition, bool) {
	index, ok := catalog.catalogByName[strings.ToLower(name)]
	if !ok {
		return CatalogDefinition{}, false
	}
	return cloneCatalogDefinition(catalog.Catalogs[index]), true
}

func (catalog *Catalog) CatalogByID(id uuid.UUID) (CatalogDefinition, bool) {
	index, ok := catalog.catalogByID[id]
	if !ok {
		return CatalogDefinition{}, false
	}
	return cloneCatalogDefinition(catalog.Catalogs[index]), true
}

func (catalog *Catalog) CatalogIDs() []uuid.UUID {
	if len(catalog.Catalogs) == 0 {
		return nil
	}
	result := make([]uuid.UUID, len(catalog.Catalogs))
	for index := range catalog.Catalogs {
		result[index] = catalog.Catalogs[index].ID
	}
	return result
}

func (definition CatalogDefinition) PredefinedItem(name string) (PredefinedCatalogItem, bool) {
	for _, item := range definition.Predefined {
		if strings.EqualFold(item.Name, name) {
			return clonePredefinedCatalogItem(item), true
		}
	}
	return PredefinedCatalogItem{}, false
}

func (definition CatalogDefinition) PredefinedByID(id uuid.UUID) (PredefinedCatalogItem, bool) {
	for _, item := range definition.Predefined {
		if item.ID == id {
			return clonePredefinedCatalogItem(item), true
		}
	}
	return PredefinedCatalogItem{}, false
}

func (catalog *Catalog) DocumentDefinition(name string) (DocumentDefinition, bool) {
	index, ok := catalog.documentByName[strings.ToLower(name)]
	if !ok {
		return DocumentDefinition{}, false
	}
	return cloneDocumentDefinition(catalog.Documents[index]), true
}

func (catalog *Catalog) DocumentByID(id uuid.UUID) (DocumentDefinition, bool) {
	index, ok := catalog.documentByID[id]
	if !ok {
		return DocumentDefinition{}, false
	}
	return cloneDocumentDefinition(catalog.Documents[index]), true
}

func (catalog *Catalog) DocumentIDs() []uuid.UUID {
	if len(catalog.Documents) == 0 {
		return nil
	}
	result := make([]uuid.UUID, len(catalog.Documents))
	for index := range catalog.Documents {
		result[index] = catalog.Documents[index].ID
	}
	return result
}

func (catalog *Catalog) InformationRegisterDefinition(name string) (InformationRegisterDefinition, bool) {
	index, ok := catalog.informationRegisterByName[strings.ToLower(name)]
	if !ok {
		return InformationRegisterDefinition{}, false
	}
	return cloneInformationRegisterDefinition(catalog.InformationRegisters[index]), true
}

func (catalog *Catalog) InformationRegisterByID(id uuid.UUID) (InformationRegisterDefinition, bool) {
	index, ok := catalog.informationRegisterByID[id]
	if !ok {
		return InformationRegisterDefinition{}, false
	}
	return cloneInformationRegisterDefinition(catalog.InformationRegisters[index]), true
}

func (catalog *Catalog) InformationRegisterIDs() []uuid.UUID {
	if len(catalog.InformationRegisters) == 0 {
		return nil
	}
	result := make([]uuid.UUID, len(catalog.InformationRegisters))
	for index := range catalog.InformationRegisters {
		result[index] = catalog.InformationRegisters[index].ID
	}
	return result
}

func (catalog *Catalog) AccumulationRegisterDefinition(name string) (AccumulationRegisterDefinition, bool) {
	index, ok := catalog.accumulationRegisterByName[strings.ToLower(name)]
	if !ok {
		return AccumulationRegisterDefinition{}, false
	}
	return cloneAccumulationRegisterDefinition(catalog.AccumulationRegisters[index]), true
}

func (catalog *Catalog) AccumulationRegisterByID(id uuid.UUID) (AccumulationRegisterDefinition, bool) {
	index, ok := catalog.accumulationRegisterByID[id]
	if !ok {
		return AccumulationRegisterDefinition{}, false
	}
	return cloneAccumulationRegisterDefinition(catalog.AccumulationRegisters[index]), true
}

func (catalog *Catalog) AccumulationRegisterIDs() []uuid.UUID {
	if len(catalog.AccumulationRegisters) == 0 {
		return nil
	}
	result := make([]uuid.UUID, len(catalog.AccumulationRegisters))
	for index := range catalog.AccumulationRegisters {
		result[index] = catalog.AccumulationRegisters[index].ID
	}
	return result
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

func DecodeCatalog(source string, reader io.Reader, manifest project.Project) (CatalogDefinition, error) {
	var value CatalogDefinition
	if err := decodeStrict(source, reader, &value); err != nil {
		return CatalogDefinition{}, err
	}
	issues := validateBase(value.Format, value.ID, value.Name, value.Title, manifest)
	switch value.Code.Type {
	case StringType:
		if value.Code.Length < 1 || value.Code.Length > 128 {
			issues = append(issues, "code.length must be 1..128 for string codes")
		}
	case NumberType:
		if value.Code.Length < 1 || value.Code.Length > 38 {
			issues = append(issues, "code.length must be 1..38 for number codes")
		}
	default:
		issues = append(issues, "code.type must be string or number")
	}
	if value.DescriptionLength < 1 || value.DescriptionLength > 1_048_576 {
		issues = append(issues, "description_length must be 1..1048576")
	}
	issues = append(issues, validateAttributes("attributes", value.Attributes, manifest, reservedCatalogObjectName)...)
	attributeNames := make(map[string]bool, len(value.Attributes))
	for _, attribute := range value.Attributes {
		attributeNames[strings.ToLower(attribute.Name)] = true
	}
	if len(value.TableParts) > 128 {
		issues = append(issues, "table_parts must not contain more than 128 items")
	}
	partNames, partIDs := map[string]bool{}, map[uuid.UUID]bool{}
	for index, part := range value.TableParts {
		prefix := fmt.Sprintf("table_parts[%d]", index)
		if part.ID.IsZero() {
			issues = append(issues, prefix+".id must be a non-zero UUID")
		}
		if partIDs[part.ID] {
			issues = append(issues, prefix+".id must be unique")
		}
		partIDs[part.ID] = true
		if !validIdentifier(part.Name) {
			issues = append(issues, prefix+".name must be a valid identifier")
		}
		folded := strings.ToLower(part.Name)
		if partNames[folded] {
			issues = append(issues, prefix+".name must be unique")
		}
		if reservedCatalogObjectName(folded) {
			issues = append(issues, prefix+".name is reserved")
		}
		if attributeNames[folded] {
			issues = append(issues, prefix+".name conflicts with an attribute")
		}
		partNames[folded] = true
		issues = append(issues, validateTitle(prefix+".title", part.Title, manifest)...)
		issues = append(issues, validateAttributes(prefix+".attributes", part.Attributes, manifest, nil)...)
	}
	for name, module := range map[string]*uuid.UUID{"object_module": value.ObjectModule, "manager_module": value.ManagerModule} {
		if module != nil && module.IsZero() {
			issues = append(issues, name+" must be a non-zero UUID")
		}
	}
	if value.ObjectModule != nil && value.ManagerModule != nil && *value.ObjectModule == *value.ManagerModule {
		issues = append(issues, "object_module and manager_module must be different")
	}
	issues = append(issues, validateObjectForms(value.Forms)...)
	issues = append(issues, validatePredefinedCatalogItems(value)...)
	if err := issuesError(source, value.Format, issues); err != nil {
		return CatalogDefinition{}, err
	}
	return value, nil
}

func validatePredefinedCatalogItems(definition CatalogDefinition) []string {
	if len(definition.Predefined) > maxObjectsPerKind {
		return []string{fmt.Sprintf("predefined must not contain more than %d items", maxObjectsPerKind)}
	}
	var issues []string
	names, ids := map[string]bool{}, map[uuid.UUID]bool{}
	for index, item := range definition.Predefined {
		prefix := fmt.Sprintf("predefined[%d]", index)
		if item.ID.IsZero() {
			issues = append(issues, prefix+".id must be a non-zero UUID")
		}
		if ids[item.ID] {
			issues = append(issues, prefix+".id must be unique")
		}
		ids[item.ID] = true
		if !validIdentifier(item.Name) || utf8.RuneCountInString(item.Name) > 128 {
			issues = append(issues, prefix+".name must be a valid identifier of at most 128 characters")
		}
		folded := strings.ToLower(item.Name)
		if names[folded] {
			issues = append(issues, prefix+".name must be unique")
		}
		names[folded] = true
		if item.Code == "" {
			if !definition.Code.Auto {
				issues = append(issues, prefix+".code is required when automatic codes are disabled")
			}
		} else if _, err := normalizeCatalogCode(definition.Code, item.Code); err != nil {
			issues = append(issues, prefix+".code is invalid: "+err.Error())
		}
		if !utf8.ValidString(item.Description) || utf8.RuneCountInString(item.Description) > definition.DescriptionLength {
			issues = append(issues, fmt.Sprintf("%s.description must not exceed %d characters", prefix, definition.DescriptionLength))
		}
		attributeNames := map[string]bool{}
		for name := range item.Attributes {
			attribute, ok := findCatalogAttribute(definition.Attributes, name)
			if !ok {
				issues = append(issues, prefix+".attributes."+name+" is unknown")
				continue
			}
			key := strings.ToLower(attribute.Name)
			if attributeNames[key] {
				issues = append(issues, prefix+".attributes contains duplicate "+attribute.Name)
			}
			attributeNames[key] = true
		}
		for _, attribute := range definition.Attributes {
			if attribute.Required && !attributeNames[strings.ToLower(attribute.Name)] {
				issues = append(issues, prefix+".attributes."+attribute.Name+" is required")
			}
		}
	}
	return issues
}

func validateAttributes(path string, attributes []Attribute, manifest project.Project, reserved func(string) bool) []string {
	if len(attributes) > 1024 {
		return []string{path + " must not contain more than 1024 items"}
	}
	var issues []string
	names, ids := map[string]bool{}, map[uuid.UUID]bool{}
	for index, attribute := range attributes {
		prefix := fmt.Sprintf("%s[%d]", path, index)
		if attribute.ID.IsZero() {
			issues = append(issues, prefix+".id must be a non-zero UUID")
		}
		if ids[attribute.ID] {
			issues = append(issues, prefix+".id must be unique")
		}
		ids[attribute.ID] = true
		if !validIdentifier(attribute.Name) {
			issues = append(issues, prefix+".name must be a valid identifier")
		}
		folded := strings.ToLower(attribute.Name)
		if names[folded] {
			issues = append(issues, prefix+".name must be unique")
		}
		if reserved != nil && reserved(folded) {
			issues = append(issues, prefix+".name is reserved")
		}
		names[folded] = true
		issues = append(issues, validateTitle(prefix+".title", attribute.Title, manifest)...)
		issues = append(issues, validateTypes(prefix+".types", attribute.Types, uuid.UUID{})...)
	}
	return issues
}

func reservedCatalogObjectName(name string) bool {
	switch strings.ToLower(name) {
	case "ссылка", "ref", "код", "code", "наименование", "description", "версия", "version",
		"пометкаудаления", "deletionmark", "имяпредопределенныхданных", "имяпредопределённыхданных", "predefineddataname":
		return true
	default:
		return false
	}
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
		referenced := item.Kind == EnumerationType || item.Kind == DefinedType || item.Kind == CatalogType || item.Kind == DocumentType
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
		case BooleanType, DateType, UUIDType, EnumerationType, DefinedType, CatalogType, DocumentType:
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

func cloneCatalogDefinition(value CatalogDefinition) CatalogDefinition {
	value.Title = cloneTitle(value.Title)
	value.Attributes = cloneAttributes(value.Attributes)
	value.TableParts = slices.Clone(value.TableParts)
	for index := range value.TableParts {
		value.TableParts[index].Title = cloneTitle(value.TableParts[index].Title)
		value.TableParts[index].Attributes = cloneAttributes(value.TableParts[index].Attributes)
	}
	if value.ObjectModule != nil {
		id := *value.ObjectModule
		value.ObjectModule = &id
	}
	if value.ManagerModule != nil {
		id := *value.ManagerModule
		value.ManagerModule = &id
	}
	value.Forms = cloneObjectForms(value.Forms)
	value.Predefined = slices.Clone(value.Predefined)
	for index := range value.Predefined {
		value.Predefined[index] = clonePredefinedCatalogItem(value.Predefined[index])
	}
	return value
}

func clonePredefinedCatalogItem(value PredefinedCatalogItem) PredefinedCatalogItem {
	if value.Attributes == nil {
		return value
	}
	value.Attributes = maps.Clone(value.Attributes)
	return value
}

func cloneAttributes(value []Attribute) []Attribute {
	result := slices.Clone(value)
	for index := range result {
		result[index].Title = cloneTitle(result[index].Title)
		result[index].Types = cloneTypes(result[index].Types)
	}
	return result
}
