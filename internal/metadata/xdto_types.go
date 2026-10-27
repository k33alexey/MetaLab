package metadata

import (
	"slices"
	"strings"
)

// The types a property of an XDTO package, an operation of a web service
// and its parameter are chosen from, by namespace (docs/block-00/
// web-servis.md, «Выбор типа из палитры параметра»; configurator-2026-10-27/
// series-2.595/notes.md, В9): the types of XML Schema and of the platform's
// namespaces are the platform's knowledge; a package of the project gives
// its own value types and object types.

// xmlSchemaTypes are the 91 types the window lists under XML Schema - the
// built-in types among the types of the schema of schemas - in its order:
// by the alphabet whatever the case.
var xmlSchemaTypes = []string{
	"all", "allNNI", "annotated", "anySimpleType", "anyType", "anyURI", "attribute", "attributeGroup", "attributeGroupRef", "base64Binary",
	"blockSet", "boolean", "byte", "complexRestrictionType", "complexType", "date", "dateTime", "decimal", "derivationControl",
	"derivationSet", "double", "duration", "element", "ENTITIES", "ENTITY", "explicitGroup", "extensionType", "facet", "float", "formChoice",
	"fullDerivationSet", "gDay", "gMonth", "gMonthDay", "group", "groupRef", "gYear", "gYearMonth", "hexBinary", "ID", "IDREF", "IDREFS",
	"int", "integer", "keybase", "language", "localComplexType", "localElement", "localSimpleType", "long", "Name", "namedAttributeGroup",
	"namedGroup", "namespaceList", "narrowMaxMin", "NCName", "negativeInteger", "NMTOKEN", "NMTOKENS", "noFixedFacet", "nonNegativeInteger",
	"nonPositiveInteger", "normalizedString", "NOTATION", "numFacet", "openAttrs", "positiveInteger", "public", "QName", "realGroup",
	"reducedDerivationControl", "restrictionType", "short", "simpleDerivationSet", "simpleExplicitGroup", "simpleExtensionType",
	"simpleRestrictionType", "simpleType", "string", "time", "token", "topLevelAttribute", "topLevelComplexType", "topLevelElement",
	"topLevelSimpleType", "typeDerivationControl", "unsignedByte", "unsignedInt", "unsignedLong", "unsignedShort", "wildcard",
}

// platformXDTOTypes are the types of the platform's namespaces the window
// lists, by namespace; of the 66 only http://v8.1c.ru/8.1/data/core is shot
// (46). The types of the rest are the question В6 of series 2.603.
var platformXDTOTypes = map[string][]string{
	"http://v8.1c.ru/8.1/data/core": {
		"AllowedLength", "AllowedSign", "Array", "BinaryDataQualifiers", "CompositeID", "DataFillError", "DataFillErrors", "DateFractions",
		"DateQualifiers", "ErrorCategory", "Exception", "FileDragMode", "FillCheckErrorStatus", "FillChecking", "FixedArray", "FixedMap",
		"FixedStructure", "GenericException", "KeyAndValue", "LocalFormattedStringType", "LocalStringItemType", "LocalStringType",
		"MainClientApplicationWindowMode", "Map", "Null", "NumberQualifiers", "ObjectVersion", "StandardBeginningDate",
		"StandardBeginningDateVariant", "StandardPeriod", "StandardPeriodVariant", "StringQualifiers", "Structure", "Type", "TypeDescription",
		"UUID", "ValueListItemType", "ValueListType", "ValueStorage", "ValueTable", "ValueTableColumn", "ValueTableIndex", "ValueTableRow",
		"ValueTree", "ValueTreeColumn", "ValueTreeRow",
	},
}

// XDTOTypes answers the types a type is chosen from, by namespace: XML
// Schema, the platform's namespaces whose types are known, and every
// package of the project - its value types and object types by the
// alphabet whatever the case, as the window lists them.
func (catalog *Catalog) XDTOTypes() map[string][]string {
	byNamespace := map[string][]string{xmlSchemaNamespace: slices.Clone(xmlSchemaTypes)}
	for space, names := range platformXDTOTypes {
		byNamespace[space] = slices.Clone(names)
	}
	for _, item := range catalog.XDTOPackages {
		names := []string{}
		if content, ok := catalog.xdtoContents[item.ID]; ok {
			for _, value := range content.ValueTypes {
				names = append(names, value.Name)
			}
			for _, object := range content.ObjectTypes {
				names = append(names, object.Name)
			}
		}
		slices.SortStableFunc(names, func(one, other string) int { return strings.Compare(strings.ToLower(one), strings.ToLower(other)) })
		byNamespace[item.Namespace] = names
	}
	return byNamespace
}
