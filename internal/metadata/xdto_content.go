package metadata

import (
	"bytes"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"strconv"
	"strings"
)

// The body of a package of exchanged types (docs/block-00/paket-xdto.md,
// «Окно — редактор пакета», «Узлы редактора»; help, ПакетXDTO,
// ТипЗначенияXDTO, ТипОбъектаXDTO, СвойствоXDTO, ВидФасетаXDTO): the
// namespaces it imports, its global properties, its value types and its
// object types - what the editor of a package shows as a tree. It is kept in
// the file beside the description (XDTOPackageContentFile) in the form the
// prototype writes it: the element «package» of the namespace of the model
// of XDTO.
//
// Every value of a node is held as said or not said: the editor shows a
// value not said greyed, as its default, and does not write it (paket-xdto,
// «Палитра узла»), so a pointer, an empty string or an empty list is a value
// left out, not a zero.

// xdtoNamespace is the namespace of the model of XDTO the body is written in;
// xmlSchemaNamespace and xmlSchemaInstanceNamespace are the two the root
// declares beside it as «xs» and «xsi».
const (
	xdtoNamespace              = "http://v8.1c.ru/8.1/xdto"
	xmlSchemaNamespace         = "http://www.w3.org/2001/XMLSchema"
	xmlSchemaInstanceNamespace = "http://www.w3.org/2001/XMLSchema-instance"
)

// XDTOContent is the body of one package.
type XDTOContent struct {
	// Namespace is the namespace the package defines - the same as its
	// description's (XDTOPackageDefinition.Namespace).
	Namespace string `json:"namespace"`
	// ElementFormQualified and AttributeFormQualified are «Квалифицированная
	// форма элемента» and «… атрибута» of the root of the tree.
	ElementFormQualified   *bool            `json:"elementFormQualified,omitempty"`
	AttributeFormQualified *bool            `json:"attributeFormQualified,omitempty"`
	Imports                []string         `json:"imports,omitempty"`
	Properties             []XDTOProperty   `json:"properties,omitempty"`
	ValueTypes             []XDTOValueType  `json:"valueTypes,omitempty"`
	ObjectTypes            []XDTOObjectType `json:"objectTypes,omitempty"`
	Unknown                *XDTOUnknown     `json:"unknown,omitempty"`
}

// XDTOName names a type or a property by its namespace and its name.
type XDTOName struct {
	Namespace string `json:"namespace"`
	Name      string `json:"name"`
}

// XDTOProperty is a property - one of the package's own or one of an object
// type: «Свойство» of the editor.
type XDTOProperty struct {
	Name       string       `json:"name,omitempty"`
	Ref        *XDTOName    `json:"ref,omitempty"`
	Type       *XDTOName    `json:"type,omitempty"`
	LowerBound *int         `json:"lowerBound,omitempty"`
	UpperBound *int         `json:"upperBound,omitempty"`
	Nillable   *bool        `json:"nillable,omitempty"`
	Fixed      *bool        `json:"fixed,omitempty"`
	Default    *string      `json:"default,omitempty"`
	Form       string       `json:"form,omitempty"`
	LocalName  string       `json:"localName,omitempty"`
	Qualified  *bool        `json:"qualified,omitempty"`
	TypeDef    *XDTOTypeDef `json:"typeDef,omitempty"`
	Unknown    *XDTOUnknown `json:"unknown,omitempty"`
}

// XDTOValueType is a value type - one of the package's own, named, or the
// definition of a type in place, nameless: «Тип значения» of the editor
// with its facets, its patterns («Образец»), its enumeration
// («Перечисление») and the types it is defined by («Определения типов»).
type XDTOValueType struct {
	Name           string            `json:"name,omitempty"`
	Base           *XDTOName         `json:"base,omitempty"`
	Variety        string            `json:"variety,omitempty"`
	ItemType       *XDTOName         `json:"itemType,omitempty"`
	MemberTypes    []XDTOName        `json:"memberTypes,omitempty"`
	Length         *int              `json:"length,omitempty"`
	MinLength      *int              `json:"minLength,omitempty"`
	MaxLength      *int              `json:"maxLength,omitempty"`
	WhiteSpace     string            `json:"whiteSpace,omitempty"`
	MinInclusive   *string           `json:"minInclusive,omitempty"`
	MinExclusive   *string           `json:"minExclusive,omitempty"`
	MaxInclusive   *string           `json:"maxInclusive,omitempty"`
	MaxExclusive   *string           `json:"maxExclusive,omitempty"`
	TotalDigits    *int              `json:"totalDigits,omitempty"`
	FractionDigits *int              `json:"fractionDigits,omitempty"`
	TypeDefs       []XDTOTypeDef     `json:"typeDefs,omitempty"`
	Patterns       []string          `json:"patterns,omitempty"`
	Enumerations   []XDTOEnumeration `json:"enumerations,omitempty"`
	Unknown        *XDTOUnknown      `json:"unknown,omitempty"`
}

// XDTOEnumeration is one value of an enumeration, with the type it is
// written as where it is said (xs:string, 6213 of 7131 in the exports).
type XDTOEnumeration struct {
	Type  *XDTOName `json:"type,omitempty"`
	Value string    `json:"value"`
}

// XDTOObjectType is an object type - one of the package's own, named, or
// the definition of a type in place, nameless: «Тип объекта» of the editor
// with its properties.
type XDTOObjectType struct {
	Name       string         `json:"name,omitempty"`
	Base       *XDTOName      `json:"base,omitempty"`
	Open       *bool          `json:"open,omitempty"`
	Abstract   *bool          `json:"abstract,omitempty"`
	Mixed      *bool          `json:"mixed,omitempty"`
	Ordered    *bool          `json:"ordered,omitempty"`
	Sequenced  *bool          `json:"sequenced,omitempty"`
	Properties []XDTOProperty `json:"properties,omitempty"`
	Unknown    *XDTOUnknown   `json:"unknown,omitempty"`
}

// XDTOTypeDef is the definition of a type in place - «Определение типа»:
// an object type or a value type with no name, one of the two.
type XDTOTypeDef struct {
	Object *XDTOObjectType `json:"object,omitempty"`
	Value  *XDTOValueType  `json:"value,omitempty"`
}

// XDTOUnknown is what a node of the body holds beyond what the model knows
// of it - attributes and elements the help does not name. It is carried as
// written and noted (NoteXDTOContentUnknown), never dropped: the iron rule
// of CONFORMANCE.md, «Принято, но не исполняется». Elements is the XML of
// each, with the namespaces it uses declared on it.
type XDTOUnknown struct {
	Attributes []XDTOAttribute `json:"attributes,omitempty"`
	Elements   []string        `json:"elements,omitempty"`
}

// XDTOAttribute is an attribute the model does not know, by its namespace,
// its name and its value as written.
type XDTOAttribute struct {
	Namespace string `json:"namespace,omitempty"`
	Name      string `json:"name"`
	Value     string `json:"value"`
}

// clone is a copy of the body sharing nothing with it.
func (content XDTOContent) clone() XDTOContent {
	var copied XDTOContent
	encoded, _ := json.Marshal(content)
	_ = json.Unmarshal(encoded, &copied)
	return copied
}

// unknownPlaces calls note with every node of the body holding what the
// model does not know (NoteXDTOContentUnknown): where it stands - the
// path of names from the root - and what it is.
func (content XDTOContent) unknownPlaces(note func(where, written string)) {
	report := func(where string, unknown *XDTOUnknown) {
		if unknown.empty() {
			return
		}
		var written []string
		for _, attribute := range unknown.Attributes {
			written = append(written, attribute.Name+"="+strconv.Quote(attribute.Value))
		}
		written = append(written, unknown.Elements...)
		note(where, strings.Join(written, " "))
	}
	var property func(where string, value XDTOProperty)
	var definition func(where string, value XDTOTypeDef)
	var valueType func(where string, value XDTOValueType)
	var objectType func(where string, value XDTOObjectType)
	property = func(where string, value XDTOProperty) {
		where += ", свойство " + value.Name
		report(where, value.Unknown)
		if value.TypeDef != nil {
			definition(where, *value.TypeDef)
		}
	}
	definition = func(where string, value XDTOTypeDef) {
		if value.Object != nil {
			objectType(where+", определение типа", *value.Object)
		}
		if value.Value != nil {
			valueType(where+", определение типа", *value.Value)
		}
	}
	valueType = func(where string, value XDTOValueType) {
		if value.Name != "" {
			where += ", тип значения " + value.Name
		}
		report(where, value.Unknown)
		for _, inner := range value.TypeDefs {
			definition(where, inner)
		}
	}
	objectType = func(where string, value XDTOObjectType) {
		if value.Name != "" {
			where += ", тип объекта " + value.Name
		}
		report(where, value.Unknown)
		for _, inner := range value.Properties {
			property(where, inner)
		}
	}
	report("", content.Unknown)
	for _, value := range content.Properties {
		property("", value)
	}
	for _, value := range content.ValueTypes {
		valueType("", value)
	}
	for _, value := range content.ObjectTypes {
		objectType("", value)
	}
}

func (unknown *XDTOUnknown) empty() bool {
	return unknown == nil || len(unknown.Attributes) == 0 && len(unknown.Elements) == 0
}

// DecodeXDTOContent reads the body of a package as the prototype writes it.
// What is not XML, not the element «package» of the model of XDTO, or a
// value of the wrong form (a bound that is not a number, a flag that is not
// true or false, a name of a type whose prefix is not declared) is refused;
// what the model does not know is carried in Unknown.
func DecodeXDTOContent(reader io.Reader) (XDTOContent, error) {
	data, err := io.ReadAll(reader)
	if err != nil {
		return XDTOContent{}, err
	}
	data = bytes.TrimPrefix(data, []byte("\uFEFF"))
	root, err := parseXDTOElement(data)
	if err != nil {
		return XDTOContent{}, fmt.Errorf("the body of the package is not XML: %w", err)
	}
	if root.name != (xml.Name{Space: xdtoNamespace, Local: "package"}) {
		return XDTOContent{}, fmt.Errorf("the body of the package is %s, not the element package of %s", root.name.Local, xdtoNamespace)
	}
	var content XDTOContent
	reading := &xdtoReading{}
	content.Namespace = reading.take(root, "targetNamespace")
	content.ElementFormQualified = reading.flag(root, "elementFormQualified")
	content.AttributeFormQualified = reading.flag(root, "attributeFormQualified")
	for _, child := range root.children {
		switch child.name {
		case xdtoElement("import"):
			content.Imports = append(content.Imports, reading.take(child, "namespace"))
			reading.rest(child, nil)
		case xdtoElement("property"):
			content.Properties = append(content.Properties, reading.property(child))
		case xdtoElement("valueType"):
			content.ValueTypes = append(content.ValueTypes, reading.valueType(child, true))
		case xdtoElement("objectType"):
			content.ObjectTypes = append(content.ObjectTypes, reading.objectType(child, true))
		default:
			content.Unknown = reading.unknownElement(content.Unknown, child)
		}
	}
	content.Unknown = reading.rest(root, content.Unknown)
	return content, reading.err
}

// xdtoNode is an element of the body as read: its name resolved, its
// attributes, its text, its children, the namespaces in scope and its own
// XML as written.
type xdtoNode struct {
	name       xml.Name
	attributes []xml.Attr
	text       string
	children   []*xdtoNode
	scope      map[string]string
	raw        []byte
	taken      map[xml.Name]bool
}

func xdtoElement(local string) xml.Name {
	return xml.Name{Space: xdtoNamespace, Local: local}
}

// parseXDTOElement reads the root element and everything inside it.
func parseXDTOElement(data []byte) (*xdtoNode, error) {
	decoder := xml.NewDecoder(bytes.NewReader(data))
	var stack []*xdtoNode
	var root *xdtoNode
	for {
		start := decoder.InputOffset()
		token, err := decoder.RawToken()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		switch token := token.(type) {
		case xml.StartElement:
			scope := map[string]string{"": "", "xml": "http://www.w3.org/XML/1998/namespace"}
			if len(stack) > 0 {
				for prefix, space := range stack[len(stack)-1].scope {
					scope[prefix] = space
				}
			}
			var attributes []xml.Attr
			for _, attribute := range token.Attr {
				switch {
				case attribute.Name.Space == "" && attribute.Name.Local == "xmlns":
					scope[""] = attribute.Value
				case attribute.Name.Space == "xmlns":
					scope[attribute.Name.Local] = attribute.Value
				default:
					attributes = append(attributes, attribute)
				}
			}
			node := &xdtoNode{scope: scope, taken: map[xml.Name]bool{}}
			space, ok := scope[token.Name.Space]
			if !ok {
				return nil, fmt.Errorf("the prefix %q of element %s is not declared", token.Name.Space, token.Name.Local)
			}
			node.name = xml.Name{Space: space, Local: token.Name.Local}
			for _, attribute := range attributes {
				if attribute.Name.Space != "" {
					space, ok := scope[attribute.Name.Space]
					if !ok {
						return nil, fmt.Errorf("the prefix %q of attribute %s is not declared", attribute.Name.Space, attribute.Name.Local)
					}
					attribute.Name.Space = space
				}
				node.attributes = append(node.attributes, attribute)
			}
			node.raw = data[start:]
			if len(stack) > 0 {
				parent := stack[len(stack)-1]
				parent.children = append(parent.children, node)
			} else if root == nil {
				root = node
			} else {
				return nil, fmt.Errorf("a second root element %s", token.Name.Local)
			}
			stack = append(stack, node)
		case xml.EndElement:
			node := stack[len(stack)-1]
			node.raw = node.raw[:decoder.InputOffset()-(int64(len(data))-int64(len(node.raw)))]
			stack = stack[:len(stack)-1]
		case xml.CharData:
			if len(stack) > 0 {
				stack[len(stack)-1].text += string(token)
			}
		}
	}
	if root == nil {
		return nil, fmt.Errorf("no element")
	}
	return root, nil
}

// xdtoReading reads the nodes of a body and keeps the first fault.
type xdtoReading struct {
	err error
}

func (reading *xdtoReading) fail(node *xdtoNode, format string, args ...any) {
	if reading.err == nil {
		reading.err = fmt.Errorf("%s: %s", node.name.Local, fmt.Sprintf(format, args...))
	}
}

func (reading *xdtoReading) attribute(node *xdtoNode, space, local string) (string, bool) {
	name := xml.Name{Space: space, Local: local}
	for _, attribute := range node.attributes {
		if attribute.Name == name {
			node.taken[name] = true
			return attribute.Value, true
		}
	}
	return "", false
}

func (reading *xdtoReading) take(node *xdtoNode, local string) string {
	value, _ := reading.attribute(node, "", local)
	return value
}

func (reading *xdtoReading) optional(node *xdtoNode, local string) *string {
	if value, ok := reading.attribute(node, "", local); ok {
		return &value
	}
	return nil
}

func (reading *xdtoReading) flag(node *xdtoNode, local string) *bool {
	value, ok := reading.attribute(node, "", local)
	if !ok {
		return nil
	}
	switch value {
	case "true":
		flag := true
		return &flag
	case "false":
		flag := false
		return &flag
	}
	reading.fail(node, "%s is %q, not true or false", local, value)
	return nil
}

func (reading *xdtoReading) number(node *xdtoNode, local string) *int {
	value, ok := reading.attribute(node, "", local)
	if !ok {
		return nil
	}
	number, err := strconv.Atoi(value)
	if err != nil {
		reading.fail(node, "%s is %q, not a whole number", local, value)
		return nil
	}
	return &number
}

// qualified resolves a name of a type written as the prototype writes it:
// «prefix:name» by the namespaces in scope, «{namespace}name», or a name of
// no prefix in the default namespace.
func (reading *xdtoReading) qualified(node *xdtoNode, written string) XDTOName {
	if strings.HasPrefix(written, "{") {
		if end := strings.IndexByte(written, '}'); end > 0 {
			return XDTOName{Namespace: written[1:end], Name: written[end+1:]}
		}
	}
	prefix, local, found := strings.Cut(written, ":")
	if !found {
		prefix, local = "", written
	}
	space, ok := node.scope[prefix]
	if !ok {
		reading.fail(node, "the prefix %q of %q is not declared", prefix, written)
	}
	return XDTOName{Namespace: space, Name: local}
}

func (reading *xdtoReading) name(node *xdtoNode, local string) *XDTOName {
	value, ok := reading.attribute(node, "", local)
	if !ok {
		return nil
	}
	name := reading.qualified(node, value)
	return &name
}

func (reading *xdtoReading) names(node *xdtoNode, local string) []XDTOName {
	value, ok := reading.attribute(node, "", local)
	if !ok {
		return nil
	}
	var names []XDTOName
	for _, written := range strings.Fields(value) {
		names = append(names, reading.qualified(node, written))
	}
	return names
}

// rest carries the attributes of node nobody took into unknown.
func (reading *xdtoReading) rest(node *xdtoNode, unknown *XDTOUnknown) *XDTOUnknown {
	for _, attribute := range node.attributes {
		if node.taken[attribute.Name] {
			continue
		}
		if unknown == nil {
			unknown = &XDTOUnknown{}
		}
		unknown.Attributes = append(unknown.Attributes, XDTOAttribute{Namespace: attribute.Name.Space, Name: attribute.Name.Local, Value: attribute.Value})
	}
	if strings.TrimSpace(node.text) != "" && len(node.children) > 0 {
		reading.fail(node, "text beside elements")
	}
	return unknown
}

func (reading *xdtoReading) unknownElement(unknown *XDTOUnknown, node *xdtoNode) *XDTOUnknown {
	if unknown == nil {
		unknown = &XDTOUnknown{}
	}
	unknown.Elements = append(unknown.Elements, string(node.raw))
	return unknown
}

func (reading *xdtoReading) property(node *xdtoNode) XDTOProperty {
	property := XDTOProperty{
		Name: reading.take(node, "name"), Ref: reading.name(node, "ref"), Type: reading.name(node, "type"),
		LowerBound: reading.number(node, "lowerBound"), UpperBound: reading.number(node, "upperBound"),
		Nillable: reading.flag(node, "nillable"), Fixed: reading.flag(node, "fixed"), Default: reading.optional(node, "default"),
		Form: reading.take(node, "form"), LocalName: reading.take(node, "localName"), Qualified: reading.flag(node, "qualified"),
	}
	for _, child := range node.children {
		if child.name == xdtoElement("typeDef") && property.TypeDef == nil {
			property.TypeDef = reading.typeDef(child)
			continue
		}
		property.Unknown = reading.unknownElement(property.Unknown, child)
	}
	property.Unknown = reading.rest(node, property.Unknown)
	return property
}

func (reading *xdtoReading) typeDef(node *xdtoNode) *XDTOTypeDef {
	kind, _ := reading.attribute(node, xmlSchemaInstanceNamespace, "type")
	switch reading.qualified(node, kind) {
	case XDTOName{Namespace: xdtoNamespace, Name: "ObjectType"}:
		object := reading.objectType(node, false)
		return &XDTOTypeDef{Object: &object}
	case XDTOName{Namespace: xdtoNamespace, Name: "ValueType"}:
		value := reading.valueType(node, false)
		return &XDTOTypeDef{Value: &value}
	}
	reading.fail(node, "the definition of a type is %q, not ObjectType or ValueType", kind)
	return nil
}

func (reading *xdtoReading) valueType(node *xdtoNode, named bool) XDTOValueType {
	value := XDTOValueType{Base: reading.name(node, "base"), Variety: reading.take(node, "variety"), ItemType: reading.name(node, "itemType"),
		MemberTypes: reading.names(node, "memberTypes"), Length: reading.number(node, "length"), MinLength: reading.number(node, "minLength"),
		MaxLength: reading.number(node, "maxLength"), WhiteSpace: reading.take(node, "whiteSpace"),
		MinInclusive: reading.optional(node, "minInclusive"), MinExclusive: reading.optional(node, "minExclusive"),
		MaxInclusive: reading.optional(node, "maxInclusive"), MaxExclusive: reading.optional(node, "maxExclusive"),
		TotalDigits: reading.number(node, "totalDigits"), FractionDigits: reading.number(node, "fractionDigits")}
	if named {
		value.Name = reading.take(node, "name")
	}
	for _, child := range node.children {
		switch child.name {
		case xdtoElement("typeDef"):
			if definition := reading.typeDef(child); definition != nil {
				value.TypeDefs = append(value.TypeDefs, *definition)
			}
		case xdtoElement("pattern"):
			value.Patterns = append(value.Patterns, child.text)
			reading.rest(child, nil)
		case xdtoElement("enumeration"):
			enumeration := XDTOEnumeration{Value: child.text}
			if kind, ok := reading.attribute(child, xmlSchemaInstanceNamespace, "type"); ok {
				name := reading.qualified(child, kind)
				enumeration.Type = &name
			}
			value.Enumerations = append(value.Enumerations, enumeration)
			if unknown := reading.rest(child, nil); !unknown.empty() {
				reading.fail(child, "an enumeration takes only its type")
			}
		default:
			value.Unknown = reading.unknownElement(value.Unknown, child)
		}
	}
	value.Unknown = reading.rest(node, value.Unknown)
	return value
}

func (reading *xdtoReading) objectType(node *xdtoNode, named bool) XDTOObjectType {
	object := XDTOObjectType{Base: reading.name(node, "base"), Open: reading.flag(node, "open"), Abstract: reading.flag(node, "abstract"),
		Mixed: reading.flag(node, "mixed"), Ordered: reading.flag(node, "ordered"), Sequenced: reading.flag(node, "sequenced")}
	if named {
		object.Name = reading.take(node, "name")
	}
	for _, child := range node.children {
		if child.name == xdtoElement("property") {
			object.Properties = append(object.Properties, reading.property(child))
			continue
		}
		object.Unknown = reading.unknownElement(object.Unknown, child)
	}
	object.Unknown = reading.rest(node, object.Unknown)
	return object
}

// EncodeXDTOContent writes the body of a package as the prototype writes it
// (the 294 bodies of the exports): a byte order mark, the root declaring the
// model of XDTO as the default namespace and XML Schema as «xs» and «xsi»,
// a node a line indented by tabs, lines ended by CR LF and the last without
// one. A name of a type of XML Schema is written «xs:…», of the model of
// XDTO without a prefix, of any other namespace by the prefix
// «d<depth>p<n>» declared on the node itself - depth 1 the root, n the
// namespaces of the node in the order they are named - and the attributes
// stand in the prototype's order.
func EncodeXDTOContent(writer io.Writer, content XDTOContent) error {
	root := &xdtoWriting{depth: 1}
	root.literal("xmlns", xdtoNamespace)
	root.literal("xmlns:xs", xmlSchemaNamespace)
	root.literal("xmlns:xsi", xmlSchemaInstanceNamespace)
	root.literal("targetNamespace", content.Namespace)
	root.flag("elementFormQualified", content.ElementFormQualified)
	root.flag("attributeFormQualified", content.AttributeFormQualified)
	for _, space := range content.Imports {
		child := root.child("import")
		child.literal("namespace", space)
	}
	for _, property := range content.Properties {
		root.property(property)
	}
	for _, value := range content.ValueTypes {
		root.valueType(root.child("valueType"), value, true)
	}
	for _, object := range content.ObjectTypes {
		root.objectType(root.child("objectType"), object, true)
	}
	root.unknown(content.Unknown)
	var out bytes.Buffer
	out.WriteString("\uFEFF")
	root.write(&out, "package", 0)
	_, err := writer.Write(bytes.TrimSuffix(out.Bytes(), []byte("\r\n")))
	return err
}

// xdtoWriting is a node being written: its attributes in order, the
// prefixes it declares, its children, its text.
type xdtoWriting struct {
	parent     *xdtoWriting
	depth      int
	tag        string
	prefixes   []string
	spaces     map[string]string
	attributes [][2]string
	children   []*xdtoWriting
	text       *string
	raw        []string
}

func (node *xdtoWriting) child(tag string) *xdtoWriting {
	child := &xdtoWriting{parent: node, depth: node.depth + 1, tag: tag}
	node.children = append(node.children, child)
	return child
}

func (node *xdtoWriting) literal(name, value string) {
	node.attributes = append(node.attributes, [2]string{name, value})
}

func (node *xdtoWriting) optional(name string, value *string) {
	if value != nil {
		node.literal(name, *value)
	}
}

func (node *xdtoWriting) some(name, value string) {
	if value != "" {
		node.literal(name, value)
	}
}

func (node *xdtoWriting) flag(name string, value *bool) {
	if value != nil {
		node.literal(name, strconv.FormatBool(*value))
	}
}

func (node *xdtoWriting) number(name string, value *int) {
	if value != nil {
		node.literal(name, strconv.Itoa(*value))
	}
}

// qualified writes a name of a type by the prefix of its namespace.
func (node *xdtoWriting) qualified(name XDTOName) string {
	switch name.Namespace {
	case xmlSchemaNamespace:
		return "xs:" + name.Name
	case xdtoNamespace:
		return name.Name
	}
	for above := node; above != nil; above = above.parent {
		if prefix, ok := above.spaces[name.Namespace]; ok {
			return prefix + ":" + name.Name
		}
	}
	if node.spaces == nil {
		node.spaces = map[string]string{}
	}
	prefix, ok := node.spaces[name.Namespace]
	if !ok {
		prefix = fmt.Sprintf("d%dp%d", node.depth, len(node.prefixes)+1)
		node.spaces[name.Namespace] = prefix
		node.prefixes = append(node.prefixes, name.Namespace)
	}
	return prefix + ":" + name.Name
}

func (node *xdtoWriting) name(attribute string, name *XDTOName) {
	if name != nil {
		node.literal(attribute, node.qualified(*name))
	}
}

func (node *xdtoWriting) property(property XDTOProperty) {
	child := node.child("property")
	child.some("name", property.Name)
	child.name("ref", property.Ref)
	child.name("type", property.Type)
	child.number("lowerBound", property.LowerBound)
	child.flag("fixed", property.Fixed)
	child.number("upperBound", property.UpperBound)
	child.optional("default", property.Default)
	child.flag("nillable", property.Nillable)
	child.some("form", property.Form)
	child.some("localName", property.LocalName)
	child.flag("qualified", property.Qualified)
	if property.TypeDef != nil {
		child.typeDef(*property.TypeDef)
	}
	child.unknown(property.Unknown)
}

func (node *xdtoWriting) typeDef(definition XDTOTypeDef) {
	child := node.child("typeDef")
	if definition.Object != nil {
		child.literal("xsi:type", "ObjectType")
		child.objectType(child, *definition.Object, false)
		return
	}
	child.literal("xsi:type", "ValueType")
	if definition.Value != nil {
		child.valueType(child, *definition.Value, false)
	}
}

func (node *xdtoWriting) valueType(child *xdtoWriting, value XDTOValueType, named bool) {
	if named {
		child.literal("name", value.Name)
	}
	child.name("base", value.Base)
	child.some("variety", value.Variety)
	child.number("length", value.Length)
	child.number("minLength", value.MinLength)
	// The members of a union are written «{namespace}name», with no prefix
	// declared for them (25 of 35 unions of the exports; the other ten, by
	// prefix, are of earlier versions and read the same).
	if len(value.MemberTypes) > 0 {
		names := make([]string, len(value.MemberTypes))
		for index, member := range value.MemberTypes {
			names[index] = "{" + member.Namespace + "}" + member.Name
		}
		child.literal("memberTypes", strings.Join(names, " "))
	}
	child.name("itemType", value.ItemType)
	child.number("maxLength", value.MaxLength)
	child.some("whiteSpace", value.WhiteSpace)
	child.optional("minInclusive", value.MinInclusive)
	child.optional("minExclusive", value.MinExclusive)
	child.optional("maxInclusive", value.MaxInclusive)
	child.optional("maxExclusive", value.MaxExclusive)
	child.number("totalDigits", value.TotalDigits)
	child.number("fractionDigits", value.FractionDigits)
	for _, definition := range value.TypeDefs {
		child.typeDef(definition)
	}
	for _, pattern := range value.Patterns {
		text := pattern
		child.child("pattern").text = &text
	}
	for _, enumeration := range value.Enumerations {
		item := child.child("enumeration")
		if enumeration.Type != nil {
			item.literal("xsi:type", item.qualified(*enumeration.Type))
		}
		text := enumeration.Value
		item.text = &text
	}
	child.unknown(value.Unknown)
}

func (node *xdtoWriting) objectType(child *xdtoWriting, object XDTOObjectType, named bool) {
	if named {
		child.literal("name", object.Name)
	}
	child.name("base", object.Base)
	child.flag("open", object.Open)
	child.flag("abstract", object.Abstract)
	child.flag("mixed", object.Mixed)
	child.flag("ordered", object.Ordered)
	child.flag("sequenced", object.Sequenced)
	for _, property := range object.Properties {
		child.property(property)
	}
	child.unknown(object.Unknown)
}

func (node *xdtoWriting) unknown(unknown *XDTOUnknown) {
	if unknown == nil {
		return
	}
	for _, attribute := range unknown.Attributes {
		name := attribute.Name
		switch attribute.Namespace {
		case "":
		case xmlSchemaInstanceNamespace:
			name = "xsi:" + name
		default:
			name = strings.TrimSuffix(node.qualified(XDTOName{Namespace: attribute.Namespace, Name: name}), "")
		}
		node.literal(name, attribute.Value)
	}
	node.raw = append(node.raw, unknown.Elements...)
}

func (node *xdtoWriting) write(out *bytes.Buffer, tag string, indent int) {
	out.WriteString(strings.Repeat("\t", indent))
	out.WriteString("<" + tag)
	for index, space := range node.prefixes {
		fmt.Fprintf(out, ` xmlns:d%dp%d="%s"`, node.depth, index+1, xdtoEscape(space, true))
	}
	for _, attribute := range node.attributes {
		fmt.Fprintf(out, ` %s="%s"`, attribute[0], xdtoEscape(attribute[1], true))
	}
	switch {
	case node.text != nil && *node.text != "":
		out.WriteString(">" + xdtoEscape(*node.text, false) + "</" + tag + ">\r\n")
	case len(node.children) == 0 && len(node.raw) == 0 && node.text == nil || node.text != nil:
		out.WriteString("/>\r\n")
	default:
		out.WriteString(">\r\n")
		for _, child := range node.children {
			child.write(out, child.tag, indent+1)
		}
		for _, raw := range node.raw {
			out.WriteString(strings.Repeat("\t", indent+1) + raw + "\r\n")
		}
		out.WriteString(strings.Repeat("\t", indent) + "</" + tag + ">\r\n")
	}
}

// xdtoEscape escapes text as the prototype does: «&», «<», «>» everywhere,
// the quote in an attribute.
func xdtoEscape(text string, attribute bool) string {
	replacer := xdtoTextEscape
	if attribute {
		replacer = xdtoAttributeEscape
	}
	return replacer.Replace(text)
}

var (
	xdtoTextEscape      = strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;")
	xdtoAttributeEscape = strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", `"`, "&quot;")
)
