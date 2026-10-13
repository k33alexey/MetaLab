package kinds

import (
	_ "embed"
	"fmt"
	"slices"
	"strings"

	"go.yaml.in/yaml/v3"
)

// Member is a property or a method of a type of the platform, under the names
// the help gives it.
type Member struct {
	Name   Words
	Method bool
	// Parameters are a method's, in order, as the help names them.
	Parameters []Parameter
}

// Parameter is one parameter of a method. Default is the value an optional
// parameter takes when left out, written as code; empty where the help does
// not say or the parameter is required.
type Parameter struct {
	Name     string
	Optional bool
	Default  string
}

// Label is the parameter as a signature shows it: Замещать = Истина.
func (parameter Parameter) Label() string {
	if parameter.Default != "" {
		return parameter.Name + " = " + parameter.Default
	}
	return parameter.Name
}

// ObjectType is the type of the object a module runs around: СправочникОбъект
// in an object module of a catalog, РегистрСведенийНаборЗаписей in a record
// set module, КонстантаМенеджерЗначения in the value module of a constant.
type ObjectType struct {
	Name Words
	// Members are the type's whatever the object; what depends on the object
	// the model adds (metadata.Catalog.ThisObjectMembers).
	Members []Member
	// Declared says the object's own fields are members too: the attributes,
	// table parts and fields an object declares, its standard attributes and
	// table parts and the version of its data. Only an object has them; a
	// record set's fields are those of its records, and a constant's value
	// manager holds one value.
	Declared bool
}

// thisObjects is the type of ЭтотОбъект in a module of an object of the kind,
// by the module's role, named by its Russian name in objects.yaml. A table of
// an external source is an object or a record set by the table (Layout) and
// keeps the module of either.
var thisObjects = map[Kind]map[Module]string{
	Catalog:                    {ObjectModule: "СправочникОбъект"},
	Document:                   {ObjectModule: "ДокументОбъект"},
	ChartOfCharacteristicTypes: {ObjectModule: "ПланВидовХарактеристикОбъект"},
	ChartOfAccounts:            {ObjectModule: "ПланСчетовОбъект"},
	ChartOfCalculationTypes:    {ObjectModule: "ПланВидовРасчетаОбъект"},
	BusinessProcess:            {ObjectModule: "БизнесПроцессОбъект"},
	Task:                       {ObjectModule: "ЗадачаОбъект"},
	ExchangePlan:               {ObjectModule: "ПланОбменаОбъект"},
	Report:                     {ObjectModule: "ОтчетОбъект"},
	DataProcessor:              {ObjectModule: "ОбработкаОбъект"},
	ExternalDataSourceTable: {ObjectModule: "ВнешнийИсточникДанныхТаблицаОбъект",
		RecordSetModule: "ВнешнийИсточникДанныхТаблицаНаборЗаписей"},
	ExternalDimensionTable: {ObjectModule: "ВнешнийИсточникДанныхКубТаблицаИзмеренияОбъект"},
	InformationRegister:    {RecordSetModule: "РегистрСведенийНаборЗаписей"},
	AccumulationRegister:   {RecordSetModule: "РегистрНакопленияНаборЗаписей"},
	AccountingRegister:     {RecordSetModule: "РегистрБухгалтерииНаборЗаписей"},
	CalculationRegister:    {RecordSetModule: "РегистрРасчетаНаборЗаписей"},
	Recalculation:          {RecordSetModule: "ПерерасчетНаборЗаписей"},
	Sequence:               {RecordSetModule: "ПоследовательностьНаборЗаписей"},
	ExternalCube:           {RecordSetModule: "ВнешнийИсточникДанныхКубНаборЗаписей"},
	Constant:               {ValueModule: "КонстантаМенеджерЗначения"},
}

//go:embed objects.yaml
var objectsSource []byte

// objectTypes are the types of objects.yaml by their Russian name.
var objectTypes = readObjectTypes("objects.yaml", objectsSource)

// readObjectTypes reads a file of types written as objects.yaml is, named
// for its panics.
func readObjectTypes(file string, source []byte) map[string]ObjectType {
	var described []struct {
		Type       []string   `yaml:"type"`
		Methods    [][]string `yaml:"methods"`
		Properties [][]string `yaml:"properties"`
	}
	if err := yaml.Unmarshal(source, &described); err != nil {
		panic(fmt.Sprintf("kinds: %s: %v", file, err))
	}
	result := make(map[string]ObjectType, len(described))
	for _, item := range described {
		if len(item.Type) != 2 {
			panic(fmt.Sprintf("kinds: %s: type %v must be named in two languages", file, item.Type))
		}
		objectType := ObjectType{Name: Words{item.Type[0], item.Type[1]}}
		for _, method := range item.Methods {
			objectType.Members = append(objectType.Members, readMember(file, objectType.Name, method, true))
		}
		for _, property := range item.Properties {
			if len(property) != 2 {
				panic(fmt.Sprintf("kinds: %s: property %v of %s takes no parameters", file, property, objectType.Name.Russian))
			}
			objectType.Members = append(objectType.Members, readMember(file, objectType.Name, property, false))
		}
		result[objectType.Name.Russian] = objectType
	}
	return result
}

func readMember(file string, owner Words, written []string, method bool) Member {
	if len(written) < 2 {
		panic(fmt.Sprintf("kinds: %s: member %v of %s must be named in two languages", file, written, owner.Russian))
	}
	member := Member{Name: Words{written[0], written[1]}, Method: method}
	for _, parameter := range written[2:] {
		name, value, defaulted := strings.Cut(parameter, " = ")
		name, optional := strings.CutSuffix(name, "?")
		member.Parameters = append(member.Parameters, Parameter{Name: name, Optional: optional || defaulted, Default: value})
	}
	return member
}

// ThisObject is the type of ЭтотОбъект in the module of an object of the kind
// in the role, the members it has whatever the object among them. False is a
// module whose ЭтотОбъект is no object of data - a form, a command, a common
// module - or a role the kind has no such module in.
func (descriptor Descriptor) ThisObject(module Module) (ObjectType, bool) {
	name, ok := thisObjects[descriptor.Kind][module]
	if !ok {
		return ObjectType{}, false
	}
	objectType := objectTypes[name]
	objectType.Members = slices.Clone(objectType.Members)
	objectType.Declared = module == ObjectModule
	return objectType, true
}
