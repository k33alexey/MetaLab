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
// lists, by namespace, in its order: by the alphabet whatever the case
// (paket-xdto.md, «Проверить пакет, экспорт схемы, окно типа»;
// configurator-2026-10-27/series-2.603/xdto-types.txt - 64 namespaces of
// the platform and the envelope of SOAP, data/core shot before, series
// 2.595). The types of current-config are the project's own objects by
// kind, whose rule the inventory does not give yet (series 2.618,
// question): the window shows it with none.
var platformXDTOTypes = map[string][]string{
	"http://schemas.xmlsoap.org/soap/envelope/": {
		"Body", "detail", "encodingStyle", "Envelope", "Fault", "Header",
	},
	"http://v8.1c.ru/8.1/data-composition-system/appearance-template": {
		"AppearanceTemplate", "AppearanceTemplateAppearance", "AppearanceTemplateArea", "AppearanceTemplateAreaItem",
	},
	"http://v8.1c.ru/8.1/data-composition-system/area-template": {
		"AreaTemplate", "AreaTemplateItem", "AreaTemplateParameter", "ChartAppearance", "ChartGroupAppearance", "ChartGroupTemplate",
		"ChartResourceAppearance", "ChartResourceTemplate", "ChartTemplate", "DetailsAreaTemplateParameter",
		"DetailsAreaTemplateParameterFieldExpression", "DocumentAreaTemplate", "ExpressionAreaTemplateParameter", "Field", "FieldAppearance",
		"TableCell", "TableCellAppearance", "TableRow", "TemplateItem", "ValueCollectionCell", "ValueCollectionHeaderCell",
		"ValueCollectionHeaderTemplate", "ValueCollectionTemplate",
	},
	"http://v8.1c.ru/8.1/data-composition-system/common": {
		"DataCompositionAccountingBalanceType", "DataCompositionAreaTemplateType", "DataCompositionBalanceType", "DataCompositionPeriodType",
		"DataSetFieldRole", "Expression", "OrderExpression",
	},
	"http://v8.1c.ru/8.1/data-composition-system/composition-template": {
		"ChartBodyTemplate", "ChartGroup", "ChartGroupBodyItem", "ChartGroupTemplate", "ChartHierarchicalGroup",
		"DataCompositionDataSetsLinkType", "DataCompositionPart", "DataCompositionPartChart", "DataCompositionPartGroup",
		"DataCompositionPartHierarchicalGroup", "DataCompositionPartHierarchicalRecords", "DataCompositionPartNestedObject",
		"DataCompositionPartRecords", "DataCompositionPartTable", "DataCompositionPartTemplate", "DataCompositionTemplate", "DataSet",
		"DataSetField", "DataSetLink", "DataSetNested", "DataSetObject", "DataSetQuery", "DataSetUnion", "DataSource", "Group", "ParameterValue",
		"PeriodAddition", "TableBodyTemplate", "TableGroup", "TableGroupBodyItem", "TableGroupTemplate", "TableHierarchicalGroup",
		"TableHierarchicalRecords", "TableRecords", "TemplateDefinition",
	},
	"http://v8.1c.ru/8.1/data-composition-system/core": {
		"Appearance", "ChoiceParameter", "ChoiceParameterLink", "ChoiceParameterLinks", "ChoiceParameters", "DataCompositionFieldsTitleType",
		"DataCompositionGroupType", "DataCompositionParameterUse", "DataCompositionPeriodAdditionType", "DataCompositionSortDirection",
		"DataCompositionTextPlacementType", "DataCompositionTotalPlacement", "DesignTimeValue", "Field", "ID", "InputParameters", "Parameter",
		"ParameterValue", "ParameterValues", "TypeLink",
	},
	"http://v8.1c.ru/8.1/data-composition-system/details": {
		"DataCompositionDetailsProcessingAction", "DetailsID", "DetailsInformation", "DetailsItem", "DetailsItemFields", "DetailsItemGroup",
		"DetailsProcessDescription", "FieldValue",
	},
	"http://v8.1c.ru/8.1/data-composition-system/result": {
		"DataCompositionResultItem", "DataCompositionResultItemType", "DataCompositionResultNestedItemsLayout",
	},
	"http://v8.1c.ru/8.1/data-composition-system/schema": {
		"AvailableValue", "CalculatedField", "DataCompositionSchema", "DataSet", "DataSetField", "DataSetFieldField", "DataSetFieldFolder",
		"DataSetFieldNestedDataSet", "DataSetLink", "DataSetObject", "DataSetQuery", "DataSetUnion", "DataSource", "FieldTemplate",
		"FieldUseRestriction", "GroupTemplate", "NestedDataCompositionSchema", "Parameter", "TemplateDefinition", "TotalField",
		"TotalFieldsTemplate",
	},
	"http://v8.1c.ru/8.1/data-composition-system/settings": {
		"AppearanceField", "AppearanceFields", "AvailableFields", "AvailableFieldUseRestriction", "AvailableFieldUseRestrictions",
		"AvailableParameterUseRestriction", "AvailableParameterUseRestrictions", "AvailableSettingsSource", "Case", "CaseItem", "ChartGroup",
		"ChartGroupOutputParameterValues", "ChartOutputParameterValues", "CompositionOutputParameterValues", "ConditionalAppearance",
		"ConditionalAppearanceItem", "DataCompositionAttributesPlacement", "DataCompositionChartLegendPlacement",
		"DataCompositionComparisonType", "DataCompositionConditionalAppearanceUse", "DataCompositionDatabaseCopyOutputType",
		"DataCompositionDataRelevanceOutputType", "DataCompositionFieldPlacement", "DataCompositionFilterApplicationType",
		"DataCompositionFilterItemsGroupType", "DataCompositionFixation", "DataCompositionGroupFieldsPlacement", "DataCompositionGroupPlacement",
		"DataCompositionGroupTemplateType", "DataCompositionGroupUseVariant", "DataCompositionPictureOutputType",
		"DataCompositionResourcesAutoPosition", "DataCompositionResourcesPlacement", "DataCompositionResourcesPlacementInChart",
		"DataCompositionSettingsItemState", "DataCompositionSettingsItemViewMode", "DataCompositionSettingsViewMode",
		"DataCompositionTextOutputType", "DataParameterValues", "Filter", "FilterItem", "FilterItemComparison", "FilterItemGroup", "GroupItem",
		"GroupItemAuto", "GroupItemField", "GroupItems", "GroupOutputParameterValues", "Order", "OrderItem", "OrderItemAuto", "OrderItemField",
		"SaveDataCompositionAppearance", "SelectedItem", "SelectedItemAuto", "SelectedItemField", "SelectedItemFolder", "Selection", "Settings",
		"SettingsComposer", "SettingsParameterValue", "SettingStructure", "SettingsVariant", "StructureItem", "StructureItemChart",
		"StructureItemGroup", "StructureItemNestedObject", "StructureItemTable", "TableGroup", "TableGroupOutputParameterValues",
		"TableOutputParameterValues", "UserField", "UserFieldCase", "UserFieldExpression", "UserFields", "UserSettings",
	},
	"http://v8.1c.ru/8.1/data/core": {
		"AllowedLength", "AllowedSign", "Array", "BinaryDataQualifiers", "CompositeID", "DataFillError", "DataFillErrors", "DateFractions",
		"DateQualifiers", "ErrorCategory", "Exception", "FileDragMode", "FillCheckErrorStatus", "FillChecking", "FixedArray", "FixedMap",
		"FixedStructure", "GenericException", "KeyAndValue", "LocalFormattedStringType", "LocalStringItemType", "LocalStringType",
		"MainClientApplicationWindowMode", "Map", "Null", "NumberQualifiers", "ObjectVersion", "StandardBeginningDate",
		"StandardBeginningDateVariant", "StandardPeriod", "StandardPeriodVariant", "StringQualifiers", "Structure", "Type", "TypeDescription",
		"UUID", "ValueListItemType", "ValueListType", "ValueStorage", "ValueTable", "ValueTableColumn", "ValueTableIndex", "ValueTableRow",
		"ValueTree", "ValueTreeColumn", "ValueTreeRow",
	},
	"http://v8.1c.ru/8.1/data/enterprise": {
		"AccountingRecordType", "AccountType", "AccumulationRecordType", "AccumulationRegisterAggregatePeriodicity",
		"AccumulationRegisterAggregateUse", "Aggregate", "AggregateList", "AnyDBRef", "AnyIBRef", "AnyMDEnum", "AnyRef", "AutoShowStateMode",
		"AutoTimeMode", "Bound", "CalculationRegisterPeriodType", "ClientApplicationBaseFontVariant", "ClientApplicationFormScaleVariant",
		"ClientApplicationInterfaceVariant", "ClientApplicationType", "ClientConnectionSpeed", "ComparisonType", "DatabaseCopiesUse",
		"DataChangeType", "DataLineChangeType", "DocumentPostingMode", "DocumentWriteMode", "Filter", "FilterItem", "FoldersAndItemsUse",
		"JobSchedule", "JobScheduleMonths", "JobScheduleWeekDays", "LinkedValueChangeMode", "MessageStatus", "ModalityUseMode", "ObjectDeletion",
		"PointOfTime", "PointOfTimeWithPeriodAdjustment", "PostingModeUse", "PredefinedDataUpdate", "ReplacementMode", "ReportResultViewMode",
		"RequiredDataRelevance", "ResultCompositionMode", "SliceUse", "SortDirection", "TransactionsIsolationLevel", "UpdateOnDataChange",
		"UserMessage", "UserMessages", "ViewModeApplicationOnSetReportResult",
	},
	"http://v8.1c.ru/8.1/data/txtedt": {
		"MaketParameter", "TextDocument",
	},
	"http://v8.1c.ru/8.1/data/ui": {
		"AbsoluteColor", "AbstractRef", "AutoColor", "AutoSeriesSeparation", "Border", "BorderType", "ChartColorPalette",
		"ChartLabelsOrientation", "ChartLabelType", "ChartLineType", "ChartType", "Color", "ControlBorderType", "Font", "FontType",
		"FormattedString", "GaugeChartQualityBand", "GaugeChartQualityBands", "GeographicalSchemeLineType", "HorizontalAlign", "Key", "Line",
		"LineType", "Picture", "PictureFormat", "PictureRef", "PivotChartType", "ShortCutType", "SizeChangeMode",
		"SpreadsheetDocumentCellLineType", "SpreadsheetDocumentDrawingLineType", "StyleRef", "TextDirection", "TextEncoding", "UseOutput",
		"VerticalAlign",
	},
	"http://v8.1c.ru/8.1/xdto": {
		"Form", "Import", "MemberTypes", "Model", "ObjectType", "Package", "Property", "Type", "ValueType", "Variety", "Whitespace",
	},
	"http://v8.1c.ru/8.2/bsl": {
		"CompileError", "CompileException", "RuntimeException", "StackTraceElement",
	},
	"http://v8.1c.ru/8.2/data/bsl": {
		"BSLAnnotation", "BSLAnnotationAttribute", "BSLLabel", "BSLModuleImage", "BSLProcedureImage", "BSLVariableImage",
	},
	"http://v8.1c.ru/8.2/data/chart": {
		"AddedItem", "BarChartPointsOrder", "BubbleChartNegativeValuesShowMode", "Chart", "ChartAnimation", "ChartAxis",
		"ChartBoundaryDetectionMethod", "ChartBubbleSizeValueSource", "ChartBubbleSizing", "ChartChanges", "ChartColorPaletteDescription",
		"ChartGridLinesShowMode", "ChartInterval", "ChartLabelArea", "ChartLabelLocation", "ChartLegendPlacement", "ChartMarkerType",
		"ChartOrientation", "ChartPlotAreaPlacement", "ChartPoints", "ChartPointsAxisValuesSource", "ChartPointsConnectionType",
		"ChartReferenceBand", "ChartReferenceBandBorderPosition", "ChartReferenceBands", "ChartReferenceLine", "ChartReferenceLinePosition",
		"ChartReferenceLines", "ChartScale", "ChartScaleLabelLocation", "ChartScaleLocation", "ChartScaleMarkLocation",
		"ChartScaleTitlePlacement", "ChartScaleTitleTextSource", "ChartSelectedObjects", "ChartSelectionMode", "ChartSemitransparencyMode",
		"ChartSeries", "ChartSeriesGraphicalRepresentationType", "ChartSeriesOrderInLegend", "ChartSeriesStackType", "ChartSpaceMode",
		"ChartSplineMode", "ChartsViewSettings", "ChartTitleAreaPlacement", "ChartTrendline", "ChartTrendlineApproximationType",
		"ChartTrendlineFactor", "ChartTrendlines", "ChartValue", "ChartValueEditState", "ChartValuesBySeriesConnectionType",
		"ChartValuesEditMode", "ChartValuesToolTipFillType", "ChartValuesToolTipShowMode", "Collect", "CollectionChange", "CollectItem",
		"DataChanges", "Dendrogram", "DendrogramContent", "DendrogramDimensionValue", "DendrogramElements", "DendrogramLinks",
		"DendrogramLinkValue", "DendrogramOrientation", "DendrogramScaleKeeping", "DoubleRect", "GanttChart", "GanttChartBackgroundIntervals",
		"GanttChartDTDataColumn", "GanttChartDTValueData", "GanttChartIntervalID", "GanttChartIntervalRepresentation",
		"GanttChartIntervalsSelectionParams", "GanttChartIntervalTextRepresentation", "GanttChartLink", "GanttChartLinkType",
		"GanttChartPointValueContent", "GanttChartScaleKeeping", "GanttChartSeriesValueContent", "GanttChartTextPlacementType",
		"GanttChartValueID", "GanttChartValuesSelectionParams", "GanttChartValueTextRepresentation", "GanttChartVerticalStretch",
		"GaugeChartValueRepresentation", "GaugeChartValuesScaleLabelsLocation", "MaxSeries", "MovedItem", "NonnumericChartValueUse",
		"PivotChart", "PivotChartLabelsOrientation", "PivotChartScaleKeeping", "PivotChartType", "PivotChartValuesShowMode", "PointData",
		"PointDimensionValue", "PointsConnectionAcrossSkippedChartValuesType", "RadarChartScaleType", "RealDataItem", "RealDataItems",
		"SeriesData", "SeriesDimensionValue", "ShowChartPopupReferenceLine", "ShowChartScaleTitle", "ShowInChart", "ShowInChartLegend",
		"ShowInGanttChart", "StockChartUsedPointValue", "TimeScale", "TimeScaleDayFormat", "TimeScaleLabel", "TimeScaleLabels", "TimeScaleLevel",
		"TimeScalePosition", "TimeScaleUnitType", "UsedChartValuesAxis",
	},
	"http://v8.1c.ru/8.2/data/data-analysis": {
		"AnalysisDataType", "AssociationRulesDataSourceType", "AssociationRulesPruneType", "ClusterizationMethod",
		"DataAnalysisAssociationRulesOrderType", "DataAnalysisColumnType", "DataAnalysisColumnTypeAssociationRules",
		"DataAnalysisColumnTypeClusterization", "DataAnalysisColumnTypeDecisionTree", "DataAnalysisColumnTypeSequentialPatterns",
		"DataAnalysisColumnTypeSummaryStatistics", "DataAnalysisDistanceMetricType", "DataAnalysisNumericValueUseType",
		"DataAnalysisResultTableFillType", "DataAnalysisSequentialPatternsOrderType", "DataAnalysisStandardizationType",
		"DataAnalysisTimeIntervalUnitType", "DecisionTreeSimplificationType", "PredictionModelColumnType",
	},
	"http://v8.1c.ru/8.2/data/formatted-document": {
		"Content", "FormattedDocument", "FormattedDocumentBookmark", "FormattedDocumentFileType", "FormattedDocumentItemType",
		"FormattedDocumentRange", "HorizontalAlignment", "Paragraph", "ParagraphType", "PictureElement", "SoftBreakElement", "TextAppearance",
		"TextElement",
	},
	"http://v8.1c.ru/8.2/data/geo": {
		"dp", "dps", "drawing", "dsi", "GeographicalSchema", "GeographicalSchemaDataSourceOrganizationType",
		"GeographicalSchemaLayerSeriesShowMode", "GeographicalSchemaLegendItemShowScaleType", "GeographicalSchemaLineType",
		"GeographicalSchemaMarkerType", "GeographicalSchemaPointObjectDrawingType", "GeographicalSchemaProjection", "GeographicalSchemaShowMode",
		"ipsv", "layer", "legend", "legItem", "lobj", "mps", "oas", "PaintingReferencePointPosition", "pgs", "pls", "po", "ps", "sad", "scsi",
		"SeriesValuesDrawingMode", "sps", "title", "vi",
	},
	"http://v8.1c.ru/8.2/data/graphscheme": {
		"ArrowStyle", "ConnectorLineType", "ConnectorTextLocation", "EventHandler", "FitPageMode", "FlowchartContextType",
		"GraphicalSchemaGridDrawMode", "GraphicalSchemaItemPictureLocation", "GraphicalSchemaShapes", "GraphSchemeItem", "Point",
		"PrintPropItem", "Transition", "UserSegment",
	},
	"http://v8.1c.ru/8.2/data/spreadsheet": {
		"Area", "Cell", "ChangedCell", "ChangedCellItem", "ChangedRow", "ChangedRowItem", "Changes", "Column", "Columns", "ColumnsItem",
		"DimensionAttributePlacementType", "DimensionPlacementType", "Drawing", "DrawingsDataSource", "DuplexPrintingType", "EmbeddedTableItem",
		"Format", "Group", "Merge", "MoxelImageMap", "MoxelImageMapItem", "NamedItem", "NamedItemCells", "NamedItemDataSource",
		"NamedItemDrawing", "NamedItemEmbeddedTable", "NamedItemType", "NewComplexType", "PageBreak", "PageOrientation",
		"PagePlacementAlternation", "Picture", "PictureSize", "PivotTable", "PivotTableLinesShowType", "PrintAccuracy", "PrintSettings", "Row",
		"RowCellsItem", "Rows", "RowsItem", "SpreadsheetDocument", "SpreadsheetDocumentAreaFillType", "SpreadsheetDocumentCellAreaType",
		"SpreadsheetDocumentDetailUse", "SpreadsheetDocumentDrawingType", "SpreadsheetDocumentExtensionAlgorithm", "SpreadsheetDocumentFileType",
		"SpreadsheetDocumentGroupHeaderPlacement", "SpreadsheetDocumentPatternType", "SpreadsheetDocumentPointerType",
		"SpreadsheetDocumentSavedPicturesDensity", "SpreadsheetDocumentStepDirectionType", "SpreadsheetDocumentTextPlacementType",
		"StandardAppearance", "TextPositionRelativeToPicture", "ViewSettings",
	},
	"http://v8.1c.ru/8.2/data/test": {
		"ostwb", "RowGotoDirection", "st_TestedClientCollaborationSystemApplicationWindowAppearanceVariant",
		"st_TestedClientCollaborationSystemVideoCallViewType", "TestClientCommand", "TestClientCommandActivate",
		"TestClientCommandAdditionClickViewStatusItem", "TestClientCommandAdditionGetViewStatusItemTexts",
		"TestClientCommandAdditionRemoveViewStatusItem", "TestClientCommandButtonCurrentChecked", "TestClientCommandButtonPress",
		"TestClientCommandCIButtonGetWindow", "TestClientCommandCIButtonPress", "TestClientCommandClearFileDialogResult",
		"TestClientCommandCurrentEnable", "TestClientCommandCurrentReadOnly", "TestClientCommandCurrentVisible",
		"TestClientCommandDecorationChoice", "TestClientCommandEditActivateDate", "TestClientCommandEditCancel", "TestClientCommandEditChoice",
		"TestClientCommandEditChoiceFromChoiceList", "TestClientCommandEditChoiceFromDropList", "TestClientCommandEditClear",
		"TestClientCommandEditClickFormattedDocumentHyperlink", "TestClientCommandEditClickFormattedStringHyperlink",
		"TestClientCommandEditClickHTMLDocumentHyperlink", "TestClientCommandEditClickHyperLine", "TestClientCommandEditCloseDropList",
		"TestClientCommandEditCreate", "TestClientCommandEditDropListIsOpen", "TestClientCommandEditForAMonthBackward",
		"TestClientCommandEditForAMonthForward", "TestClientCommandEditForAYearBackward", "TestClientCommandEditForAYearForward",
		"TestClientCommandEditGetData", "TestClientCommandEditGetDocumentDataAreaHorizontalSize",
		"TestClientCommandEditGetDocumentDataAreaVerticalSize", "TestClientCommandEditGetHTML", "TestClientCommandEditGetSelectedArea",
		"TestClientCommandEditGetSelectedAreaField", "TestClientCommandEditGetSelectedAreaPresentation",
		"TestClientCommandEditGetStatePresentation", "TestClientCommandEditGotoValue", "TestClientCommandEditIncludedInMergedArea",
		"TestClientCommandEditInputHTML", "TestClientCommandEditOpen", "TestClientCommandEditOpenChoiceList",
		"TestClientCommandEditOpenDropList", "TestClientCommandEditSelectOption", "TestClientCommandEditSendBeginEditSelectedArea",
		"TestClientCommandEditSendCurrentModelsEdit", "TestClientCommandEditSendFinishEditSelectedArea", "TestClientCommandEditSendSelectArea",
		"TestClientCommandEditSetCheck", "TestClientCommandEditSpinDown", "TestClientCommandEditSpinUp",
		"TestClientCommandEditTextIsWithinAreaBounds", "TestClientCommandEditWaitForDropListData", "TestClientCommandEditWriteContentToFile",
		"TestClientCommandExecuteChoiceFromList", "TestClientCommandExecuteChoiceFromMenu", "TestClientCommandFormActivateNext",
		"TestClientCommandFormActivatePrev", "TestClientCommandFrameChooseUserMessage", "TestClientCommandFrameCloseUserMessagesPanel",
		"TestClientCommandFrameExecuteCommand", "TestClientCommandFrameGetActiveParticipantsList", "TestClientCommandFrameGetCI",
		"TestClientCommandFrameGetCollaborationSystemVideoCallViewType", "TestClientCommandFrameGetCollaborationWindowsState",
		"TestClientCommandFrameGetCurrentGalleryPageParticipantsList", "TestClientCommandFrameGetCurrentPageParticipantsList",
		"TestClientCommandFrameGetForegroundParticipant", "TestClientCommandFrameGetParticipantsGalleryNextPageAvailable",
		"TestClientCommandFrameGetParticipantsGalleryPrevPageAvailable", "TestClientCommandFrameGetParticipantsListNextPageAvailable",
		"TestClientCommandFrameGetParticipantsListPrevPageAvailable", "TestClientCommandFrameGetUserMessageTexts",
		"TestClientCommandFrameGotoNextWindow", "TestClientCommandFrameGotoPrevWindow", "TestClientCommandFrameGotoStartPage",
		"TestClientCommandFrameSetCollaborationSystemVideoCallViewType", "TestClientCommandFrameShowParticipantsGalleryNextPage",
		"TestClientCommandFrameShowParticipantsGalleryPrevPage", "TestClientCommandFrameShowParticipantsListNextPage",
		"TestClientCommandFrameShowParticipantsListPrevPage", "TestClientCommandGetActiveWindow", "TestClientCommandGetChilds",
		"TestClientCommandGetChoiceListPresentation", "TestClientCommandGetCollaborationSystemPerformanceIndicators",
		"TestClientCommandGetCommandBar", "TestClientCommandGetContextMenu", "TestClientCommandGetCurrentErrorInfo",
		"TestClientCommandGetFormattedStringHyperlinksPresentations", "TestClientCommandGetObjectByPath", "TestClientCommandGetPerformanceInfo",
		"TestClientCommandGetShownText", "TestClientCommandGetText", "TestClientCommandGetToolTip", "TestClientCommandGroupCollapse",
		"TestClientCommandGroupCurrentOpened", "TestClientCommandGroupExpand", "TestClientCommandReadyForCloseConnection",
		"TestClientCommandSetFileDialogResult", "TestClientCommandTestProcess", "TestClientCommandTitleIsShown", "TestClientCommandTypeChar",
		"TestClientCommandUIRecordingActions", "TestClientCommandUserAvailabilityStatus", "TestClientFieldBeginEditMode",
		"TestClientFieldCurrentEditMode", "TestClientFieldFinishEditMode", "TestClientFieldTableChoice", "TestClientFieldTableFirst",
		"TestClientFieldTableGetCurrentCellValue", "TestClientFieldTableGetCurrentColumn", "TestClientFieldTableGetSelectedRows",
		"TestClientFieldTableGotoRow", "TestClientFieldTableIsCurrentRowNodeExpanded", "TestClientFieldTableIsPossibleExpandNode",
		"TestClientFieldTableLast", "TestClientFieldTableLevelDown", "TestClientFieldTableLevelUp", "TestClientFieldTableNext",
		"TestClientFieldTablePrev", "TestClientFieldTableSelectAllRows", "TestClientFieldTableSendActivateColumn",
		"TestClientFieldTableSendAddRow", "TestClientFieldTableSendCollapseNode", "TestClientFieldTableSendCopyRow",
		"TestClientFieldTableSendDeleteRow", "TestClientFieldTableSendExpandNode", "TestClientFieldTableSendMarkToDeleteRow",
		"TestClientFieldTableSetOrder", "TestClientFormClose", "TestClientFormCurrentModified", "TestClientFormGetActiveItem",
		"TestClientFormGetCurrentPage", "TestClientFormGetDefaultButton", "TesteeCI", "TesteeCIButton", "TesteeCIGroup", "TesteeFieldButton",
		"TesteeFieldDecoration", "TesteeFieldEdit", "TesteeFieldGroup", "TesteeFieldTable", "TesteeFormItemAddition", "TesteeManagedForm",
		"TesteeOSWindow", "TesteeTopLevelFrame", "twb", "UIRecordingActions", "v8stwb",
	},
	"http://v8.1c.ru/8.2/managed-application/cmi": {
		"ClientCMIStartUpDataType", "ClientCMIUpdateInterfaceDataType", "Command", "CommandIDs", "CommandInfo", "CommandInterfaceItems",
		"CommandParameterUseMode", "CommandPath", "CommandsInfo", "CommansShortCutInfo", "CommansShortCutInfoVector", "Fragment",
		"FuncMenuCommand", "FuncMenuGroup", "FuncMenuGroupType", "FuncMenuHighlight", "Group", "GroupCategories", "GroupCategory", "GroupInfo",
		"HandlerInfo", "HandlerInfoByID", "HandlerInfoEx", "HandlersInfosVector", "ModuleInfo", "ModuleInfoEx", "Section", "SectionInfo",
		"SubsystemAndPanel", "URLByNavigationPoint", "URLsByNavigationPoints",
	},
	"http://v8.1c.ru/8.2/managed-application/core": {
		"AboutInfo", "Alias", "AllowedIncomingShareRequestType", "AllowedIncomingShareRequestTypes", "ApplicationInfo", "ApplicationUsePurpose",
		"BinaryDataQualifiers", "BoolFormat", "ChoiceParameter", "ChoiceParameterItem", "ChoiceParameterLink", "ChoiceParameterLinks",
		"ChoiceParameters", "ClientDisplayInformation", "ClientRunMode", "CommandGroupCategory", "ConstValueKey", "ConstValueKeySep",
		"ConstValueKeyTypeSet", "DateFormat", "DateQualifiers", "DesktopFormInfo", "DesktopInfo", "Enum", "EnumPresentations", "EnumTypeSet",
		"EnumVal", "EnumVals", "EnumValue", "ErrorMessageDisplayVariant", "ErrorMessagesTexts", "ErrorMessageTemplatesCollectionItem",
		"ErrorMessageTexts", "ErrorOnStartupProcessingSettings", "ErrorProcessingSettings", "ErrorReportingMode", "ExtKey", "ExtKeyKey",
		"ExtKeyTypeSet", "ExtReference", "ExtRefTypeSet", "GlobalSearchFetch", "GlobalSearchHistory", "GlobalSearchHistoryItem",
		"GlobalSearchQuery", "GlobalSearchQueryItem", "GlobalSearchResult", "GlobalSearchResultItem", "GlobalSearchResultItemAction",
		"GlobalSearchResultItemActionCollection", "InfoRegKey", "InfoRegKeyDim", "InfoRegKeyTypeSet", "InterfaceLayouter", "LangSettings",
		"LanguageInfo", "LFEFChoiceHistory", "LFEFChoiceHistoryGroup", "LFEFChoiceHistoryItem", "LFEFChoiceHistoryItems",
		"LFEFChoiceHistorySample", "LocaleBoolFormat", "LocaleDateFormat", "LocaleInfo", "LocaleNumericFormat", "LocaleWeekInfo",
		"MainWindowMode", "MCDigestDeltaItem", "MCDigestItem", "MngLocale", "MobileApplicationFunctionalities", "MobileApplicationURL",
		"MobileApplicationURLs", "NavigationPoint", "NumberQualifiers", "NumericFormat", "Panel", "PanelDef", "PanelGroup", "PanelLayouter",
		"Reference", "RefTypeSet", "RegKey", "RegKeyTypeSet", "RequiredMobileApplicationPermissionMessages",
		"RequiredMobileApplicationPermissions", "RequiredPermission", "RequiredPermissionMessage", "RequiredPermissions", "ResourceString",
		"ResourceStrings", "SearchBlock", "SectionPanelRepresentation", "SettingsChoice", "SplashInfo", "StandardGlobalSearchType", "StartInfo",
		"StringQualifiers", "StyleEntriesInfo", "StyleEntryInfo", "StyleEntryKind", "SubsystemVerInfo", "SysEnum", "SysEnums", "SysEnumValue",
		"Type", "TypeIDList", "TypeLink", "TypePresentation", "TypePresentations", "TypeSet", "TypeSetDef", "TypeSystem", "UpdateInfo",
		"UsedFunctionality", "UsedFunctionalityFlag", "UserFavoriteItem", "UserFavorites", "UUIDAndStringsVectorType",
	},
	"http://v8.1c.ru/8.2/managed-application/dynamic-list-composition-template": {
		"DataPartCell", "DataPartHeader", "DataPartHeaderColumn", "DataPartHeaderOrderColumn", "DataPartHeaderTempTablesDigest",
		"DataPartHeaderTupleOrder", "DataPartRow",
	},
	"http://v8.1c.ru/8.2/managed-application/dynamic-list-server": {
		"FieldAppearance", "ULLegacyDCSCATFieldWithName", "ULSortTuple",
	},
	"http://v8.1c.ru/8.2/managed-application/edbexception": {
		"EDBException",
	},
	"http://v8.1c.ru/8.2/managed-application/InteractiveSecurityException": {
		"InteractiveSecurityException",
	},
	"http://v8.1c.ru/8.2/managed-application/logform": {
		"Addition", "AdditionSource", "AddProp", "AddProps", "AutoCapitalizationOnTextInput", "AutoCorrectionOnTextInput", "AutofillHint",
		"AutoMenuGroupData", "AutoSaveFormDataInSettings", "AutoShowClearButtonMode", "AutoShowOpenButtonMode", "Button",
		"ButtonGroupRepresentation", "ButtonLocationInCommandBar", "ButtonRepresentation", "ButtonsGroupData", "ButtonShape",
		"ButtonShapeRepresentation", "BWAValue", "CalendarFieldData", "ChartFieldData", "CheckBoxFieldData", "CheckBoxType",
		"ChoiceButtonRepresentation", "ChoiceDataGetModeOnInputByString", "ChoiceHistoryOnInput", "CollapseFormItemsByImportance",
		"ColumnsGroup", "ColumnsGroupData", "Command", "CommandsInclusion", "ContextMenuGroupData", "CurrentRowUse", "DataPath", "Decoration",
		"DendrogramFieldData", "DisplayImportance", "DrawingSelectionShowMode", "EditTextUpdate", "ElementsChanges", "ElementsChangesItem",
		"Event", "EventHandler", "EventHandlerCallType", "EventHandlers", "Field", "FileDragMode", "FlowchartFieldData", "FoldersAndItems",
		"Form", "FormattedDocFieldData", "FormattedString", "FormBaseFontVariant", "FormButtonPictureLocation", "FormChildrenAlign",
		"FormChildrenGroup", "FormChildrenWidth", "FormDateSelectionMode", "FormElementCommandBarLocation", "FormElementOrientation",
		"FormElementOrigin", "FormElementTitleLocation", "FormEnterKeyBehavior", "FormFixedInTable", "FormItemSpacing",
		"FormPagesRepresentation", "FormPagesState", "FormProgressBarRepresentation", "FormStandardURLVariant", "FormWindowOpeningMode",
		"FullTextSearchOnInputByString", "GanttChartFieldData", "GanttChartIntervalsSelectionMode", "GanttChartTableLocation",
		"GanttChartValuesSelectionMode", "GeographicalMapFieldData", "Group", "HeightControlVariant", "HTMLFieldData", "ImageFieldData",
		"IncompleteItemChoiceMode", "InputFieldData", "InputFieldMultipleValuePictureShape", "InputFieldMultipleValuePictureSize",
		"ItemHorizontalAlignment", "ItemRef", "ItemVerticalAlignment", "LFEDataPathRuntimeValue", "LFTableListEditingMode",
		"LogFormElementAdditionKind", "LogFormScrollMode", "LogFormShowConversations", "LogFormUserAddedElement", "LogFormUserButtonChange",
		"LogFormUserChangesDescription", "LogFormUserFieldChange", "LogFormUserMovedElement", "LogFormUserTableChange", "MainElement",
		"ManagedFormButtonType", "ManagedFormDecorationType", "ManagedFormFieldType", "ManagedFormGroupType", "MarkingStyle",
		"MenuElementPlacementArea", "MenuGroupData", "OnMainServerUnavalableBehavior", "OnScreenKeyboardReturnKeyText", "PageGroupData",
		"PagesGroupData", "PDFDocumentFieldData", "PeriodFieldData", "PictureDecorationData", "PictureSize", "PlannerFieldData", "Predefined",
		"ProgressBarFieldData", "Property", "RadioButtonsData", "RadioButtonType", "RefreshRequestMethod", "ReportFormType",
		"RepresentationInContextMenu", "SaveFormDataInSettings", "SearchControlData", "SearchControlLocation", "SearchOnInput",
		"SearchStringData", "SearchStringLocation", "SearchStringModeOnInputByString", "SelectionShowMode", "SpecialTextInputMode",
		"SpellCheckingOnTextInput", "SpreadSheetDocumentScrollBarUse", "SpreadsheetFieldData", "SubmenuGroupData", "Table",
		"TableBehaviorOnHorizontalCompression", "TableCurrentRowUse", "TableFieldEditMode", "TableHeightControlVariant", "TableInitialListView",
		"TableInitialTreeView", "TableRepresentation", "TableRowInputMode", "TableRowSelectionMode", "TableScrollBarUse", "TableSelectionMode",
		"TextDecorationData", "TextDocFieldData", "TextFieldData", "TooltipRepresentation", "TrackBarFieldData", "UsedServer",
		"UsualGroupBehavior", "UsualGroupControlRepresentation", "UsualGroupData", "UsualGroupRepresentation", "UsualGroupThroughAlign",
		"VerticalAlign", "ViewScalingMode", "ViewStatusData", "ViewStatusLocation", "WarningOnEditRepresentation",
	},
	"http://v8.1c.ru/8.2/managed-application/logform/layouter": {
		"BackObject", "Button", "BWDValue", "Calendar", "CaptionButtons", "CaptionButtonsLocation", "Chart", "CheckBox",
		"ChoiceButtonRepresentation", "ChoiceObjects", "ColumnMode", "ColumnsGroup", "Command", "CommandBar", "ContextMenu", "ControlCategory",
		"Dendrogram", "Direction", "DrawingSelectionShowMode", "ElementShiftMode", "EmptyElement", "Flowchart", "Form", "FormattedDocument",
		"FormChanges", "FormChangesAdded", "FormChangesAddedItem", "FormChangesChanged", "FormChangesChangedItem", "FormChangesMoved",
		"FormChangesMovedItem", "FormChangesRemoved", "FormChangesRemovedItem", "FormChangesShifted", "FormChangesShiftedItem",
		"FormDateSelectionMode", "FormElementOrientation", "FormEnterKeyBehavior", "FormPanelTabsRepresentation",
		"FormProgressBarRepresentation", "FormTrackBarMarking", "GanttChart", "GeographicalMap", "GroupBox", "HTML", "HTMLChanges",
		"HTMLChangesItem", "Image", "IncompleteItemChoiceMode", "Item", "ItemHorizontalAlignment", "ItemVerticalAlignment", "Label",
		"MenuElementRepresentation", "MenuSeparator", "Navigator", "Page", "Panel", "Period", "PictureSize", "Planner", "ProgressBar",
		"RadioButton", "SearchOnInputForbiddenMode", "SelectionShowMode", "SeparatePanel", "Separator", "Spreadsheet",
		"SpreadSheetDocumentScrollBarUse", "StretchableMode", "Submenu", "TableBox", "TableColumn", "TableColumnsGroup", "TableFieldEditMode",
		"TableInitialListView", "TableInitialTreeView", "TableRowInputMode", "TableRowSelectionMode", "TableScrollBarUse", "TableSelectionMode",
		"TextBox", "TextDocument", "TrackBar", "Tumbler", "VerticalLine", "ViewScalingMode", "ViewStatus", "WarningBeforeEditRepresentation",
	},
	"http://v8.1c.ru/8.2/managed-application/logformexception": {
		"BSLExceptionValue", "FindByStringException", "LogFormException", "UniversalListException",
	},
	"http://v8.1c.ru/8.2/managed-application/modules": {
		"MethodCall", "MethodDef", "MethodDefs", "ModuleContentDef", "ModuleDef", "ModulesDef", "ParamDef", "ParamDirection",
		"ProcessFormsCloseParam",
	},
	"http://v8.1c.ru/8.2/managed-application/objectlocksexception": {
		"ObjectLocksException", "SeanceToFinish",
	},
	"http://v8.1c.ru/8.2/managed-application/remoteexception": {
		"RemoteException",
	},
	"http://v8.1c.ru/8.2/managed-application/seancecontextexception": {
		"SeanceContextException",
	},
	"http://v8.1c.ru/8.2/managed-application/SecondFactorAuthException": {
		"SecondFactorAuthException",
	},
	"http://v8.1c.ru/8.2/managed-application/user-settings": {
		"AddInSettings", "AddInSettingsItem", "ClientSettings", "ComparisonSettings", "HelpSettings", "PrintSettings", "WindowSettings",
	},
	"http://v8.1c.ru/8.2/roles": {
		"ObjectType", "RestrictionTemplateType", "Rights", "RightType", "RlsType",
	},
	"http://v8.1c.ru/8.2/virtual-resource-system": {
		"AccessTokenAuthentication", "AccessTokenIssuer", "AccessTokenIssuers", "Analytics", "Authentication", "AuthenticationLogonView",
		"AuthMethod", "Debug", "DebugProtocol", "Exception", "HttpService", "HttpServices", "MobileAppRecord", "MobileApps", "OData", "OpenID",
		"OpenIDConnectRP", "OpenIDProviderParams", "OpenIDRelyingPartyParams", "Point", "Pool", "ProgressiveWebApplication", "RDbgPoint",
		"WebService", "WebServices", "ZoneInfo", "ZoneInfos",
	},
	"http://v8.1c.ru/8.3/data/entext": {
		"AppearanceArea", "AppearanceAreaItem", "AppearanceAreaType", "AppearanceSetting", "AppearanceSettingItem", "ConditionalAppearance",
		"ConditionalAppearanceItem", "ReportBuilderDimensionType",
	},
	"http://v8.1c.ru/8.3/data/ext": {
		"AdministrationActionOnResourceConsumptionLimitExcess", "AdministrationAssignmentRuleType", "AdministrationConnectionSecurityLevel",
		"AdministrationInfoBaseDeletionMode", "AdministrationProcessChoicePriority", "AdministrationResourceConsumptionCounterFilterType",
		"AdministrationResourceConsumptionCounterGroupType", "AdministrationTest", "AdministrationWorkProcessStatus", "ClientNotification",
		"DeliverableNotificationSendErrorType", "DeliverableNotificationSubscriberID", "DeliverableNotificationSubscriberType",
		"DocumentScanningPage", "DocumentScanningParameters", "DocumentScanningQualityParameters", "GeographicCoordinates",
		"InAppPurchaseReceipt", "InAppPurchaseService", "LocationData", "MultimediaData", "SoundAlert",
	},
	"http://v8.1c.ru/8.3/data/mob": {
		"AccountCalendarData", "AccountCalendarEventData", "AccountContactData", "AddressData", "CalendarAccount", "CalendarData",
		"CalendarEventData", "CalendarEventRecurrence", "CallLogCallType", "CallLogRecord", "ContactAccount", "ContactData",
		"ContactDataAddressType", "ContactDataElement", "ContactDataElementInstantMessaging", "ContactDataEmailAddressType",
		"ContactDataIMAddressType", "ContactDataPhoneNumberType", "ContactDataRelationType", "ContactDataURLType", "InAppPurchaseReceiptData",
		"LocalCalendarEventKey", "LocalCalendarKey", "LocalContactKey", "MMSAttachment", "SMSLogRecord", "SMSMessage",
	},
	"http://v8.1c.ru/8.3/data/pdf": {
		"Changes", "PDFAttachment", "PDFAttachmentCollection", "PDFAttachmentRelationshipType", "PDFDocument", "PDFDocumentFileType",
		"PDFDocumentPage", "PDFModificationAccessPermissions", "PDFSignatureType",
	},
	"http://v8.1c.ru/8.3/data/planner": {
		"BWAValue", "NewPlannerItemsTextType", "Planner", "PlannerBackgroundInterval", "PlannerBackgroundIntervalLabel", "PlannerDimension",
		"PlannerDimensionItem", "PlannerItem", "PlannerItemAction", "PlannerItemActionLocation", "PlannerItemEnableEditMode",
		"PlannerItemsBehaviorWhenSpaceInsufficient", "PlannerItemSchedule", "PlannerItemScheduleMonths", "PlannerItemScheduleWeekDays",
		"PlannerItemsTimeRepresentation", "PlannerRepresentationPeriod",
	},
	"http://v8.1c.ru/8.3/dbcopies": {
		"UsedDatabaseCopies", "UsedDatabaseCopy",
	},
	"http://v8.1c.ru/8.3/debugger/debugArea": {
		"DebugAreaDSPairInfo", "DebugAreaInfo", "DebugAreas", "DebugAreaUserInfo", "DebugTargetFilters",
	},
	"http://v8.1c.ru/8.3/debugger/debugAutoAttach": {
		"DebugAutoAttachSettings", "DebugAutoAttachSettingsProperty",
	},
	"http://v8.1c.ru/8.3/debugger/debugBaseData": {
		"AttachDebugUIResult", "BSLModuleIdInternal", "BSLModuleIdInternalStr", "BSLModuleIdUser", "BSLModuleType", "DbgTargetState",
		"DbgTargetStateInfo", "DebugStepAction", "DebugTargetId", "DebugTargetIdLight", "DebugTargetIdStr", "DebugTargetType",
		"IsServerInfoBase", "StackItemViewInfoData",
	},
	"http://v8.1c.ru/8.3/debugger/debugBreakpoints": {
		"BPWorkspaceInternal", "BPWorkspaceUser", "BreakpointInfo", "ModuleBPInfoInternal", "ModuleBPInfoUser",
	},
	"http://v8.1c.ru/8.3/debugger/debugCalculations": {
		"BaseValueInfoData", "CalculationResultBaseData", "CalculationResultContextPropertyInfo", "CalculationResultEnumAndColPropertyInfo",
		"CalculationResultObjData", "CalculationResultState", "CalculationResultStorage", "CalculationSourceDataStorage", "ContextPropertyData",
		"DbgPresentationOptionsOfStringValue", "NewValueInfo", "NewValueVariant", "SourceCalculationDataInfo", "SourceCalculationDataItem",
		"SourceCalculationDataItemType", "ViewInterface",
	},
	"http://v8.1c.ru/8.3/debugger/debugDBGUICommands": {
		"DBGUIExtCmdInfoBase", "DBGUIExtCmdInfoCallStackFormed", "DBGUIExtCmdInfoCorrectedBP", "DBGUIExtCmdInfoErrorViewInfo",
		"DBGUIExtCmdInfoExprEvaluated", "DBGUIExtCmdInfoForegroundHelperProcess", "DBGUIExtCmdInfoForegroundHelperRequest",
		"DBGUIExtCmdInfoForegroundHelperSet", "DBGUIExtCmdInfoMeasure", "DBGUIExtCmdInfoQuit", "DBGUIExtCmdInfoRte",
		"DBGUIExtCmdInfoRteBPCondition", "DBGUIExtCmdInfoStarted", "DBGUIExtCmdInfoValueModifyResult", "DBGUIExtCmds",
		"DBGUIExtCmdShowMetadataObject",
	},
	"http://v8.1c.ru/8.3/debugger/debugForegroundData": {
		"ForegroundWindowData",
	},
	"http://v8.1c.ru/8.3/debugger/debugMeasure": {
		"PerformanceInfoLine", "PerformanceInfoMain", "PerformanceInfoModule",
	},
	"http://v8.1c.ru/8.3/debugger/debugRDBGRequestResponse": {
		"DebuggerOptions", "HTTPInitialDebugSettingsData", "HTTPServerInitialDebugSettingsData", "MiscRDbgGetAPIVerResponse",
		"RDBGApplyDebuggerForegroundHelperData", "RDBGAttachDebugUIRequest", "RDBGAttachDebugUIResponse", "RDBGAttachDetachDbgTargetResponse",
		"RDBGAttachDetachDebugTargetsRequest", "RDbgBaseRequest", "RDBGCheckTerminateAbilityRequest", "RDBGCheckTerminateAbilityResponse",
		"RDBGDetachDebugUIRequest", "RDBGDetachDebugUIResponse", "RDBGEvalExprRequest", "RDBGEvalExprResponse", "RDBGEvalLocalVariablesRequest",
		"RDBGEvalLocalVariablesResponse", "RDBGFinishUpdateIBRequest", "RDBGFinishUpdateIBResponse", "RDBGGetAutoAttachSettingsRequest",
		"RDBGGetAutoAttachSettingsResponse", "RDBGGetBreakpointsRequest", "RDBGGetBreakpointsResponse", "RDBGGetCallStackRequest",
		"RDBGGetCallStackResponse", "RDBGGetDbgAllTargetStatesRequest", "RDBGGetDbgAllTargetStatesResponse", "RDBGGetDbgTargetStateRequest",
		"RDBGGetDbgTargetStateResponse", "RDBGGetDebugIDRequest", "RDBGGetDebugIDResponse", "RDBGGetDebugTargetForegroundHelperRequest",
		"RDBGGetDebugTargetForegroundHelperResponse", "RDBGGetForegroundHelperDataRequest", "RDBGGetForegroundHelperDataResponse",
		"RDBGGetInaccessibleModulesRequest", "RDBGGetInaccessibleModulesResponse", "RDBGGetListOfDebugAreaRequest",
		"RDBGGetListOfDebugAreaResponse", "RDBGGetRunTimeErrorProcessingRequest", "RDBGGetRunTimeErrorProcessingResponse",
		"RDBGModifyValueRequest", "RDBGModifyValueResponse", "RDBGNotifyOnFinishSeanceRequest", "RDBGPingDebugUIRequest",
		"RDBGPingDebugUIResponse", "RDBGRestartRequest", "RDBGSetAutoAttachSettingsRequest", "RDBGSetBreakpointsRequest",
		"RDBGSetBreamOnNextStatementRequest", "RDBGSetDebuggerForegroundHelperDataRequest", "RDBGSetInaccessibleModulesRequest",
		"RDBGSetInitialDebugSettingsRequest", "RDBGSetListOfDebugAreaRequest", "RDBGSetMeasureModeRequest",
		"RDBGSetRunTimeErrorProcessingRequest", "RDBGSGetDbgTargetsRequest", "RDBGSGetDbgTargetsResponse", "RDBGStartUpdateIBRequest",
		"RDBGStartUpdateIBResponse", "RDBGStepRequest", "RDBGStepResponse", "RDBGTerminateRequest", "RDBGTestRequest",
	},
	"http://v8.1c.ru/8.3/debugger/debugRTEFilter": {
		"RteFilterItem", "RteFilterStorage",
	},
	"http://v8.1c.ru/8.3/debugger/debugRTEInfo": {
		"CheckBPCondErrorInfo", "ErrorType", "ErrorViewInfoData",
	},
	"http://v8.1c.ru/8.3/ecs/core": {
		"ApplicationID", "Backup", "BackupID", "BindParams", "BotID", "CollaborationSystemDataDumpStatus",
		"CollaborationSystemFromDataDumpRestoreStatus", "CollaborationSystemMessageButtonPanelButtonAction",
		"CollaborationSystemMessageButtonPanelButtonType", "ConversationID", "IntegrationID", "MessageID", "MessageTemplate",
		"MessageTemplateID", "PreBindResponse", "RegistrationData", "Status", "UserID",
	},
	"http://v8.1c.ru/8.3/managed-application/logform/hippo": {
		"BackObject", "Button", "Calendar", "CategoryHolder", "CBLocation", "Chart", "CheckBox", "ChoiceObjects", "Command", "CommandBar",
		"ContextMenu", "ControlCategory", "Delta", "Dendrogram", "Doubles", "Element", "Flowchart", "Form", "FormattedDoc", "GanttChart",
		"GBBehavior", "GeographicalMap", "GroupBox", "HLEdge", "HLMenuElementRepresentation", "HTML", "Image", "Item", "Label", "LayElement",
		"LayElementWCM", "LayFormElement", "LayFormElementExt", "LayFormGroup", "LogGroup", "LogicGroupInternalRepresentation", "LogicGroupRole",
		"MenuSeparator", "MobileAndroidTopBookmarks", "MobileBottomBookmarks", "MobileGroup", "MobileSwipeBookmarks", "Moxel", "Navigator",
		"Page", "Pages", "Panel", "PDFDocument", "Period", "Pictures", "Planner", "ProgressBar", "RadioButton", "RadioGroup", "RichElement",
		"SeparatePanel", "SeparatePanelAppearance", "SeparatePanelRole", "SplitDirection", "SplitMode", "Splitter",
		"SpreadSheetDocumentScrollBarUse", "Strings", "Submenu", "Switcher", "Table", "TableColumn", "TableColumnsGroup", "TextBox", "TextDoc",
		"TrackBar", "Tumbler", "VGridItem", "ViewStatus", "VirtualGrid", "VirtualGridDescr", "VirtualGridElement",
	},
	"http://v8.1c.ru/8.3/mobile-application/app": {
		"AccumulationRegister", "AccumulationRegisterDimension", "AddInTemplate", "Attribute", "BaseMobileApp", "Catalog", "CMDOrder",
		"CMDPalcement", "CMICommandVisibilityItem", "CMISubsystemVisibilityItem", "CommandGroup", "CommandGroupCategory",
		"CommandInterfaceFragment", "CommonForm", "CommonModule", "CommonPicture", "CommonTemplate", "Constant", "CustomCommand",
		"DataProcessor", "DBName", "DBNames", "DesTimeDBObjRefType", "Document", "DocumentJournal", "DocumentJournalColumn", "DocumentNumerator",
		"Enum", "EnumValue", "EventSubscription", "ExchangePlan", "ExchangePlanContentItem", "FarRef", "FunctionalOption",
		"FunctionalOptionsParameter", "InformationRegister", "InformationRegisterDimension", "Language", "LogFormChoiceListDesTimeValue",
		"MDBaseObj", "MDForm", "MDObjReference", "MDPropertyObj", "MDTemplate", "MDTypedObj", "MobileApp", "MobileAppInfo",
		"MobileApplicationPlatformModule", "MobileClient", "ReferencePredefined", "Report", "RequiredPlatformModule", "Resource", "Role",
		"Sequence", "SequenceDimension", "SessionParam", "StandardAttributeDescription", "StartPageWorkingArea",
		"StartPageWorkingAreaColumnItem", "Subsystem", "TabularSection", "WSReference",
	},
	"http://v8.1c.ru/8.3/mobile-application/form": {
		"AdjustableBoolean", "AdjustableBooleanItemType", "Attribute", "Attributes", "AutoMenuGroupData", "Button", "ButtonsGroupData",
		"CalendarFieldData", "ChartFieldData", "CheckBoxFieldData", "CMIFragmentCmdKind", "CMIFragmentRecord", "Column", "Columns",
		"ColumnsGroupData", "Command", "CommandsContent", "ContextMenuGroupData", "Decoration", "EAI_Chart", "EAI_DynamicList", "EAI_Planner",
		"EAI_SpreadsheetDocument", "EAI_ValueList", "Event", "ExtAttributeInformation", "Field", "FieldPath", "FieldPathList",
		"FormattedDocFieldData", "FormattedStringType", "Group", "HTMLFieldData", "ImageFieldData", "InputFieldData", "MainElement",
		"MDObjReference", "MenuGroupData", "MobileForm", "PageGroupData", "PagesGroupData", "Parameter", "PictureDecorationData",
		"PlannerFieldData", "ProgressBarFieldData", "RadioButtonsData", "SpreadsheetFieldData", "SubmenuGroupData", "Table",
		"TextDecorationData", "TextFieldData", "TrackBarFieldData", "UsualGroupData",
	},
	"http://v8.1c.ru/8.3/stt/core": {
		"RegistrationData", "SpeechProcessingLocationUseVariant", "SpeechToTextDelayedID", "SpeechToTextInformationDelayed",
		"SpeechToTextModelDescription", "SpeechToTextModelID", "SpeechToTextModelParameters", "SpeechToTextPhraseData", "SpeechToTextPhraseWord",
		"SpeechToTextResult", "TextToSpeechVoiceDescription", "TextToSpeechVoiceParameterDescription",
		"TextToSpeechVoiceParameterValueDescription",
	},
	"http://v8.1c.ru/8.3/TestAndRepairExchangePlanIntegrityReport": {
		"ExchangePlanIntegrityErrorType", "ExchangePlanIntegrityRepairOperationType", "TestAndRepairExchangePlanIntegrity",
		"TestAndRepairExchangePlanIntegrityNodeInfo", "TestAndRepairExchangePlanIntegrityNodes", "TestAndRepairExchangePlanIntegrityReport",
	},
	"http://v8.1c.ru/8.3/xcf/dumpinfo": {
		"ConfigDumpInfo", "ConfigVersions", "MetadataVersion",
	},
	"http://v8.1c.ru/8.3/xcf/enums": {
		"AccountMainPresentation", "AccumulationRegisterType", "AttributeUse", "BaseEnumType", "BinaryDataBlockStorageUseMode",
		"BinaryDataStorageLocationUse", "BinaryDataStorageMode", "BusinessProcessNumberPeriodicity", "BusinessProcessNumberType",
		"CalculationRegisterPeriodicity", "CalculationTypeMainPresentation", "CatalogCodesSeries", "CatalogCodeType", "CatalogMainPresentation",
		"CharacteristicKindCodesSeries", "CharacteristicTypeMainPresentation", "CharOfAccountCodeSeries", "ChartOfCalculationTypesBaseUse",
		"ChartOfCalculationTypesCodeType", "ChoiceDataGetModeOnInputByString", "ChoiceMode", "CommonAttributeAuthenticationSeparation",
		"CommonAttributeAutoUse", "CommonAttributeConfigurationExtensionsSeparation", "CommonAttributeDataSeparation",
		"CommonAttributeSeparatedDataUse", "CommonAttributeUse", "CommonAttributeUsersSeparation", "CompatibilityMode",
		"ConfigurationExtensionPurpose", "CreateOnInput", "DatabaseTablespacesUseMode", "DataExchangeMainPresentation", "DataHistoryUse",
		"DefaultDataLockControlMode", "DocumentNumberPeriodicity", "DocumentNumberType", "EditType", "ExternalDataSourceTableDataType",
		"ExternalDataSourceTableType", "FormatVersion", "FormType", "FullTextSearchOnInputByString", "FullTextSearchUsing", "HierarchyType",
		"HTTPMethod", "Indexing", "InformationRegisterPeriodicity", "IntegrationServiceChannelMessageDirection", "InterfaceCompatibilityMode",
		"ModalityUseMode", "MoveBoundaryOnPosting", "ObjectAutonumerationMode", "ObjectBelonging", "Posting", "PredefinedDataUpdate",
		"PropertyState", "RealTimePosting", "RegisterRecordsDeletion", "RegisterRecordsWritingOnPost", "RegisterWriteMode", "ReturnValuesReuse",
		"ScriptVariant", "SearchStringModeOnInputByString", "SequenceFilling", "SessionReuseMode", "StyleElementType", "SubordinationUse",
		"SynchronousPlatformExtensionAndAddInCallUseMode", "TaskMainPresentation", "TaskNumberAutoPrefix", "TaskNumberType", "TemplateType",
		"TransferDirection", "TypeCategories", "TypeReductionMode", "UseQuickChoice", "XCFFormat",
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
