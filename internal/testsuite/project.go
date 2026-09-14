package testsuite

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/k33alexey/MetaLab/internal/bsl/bytecode"
	"github.com/k33alexey/MetaLab/internal/bsl/compiler"
	"github.com/k33alexey/MetaLab/internal/bsl/syntax"
	"github.com/k33alexey/MetaLab/internal/metadata"
	"github.com/k33alexey/MetaLab/internal/uuid"
)

const (
	maxProjectModules = 10_000
	maxProjectSource  = 16 << 20
)

type moduleDescriptor struct {
	name           string
	predefined     []string
	defaultContext syntax.ExecutionContext
}

// CompileProject builds one test program from application and test modules.
func CompileProject(root string) (*bytecode.Program, error) {
	catalog, err := metadata.Load(root)
	if err != nil {
		return nil, err
	}
	descriptors := projectModuleDescriptors(catalog)
	sources := make([]compiler.ModuleSource, 0, 32)
	sourceBytes := 0
	for _, directory := range []string{"modules", "tests"} {
		entries, err := os.ReadDir(filepath.Join(root, directory))
		if err != nil {
			return nil, fmt.Errorf("read BSL %s: %w", directory, err)
		}
		for _, entry := range entries {
			if entry.IsDir() || entry.Type()&os.ModeSymlink != 0 || filepath.Ext(entry.Name()) != ".bsl" {
				continue
			}
			if len(sources) >= maxProjectModules {
				return nil, fmt.Errorf("test runner supports at most %d BSL modules", maxProjectModules)
			}
			relative := filepath.ToSlash(filepath.Join(directory, entry.Name()))
			content, err := readBoundedFile(filepath.Join(root, filepath.FromSlash(relative)), maxProjectSource-sourceBytes)
			if err != nil {
				return nil, fmt.Errorf("read %s: %w", relative, err)
			}
			if len(content) > maxProjectSource-sourceBytes {
				return nil, fmt.Errorf("test BSL source exceeds %d bytes", maxProjectSource)
			}
			sourceBytes += len(content)
			id := strings.TrimSuffix(entry.Name(), ".bsl")
			descriptor := descriptors[id]
			if descriptor.name == "" {
				descriptor.name = "Модуль" + strings.ReplaceAll(id, "-", "")
				if directory == "tests" {
					descriptor.name = "Тест" + strings.ReplaceAll(id, "-", "")
				}
			}
			sources = append(sources, compiler.ModuleSource{
				Name: descriptor.name, Filename: relative, Source: string(content),
				PredefinedVariables: append([]string(nil), descriptor.predefined...),
				DefaultContext:      descriptor.defaultContext,
			})
		}
	}
	program, diagnostics := compiler.CompileModules(sources)
	if len(diagnostics) != 0 {
		return nil, fmt.Errorf("cannot run tests: %s", diagnostics[0].Error())
	}
	return program, nil
}

func projectModuleDescriptors(catalog *metadata.Catalog) map[string]moduleDescriptor {
	result := make(map[string]moduleDescriptor)
	add := func(id *uuid.UUID, name string, predefined ...string) {
		if id != nil {
			result[id.String()] = moduleDescriptor{name: name, predefined: predefined}
		}
	}
	for _, item := range catalog.Catalogs {
		add(item.ObjectModule, "МодульОбъектаСправочника."+item.Name, "ЭтотОбъект", "ThisObject")
		add(item.ManagerModule, "МодульМенеджераСправочника."+item.Name)
	}
	for _, item := range catalog.Documents {
		add(item.ObjectModule, "МодульОбъектаДокумента."+item.Name, "ЭтотОбъект", "ThisObject", "Движения", "Movements")
		add(item.ManagerModule, "МодульМенеджераДокумента."+item.Name)
	}
	for _, item := range catalog.InformationRegisters {
		add(item.RecordSetModule, "МодульНабораЗаписейРегистраСведений."+item.Name, "ЭтотОбъект", "ThisObject")
		add(item.ManagerModule, "МодульМенеджераРегистраСведений."+item.Name)
	}
	for _, item := range catalog.AccumulationRegisters {
		add(item.RecordSetModule, "МодульНабораЗаписейРегистраНакопления."+item.Name, "ЭтотОбъект", "ThisObject")
		add(item.ManagerModule, "МодульМенеджераРегистраНакопления."+item.Name)
	}
	for _, item := range catalog.CommonModules {
		result[item.Module.String()] = moduleDescriptor{name: item.Name, defaultContext: item.DefaultContext()}
	}
	return result
}

func readBoundedFile(path string, maximum int) ([]byte, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	content, err := io.ReadAll(io.LimitReader(file, int64(maximum)+1))
	if err != nil {
		return nil, err
	}
	if len(content) > maximum {
		return nil, fmt.Errorf("file exceeds %d bytes", maximum)
	}
	return content, nil
}
