package metadata

import (
	"fmt"
	"slices"
	"strconv"
	"strings"
)

// The XML schema of a package of exchanged types - «Экспорт XML-схемы…» of
// its editor (docs/block-00/paket-xdto.md, «Проверить пакет, экспорт схемы,
// окно типа», «Досъёмка 28.10.2026»; schemas the configurator wrote,
// configurator-2026-10-27/series-2.603/mlxdto1-3.xsd and
// configurator-2026-10-28/v5-xsd). The file is the prototype's form: a mark
// of byte order, lines ending CR LF, a tab a level. The root declares
// «nsN» for the imported namespaces in the order of their directives,
// «tns» for the package's own and «xs»; then come the imports, the global
// properties, the value types and the object types in the order of the
// package.
//
// Two rough edges of the prototype are not copied (the owner's rule of
// platform bugs): it writes the global properties in an order of its own,
// neither the package's nor the alphabet's (SOAP_Envelope_1_1.xsd), and it
// marks a global declaration form="qualified" when the form of the package
// is unqualified, which XML Schema does not allow on a global declaration -
// a global one is always qualified.

// XDTOSchemaIssue is what makes a package unfit for a schema: the code and
// the words of the prototype's check (paket-xdto.md, «Проверить пакет»).
type XDTOSchemaIssue struct {
	Code string `json:"code"`
	Type string `json:"type,omitempty"`
	Text string `json:"text"`
}

const xmlSchemaPrefix = "xs"

// ExportXDTOSchema writes the schema of the package, or answers the issues
// that keep it from being written.
func ExportXDTOSchema(content XDTOContent, catalog *Catalog) ([]byte, []XDTOSchemaIssue) {
	writer := xdtoSchemaWriter{content: content, packages: xdtoBodies{content: content, catalog: catalog}}
	if issues := CheckXDTOContent(content, catalog); len(issues) > 0 {
		return nil, issues
	}
	writer.schema()
	// The last line ends with no line break.
	return []byte("\uFEFF" + strings.TrimSuffix(writer.text.String(), "\r\n")), nil
}

type xdtoSchemaWriter struct {
	content  XDTOContent
	packages xdtoBodies
	text     strings.Builder
}

// line writes one line at its depth.
func (writer *xdtoSchemaWriter) line(depth int, text string) {
	writer.text.WriteString(strings.Repeat("\t", depth) + text + "\r\n")
}

// prefixed writes a name of the schema with the prefix of its namespace.
func (writer *xdtoSchemaWriter) prefixed(name XDTOName) string {
	switch name.Namespace {
	case xmlSchemaNamespace:
		return xmlSchemaPrefix + ":" + name.Name
	case writer.content.Namespace:
		return "tns:" + name.Name
	}
	return "ns" + strconv.Itoa(slices.Index(writer.content.Imports, name.Namespace)+1) + ":" + name.Name
}

func (writer *xdtoSchemaWriter) schema() {
	content := writer.content
	root := "<xs:schema"
	for index, namespace := range content.Imports {
		root += fmt.Sprintf(" xmlns:ns%d=%s", index+1, xmlQuoted(namespace))
	}
	form := func(qualified *bool, otherwise bool) string {
		if qualified != nil && *qualified || qualified == nil && otherwise {
			return "qualified"
		}
		return "unqualified"
	}
	root += " xmlns:tns=" + xmlQuoted(content.Namespace) + " xmlns:xs=" + xmlQuoted(xmlSchemaNamespace) + " targetNamespace=" + xmlQuoted(content.Namespace) +
		" attributeFormDefault=" + xmlQuoted(form(content.AttributeFormQualified, false)) + " elementFormDefault=" + xmlQuoted(form(content.ElementFormQualified, true))
	if len(content.Imports)+len(content.Properties)+len(content.ValueTypes)+len(content.ObjectTypes) == 0 {
		writer.line(0, root+"/>")
		return
	}
	writer.line(0, root+">")
	for _, namespace := range content.Imports {
		writer.line(1, "<xs:import namespace="+xmlQuoted(namespace)+"/>")
	}
	for _, property := range content.Properties {
		writer.property(1, property, true, false)
	}
	for _, value := range content.ValueTypes {
		writer.valueType(1, value)
	}
	for _, object := range content.ObjectTypes {
		writer.objectType(1, object)
	}
	writer.line(0, "</xs:schema>")
}

// property writes a property: an attribute, or an element - global or in the
// sequence of an object type.
func (writer *xdtoSchemaWriter) property(depth int, property XDTOProperty, global, unordered bool) {
	tag := "xs:element"
	if property.Form == "Attribute" {
		tag = "xs:attribute"
	}
	head := "<" + tag
	if property.Ref != nil {
		head += " ref=" + xmlQuoted(writer.prefixed(*property.Ref))
	} else {
		name := property.Name
		if property.LocalName != "" {
			name = property.LocalName
		}
		head += " name=" + xmlQuoted(name)
	}
	// Any type is what an element is of when its type is not said.
	if property.Type != nil && *property.Type != (XDTOName{Namespace: xmlSchemaNamespace, Name: "anyType"}) {
		head += " type=" + xmlQuoted(writer.prefixed(*property.Type))
	}
	if tag == "xs:attribute" {
		if !global && (property.LowerBound == nil || *property.LowerBound > 0) {
			head += ` use="required"`
		}
	} else if !global {
		if property.Nillable != nil && *property.Nillable {
			head += ` nillable="true"`
		}
		if property.LowerBound != nil && *property.LowerBound != 1 {
			head += " minOccurs=" + xmlQuoted(strconv.Itoa(*property.LowerBound))
		}
		if property.UpperBound != nil && *property.UpperBound != 1 {
			if *property.UpperBound < 0 {
				head += ` maxOccurs="unbounded"`
			} else {
				head += " maxOccurs=" + xmlQuoted(strconv.Itoa(*property.UpperBound))
			}
		}
	}
	if property.TypeDef == nil {
		writer.line(depth, head+"/>")
		return
	}
	writer.line(depth, head+">")
	if property.TypeDef.Value != nil {
		writer.valueType(depth+1, *property.TypeDef.Value)
	} else if property.TypeDef.Object != nil {
		writer.objectType(depth+1, *property.TypeDef.Object)
	}
	writer.line(depth, "</"+tag+">")
}

// valueType writes a simple type: named at the top of the schema, anonymous
// inside a property.
func (writer *xdtoSchemaWriter) valueType(depth int, value XDTOValueType) {
	head := "<xs:simpleType"
	if value.Name != "" {
		head += " name=" + xmlQuoted(value.Name)
	}
	writer.line(depth, head+">")
	switch value.Variety {
	case "List":
		list := "<xs:list"
		if value.ItemType != nil {
			list += " itemType=" + xmlQuoted(writer.prefixed(*value.ItemType))
		}
		writer.line(depth+1, list+"/>")
	case "Union":
		members := make([]string, len(value.MemberTypes))
		for index, member := range value.MemberTypes {
			members[index] = writer.prefixed(member)
		}
		writer.line(depth+1, "<xs:union memberTypes="+xmlQuoted(strings.Join(members, " "))+"/>")
	default:
		restriction := "<xs:restriction"
		if value.Base != nil {
			restriction += " base=" + xmlQuoted(writer.prefixed(*value.Base))
		}
		facets := writer.facets(value)
		if len(facets) == 0 {
			writer.line(depth+1, restriction+"/>")
			break
		}
		writer.line(depth+1, restriction+">")
		for _, facet := range facets {
			writer.line(depth+2, facet)
		}
		writer.line(depth+1, "</xs:restriction>")
	}
	writer.line(depth, "</xs:simpleType>")
}

// facets are the facets of a value type in the order the prototype writes
// them.
func (writer *xdtoSchemaWriter) facets(value XDTOValueType) []string {
	var facets []string
	number := func(name string, value *int) {
		if value != nil {
			facets = append(facets, "<xs:"+name+" value="+xmlQuoted(strconv.Itoa(*value))+"/>")
		}
	}
	text := func(name string, value *string) {
		if value != nil {
			facets = append(facets, "<xs:"+name+" value="+xmlQuoted(*value)+"/>")
		}
	}
	number("length", value.Length)
	number("minLength", value.MinLength)
	number("maxLength", value.MaxLength)
	for _, pattern := range value.Patterns {
		facets = append(facets, "<xs:pattern value="+xmlQuoted(pattern)+"/>")
	}
	if value.WhiteSpace != "" {
		facets = append(facets, "<xs:whiteSpace value="+xmlQuoted(strings.ToLower(value.WhiteSpace))+"/>")
	}
	text("minInclusive", value.MinInclusive)
	text("minExclusive", value.MinExclusive)
	text("maxInclusive", value.MaxInclusive)
	text("maxExclusive", value.MaxExclusive)
	number("totalDigits", value.TotalDigits)
	number("fractionDigits", value.FractionDigits)
	for _, enumeration := range value.Enumerations {
		facets = append(facets, "<xs:enumeration value="+xmlQuoted(enumeration.Value)+"/>")
	}
	return facets
}

// objectType writes a complex type.
func (writer *xdtoSchemaWriter) objectType(depth int, object XDTOObjectType) {
	head := "<xs:complexType"
	if object.Name != "" {
		head += " name=" + xmlQuoted(object.Name)
	}
	if object.Abstract != nil && *object.Abstract {
		head += ` abstract="true"`
	}
	if object.Mixed != nil && *object.Mixed {
		head += ` mixed="true"`
	}
	var elements, attributes []XDTOProperty
	var text *XDTOProperty
	for index, property := range object.Properties {
		switch property.Form {
		case "Attribute":
			attributes = append(attributes, property)
		case "Text":
			text = &object.Properties[index]
		default:
			elements = append(elements, property)
		}
	}
	open := writer.packages.open(object, 0)
	if text != nil && len(elements) == 0 && !open {
		writer.line(depth, head+">")
		writer.line(depth+1, "<xs:simpleContent>")
		base := "<xs:extension"
		if text.Type != nil {
			base += " base=" + xmlQuoted(writer.prefixed(*text.Type))
		}
		writer.line(depth+2, base+">")
		for _, attribute := range attributes {
			writer.property(depth+3, attribute, false, false)
		}
		writer.line(depth+2, "</xs:extension>")
		writer.line(depth+1, "</xs:simpleContent>")
		writer.line(depth, "</xs:complexType>")
		return
	}
	if len(elements) == 0 && len(attributes) == 0 && !open && object.Base == nil {
		writer.line(depth, head+"/>")
		return
	}
	writer.line(depth, head+">")
	inner := depth + 1
	if object.Base != nil {
		writer.line(depth+1, "<xs:complexContent>")
		extension := "<xs:extension base=" + xmlQuoted(writer.prefixed(*object.Base))
		if len(elements) == 0 && len(attributes) == 0 && !open {
			writer.line(depth+2, extension+"/>")
			writer.line(depth+1, "</xs:complexContent>")
			writer.line(depth, "</xs:complexType>")
			return
		}
		writer.line(depth+2, extension+">")
		inner = depth + 3
	}
	group := writer.packages.group(object, writer.content.Namespace)
	if len(elements) > 0 || open {
		writer.line(inner, "<"+group+">")
		for _, element := range elements {
			writer.property(inner+1, element, false, group == "xs:choice")
		}
		if open {
			writer.line(inner+1, `<xs:any namespace="##any" processContents="lax" minOccurs="0" maxOccurs="unbounded"/>`)
		}
		writer.line(inner, "</"+group+">")
	}
	for _, attribute := range attributes {
		writer.property(inner, attribute, false, false)
	}
	if open {
		writer.line(inner, `<xs:anyAttribute namespace="##any" processContents="lax"/>`)
	}
	if object.Base != nil {
		writer.line(depth+2, "</xs:extension>")
		writer.line(depth+1, "</xs:complexContent>")
	}
	writer.line(depth, "</xs:complexType>")
}

// xdtoBodies are the bodies of the packages a base of an object type is
// found in: the package written or checked, and those of the project.
type xdtoBodies struct {
	content XDTOContent
	catalog *Catalog
}

// base answers the object type an object type of the package of space
// extends, and the namespace of the base: in the package itself or in any
// package of the project (paket-xdto.md, «Досъёмка 28.10.2026, поздний
// вечер»: a base from another package passes its flags too). depth, the
// bases walked so far, stops a loop of bases.
func (bodies xdtoBodies) base(object XDTOObjectType, depth int) (XDTOObjectType, string, bool) {
	if object.Base == nil || depth > 64 {
		return XDTOObjectType{}, "", false
	}
	types := bodies.content.ObjectTypes
	if object.Base.Namespace != bodies.content.Namespace {
		types = nil
		if bodies.catalog != nil {
			for _, item := range bodies.catalog.XDTOPackages {
				if body, ok := bodies.catalog.xdtoContents[item.ID]; ok && item.Namespace == object.Base.Namespace {
					types = body.ObjectTypes
					break
				}
			}
		}
	}
	for _, base := range types {
		if base.Name == object.Base.Name {
			return base, object.Base.Namespace, true
		}
	}
	return XDTOObjectType{}, "", false
}

// open reports whether an object type is open - any element and any
// attribute after its own. A type that does not say takes it from its base,
// of the package or of another: the extension of an open type is open too
// (paket-xdto.md, «Досъёмка 28.10.2026, вечер», «…поздний вечер»;
// seriya-2631/10-v5-open-inherit.xsd, vopr-2.636/33).
func (bodies xdtoBodies) open(object XDTOObjectType, depth int) bool {
	if object.Open == nil {
		if base, _, ok := bodies.base(object, depth); ok {
			return bodies.open(base, depth+1)
		}
		return false
	}
	return *object.Open
}

// flags are «Упорядоченный» and «Последовательный» of an object type of
// the package of space, nil where nothing says. A type that does not say
// takes «Упорядоченный» from its base, of the package or of another, and
// «Последовательный» from a base of its own package alone: a type of
// CML208 of erp extending Контрагент, unordered and sequenced, is written
// a choice by the prototype, D of vopr-2.636/33, whose base of another
// package is so too, all (paket-xdto.md, «Досъёмка 28.10.2026, поздний
// вечер»).
func (bodies xdtoBodies) flags(object XDTOObjectType, space string, depth int) (ordered, sequenced *bool) {
	ordered, sequenced = object.Ordered, object.Sequenced
	if ordered != nil && sequenced != nil {
		return ordered, sequenced
	}
	if base, baseSpace, ok := bodies.base(object, depth); ok {
		baseOrdered, baseSequenced := bodies.flags(base, baseSpace, depth+1)
		if ordered == nil {
			ordered = baseOrdered
		}
		if sequenced == nil && baseSpace == space {
			sequenced = baseSequenced
		}
	}
	return ordered, sequenced
}

// group is the group the elements of an object type of the package of
// space are written in: ordered - a sequence, as by default; not ordered
// and sequenced - a choice; neither - all (paket-xdto.md, «Экспорт
// XML-схемы», «Досъёмка 28.10.2026, поздний вечер»).
func (bodies xdtoBodies) group(object XDTOObjectType, space string) string {
	ordered, sequenced := bodies.flags(object, space, 0)
	switch {
	case ordered == nil || *ordered:
		return "xs:sequence"
	case sequenced != nil && *sequenced:
		return "xs:choice"
	}
	return "xs:all"
}

// xmlQuoted is a value of an attribute in double quotes, escaped as the
// prototype escapes one (xdtoAttributeEscape).
func xmlQuoted(value string) string { return `"` + xdtoAttributeEscape.Replace(value) + `"` }

// CheckXDTOContent is the prototype's check of a package (paket-xdto.md,
// «Проверить пакет»; snapshots series-2.603/22, 23), against the types of
// XML Schema, of the platform's namespaces and of the other packages of the
// project (catalog): the type of a global property not found in the package
// and the namespaces it imports - xdto-package-4.4; a member of a union not
// found - xdto-valueType-4.1, one for each; a member that is an object type
// - xdto-valueType-4.1.2. A type of a property of a type, a base, an item of
// a list from a namespace the package does not import cannot be written in
// the schema either; the prototype's words for it are not shot, and ML says
// it in its own (question В5 of series 2.631).
func CheckXDTOContent(content XDTOContent, catalog *Catalog) []XDTOSchemaIssue {
	check := xdtoCheck{content: content, known: catalog.XDTOTypes(), objects: map[XDTOName]bool{}, packages: xdtoBodies{content: content, catalog: catalog}}
	for _, item := range catalog.XDTOPackages {
		if body, ok := catalog.xdtoContents[item.ID]; ok && item.Namespace != content.Namespace {
			for _, object := range body.ObjectTypes {
				check.objects[XDTOName{item.Namespace, object.Name}] = true
			}
		}
	}
	own := []string{}
	for _, value := range content.ValueTypes {
		own = append(own, value.Name)
	}
	for _, object := range content.ObjectTypes {
		own = append(own, object.Name)
		check.objects[XDTOName{content.Namespace, object.Name}] = true
	}
	check.known[content.Namespace] = own
	for _, property := range content.Properties {
		if property.Type != nil && !check.found(*property.Type) {
			check.issues = append(check.issues, XDTOSchemaIssue{Code: "xdto-package-4.4",
				Text: "Тип глобального свойства '" + property.Name + "' не найден в пакете типов '" + XDTONamespaceShown(content.Namespace) + "' и его зависимостях"})
		}
		check.typeDef(property.TypeDef)
	}
	for _, value := range content.ValueTypes {
		check.valueType(value, value.Name)
	}
	for _, object := range content.ObjectTypes {
		check.objectType(object)
	}
	return check.issues
}

type xdtoCheck struct {
	content  XDTOContent
	packages xdtoBodies
	// known are the names of the types of each namespace.
	known   map[string][]string
	objects map[XDTOName]bool
	issues  []XDTOSchemaIssue
}

// found reports whether a type is one of XML Schema, of the package, or of
// a namespace it imports.
func (check *xdtoCheck) found(name XDTOName) bool {
	if name.Namespace != xmlSchemaNamespace && name.Namespace != check.content.Namespace && !slices.Contains(check.content.Imports, name.Namespace) {
		return false
	}
	return slices.Contains(check.known[name.Namespace], name.Name)
}

// imported names a reference the schema cannot write: its namespace is not
// imported by the package.
func (check *xdtoCheck) imported(name *XDTOName) {
	if name == nil || name.Namespace == xmlSchemaNamespace || name.Namespace == check.content.Namespace ||
		slices.Contains(check.content.Imports, name.Namespace) {
		return
	}
	check.issues = append(check.issues, XDTOSchemaIssue{
		Text: "Пространство имен '" + XDTONamespaceShown(name.Namespace) + "' типа '" + name.Name + "' не импортировано пакетом '" + XDTONamespaceShown(check.content.Namespace) + "'"})
}

func (check *xdtoCheck) valueType(value XDTOValueType, name string) {
	check.imported(value.Base)
	check.imported(value.ItemType)
	for _, member := range value.MemberTypes {
		switch {
		case !check.found(member):
			check.issues = append(check.issues, XDTOSchemaIssue{Code: "xdto-valueType-4.1", Type: name,
				Text: "Тип объединения '{" + XDTONamespaceShown(member.Namespace) + "}" + member.Name + "' отсутствует или не определен"})
		case check.objects[member]:
			check.issues = append(check.issues, XDTOSchemaIssue{Code: "xdto-valueType-4.1.2", Type: name, Text: "Тип объединения должен являться типом значения"})
		}
	}
	for _, definition := range value.TypeDefs {
		check.typeDef(&definition)
	}
}

func (check *xdtoCheck) objectType(object XDTOObjectType) {
	check.imported(object.Base)
	// «Открытый» = Ложь said by an extension of an open type
	// (paket-xdto.md, «Досъёмка 28.10.2026, поздний вечер»; snapshot
	// vopr-2.636/31).
	if object.Open != nil && !*object.Open {
		if base, _, ok := check.packages.base(object, 0); ok && check.packages.open(base, 1) {
			check.issues = append(check.issues, XDTOSchemaIssue{Code: "xdto-objectType-5.4", Type: object.Name,
				Text: "Изменение открытой модели содержания типа '{" + XDTONamespaceShown(check.content.Namespace) + "}" + object.Name + "' при наследовании расширением"})
		}
	}
	for _, property := range object.Properties {
		check.imported(property.Type)
		check.imported(property.Ref)
		check.typeDef(property.TypeDef)
	}
}

func (check *xdtoCheck) typeDef(definition *XDTOTypeDef) {
	switch {
	case definition == nil:
	case definition.Value != nil:
		check.valueType(*definition.Value, definition.Value.Name)
	case definition.Object != nil:
		check.objectType(*definition.Object)
	}
}
