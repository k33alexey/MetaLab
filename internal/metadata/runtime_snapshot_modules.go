package metadata

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/k33alexey/MetaLab/internal/bsl/syntax"
	"github.com/k33alexey/MetaLab/internal/project"
	"github.com/k33alexey/MetaLab/internal/uuid"
)

const (
	maxProjectModules = 10_000
	maxProjectSource  = 16 << 20
)

type moduleNameDescriptor struct {
	name           string
	predefined     []string
	defaultContext syntax.ExecutionContext
}

// LoadProjectModules reads every project BSL module from disk and names each
// one canonically, ready to be persisted into a RuntimeSnapshot. This is the
// only place ML Project files are read for BSL purposes - ML Service itself
// never touches disk, it compiles from what "Сохранить данные" stores here.
func LoadProjectModules(root string, catalog *Catalog) ([]RuntimeModule, error) {
	descriptors := moduleNameDescriptors(catalog)
	var relativePaths []string
	entries, err := os.ReadDir(filepath.Join(root, "modules"))
	if err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("read BSL modules: %w", err)
	}
	for _, entry := range entries {
		if entry.IsDir() || entry.Type()&os.ModeSymlink != 0 || filepath.Ext(entry.Name()) != ".bsl" {
			continue
		}
		relativePaths = append(relativePaths, filepath.ToSlash(filepath.Join("modules", entry.Name())))
	}
	objectPaths, err := project.ObjectFolderSourcePaths(root)
	if err != nil {
		return nil, err
	}
	for _, relative := range objectPaths {
		if filepath.Ext(relative) == ".bsl" {
			relativePaths = append(relativePaths, relative)
		}
	}
	modules := make([]RuntimeModule, 0, len(relativePaths))
	sourceBytes := 0
	for _, relative := range relativePaths {
		if len(modules) >= maxProjectModules {
			return nil, fmt.Errorf("project supports at most %d BSL modules", maxProjectModules)
		}
		content, err := readBoundedModuleFile(filepath.Join(root, filepath.FromSlash(relative)), maxProjectSource-sourceBytes)
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", relative, err)
		}
		sourceBytes += len(content)
		id := strings.TrimSuffix(filepath.Base(relative), ".bsl")
		descriptor := descriptors[id]
		if descriptor.name == "" {
			descriptor.name = "Модуль" + strings.ReplaceAll(id, "-", "")
		}
		modules = append(modules, RuntimeModule{
			Name: descriptor.name, Filename: relative, Source: string(content),
			PredefinedVariables: append([]string(nil), descriptor.predefined...),
			DefaultContext:      descriptor.defaultContext,
		})
	}
	return modules, nil
}

func moduleNameDescriptors(catalog *Catalog) map[string]moduleNameDescriptor {
	result := make(map[string]moduleNameDescriptor)
	add := func(id *uuid.UUID, name string, predefined ...string) {
		if id != nil {
			result[id.String()] = moduleNameDescriptor{name: name, predefined: predefined}
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
		result[item.Module.String()] = moduleNameDescriptor{name: item.Name, defaultContext: item.DefaultContext()}
	}
	return result
}

func readBoundedModuleFile(path string, maximum int) ([]byte, error) {
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
