package metadata

import (
	"fmt"
	"slices"
	"strconv"
	"strings"
)

// The model of a description of a WS reference, as the window viewing it
// shows it (docs/block-00/ws-ssylka.md, «Окно», the pairs «описание —
// окно»): «Модель данных» - a package of XDTO a schema of the description
// and a schema it imports - and «Web-сервисы» - the services, their ports,
// the operations of the ports and their parameters.
//
// A schema becomes a package: its global elements its properties, in the
// order of the description (the prototype's order is none - ML-STUDIO.md);
// its named complex types its object types by the alphabet, then the
// nameless types of its global elements under the name of the element, by
// the alphabet with case, each followed by the nameless types of its
// fields «<элемент>_<поле>»; its named simple types its value types. A
// value written in the schema is said, one left out is not.
//
// An operation of document style is unwrapped - its parameters the fields
// of the element of its request, then the fields of the response the
// request has not («Выходной»), a field of both «ВходнойВыходной», a
// response of one field its type of return - when the elements of the
// request and of the response are of a complex type of fields, each field
// typed by a name; else it has one parameter named by the part of the
// message, of the type of the element of the request, and returns the type
// of the element of the response (ws-ssylka.md, «Правило найдено
// четвёртым заходом»; 23 operations of three descriptions).

// WSDefinitionModel is what a description of a service describes.
type WSDefinitionModel struct {
	Packages []XDTOContent `json:"packages"`
	Services []WSService   `json:"services"`
}

// WSService is a service of a description: its namespace, name and
// documentation, and its ports.
type WSService struct {
	Namespace     string   `json:"namespace"`
	Name          string   `json:"name"`
	Documentation string   `json:"documentation,omitempty"`
	Ports         []WSPort `json:"ports"`
}

// WSPort is a port of a service - «точка подключения» - with the operations
// of its binding.
type WSPort struct {
	Name          string        `json:"name"`
	Documentation string        `json:"documentation,omitempty"`
	Operations    []WSOperation `json:"operations"`
}

// WSOperation is an operation: its type of return - none where the
// response has several fields or there is none - and its parameters.
type WSOperation struct {
	Name                string        `json:"name"`
	Documentation       string        `json:"documentation,omitempty"`
	ReturnType          *XDTOName     `json:"returnType,omitempty"`
	Nillable            bool          `json:"nillable,omitempty"`
	ReturnDocumentation string        `json:"returnDocumentation,omitempty"`
	Parameters          []WSParameter `json:"parameters"`
}

// WSParameter is a parameter of an operation: direction «in», «out» or
// «in-out».
type WSParameter struct {
	Name          string    `json:"name"`
	Type          *XDTOName `json:"type,omitempty"`
	Direction     string    `json:"direction"`
	Nillable      bool      `json:"nillable,omitempty"`
	Documentation string    `json:"documentation,omitempty"`
}

// ReadWSDefinition reads a description of a service and the schemas kept
// beside it (schema answers the content of «N.xsd») into its model.
func ReadWSDefinition(definition []byte, schema func(name string) ([]byte, error)) (WSDefinitionModel, error) {
	root, err := parseXDTOElement(trimByteOrderMark(definition))
	if err != nil {
		return WSDefinitionModel{}, fmt.Errorf("the description is not XML: %w", err)
	}
	reading := &wsReading{schema: schema, elements: map[XDTOName]*xdtoNode{}, complexTypes: map[XDTOName]*xdtoNode{}, read: map[string]bool{}}
	for _, types := range wsdlChildren(root, "types") {
		for _, node := range types.children {
			if node.name.Space == xmlSchemaNamespace && node.name.Local == "schema" {
				reading.schemaPackage(node)
			}
		}
	}
	if reading.err != nil {
		return WSDefinitionModel{}, reading.err
	}
	return WSDefinitionModel{Packages: reading.packages, Services: reading.services(root)}, reading.err
}

// trimByteOrderMark drops the mark the prototype writes before a document.
func trimByteOrderMark(data []byte) []byte {
	return []byte(strings.TrimPrefix(string(data), "\uFEFF"))
}

// wsdlChildren are the children of node of WSDL 1.1 named local.
func wsdlChildren(node *xdtoNode, local string) []*xdtoNode {
	return xmlChildren(node, wsdl11Namespace, local)
}

func xmlChildren(node *xdtoNode, space, local string) []*xdtoNode {
	var found []*xdtoNode
	for _, child := range node.children {
		if child.name.Space == space && child.name.Local == local {
			found = append(found, child)
		}
	}
	return found
}

func xmlAttribute(node *xdtoNode, local string) (string, bool) {
	for _, attribute := range node.attributes {
		if attribute.Name.Space == "" && attribute.Name.Local == local {
			return attribute.Value, true
		}
	}
	return "", false
}

// wsReading reads the schemas of a description and keeps what operations
// look up: the global elements and named complex types, by name.
type wsReading struct {
	schema       func(string) ([]byte, error)
	packages     []XDTOContent
	elements     map[XDTOName]*xdtoNode
	complexTypes map[XDTOName]*xdtoNode
	read         map[string]bool
	err          error
}

// qname resolves a name of a type or a message written in node.
func (reading *wsReading) qname(node *xdtoNode, written string) XDTOName {
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
	if !ok && reading.err == nil {
		reading.err = fmt.Errorf("the prefix %q of %q is not declared", prefix, written)
	}
	return XDTOName{Namespace: space, Name: local}
}

// schemaPackage makes a package of a schema and of every schema it imports
// that is kept beside the description, each once, in the order met.
func (reading *wsReading) schemaPackage(schema *xdtoNode) {
	space, _ := xmlAttribute(schema, "targetNamespace")
	content := XDTOContent{Namespace: space}
	if form, ok := xmlAttribute(schema, "elementFormDefault"); ok {
		qualified := form == "qualified"
		content.ElementFormQualified = &qualified
	}
	if form, ok := xmlAttribute(schema, "attributeFormDefault"); ok {
		qualified := form == "qualified"
		content.AttributeFormQualified = &qualified
	}
	var imported []*xdtoNode
	type nameless struct {
		name string
		node *xdtoNode
	}
	var named []XDTOObjectType
	var anonymous []nameless
	for _, node := range schema.children {
		if node.name.Space != xmlSchemaNamespace {
			continue
		}
		name, _ := xmlAttribute(node, "name")
		switch node.name.Local {
		case "import":
			if namespace, ok := xmlAttribute(node, "namespace"); ok {
				content.Imports = append(content.Imports, namespace)
			}
			if location, ok := xmlAttribute(node, "schemaLocation"); ok {
				if kept, found := strings.CutPrefix(location, "file://"); found && WSReferenceSchemaFile(kept) && !reading.read[kept] {
					reading.read[kept] = true
					data, err := reading.schema(kept)
					if err != nil {
						reading.err = fmt.Errorf("%s: %w", kept, err)
						return
					}
					child, err := parseXDTOElement(trimByteOrderMark(data))
					if err != nil {
						reading.err = fmt.Errorf("%s is not XML: %w", kept, err)
						return
					}
					imported = append(imported, child)
				}
			}
		case "element":
			reading.elements[XDTOName{Namespace: space, Name: name}] = node
			property := XDTOProperty{Name: name, Nillable: said(xsdFlag(node, "nillable"))}
			if written, ok := xmlAttribute(node, "type"); ok {
				typed := reading.qname(node, written)
				property.Type = &typed
			} else if inner := xmlChildren(node, xmlSchemaNamespace, "complexType"); len(inner) > 0 {
				property.Type = &XDTOName{Namespace: space, Name: name}
				anonymous = append(anonymous, nameless{name, inner[0]})
			}
			content.Properties = append(content.Properties, property)
		case "complexType":
			reading.complexTypes[XDTOName{Namespace: space, Name: name}] = node
			named = append(named, reading.objectType(node, name, space, nil))
		case "simpleType":
			content.ValueTypes = append(content.ValueTypes, reading.valueType(node, name))
		}
	}
	byName := func(one, other string) int { return strings.Compare(one, other) }
	slices.SortStableFunc(named, func(one, other XDTOObjectType) int { return byName(one.Name, other.Name) })
	slices.SortStableFunc(anonymous, func(one, other nameless) int { return byName(one.name, other.name) })
	slices.SortStableFunc(content.ValueTypes, func(one, other XDTOValueType) int { return byName(one.Name, other.Name) })
	content.ObjectTypes = named
	for _, item := range anonymous {
		var inner []XDTOObjectType
		object := reading.objectType(item.node, item.name, space, &inner)
		content.ObjectTypes = append(append(content.ObjectTypes, object), inner...)
	}
	reading.packages = append(reading.packages, content)
	for _, child := range imported {
		reading.schemaPackage(child)
	}
}

// xsdFlag is a flag of a schema said true - nillable, mixed.
func xsdFlag(node *xdtoNode, local string) bool {
	value, _ := xmlAttribute(node, local)
	return value == "true" || value == "1"
}

func said(value bool) *bool {
	if !value {
		return nil
	}
	return &value
}

// objectType makes an object type of a complex type: name is its name -
// of the type, or of the element a nameless type belongs to; inner, where
// given, collects the nameless types of its fields «<name>_<поле>», each
// after the types of its own fields.
func (reading *wsReading) objectType(node *xdtoNode, name, space string, inner *[]XDTOObjectType) XDTOObjectType {
	object := XDTOObjectType{Name: name}
	if xsdFlag(node, "mixed") {
		object.Mixed, object.Sequenced = said(true), said(true)
	}
	var walk func(*xdtoNode)
	walk = func(holder *xdtoNode) {
		for _, child := range holder.children {
			if child.name.Space != xmlSchemaNamespace {
				continue
			}
			switch child.name.Local {
			case "sequence", "all", "choice":
				walk(child)
			case "complexContent":
				for _, derived := range child.children {
					if base, ok := xmlAttribute(derived, "base"); ok {
						typed := reading.qname(derived, base)
						object.Base = &typed
					}
					walk(derived)
				}
			case "simpleContent":
				// A content of text with attributes: the attributes, then
				// the text «__content» of the type extended (ws-ssylka.md,
				// TelephoneNumber); the type has no base.
				for _, derived := range child.children {
					walk(derived)
					text := XDTOProperty{Name: "__content", Form: "Text"}
					if base, ok := xmlAttribute(derived, "base"); ok {
						typed := reading.qname(derived, base)
						text.Type = &typed
					}
					object.Properties = append(object.Properties, text)
				}
			case "any":
				object.Open = said(true)
			case "attribute":
				property := XDTOProperty{Form: "Attribute"}
				property.Name, _ = xmlAttribute(child, "name")
				if written, ok := xmlAttribute(child, "type"); ok {
					typed := reading.qname(child, written)
					property.Type = &typed
				}
				object.Properties = append(object.Properties, property)
			case "element":
				object.Properties = append(object.Properties, reading.field(child, name, space, inner))
			}
		}
	}
	walk(node)
	return object
}

// field makes a property of a field of a complex type.
func (reading *wsReading) field(node *xdtoNode, owner, space string, inner *[]XDTOObjectType) XDTOProperty {
	property := XDTOProperty{}
	property.Name, _ = xmlAttribute(node, "name")
	if written, ok := xmlAttribute(node, "ref"); ok {
		ref := reading.qname(node, written)
		property.Ref = &ref
	}
	if written, ok := xmlAttribute(node, "type"); ok {
		typed := reading.qname(node, written)
		property.Type = &typed
	}
	if written, ok := xmlAttribute(node, "minOccurs"); ok {
		if number, err := strconv.Atoi(written); err == nil {
			property.LowerBound = &number
		}
	}
	if written, ok := xmlAttribute(node, "maxOccurs"); ok {
		number := -1
		if written != "unbounded" {
			number, _ = strconv.Atoi(written)
		}
		property.UpperBound = &number
	}
	property.Nillable = said(xsdFlag(node, "nillable"))
	if complexes := xmlChildren(node, xmlSchemaNamespace, "complexType"); len(complexes) > 0 && inner != nil {
		name := owner + "_" + property.Name
		property.Type = &XDTOName{Namespace: space, Name: name}
		var deeper []XDTOObjectType
		object := reading.objectType(complexes[0], name, space, &deeper)
		*inner = append(append(*inner, object), deeper...)
	}
	if simples := xmlChildren(node, xmlSchemaNamespace, "simpleType"); len(simples) > 0 {
		value := reading.valueType(simples[0], "")
		property.TypeDef = &XDTOTypeDef{Value: &value}
	}
	return property
}

// valueType makes a value type of a simple type: a restriction with its
// facets and enumeration, a list or a union.
func (reading *wsReading) valueType(node *xdtoNode, name string) XDTOValueType {
	value := XDTOValueType{Name: name}
	number := func(facet *xdtoNode) *int {
		written, _ := xmlAttribute(facet, "value")
		parsed, err := strconv.Atoi(written)
		if err != nil {
			return nil
		}
		return &parsed
	}
	text := func(facet *xdtoNode) *string {
		written, _ := xmlAttribute(facet, "value")
		return &written
	}
	for _, child := range node.children {
		if child.name.Space != xmlSchemaNamespace {
			continue
		}
		switch child.name.Local {
		case "restriction":
			if base, ok := xmlAttribute(child, "base"); ok {
				typed := reading.qname(child, base)
				value.Base = &typed
			}
			for _, facet := range child.children {
				switch facet.name.Local {
				case "enumeration":
					written, _ := xmlAttribute(facet, "value")
					value.Enumerations = append(value.Enumerations, XDTOEnumeration{Value: written})
				case "pattern":
					written, _ := xmlAttribute(facet, "value")
					value.Patterns = append(value.Patterns, written)
				case "length":
					value.Length = number(facet)
				case "minLength":
					value.MinLength = number(facet)
				case "maxLength":
					value.MaxLength = number(facet)
				case "totalDigits":
					value.TotalDigits = number(facet)
				case "fractionDigits":
					value.FractionDigits = number(facet)
				case "whiteSpace":
					value.WhiteSpace = *text(facet)
				case "minInclusive":
					value.MinInclusive = text(facet)
				case "minExclusive":
					value.MinExclusive = text(facet)
				case "maxInclusive":
					value.MaxInclusive = text(facet)
				case "maxExclusive":
					value.MaxExclusive = text(facet)
				}
			}
		case "list":
			value.Variety = "List"
			if item, ok := xmlAttribute(child, "itemType"); ok {
				typed := reading.qname(child, item)
				value.ItemType = &typed
			}
		case "union":
			value.Variety = "Union"
			if members, ok := xmlAttribute(child, "memberTypes"); ok {
				for _, member := range strings.Fields(members) {
					value.MemberTypes = append(value.MemberTypes, reading.qname(child, member))
				}
			}
		}
	}
	return value
}

// wsMessagePart is a part of a message: by the element it carries
// (document style) or by its type.
type wsMessagePart struct {
	name           string
	element, typed *XDTOName
}

// services reads the services of the description: each port by its
// binding and the port type of the binding, the operations in its order.
func (reading *wsReading) services(root *xdtoNode) []WSService {
	space, _ := xmlAttribute(root, "targetNamespace")
	messages := map[XDTOName][]wsMessagePart{}
	for _, message := range wsdlChildren(root, "message") {
		name, _ := xmlAttribute(message, "name")
		var parts []wsMessagePart
		for _, part := range wsdlChildren(message, "part") {
			item := wsMessagePart{}
			item.name, _ = xmlAttribute(part, "name")
			if written, ok := xmlAttribute(part, "element"); ok {
				element := reading.qname(part, written)
				item.element = &element
			}
			if written, ok := xmlAttribute(part, "type"); ok {
				typed := reading.qname(part, written)
				item.typed = &typed
			}
			parts = append(parts, item)
		}
		messages[XDTOName{Namespace: space, Name: name}] = parts
	}
	portTypes := map[XDTOName]*xdtoNode{}
	for _, portType := range wsdlChildren(root, "portType") {
		name, _ := xmlAttribute(portType, "name")
		portTypes[XDTOName{Namespace: space, Name: name}] = portType
	}
	bindings := map[XDTOName]XDTOName{}
	for _, binding := range wsdlChildren(root, "binding") {
		name, _ := xmlAttribute(binding, "name")
		written, _ := xmlAttribute(binding, "type")
		bindings[XDTOName{Namespace: space, Name: name}] = reading.qname(binding, written)
	}
	services := []WSService{}
	for _, service := range wsdlChildren(root, "service") {
		item := WSService{Namespace: space, Documentation: wsdlDocumentation(service), Ports: []WSPort{}}
		item.Name, _ = xmlAttribute(service, "name")
		for _, port := range wsdlChildren(service, "port") {
			own := WSPort{Documentation: wsdlDocumentation(port), Operations: []WSOperation{}}
			own.Name, _ = xmlAttribute(port, "name")
			written, _ := xmlAttribute(port, "binding")
			if portType := portTypes[bindings[reading.qname(port, written)]]; portType != nil {
				for _, operation := range wsdlChildren(portType, "operation") {
					own.Operations = append(own.Operations, reading.operation(operation, messages))
				}
			}
			item.Ports = append(item.Ports, own)
		}
		services = append(services, item)
	}
	return services
}

// wsdlDocumentation is the text of the documentation of node.
func wsdlDocumentation(node *xdtoNode) string {
	for _, child := range wsdlChildren(node, "documentation") {
		return strings.TrimSpace(child.text)
	}
	return ""
}

// operation reads an operation of a port type by its messages.
func (reading *wsReading) operation(node *xdtoNode, messages map[XDTOName][]wsMessagePart) WSOperation {
	operation := WSOperation{Documentation: wsdlDocumentation(node), Parameters: []WSParameter{}}
	operation.Name, _ = xmlAttribute(node, "name")
	parts := func(local string) []wsMessagePart {
		for _, child := range wsdlChildren(node, local) {
			written, _ := xmlAttribute(child, "message")
			return messages[reading.qname(child, written)]
		}
		return nil
	}
	input, output := parts("input"), parts("output")
	request, response := wsSinglePart(input), wsSinglePart(output)
	if request != nil && request.element != nil && (response == nil || response.element != nil) {
		requestFields, requestOK := reading.wrapperFields(*request.element)
		responseFields, responseOK := []*xdtoNode(nil), true
		if response != nil {
			responseFields, responseOK = reading.wrapperFields(*response.element)
		}
		if requestOK && responseOK {
			reading.unwrap(&operation, requestFields, responseFields)
			return operation
		}
		operation.Parameters = append(operation.Parameters, WSParameter{Name: request.name, Type: reading.elementType(*request.element), Direction: "in"})
		if response != nil {
			operation.ReturnType = reading.elementType(*response.element)
		}
		return operation
	}
	// A message of parts by type (the style of remote procedure calls): a
	// parameter a part, the one part of the response the type of return.
	// The inventory shows no such description (series 2.603, question В8).
	for _, part := range input {
		operation.Parameters = append(operation.Parameters, WSParameter{Name: part.name, Type: part.typed, Direction: "in"})
	}
	if len(output) == 1 {
		operation.ReturnType = output[0].typed
	}
	return operation
}

func wsSinglePart(parts []wsMessagePart) *wsMessagePart {
	if len(parts) != 1 {
		return nil
	}
	return &parts[0]
}

// elementType is the type of a global element: the type it names, or its
// nameless type, named after it.
func (reading *wsReading) elementType(element XDTOName) *XDTOName {
	node := reading.elements[element]
	if node == nil {
		return &element
	}
	if written, ok := xmlAttribute(node, "type"); ok {
		typed := reading.qname(node, written)
		return &typed
	}
	return &element
}

// wrapperFields are the fields of the element of a request or a response
// and whether the operation may be unwrapped by it: a complex type - its
// own or named - of a sequence of fields, each typed by a name.
func (reading *wsReading) wrapperFields(element XDTOName) ([]*xdtoNode, bool) {
	node := reading.elements[element]
	if node == nil {
		return nil, false
	}
	var complex *xdtoNode
	if written, ok := xmlAttribute(node, "type"); ok {
		complex = reading.complexTypes[reading.qname(node, written)]
	} else if inner := xmlChildren(node, xmlSchemaNamespace, "complexType"); len(inner) > 0 {
		complex = inner[0]
	}
	if complex == nil {
		return nil, false
	}
	sequences := xmlChildren(complex, xmlSchemaNamespace, "sequence")
	if len(sequences) == 0 {
		return nil, false
	}
	var fields []*xdtoNode
	for _, child := range sequences[0].children {
		if child.name.Space != xmlSchemaNamespace {
			continue
		}
		if child.name.Local != "element" {
			return nil, false
		}
		if _, typed := xmlAttribute(child, "type"); !typed {
			return nil, false
		}
		fields = append(fields, child)
	}
	return fields, len(fields) > 0
}

// unwrap gives an operation the fields of its request and response.
func (reading *wsReading) unwrap(operation *WSOperation, request, response []*xdtoNode) {
	fieldType := func(field *xdtoNode) *XDTOName {
		written, _ := xmlAttribute(field, "type")
		typed := reading.qname(field, written)
		return &typed
	}
	name := func(field *xdtoNode) string {
		value, _ := xmlAttribute(field, "name")
		return value
	}
	asked := map[string]int{}
	for _, field := range request {
		asked[name(field)] = len(operation.Parameters)
		operation.Parameters = append(operation.Parameters, WSParameter{Name: name(field), Type: fieldType(field), Direction: "in"})
	}
	if len(response) == 1 {
		operation.ReturnType = fieldType(response[0])
		minimum, _ := xmlAttribute(response[0], "minOccurs")
		operation.Nillable = minimum == "0"
		return
	}
	for _, field := range response {
		if at, both := asked[name(field)]; both {
			operation.Parameters[at].Direction = "in-out"
			continue
		}
		operation.Parameters = append(operation.Parameters, WSParameter{Name: name(field), Type: fieldType(field), Direction: "out"})
	}
}
