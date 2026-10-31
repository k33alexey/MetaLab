package metadata

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

// wsShop is a description written for the test: a schema of global elements
// of their own types and of named ones, a type of text with an attribute, a
// type open and mixed, an enumeration, a field of a nameless type, and a
// schema imported from «1.xsd» that imports «2.xsd»; six operations, one of
// each rule.
const wsShop = `<definitions xmlns="http://schemas.xmlsoap.org/wsdl/" xmlns:soap="http://schemas.xmlsoap.org/wsdl/soap/"
  xmlns:tns="urn:shop" xmlns:s="http://www.w3.org/2001/XMLSchema" targetNamespace="urn:shop">
  <types>
    <s:schema targetNamespace="urn:shop" elementFormDefault="qualified">
      <s:import namespace="urn:goods" schemaLocation="file://1.xsd"/>
      <s:element name="Price"><s:complexType><s:sequence><s:element name="code" type="s:string" minOccurs="0"/><s:element name="day" type="s:date"/></s:sequence></s:complexType></s:element>
      <s:element name="PriceResponse"><s:complexType><s:sequence><s:element name="sum" type="s:decimal" minOccurs="0"/></s:sequence></s:complexType></s:element>
      <s:element name="Swap" type="tns:Pair"/>
      <s:element name="SwapResponse" type="tns:Pair2"/>
      <s:element name="Ping"><s:complexType><s:sequence/></s:complexType></s:element>
      <s:element name="PingResponse"><s:complexType><s:sequence><s:element name="ok" type="s:boolean"/></s:sequence></s:complexType></s:element>
      <s:element name="Deep"><s:complexType><s:sequence><s:element name="item"><s:complexType><s:sequence><s:element name="code" type="s:string"/></s:sequence></s:complexType></s:element></s:sequence></s:complexType></s:element>
      <s:element name="DeepResponse"><s:complexType><s:sequence><s:element name="ok" type="s:boolean"/></s:sequence></s:complexType></s:element>
      <s:element name="Bag"><s:complexType mixed="true"><s:sequence><s:any/></s:sequence></s:complexType></s:element>
      <s:element name="Note"><s:complexType><s:sequence><s:element name="note"/></s:sequence></s:complexType></s:element>
      <s:element name="alpha"><s:complexType/></s:element>
      <s:complexType name="Phone"><s:simpleContent><s:extension base="s:string"><s:attribute name="kind" type="tns:Kind"/></s:extension></s:simpleContent></s:complexType>
      <s:complexType name="Pair"><s:sequence><s:element name="left" type="s:int"/><s:element name="right" type="s:int" maxOccurs="unbounded"/></s:sequence></s:complexType>
      <s:complexType name="Pair2"><s:sequence><s:element name="left" type="s:int"/><s:element name="total" type="s:int" nillable="true"/></s:sequence></s:complexType>
      <s:simpleType name="Kind"><s:restriction base="s:string"><s:enumeration value="Home"/><s:enumeration value="Work"/><s:maxLength value="4"/></s:restriction></s:simpleType>
    </s:schema>
  </types>
  <message name="PriceIn"><part name="parameters" element="tns:Price"/></message>
  <message name="PriceOut"><part name="parameters" element="tns:PriceResponse"/></message>
  <message name="SwapIn"><part name="parameters" element="tns:Swap"/></message>
  <message name="SwapOut"><part name="parameters" element="tns:SwapResponse"/></message>
  <message name="PingIn"><part name="parameters" element="tns:Ping"/></message>
  <message name="PingOut"><part name="parameters" element="tns:PingResponse"/></message>
  <message name="DeepIn"><part name="parameters" element="tns:Deep"/></message>
  <message name="DeepOut"><part name="parameters" element="tns:DeepResponse"/></message>
  <message name="CallIn"><part name="a" type="s:int"/><part name="b" type="s:int"/></message>
  <message name="CallOut"><part name="result" type="s:int"/></message>
  <message name="SplitOut"><part name="left" type="s:int"/><part name="right" type="s:int"/></message>
  <portType name="ShopPort">
    <operation name="Price"><documentation>Цена товара</documentation><input message="tns:PriceIn"/><output message="tns:PriceOut"/></operation>
    <operation name="Swap"><input message="tns:SwapIn"/><output message="tns:SwapOut"/></operation>
    <operation name="Ping"><input message="tns:PingIn"/><output message="tns:PingOut"/></operation>
    <operation name="Deep"><input message="tns:DeepIn"/><output message="tns:DeepOut"/></operation>
    <operation name="Call"><input message="tns:CallIn"/><output message="tns:CallOut"/></operation>
    <operation name="Split"><input message="tns:CallIn"/><output message="tns:SplitOut"/></operation>
  </portType>
  <binding name="ShopSoap" type="tns:ShopPort"/>
  <binding name="ShopSoap12" type="tns:ShopPort"/>
  <service name="Shop"><documentation>Магазин</documentation>
    <port name="ShopSoap" binding="tns:ShopSoap"/><port name="ShopSoap12" binding="tns:ShopSoap12"/>
  </service>
</definitions>`

const wsGoods = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" targetNamespace="urn:goods">
  <xs:import namespace="urn:units" schemaLocation="file://2.xsd"/>
  <xs:element name="Item" type="xs:string"/>
  <xs:complexType name="Box"><xs:sequence><xs:element name="weight" type="xs:double"/></xs:sequence></xs:complexType>
</xs:schema>`

// The description of a WS reference is read into the model its window
// shows (ws-ssylka.md, «Окно», the pairs «описание — окно» and «Правило
// найдено четвёртым заходом»; probed on the four descriptions of the
// inventory - the calculator, ContragentWebService with its 1.xsd,
// ws-reports and its copy of the inputs filled: the packages, their
// properties and types, and the 23 operations came out as shot). The
// expectations are written out here from the inventory.
// Defects caught: a schema imported not made a package of its own; a
// property out of the order of the description; an object type out of the
// prototype's order - named by the alphabet, then the nameless ones of the
// global elements, each followed by those of its fields «<элемент>_<поле>»;
// a type of text read without «__content», its attribute not «Атрибут»; a
// field of no type given one;
// any and mixed not open, mixed and sequenced; an operation unwrapped
// against the rule or not unwrapped by it; a field of both «Входной»; a
// response of one field not the type of return, its minOccurs="0" not
// «Возможно пустое»; a field of minOccurs="0" or nillable="true" not
// «Возможно пустое» (series-2.603/29-31); a package before the one it
// imports (series-2.603/28); a call of several parts of the response
// without its parameters «Выходной», or with a type of return
// (series-2.603/36-38).
func TestTheDescriptionOfAWSReferenceIsReadAsItsWindowShowsIt(t *testing.T) {
	model, err := ReadWSDefinition([]byte("\uFEFF"+wsShop), func(name string) ([]byte, error) {
		switch name {
		case "1.xsd":
			return []byte(wsGoods), nil
		case "2.xsd":
			return []byte(`<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" targetNamespace="urn:units"><xs:element name="Unit" type="xs:string"/></xs:schema>`), nil
		}
		return nil, errors.New("no " + name)
	})
	if err != nil {
		t.Fatal(err)
	}
	type shown struct {
		Namespace   string
		Imports     []string
		Properties  []string
		ObjectTypes []string
		ValueTypes  []string
	}
	var packages []shown
	for _, item := range model.Packages {
		one := shown{Namespace: item.Namespace, Imports: item.Imports}
		for _, property := range item.Properties {
			typed := ""
			if property.Type != nil {
				typed = property.Type.Name
			}
			one.Properties = append(one.Properties, property.Name+":"+typed)
		}
		for _, object := range item.ObjectTypes {
			var fields []string
			for _, field := range object.Properties {
				fields = append(fields, field.Name+"/"+field.Form)
			}
			flags := ""
			for _, flag := range []*bool{object.Open, object.Mixed, object.Sequenced} {
				if flag != nil && *flag {
					flags += "+"
				} else {
					flags += "-"
				}
			}
			one.ObjectTypes = append(one.ObjectTypes, object.Name+"("+strings.Join(fields, ",")+")"+flags)
		}
		for _, value := range item.ValueTypes {
			var values []string
			for _, enumeration := range value.Enumerations {
				values = append(values, enumeration.Value)
			}
			one.ValueTypes = append(one.ValueTypes, value.Name+"="+strings.Join(values, ","))
		}
		packages = append(packages, one)
	}
	// The packages imported before the package importing them, the deepest
	// first (series-2.603/28).
	want := []shown{
		{Namespace: "urn:units", Properties: []string{"Unit:string"}},
		{Namespace: "urn:goods", Imports: []string{"urn:units"}, Properties: []string{"Item:string"}, ObjectTypes: []string{"Box(weight/)---"}},
		{Namespace: "urn:shop", Imports: []string{"urn:goods"},
			Properties: []string{"Price:Price", "PriceResponse:PriceResponse", "Swap:Pair", "SwapResponse:Pair2", "Ping:Ping", "PingResponse:PingResponse",
				"Deep:Deep", "DeepResponse:DeepResponse", "Bag:Bag", "Note:Note", "alpha:alpha"},
			ObjectTypes: []string{"Pair(left/,right/)---", "Pair2(left/,total/)---", "Phone(kind/Attribute,__content/Text)---",
				"Bag()+++", "Deep(item/)---", "Deep_item(code/)---", "DeepResponse(ok/)---", "Note(note/)---", "Ping()---", "PingResponse(ok/)---",
				"Price(code/,day/)---", "PriceResponse(sum/)---", "alpha()---"},
			ValueTypes: []string{"Kind=Home,Work"}},
	}
	got, _ := json.Marshal(packages)
	expected, _ := json.Marshal(want)
	if string(got) != string(expected) {
		t.Errorf("the packages:\n%s\nwant\n%s", got, expected)
	}
	shop := model.Packages[2]
	if form := shop.ElementFormQualified; form == nil || !*form || shop.AttributeFormQualified != nil {
		t.Error("the qualified forms of the schema are not as written")
	}
	right := shop.ObjectTypes[0].Properties[1]
	if right.UpperBound == nil || *right.UpperBound != -1 || right.LowerBound != nil {
		t.Errorf("maxOccurs unbounded: %+v", right)
	}
	var operations []string
	if len(model.Services) != 1 || model.Services[0].Name != "Shop" || model.Services[0].Documentation != "Магазин" || len(model.Services[0].Ports) != 2 ||
		model.Services[0].Ports[1].Name != "ShopSoap12" {
		t.Fatalf("the services: %+v", model.Services)
	}
	for _, operation := range model.Services[0].Ports[0].Operations {
		var parameters []string
		for _, parameter := range operation.Parameters {
			typed := ""
			if parameter.Type != nil {
				typed = parameter.Type.Name
			}
			parameters = append(parameters, parameter.Name+":"+typed+":"+parameter.Direction+map[bool]string{true: "?"}[parameter.Nillable])
		}
		returned := ""
		if operation.ReturnType != nil {
			returned = operation.ReturnType.Name
		}
		operations = append(operations, operation.Name+"("+strings.Join(parameters, ",")+")"+returned+map[bool]string{true: "?"}[operation.Nillable])
	}
	if got, want := strings.Join(operations, " "),
		"Price(code:string:in?,day:date:in)decimal? Swap(left:int:in-out,right:int:in,total:int:out?) "+
			"Ping(parameters:Ping:in)PingResponse Deep(parameters:Deep:in)DeepResponse Call(a:int:in,b:int:in)int "+
			"Split(a:int:in,b:int:in,left:int:out,right:int:out)"; got != want {
		t.Errorf("the operations:\n%s\nwant\n%s", got, want)
	}
	if model.Services[0].Ports[0].Operations[0].Documentation != "Цена товара" {
		t.Error("the documentation of an operation is lost")
	}
}

// A description the model cannot read answers why: not XML, a name by a
// prefix nobody declared, a schema named and not kept.
// Defects caught: such a description shown as an empty one.
func TestADescriptionThatCannotBeReadSaysWhy(t *testing.T) {
	none := func(string) ([]byte, error) { return nil, errors.New("not kept") }
	for what, description := range map[string]string{
		"not XML": "<definitions",
		"prefix":  `<definitions xmlns="http://schemas.xmlsoap.org/wsdl/"><message name="M"><part name="p" element="x:E"/></message></definitions>`,
		"schema": `<definitions xmlns="http://schemas.xmlsoap.org/wsdl/"><types><s:schema xmlns:s="http://www.w3.org/2001/XMLSchema">` +
			`<s:import namespace="u" schemaLocation="file://1.xsd"/></s:schema></types></definitions>`,
	} {
		if _, err := ReadWSDefinition([]byte(description), none); err == nil {
			t.Errorf("%s: read", what)
		}
	}
}
