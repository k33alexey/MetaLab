package testsuite

import (
	"sort"
	"strings"
	"sync"

	"github.com/k33alexey/MetaLab/internal/bsl/bytecode"
)

type FileCoverage struct {
	Path    string  `json:"path"`
	Covered int     `json:"covered"`
	Total   int     `json:"total"`
	Percent float64 `json:"percent"`
}

type Coverage struct {
	Files   []FileCoverage `json:"files"`
	Covered int            `json:"covered"`
	Total   int            `json:"total"`
	Percent float64        `json:"percent"`
}

type lineKey struct {
	path string
	line int
}

type coverageCollector struct {
	program *bytecode.Program
	total   map[lineKey]struct{}
	mu      sync.Mutex
	covered map[lineKey]struct{}
}

func newCoverageCollector(program *bytecode.Program) *coverageCollector {
	collector := &coverageCollector{program: program, total: make(map[lineKey]struct{}), covered: make(map[lineKey]struct{})}
	if program == nil {
		return collector
	}
	for index := range program.Functions {
		function := &program.Functions[index]
		path := collector.functionPath(function)
		if !strings.HasPrefix(path, "modules/") {
			continue
		}
		for _, instruction := range function.Code {
			if instruction.Span.Start.Line > 0 {
				collector.total[lineKey{path: path, line: instruction.Span.Start.Line}] = struct{}{}
			}
		}
	}
	return collector
}

func (collector *coverageCollector) functionPath(function *bytecode.Function) string {
	if collector == nil || collector.program == nil || function == nil || int(function.Module) >= len(collector.program.Modules) {
		return ""
	}
	return collector.program.Modules[function.Module].Source
}

func (collector *coverageCollector) ObserveInstruction(function *bytecode.Function, instruction bytecode.Instruction) {
	path := collector.functionPath(function)
	key := lineKey{path: path, line: instruction.Span.Start.Line}
	if _, ok := collector.total[key]; !ok {
		return
	}
	collector.mu.Lock()
	collector.covered[key] = struct{}{}
	collector.mu.Unlock()
}

func (collector *coverageCollector) report() Coverage {
	collector.mu.Lock()
	covered := make(map[lineKey]struct{}, len(collector.covered))
	for key := range collector.covered {
		covered[key] = struct{}{}
	}
	collector.mu.Unlock()
	byPath := make(map[string]*FileCoverage)
	for key := range collector.total {
		item := byPath[key.path]
		if item == nil {
			item = &FileCoverage{Path: key.path}
			byPath[key.path] = item
		}
		item.Total++
		if _, ok := covered[key]; ok {
			item.Covered++
		}
	}
	result := Coverage{Files: make([]FileCoverage, 0, len(byPath))}
	for _, item := range byPath {
		item.Percent = coveragePercent(item.Covered, item.Total)
		result.Files = append(result.Files, *item)
		result.Covered += item.Covered
		result.Total += item.Total
	}
	sort.Slice(result.Files, func(left, right int) bool { return result.Files[left].Path < result.Files[right].Path })
	result.Percent = coveragePercent(result.Covered, result.Total)
	return result
}

func coveragePercent(covered, total int) float64 {
	if total == 0 {
		return 100
	}
	return float64(covered) * 100 / float64(total)
}
