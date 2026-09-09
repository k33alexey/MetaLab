package studio

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/k33alexey/MetaLab/internal/bsl/syntax"
	"github.com/k33alexey/MetaLab/internal/metadata"
	"github.com/k33alexey/MetaLab/internal/project"
	"github.com/k33alexey/MetaLab/internal/uuid"
	"go.yaml.in/yaml/v3"
)

const (
	maxBSLNavigationOccurrences = 2_000_000
	maxBSLNavigationLocations   = 2_000
	maxProjectSearchBytes       = 256 << 20
	maxProjectSearchResults     = 500
)

var (
	ErrBSLSymbolNotFound = errors.New("BSL symbol not found at cursor")
	ErrBSLRenameConflict = errors.New("BSL rename conflicts with an existing symbol")
)

// StudioLocation identifies a source location shown by navigation and search.
type StudioLocation struct {
	Path    string   `json:"path"`
	Range   BSLRange `json:"range"`
	Kind    string   `json:"kind"`
	Preview string   `json:"preview,omitempty"`
}

// BSLSymbolTarget describes the semantic symbol at the editor cursor.
type BSLSymbolTarget struct {
	Name       string         `json:"name"`
	Kind       string         `json:"kind"`
	Definition StudioLocation `json:"definition"`
	CanRename  bool           `json:"canRename"`
}

// BSLNavigation is a bounded definition or usages result.
type BSLNavigation struct {
	Symbol    BSLSymbolTarget  `json:"symbol"`
	Locations []StudioLocation `json:"locations"`
	Truncated bool             `json:"truncated,omitempty"`
}

// BSLRenameResult reports the files changed by one semantic rename.
type BSLRenameResult struct {
	OldName string     `json:"oldName"`
	NewName string     `json:"newName"`
	Changed []string   `json:"changed"`
	Current SourceFile `json:"current"`
}

// ProjectSearchResult is one full-text match in an ML Project source.
type ProjectSearchResult struct {
	Locations []StudioLocation `json:"locations"`
	Truncated bool             `json:"truncated,omitempty"`
}

type bslNavigationIndex struct {
	modules           map[string]bslSemanticModule
	publicModules     map[string]string
	moduleDefinitions map[string]StudioLocation
	metadata          map[string]bslResolvedSymbol
	truncated         bool
}

type bslSemanticModule struct {
	path         string
	name         string
	public       bool
	predefined   []string
	revision     string
	valid        bool
	lines        []string
	routines     []syntax.Span
	declarations []bslSemanticDeclaration
	byName       map[bslDeclarationKey]bslSemanticDeclaration
	occurrences  []bslSemanticOccurrence
}

type bslDeclarationKey struct {
	scope int
	name  string
	call  bool
}

type bslSemanticDeclaration struct {
	name     string
	kind     string
	span     syntax.Span
	scope    int
	exported bool
}

type bslSemanticOccurrence struct {
	name      string
	chain     []string
	component int
	call      bool
	span      syntax.Span
	scope     int
}

type bslResolvedSymbol struct {
	key        string
	name       string
	kind       string
	definition StudioLocation
	path       string
	scope      int
	exported   bool
	canRename  bool
}

type projectSearchIndex struct {
	files     []projectSearchFile
	truncated bool
}

type projectSearchFile struct {
	path    string
	kind    string
	content string
}

// NavigateBSL resolves a definition or semantic usages from unsaved editor text.
func (workspace *Workspace) NavigateBSL(relative, source string, position BSLPosition, mode string) (BSLNavigation, error) {
	relative, language, err := validateEditablePath(relative)
	if err != nil || language != "bsl" {
		return BSLNavigation{}, fmt.Errorf("BSL navigation requires a canonical project module path")
	}
	if mode != "definition" && mode != "usages" {
		return BSLNavigation{}, fmt.Errorf("BSL navigation mode must be definition or usages")
	}
	if err := validateEditorSource(source); err != nil {
		return BSLNavigation{}, err
	}
	offset, err := bslOffset(source, position)
	if err != nil {
		return BSLNavigation{}, err
	}

	workspace.mu.Lock()
	defer workspace.mu.Unlock()
	if workspace.bslNavigation == nil {
		workspace.bslNavigation, err = workspace.buildBSLNavigationIndex()
		if err != nil {
			return BSLNavigation{}, err
		}
	}
	index := workspace.bslNavigation
	current, ok := index.modules[relative]
	if !ok {
		return BSLNavigation{}, ErrBSLSymbolNotFound
	}
	current = buildBSLSemanticModule(relative, source, current.name, current.public, current.predefined)
	target, ok := resolveBSLSymbolAt(index, current, offset)
	if !ok {
		return BSLNavigation{}, ErrBSLSymbolNotFound
	}
	result := BSLNavigation{Symbol: publicBSLSymbol(target)}
	if mode == "definition" {
		result.Locations = []StudioLocation{target.definition}
		return result, nil
	}
	modules := semanticModulesWithCurrent(index.modules, current)
	result.Locations, result.Truncated = collectBSLSymbolLocations(index, modules, target, false)
	result.Truncated = result.Truncated || index.truncated
	return result, nil
}

// RenameBSL performs a checked semantic rename on saved BSL sources.
func (workspace *Workspace) RenameBSL(relative string, position BSLPosition, newName, expectedRevision string) (BSLRenameResult, error) {
	relative, language, err := validateEditablePath(relative)
	if err != nil || language != "bsl" {
		return BSLRenameResult{}, fmt.Errorf("BSL rename requires a canonical project module path")
	}
	if !validBSLIdentifier(newName) {
		return BSLRenameResult{}, fmt.Errorf("new BSL name must be a non-keyword identifier")
	}

	workspace.mu.Lock()
	defer workspace.mu.Unlock()
	current, err := workspace.readSource(relative)
	if err != nil {
		return BSLRenameResult{}, err
	}
	if len(expectedRevision) != len(current.Revision) {
		return BSLRenameResult{}, fmt.Errorf("expected source revision is invalid")
	}
	if !strings.EqualFold(current.Revision, expectedRevision) {
		return BSLRenameResult{}, ErrSourceChanged
	}
	offset, err := bslOffset(current.Content, position)
	if err != nil {
		return BSLRenameResult{}, err
	}
	index, err := workspace.buildBSLNavigationIndex()
	if err != nil {
		return BSLRenameResult{}, err
	}
	if index.truncated {
		return BSLRenameResult{}, fmt.Errorf("safe rename is unavailable because the BSL index is truncated")
	}
	module, ok := index.modules[relative]
	if !ok {
		return BSLRenameResult{}, ErrBSLSymbolNotFound
	}
	target, ok := resolveBSLSymbolAt(index, module, offset)
	if !ok {
		return BSLRenameResult{}, ErrBSLSymbolNotFound
	}
	if !target.canRename {
		return BSLRenameResult{}, fmt.Errorf("%s cannot be renamed from BSL", target.kind)
	}
	if target.name == newName {
		return BSLRenameResult{}, fmt.Errorf("new BSL name is unchanged")
	}
	if renameConflicts(index, target, newName) {
		return BSLRenameResult{}, fmt.Errorf("%w: %s", ErrBSLRenameConflict, newName)
	}
	if err := ensureRenameSourcesValid(index, target); err != nil {
		return BSLRenameResult{}, err
	}

	locations, truncated := collectBSLSymbolLocations(index, index.modules, target, true)
	if truncated {
		return BSLRenameResult{}, fmt.Errorf("safe rename exceeds the limit of %d occurrences", maxBSLNavigationLocations)
	}
	if len(locations) == 0 {
		return BSLRenameResult{}, ErrBSLSymbolNotFound
	}
	byPath := make(map[string][]StudioLocation)
	for _, location := range locations {
		byPath[location.Path] = append(byPath[location.Path], location)
	}
	changes := make(map[string][]byte, len(byPath))
	originals := make(map[string][]byte, len(byPath))
	for path, items := range byPath {
		file, readErr := workspace.readSource(path)
		if readErr != nil {
			return BSLRenameResult{}, readErr
		}
		if file.Revision != index.modules[path].revision {
			return BSLRenameResult{}, ErrSourceChanged
		}
		updated, replaceErr := replaceBSLLocations(file.Content, items, newName)
		if replaceErr != nil {
			return BSLRenameResult{}, replaceErr
		}
		if err := validateEditorSource(updated); err != nil {
			return BSLRenameResult{}, fmt.Errorf("rename exceeds source limits in %s: %w", path, err)
		}
		if _, diagnostics := syntax.Parse(path, updated); len(diagnostics) != 0 {
			return BSLRenameResult{}, fmt.Errorf("rename produced invalid BSL in %s", path)
		}
		changes[path] = []byte(updated)
		originals[path] = []byte(file.Content)
	}
	if err := workspace.replaceSourcesLocked(changes, originals); err != nil {
		return BSLRenameResult{}, err
	}
	workspace.invalidateStudioIndexesLocked()
	changed := make([]string, 0, len(changes))
	for path := range changes {
		changed = append(changed, path)
	}
	sort.Strings(changed)
	updatedCurrent, err := workspace.readSource(relative)
	if err != nil {
		return BSLRenameResult{}, err
	}
	return BSLRenameResult{OldName: target.name, NewName: newName, Changed: changed, Current: updatedCurrent}, nil
}

// SearchProject performs a bounded case-insensitive search over YAML and BSL sources.
func (workspace *Workspace) SearchProject(query string) (ProjectSearchResult, error) {
	query = strings.TrimSpace(query)
	if query == "" || utf8.RuneCountInString(query) > 256 {
		return ProjectSearchResult{}, fmt.Errorf("search query must contain between 1 and 256 characters")
	}
	workspace.mu.Lock()
	defer workspace.mu.Unlock()
	var err error
	if workspace.projectSearch == nil {
		workspace.projectSearch, err = workspace.buildProjectSearchIndex()
		if err != nil {
			return ProjectSearchResult{}, err
		}
	}
	result := ProjectSearchResult{Locations: make([]StudioLocation, 0, 32), Truncated: workspace.projectSearch.truncated}
	needle := []rune(strings.ToLower(query))
	for _, file := range workspace.projectSearch.files {
		for lineIndex, line := range splitSourceLines(file.content) {
			lower := []rune(strings.ToLower(line))
			for start := indexRunes(lower, needle, 0); start >= 0; start = indexRunes(lower, needle, start+max(1, len(needle))) {
				if len(result.Locations) == maxProjectSearchResults {
					result.Truncated = true
					return result, nil
				}
				result.Locations = append(result.Locations, StudioLocation{
					Path: file.path, Kind: file.kind, Preview: compactPreview(line),
					Range: BSLRange{Start: BSLPosition{Line: lineIndex + 1, Column: start + 1}, End: BSLPosition{Line: lineIndex + 1, Column: start + len(needle) + 1}},
				})
			}
		}
	}
	return result, nil
}

func (workspace *Workspace) buildBSLNavigationIndex() (*bslNavigationIndex, error) {
	result := &bslNavigationIndex{
		modules: make(map[string]bslSemanticModule), publicModules: make(map[string]string),
		moduleDefinitions: make(map[string]StudioLocation), metadata: make(map[string]bslResolvedSymbol),
	}
	catalog, _ := metadata.Load(workspace.root)
	descriptors := workspace.moduleDescriptors(catalog)
	occurrenceCount := 0
	for _, directory := range []string{"modules", "tests"} {
		entries, err := os.ReadDir(filepath.Join(workspace.root, directory))
		if err != nil {
			return nil, fmt.Errorf("read BSL %s: %w", directory, err)
		}
		for _, entry := range entries {
			if entry.Name() == ".gitkeep" || entry.IsDir() || entry.Type()&os.ModeSymlink != 0 {
				continue
			}
			if len(result.modules) >= 10_000 {
				result.truncated = true
				continue
			}
			relative := filepath.ToSlash(filepath.Join(directory, entry.Name()))
			file, err := workspace.readSource(relative)
			if err != nil {
				return nil, err
			}
			id := strings.TrimSuffix(entry.Name(), ".bsl")
			descriptor := descriptors[id]
			if descriptor.name == "" {
				descriptor.name = "Модуль" + strings.ReplaceAll(id, "-", "")
				if directory == "tests" {
					descriptor.name = "Тест" + strings.ReplaceAll(id, "-", "")
				}
			}
			module := buildBSLSemanticModule(relative, file.Content, descriptor.name, descriptor.public, descriptor.predefined)
			module.revision = file.Revision
			if occurrenceCount+len(module.occurrences) > maxBSLNavigationOccurrences {
				result.truncated = true
				continue
			}
			result.modules[relative] = module
			occurrenceCount += len(module.occurrences)
			if module.public {
				result.publicModules[strings.ToLower(module.name)] = relative
			}
		}
	}
	workspace.indexMetadataDefinitions(result)
	return result, nil
}

func buildBSLSemanticModule(path, source, name string, public bool, predefined []string) bslSemanticModule {
	parsed, tokens, diagnostics := syntax.ParseWithTokens(path, source)
	result := bslSemanticModule{path: path, name: name, public: public, predefined: append([]string(nil), predefined...), valid: len(diagnostics) == 0, lines: splitSourceLines(source)}
	for _, routine := range parsed.Routines {
		result.routines = append(result.routines, routine.SourceSpan)
	}
	for _, variable := range parsed.Variables {
		span := identifierTokenSpan(tokens, variable.SourceSpan, variable.Name)
		result.declarations = append(result.declarations, bslSemanticDeclaration{name: variable.Name, kind: "variable", span: span, scope: -1, exported: variable.Export})
	}
	for routineIndex, routine := range parsed.Routines {
		kind := "procedure"
		if routine.Function {
			kind = "function"
		}
		result.declarations = append(result.declarations, bslSemanticDeclaration{
			name: routine.Name, kind: kind, span: routineDeclarationSpan(tokens, routine), scope: -1, exported: routine.Export,
		})
		addRoutineDeclarations(&result, tokens, routine, routineIndex)
	}
	result.byName = make(map[bslDeclarationKey]bslSemanticDeclaration, len(result.declarations))
	for _, declaration := range result.declarations {
		call := declaration.kind == "function" || declaration.kind == "procedure"
		key := bslDeclarationKey{scope: declaration.scope, name: strings.ToLower(declaration.name), call: call}
		if _, exists := result.byName[key]; exists {
			result.valid = false
			continue
		}
		result.byName[key] = declaration
	}
	result.occurrences = semanticOccurrences(tokens, result.routines)
	return result
}

func addRoutineDeclarations(module *bslSemanticModule, tokens []syntax.Token, routine *syntax.Routine, scope int) {
	seen := make(map[string]bool)
	moduleNames := make(map[string]bool)
	for _, declaration := range module.declarations {
		if declaration.scope == -1 && declaration.kind == "variable" {
			moduleNames[strings.ToLower(declaration.name)] = true
		}
	}
	for _, name := range module.predefined {
		moduleNames[strings.ToLower(name)] = true
	}
	for _, parameter := range routine.Parameters {
		key := strings.ToLower(parameter.Name)
		seen[key] = true
		module.declarations = append(module.declarations, bslSemanticDeclaration{
			name: parameter.Name, kind: "parameter", span: identifierTokenSpan(tokens, parameter.SourceSpan, parameter.Name), scope: scope,
		})
	}
	var explicit, candidates []bslSemanticDeclaration
	walkBSLStatements(routine.Body, func(statement syntax.Statement) {
		switch value := statement.(type) {
		case *syntax.VariableStatement:
			for _, variable := range value.Variables {
				explicit = append(explicit, bslSemanticDeclaration{name: variable.Name, kind: "variable", span: identifierTokenSpan(tokens, variable.SourceSpan, variable.Name), scope: scope})
			}
		case *syntax.AssignmentStatement:
			if identifier, ok := value.Target.(*syntax.IdentifierExpression); ok {
				candidates = append(candidates, bslSemanticDeclaration{name: identifier.Name, kind: "variable", span: identifier.SourceSpan, scope: scope})
			}
		case *syntax.ForStatement:
			candidates = append(candidates, bslSemanticDeclaration{name: value.Variable, kind: "variable", span: loopVariableSpan(tokens, value.SourceSpan, value.Variable), scope: scope})
		case *syntax.ForEachStatement:
			candidates = append(candidates, bslSemanticDeclaration{name: value.Variable, kind: "variable", span: loopVariableSpan(tokens, value.SourceSpan, value.Variable), scope: scope})
		}
	})
	sort.SliceStable(explicit, func(left, right int) bool {
		return explicit[left].span.Start.Offset < explicit[right].span.Start.Offset
	})
	for _, declaration := range explicit {
		key := strings.ToLower(declaration.name)
		if key == "" {
			continue
		}
		if seen[key] {
			module.valid = false
			continue
		}
		seen[key] = true
		module.declarations = append(module.declarations, declaration)
	}
	sort.SliceStable(candidates, func(left, right int) bool {
		return candidates[left].span.Start.Offset < candidates[right].span.Start.Offset
	})
	for _, candidate := range candidates {
		key := strings.ToLower(candidate.name)
		if key == "" || seen[key] || moduleNames[key] {
			continue
		}
		seen[key] = true
		module.declarations = append(module.declarations, candidate)
	}
}

func walkBSLStatements(statements []syntax.Statement, visit func(syntax.Statement)) {
	for _, statement := range statements {
		visit(statement)
		switch value := statement.(type) {
		case *syntax.IfStatement:
			for _, branch := range value.Branches {
				walkBSLStatements(branch.Body, visit)
			}
			walkBSLStatements(value.ElseBody, visit)
		case *syntax.WhileStatement:
			walkBSLStatements(value.Body, visit)
		case *syntax.ForStatement:
			walkBSLStatements(value.Body, visit)
		case *syntax.ForEachStatement:
			walkBSLStatements(value.Body, visit)
		case *syntax.TryStatement:
			walkBSLStatements(value.Body, visit)
			walkBSLStatements(value.ExceptBody, visit)
		}
	}
}

func semanticOccurrences(tokens []syntax.Token, routines []syntax.Span) []bslSemanticOccurrence {
	result := make([]bslSemanticOccurrence, 0, len(tokens)/3)
	for index := 0; index < len(tokens); {
		if tokens[index].Kind != syntax.Identifier {
			index++
			continue
		}
		indices := []int{index}
		cursor := index + 1
		for cursor+1 < len(tokens) && (tokens[cursor].Kind == syntax.Dot || tokens[cursor].Kind == syntax.DotTrailing) && tokens[cursor+1].Kind == syntax.Identifier {
			indices = append(indices, cursor+1)
			cursor += 2
		}
		chain := make([]string, len(indices))
		for part, tokenIndex := range indices {
			chain[part] = strings.ToLower(tokens[tokenIndex].Value)
		}
		for part, tokenIndex := range indices {
			token := tokens[tokenIndex]
			result = append(result, bslSemanticOccurrence{
				name: token.Value, chain: chain, component: part, call: tokenIndex+1 < len(tokens) && tokens[tokenIndex+1].Kind == syntax.LeftParen, span: token.Span,
				scope: routineScopeAt(routines, token.Span.Start.Offset),
			})
		}
		index = cursor
	}
	return result
}

func resolveBSLSymbolAt(index *bslNavigationIndex, module bslSemanticModule, offset int) (bslResolvedSymbol, bool) {
	for _, occurrence := range module.occurrences {
		if offset >= occurrence.span.Start.Offset && offset <= occurrence.span.End.Offset {
			return resolveBSLOccurrence(index, module, occurrence)
		}
	}
	return bslResolvedSymbol{}, false
}

func resolveBSLOccurrence(index *bslNavigationIndex, module bslSemanticModule, occurrence bslSemanticOccurrence) (bslResolvedSymbol, bool) {
	if len(occurrence.chain) > 1 {
		if modulePath, ok := index.publicModules[occurrence.chain[0]]; ok {
			if occurrence.component == 0 {
				location, exists := index.moduleDefinitions[occurrence.chain[0]]
				if exists {
					return externalBSLSymbol("module:"+modulePath, occurrence.name, "module", location), true
				}
			}
			if occurrence.component == 1 {
				if declaration, exists := findModuleDeclaration(index.modules[modulePath], occurrence.name, true, occurrence.call); exists {
					return declarationBSLSymbol(index.modules[modulePath], declaration), true
				}
			}
		}
		if occurrence.component > 0 {
			metadataKey := strings.Join(occurrence.chain[:occurrence.component+1], "\x00")
			if symbol, ok := index.metadata[metadataKey]; ok {
				return symbol, true
			}
		}
		return bslResolvedSymbol{}, false
	}
	if occurrence.call {
		if declaration, ok := findModuleDeclaration(module, occurrence.name, false, true); ok {
			return declarationBSLSymbol(module, declaration), true
		}
		return bslResolvedSymbol{}, false
	}
	if occurrence.scope >= 0 {
		if declaration, ok := findScopedDeclaration(module, occurrence.name, occurrence.scope); ok {
			return declarationBSLSymbol(module, declaration), true
		}
	}
	if declaration, ok := findModuleDeclaration(module, occurrence.name, false, false); ok {
		return declarationBSLSymbol(module, declaration), true
	}
	if modulePath, ok := index.publicModules[strings.ToLower(occurrence.name)]; ok {
		location, exists := index.moduleDefinitions[strings.ToLower(occurrence.name)]
		if exists {
			return externalBSLSymbol("module:"+modulePath, occurrence.name, "module", location), true
		}
	}
	return bslResolvedSymbol{}, false
}

func declarationBSLSymbol(module bslSemanticModule, declaration bslSemanticDeclaration) bslResolvedSymbol {
	location := StudioLocation{Path: module.path, Range: editorRange(declaration.span), Kind: declaration.kind, Preview: declaration.name}
	return bslResolvedSymbol{
		key:  fmt.Sprintf("bsl:%s:%d:%d", module.path, declaration.scope, declaration.span.Start.Offset),
		name: declaration.name, kind: declaration.kind, definition: location, path: module.path,
		scope: declaration.scope, exported: module.public && declaration.exported, canRename: true,
	}
}

func externalBSLSymbol(key, name, kind string, location StudioLocation) bslResolvedSymbol {
	return bslResolvedSymbol{key: key, name: name, kind: kind, definition: location, path: location.Path}
}

func publicBSLSymbol(symbol bslResolvedSymbol) BSLSymbolTarget {
	return BSLSymbolTarget{Name: symbol.name, Kind: symbol.kind, Definition: symbol.definition, CanRename: symbol.canRename}
}

func collectBSLSymbolLocations(index *bslNavigationIndex, modules map[string]bslSemanticModule, target bslResolvedSymbol, includeDefinition bool) ([]StudioLocation, bool) {
	paths := make([]string, 0, len(modules))
	for path := range modules {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	locations := make([]StudioLocation, 0, 16)
	for _, path := range paths {
		module := modules[path]
		for _, occurrence := range module.occurrences {
			resolved, ok := resolveBSLOccurrence(index, module, occurrence)
			if !ok || resolved.key != target.key {
				continue
			}
			location := StudioLocation{Path: path, Range: editorRange(occurrence.span), Kind: target.kind, Preview: sourceLinePreview(module.lines, occurrence.span.Start.Line)}
			if !includeDefinition && sameStudioLocation(location, target.definition) {
				continue
			}
			if len(locations) == maxBSLNavigationLocations {
				return locations, true
			}
			locations = append(locations, location)
		}
	}
	return locations, false
}

func semanticModulesWithCurrent(modules map[string]bslSemanticModule, current bslSemanticModule) map[string]bslSemanticModule {
	result := make(map[string]bslSemanticModule, len(modules))
	for path, module := range modules {
		result[path] = module
	}
	result[current.path] = current
	return result
}

func findScopedDeclaration(module bslSemanticModule, name string, scope int) (bslSemanticDeclaration, bool) {
	declaration, ok := module.byName[bslDeclarationKey{scope: scope, name: strings.ToLower(name)}]
	return declaration, ok
}

func findModuleDeclaration(module bslSemanticModule, name string, exportedOnly, call bool) (bslSemanticDeclaration, bool) {
	declaration, ok := module.byName[bslDeclarationKey{scope: -1, name: strings.ToLower(name), call: call}]
	if !ok || exportedOnly && !declaration.exported {
		return bslSemanticDeclaration{}, false
	}
	return declaration, true
}

func renameConflicts(index *bslNavigationIndex, target bslResolvedSymbol, newName string) bool {
	module := index.modules[target.path]
	key := strings.ToLower(newName)
	if standaloneBSLSignatures[key] || reservedBSLNavigationNames[key] {
		return true
	}
	if _, exists := index.publicModules[key]; exists {
		return true
	}
	for _, predefined := range module.predefined {
		if strings.EqualFold(predefined, newName) {
			return true
		}
	}
	for _, declaration := range module.declarations {
		if editorRange(declaration.span) == target.definition.Range && strings.EqualFold(declaration.name, target.name) {
			continue
		}
		if strings.EqualFold(declaration.name, newName) {
			return true
		}
	}
	return false
}

var reservedBSLNavigationNames = map[string]bool{
	"справочники": true, "catalogs": true, "документы": true, "documents": true,
	"регистрысведений": true, "informationregisters": true, "регистрынакопления": true, "accumulationregisters": true,
	"константы": true, "constants": true, "перечисления": true, "enums": true,
	"определяемыетипы": true, "definedtypes": true, "этотобъект": true, "thisobject": true,
	"движения": true, "movements": true, "режимблокировкиданных": true, "datalockmode": true,
	"виддвижениянакопления": true, "accumulationmovementkind": true,
	"режимзаписидокумента": true, "documentwritemode": true,
	"режимпроведениядокумента": true, "documentpostingmode": true,
}

func ensureRenameSourcesValid(index *bslNavigationIndex, target bslResolvedSymbol) error {
	for path, module := range index.modules {
		if !target.exported && path != target.path {
			continue
		}
		if !module.valid {
			return fmt.Errorf("safe rename requires valid BSL source: %s", path)
		}
	}
	return nil
}

func replaceBSLLocations(source string, locations []StudioLocation, newName string) (string, error) {
	type edit struct{ start, end int }
	edits := make([]edit, 0, len(locations))
	for _, location := range locations {
		start, err := bslOffset(source, location.Range.Start)
		if err != nil {
			return "", err
		}
		end, err := bslOffset(source, location.Range.End)
		if err != nil || end < start {
			return "", fmt.Errorf("invalid BSL rename range")
		}
		edits = append(edits, edit{start: start, end: end})
	}
	sort.Slice(edits, func(left, right int) bool { return edits[left].start > edits[right].start })
	result := source
	last := len(source) + 1
	for _, item := range edits {
		if item.end > last || item.start < 0 || item.end > len(result) {
			return "", fmt.Errorf("overlapping BSL rename ranges")
		}
		result = result[:item.start] + newName + result[item.end:]
		last = item.start
	}
	return result, nil
}

func validBSLIdentifier(value string) bool {
	if value == "" || !utf8.ValidString(value) || utf8.RuneCountInString(value) > 128 {
		return false
	}
	for index, character := range []rune(value) {
		if index == 0 {
			if character != '_' && !unicode.IsLetter(character) {
				return false
			}
		} else if character != '_' && !unicode.IsLetter(character) && !unicode.IsDigit(character) {
			return false
		}
	}
	_, keyword := syntax.KeywordKind(value)
	return !keyword
}

func validateEditorSource(source string) error {
	if len(source) > MaxEditableFileBytes || !utf8.ValidString(source) || strings.IndexByte(source, 0) >= 0 {
		return fmt.Errorf("editable source must be valid UTF-8 and at most %d bytes", MaxEditableFileBytes)
	}
	return nil
}

func routineScopeAt(routines []syntax.Span, offset int) int {
	index := sort.Search(len(routines), func(index int) bool { return routines[index].End.Offset >= offset })
	if index < len(routines) && offset >= routines[index].Start.Offset {
		return index
	}
	return -1
}

func identifierTokenSpan(tokens []syntax.Token, span syntax.Span, name string) syntax.Span {
	for _, token := range tokens[firstBSLTokenAt(tokens, span.Start.Offset):] {
		if token.Span.Start.Offset >= span.End.Offset {
			break
		}
		if token.Span.Start.Offset >= span.Start.Offset && token.Span.End.Offset <= span.End.Offset && token.Kind == syntax.Identifier && strings.EqualFold(token.Value, name) {
			return token.Span
		}
	}
	return span
}

func routineDeclarationSpan(tokens []syntax.Token, routine *syntax.Routine) syntax.Span {
	seenDeclaration := false
	for _, token := range tokens[firstBSLTokenAt(tokens, routine.SourceSpan.Start.Offset):] {
		if token.Span.Start.Offset >= routine.SourceSpan.End.Offset {
			break
		}
		if token.Span.Start.Offset < routine.SourceSpan.Start.Offset || token.Span.End.Offset > routine.SourceSpan.End.Offset {
			continue
		}
		if token.Kind == syntax.Function || token.Kind == syntax.Procedure {
			seenDeclaration = true
			continue
		}
		if seenDeclaration && token.Kind == syntax.Identifier && strings.EqualFold(token.Value, routine.Name) {
			return token.Span
		}
	}
	return routine.SourceSpan
}

func loopVariableSpan(tokens []syntax.Token, span syntax.Span, name string) syntax.Span {
	seenFor := false
	for _, token := range tokens[firstBSLTokenAt(tokens, span.Start.Offset):] {
		if token.Span.Start.Offset >= span.End.Offset {
			break
		}
		if token.Span.Start.Offset < span.Start.Offset || token.Span.End.Offset > span.End.Offset {
			continue
		}
		if token.Kind == syntax.For {
			seenFor = true
			continue
		}
		if seenFor && token.Kind == syntax.Identifier && strings.EqualFold(token.Value, name) {
			return token.Span
		}
	}
	return span
}

func firstBSLTokenAt(tokens []syntax.Token, offset int) int {
	return sort.Search(len(tokens), func(index int) bool { return tokens[index].Span.End.Offset >= offset })
}

func sameStudioLocation(left, right StudioLocation) bool {
	return left.Path == right.Path && left.Range == right.Range
}

func sourceLinePreview(lines []string, line int) string {
	if line < 1 || line > len(lines) {
		return ""
	}
	return compactPreview(lines[line-1])
}

func compactPreview(value string) string {
	value = strings.TrimSpace(value)
	characters := []rune(value)
	if len(characters) > 240 {
		return string(characters[:240]) + "…"
	}
	return value
}

func splitSourceLines(source string) []string {
	normalized := strings.ReplaceAll(source, "\r\n", "\n")
	normalized = strings.ReplaceAll(normalized, "\r", "\n")
	return strings.Split(normalized, "\n")
}

func indexRunes(value, search []rune, start int) int {
	if len(search) == 0 || start < 0 {
		return -1
	}
	for index := start; index+len(search) <= len(value); index++ {
		matched := true
		for offset := range search {
			if value[index+offset] != search[offset] {
				matched = false
				break
			}
		}
		if matched {
			return index
		}
	}
	return -1
}

func (workspace *Workspace) indexMetadataDefinitions(index *bslNavigationIndex) {
	aliases := map[string][]string{
		"catalogs": {"справочники", "catalogs"}, "documents": {"документы", "documents"},
		"information-registers":  {"регистрысведений", "informationregisters"},
		"accumulation-registers": {"регистрынакопления", "accumulationregisters"},
		"constants":              {"константы", "constants"}, "enumerations": {"перечисления", "enums"},
		"defined-types": {"определяемыетипы", "definedtypes"},
	}
	for kind, roots := range aliases {
		entries, err := os.ReadDir(filepath.Join(workspace.root, "metadata", kind))
		if err != nil {
			continue
		}
		for _, entry := range entries {
			if entry.IsDir() || entry.Type()&os.ModeSymlink != 0 || filepath.Ext(entry.Name()) != ".yaml" {
				continue
			}
			path := filepath.ToSlash(filepath.Join("metadata", kind, entry.Name()))
			file, err := workspace.readSource(path)
			if err != nil {
				continue
			}
			name, nameRange, values := yamlNamedDefinitions(file.Content)
			if name == "" {
				continue
			}
			location := StudioLocation{Path: path, Range: nameRange, Kind: "metadata", Preview: name}
			for _, root := range roots {
				key := strings.ToLower(root) + "\x00" + strings.ToLower(name)
				index.metadata[key] = externalBSLSymbol("metadata:"+path, name, "metadata", location)
				if kind == "enumerations" {
					for value, valueRange := range values {
						valueLocation := StudioLocation{Path: path, Range: valueRange, Kind: "enum-value", Preview: value}
						valueKey := key + "\x00" + strings.ToLower(value)
						index.metadata[valueKey] = externalBSLSymbol("metadata:"+path+":"+strings.ToLower(value), value, "enum-value", valueLocation)
					}
				}
			}
		}
	}
	entries, err := os.ReadDir(filepath.Join(workspace.root, "metadata", "common-modules"))
	if err != nil {
		return
	}
	for _, entry := range entries {
		if entry.IsDir() || entry.Type()&os.ModeSymlink != 0 || filepath.Ext(entry.Name()) != ".yaml" {
			continue
		}
		path := filepath.ToSlash(filepath.Join("metadata", "common-modules", entry.Name()))
		file, readErr := workspace.readSource(path)
		if readErr != nil {
			continue
		}
		var descriptor struct {
			Name   string    `yaml:"name"`
			Module uuid.UUID `yaml:"module"`
		}
		if yaml.Unmarshal([]byte(file.Content), &descriptor) != nil || descriptor.Name == "" || descriptor.Module.IsZero() {
			continue
		}
		modulePath, pathErr := project.ModulePath(descriptor.Module)
		if pathErr != nil {
			continue
		}
		_, nameRange, _ := yamlNamedDefinitions(file.Content)
		index.moduleDefinitions[strings.ToLower(descriptor.Name)] = StudioLocation{Path: path, Range: nameRange, Kind: "module", Preview: descriptor.Name}
		index.publicModules[strings.ToLower(descriptor.Name)] = modulePath
	}
}

func yamlNamedDefinitions(source string) (string, BSLRange, map[string]BSLRange) {
	var document yaml.Node
	if yaml.Unmarshal([]byte(source), &document) != nil || len(document.Content) == 0 {
		return "", BSLRange{}, nil
	}
	root := document.Content[0]
	nameNode := yamlMappingValue(root, "name")
	if nameNode == nil {
		return "", BSLRange{}, nil
	}
	values := make(map[string]BSLRange)
	if sequence := yamlMappingValue(root, "values"); sequence != nil && sequence.Kind == yaml.SequenceNode {
		for _, item := range sequence.Content {
			if valueName := yamlMappingValue(item, "name"); valueName != nil && valueName.Value != "" {
				values[valueName.Value] = yamlNodeRange(valueName)
			}
		}
	}
	return nameNode.Value, yamlNodeRange(nameNode), values
}

func yamlMappingValue(node *yaml.Node, key string) *yaml.Node {
	if node == nil || node.Kind != yaml.MappingNode {
		return nil
	}
	for index := 0; index+1 < len(node.Content); index += 2 {
		if node.Content[index].Value == key {
			return node.Content[index+1]
		}
	}
	return nil
}

func yamlNodeRange(node *yaml.Node) BSLRange {
	length := utf8.RuneCountInString(node.Value)
	return BSLRange{Start: BSLPosition{Line: node.Line, Column: node.Column}, End: BSLPosition{Line: node.Line, Column: node.Column + length}}
}

func (workspace *Workspace) buildProjectSearchIndex() (*projectSearchIndex, error) {
	result := &projectSearchIndex{}
	paths := []string{project.ManifestFile}
	for _, directory := range []string{"modules", "tests", "forms", "reports"} {
		entries, err := os.ReadDir(filepath.Join(workspace.root, directory))
		if err != nil {
			return nil, err
		}
		for _, entry := range entries {
			if !entry.IsDir() && entry.Type()&os.ModeSymlink == 0 && entry.Name() != ".gitkeep" {
				paths = append(paths, filepath.ToSlash(filepath.Join(directory, entry.Name())))
			}
		}
	}
	for _, kind := range project.MetadataKinds() {
		entries, err := os.ReadDir(filepath.Join(workspace.root, "metadata", kind))
		if err != nil {
			continue
		}
		for _, entry := range entries {
			if !entry.IsDir() && entry.Type()&os.ModeSymlink == 0 && entry.Name() != ".gitkeep" {
				paths = append(paths, filepath.ToSlash(filepath.Join("metadata", kind, entry.Name())))
			}
		}
	}
	sort.Strings(paths)
	total := 0
	for _, path := range paths {
		file, err := workspace.readSource(path)
		if err != nil {
			continue
		}
		if total+len(file.Content) > maxProjectSearchBytes {
			result.truncated = true
			continue
		}
		total += len(file.Content)
		result.files = append(result.files, projectSearchFile{path: path, kind: file.Language, content: file.Content})
	}
	return result, nil
}
