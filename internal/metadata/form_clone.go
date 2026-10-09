package metadata

import (
	"maps"
	"slices"
)

// The runtime snapshot hands out a form for every opening of it, and the
// caller is free to change what it got. So the form is copied whole: nothing
// the copy holds - a text, a list, a value behind a pointer - is shared with
// the snapshot, and a change to one opening reaches neither the snapshot nor
// any other opening. TestARuntimeFormSharesNothingWithTheSnapshot fills every
// field of the form by reflection and fails on any shared one, so a field
// added to the form without being copied here is caught there.

func cloneRuntimeForm(value ManagedForm) ManagedForm {
	result := value
	result.Title = cloneTitle(value.Title)
	result.Explanation = maps.Clone(value.Explanation)
	result.ExtendedPresentation = maps.Clone(value.ExtendedPresentation)
	result.Purposes = slices.Clone(value.Purposes)
	result.SettingsStorage = clonePointer(value.SettingsStorage)
	result.Events = maps.Clone(value.Events)
	result.Commands = make([]ManagedFormCommand, len(value.Commands))
	for index, command := range value.Commands {
		result.Commands[index] = command
		result.Commands[index].Title = cloneTitle(command.Title)
		result.Commands[index].ToolTip = maps.Clone(command.ToolTip)
		result.Commands[index].Picture = command.Picture.clone()
		result.Commands[index].FunctionalOptions = slices.Clone(command.FunctionalOptions)
		result.Commands[index].Use = command.Use.clone()
	}
	result.Items = cloneRuntimeFormElements(value.Items)
	result.AutoCommandBar = value.AutoCommandBar.clone()
	result.Attributes = cloneFormAttributes(value.Attributes)
	result.ConditionalAppearance = cloneConditionalAppearance(value.ConditionalAppearance)
	result.Parameters = cloneFormParameters(value.Parameters)
	result.CommandInterface = value.CommandInterface.clone()
	result.MobileCommandBar = slices.Clone(value.MobileCommandBar)
	return result
}

func cloneRuntimeFormElements(values []ManagedFormElement) []ManagedFormElement {
	result := make([]ManagedFormElement, len(values))
	for index, value := range values {
		value.Title = cloneTitle(value.Title)
		if value.CommandParameter != nil {
			parameter := *value.CommandParameter
			parameter.Types = cloneTypes(parameter.Types)
			value.CommandParameter = &parameter
		}
		value.ToolTip = maps.Clone(value.ToolTip)
		value.UserVisible = value.UserVisible.clone()
		value.SkipOnInput = clonePointer(value.SkipOnInput)
		value.WarningOnEdit = maps.Clone(value.WarningOnEdit)
		value.HorizontalStretch = clonePointer(value.HorizontalStretch)
		value.VerticalStretch = clonePointer(value.VerticalStretch)
		value.FieldLook = value.FieldLook.clone()
		value.FieldColumn = value.FieldColumn.clone()
		value.FieldButtons = value.FieldButtons.clone()
		value.FieldTextInput = value.FieldTextInput.clone()
		value.FieldFormat = value.FieldFormat.clone()
		value.InputFieldChoice = value.InputFieldChoice.clone()
		value.ChoiceList = cloneChoiceList(value.ChoiceList)
		value.InputFieldChoiceParameters = value.InputFieldChoiceParameters.clone()
		value.EqualItemsWidth, value.EqualColumnsWidth = clonePointer(value.EqualItemsWidth), clonePointer(value.EqualColumnsWidth)
		value.ValuesPicture = value.ValuesPicture.clone()
		value.NonselectedPictureText = maps.Clone(value.NonselectedPictureText)
		value.FieldDocument = value.FieldDocument.clone()
		value.ShowCurrentDate = clonePointer(value.ShowCurrentDate)
		value.WidthInMonths, value.HeightInMonths = clonePointer(value.WidthInMonths), clonePointer(value.HeightInMonths)
		value.CollapsedTitle = maps.Clone(value.CollapsedTitle)
		value.Picture = value.Picture.clone()
		value.ScrollOnCompress = clonePointer(value.ScrollOnCompress)
		value.TableProperties = value.TableProperties.clone()
		value.TableAdditions = value.TableAdditions.clone()
		value.Events = maps.Clone(value.Events)
		if tooltip := value.ExtendedTooltip; tooltip != nil {
			copied := cloneRuntimeFormElements([]ManagedFormElement{*tooltip})[0]
			value.ExtendedTooltip = &copied
		}
		value.ContextMenu, value.AutoCommandBar = value.ContextMenu.clone(), value.AutoCommandBar.clone()
		value.Children = cloneRuntimeFormElements(value.Children)
		result[index] = value
	}
	return result
}

func (look FieldLook) clone() FieldLook {
	look.TitleTextColor, look.TitleBackColor = look.TitleTextColor.clone(), look.TitleBackColor.clone()
	look.TextColor, look.BackColor, look.BorderColor = look.TextColor.clone(), look.BackColor.clone(), look.BorderColor.clone()
	look.TitleFont, look.Font = look.TitleFont.clone(), look.Font.clone()
	look.Border = look.Border.clone()
	return look
}

func (column FieldColumn) clone() FieldColumn {
	column.HeaderPicture, column.FooterPicture = column.HeaderPicture.clone(), column.FooterPicture.clone()
	column.FooterText = maps.Clone(column.FooterText)
	column.FooterFont = column.FooterFont.clone()
	column.FooterTextColor, column.FooterBackColor = column.FooterTextColor.clone(), column.FooterBackColor.clone()
	return column
}

func (buttons FieldButtons) clone() FieldButtons {
	buttons.ChoiceButton, buttons.OpenButton, buttons.ClearButton = clonePointer(buttons.ChoiceButton), clonePointer(buttons.OpenButton), clonePointer(buttons.ClearButton)
	buttons.CreateButton, buttons.DropListButton = clonePointer(buttons.CreateButton), clonePointer(buttons.DropListButton)
	buttons.SpinButton, buttons.ChoiceListButton = clonePointer(buttons.SpinButton), clonePointer(buttons.ChoiceListButton)
	buttons.ChoiceButtonPicture = buttons.ChoiceButtonPicture.clone()
	return buttons
}

func (input FieldTextInput) clone() FieldTextInput {
	input.MultiLine, input.ExtendedEdit, input.PasswordMode = clonePointer(input.MultiLine), clonePointer(input.ExtendedEdit), clonePointer(input.PasswordMode)
	input.InputHint = maps.Clone(input.InputHint)
	return input
}

func (format FieldFormat) clone() FieldFormat {
	format.Format, format.EditFormat = maps.Clone(format.Format), maps.Clone(format.EditFormat)
	format.MarkNegatives, format.AutoMarkIncomplete = clonePointer(format.MarkNegatives), clonePointer(format.AutoMarkIncomplete)
	return format
}

func (choice InputFieldChoice) clone() InputFieldChoice {
	choice.QuickChoice, choice.AutoChoiceIncomplete = clonePointer(choice.QuickChoice), clonePointer(choice.AutoChoiceIncomplete)
	choice.ChoiceFormGone = clonePointer(choice.ChoiceFormGone)
	if form := choice.ChoiceForm; form != nil {
		copied := *form
		copied.Object = clonePointer(form.Object)
		choice.ChoiceForm = &copied
	}
	return choice
}

func (parameters InputFieldChoiceParameters) clone() InputFieldChoiceParameters {
	parameters.ChoiceParameters = slices.Clone(parameters.ChoiceParameters)
	for index := range parameters.ChoiceParameters {
		parameters.ChoiceParameters[index].Values = slices.Clone(parameters.ChoiceParameters[index].Values)
	}
	parameters.ChoiceParameterLinks = slices.Clone(parameters.ChoiceParameterLinks)
	parameters.TypeLink = clonePointer(parameters.TypeLink)
	parameters.AvailableTypes = cloneTypes(parameters.AvailableTypes)
	return parameters
}

func (document FieldDocument) clone() FieldDocument {
	for _, flag := range []**bool{&document.Edit, &document.Protection,
		&document.ShowHeaders, &document.ShowGrid, &document.ShowGroups, &document.ShowCellNames, &document.ShowRowAndColumnNames,
		&document.EnableDrag, &document.EnableStartDrag} {
		*flag = clonePointer(*flag)
	}
	document.ExcludedCommands = slices.Clone(document.ExcludedCommands)
	return document
}

func cloneChoiceList(list []FormChoiceListItem) []FormChoiceListItem {
	if list == nil {
		return nil
	}
	result := make([]FormChoiceListItem, len(list))
	for index, item := range list {
		item.Presentation = maps.Clone(item.Presentation)
		result[index] = item
	}
	return result
}

func (value *ColorValue) clone() *ColorValue {
	if value == nil {
		return nil
	}
	colour := *value
	colour.From = cloneStyleItemReference(colour.From)
	return &colour
}

func (value *BorderValue) clone() *BorderValue {
	if value == nil {
		return nil
	}
	border := *value
	border.From = cloneStyleItemReference(border.From)
	return &border
}

func (right *FormAttributeRight) clone() *FormAttributeRight {
	if right == nil {
		return nil
	}
	value := *right
	value.Roles = slices.Clone(value.Roles)
	return &value
}

func cloneFormAttributes(values []FormAttribute) []FormAttribute {
	if values == nil {
		return nil
	}
	result := make([]FormAttribute, len(values))
	for index, value := range values {
		value.Title = maps.Clone(value.Title)
		value.Types, value.ValueType = cloneTypes(value.Types), cloneTypes(value.ValueType)
		value.FunctionalOptions = slices.Clone(value.FunctionalOptions)
		value.UseAlways, value.SaveInSettings = slices.Clone(value.UseAlways), slices.Clone(value.SaveInSettings)
		value.View, value.Edit = value.View.clone(), value.Edit.clone()
		value.Columns = cloneFormAttributeColumns(value.Columns)
		if value.AdditionalColumns != nil {
			additional := make([]FormAdditionalColumns, len(value.AdditionalColumns))
			for position, columns := range value.AdditionalColumns {
				columns.Columns = cloneFormAttributeColumns(columns.Columns)
				additional[position] = columns
			}
			value.AdditionalColumns = additional
		}
		value.Chart = value.Chart.clone()
		value.GanttChart = value.GanttChart.clone()
		value.Planner = value.Planner.clone()
		if value.DynamicList != nil {
			list := *value.DynamicList
			list.MainTable = clonePointer(list.MainTable)
			list.KeyFields = slices.Clone(list.KeyFields)
			cloneListComposition(&list)
			value.DynamicList = &list
		}
		result[index] = value
	}
	return result
}

func cloneFormAttributeColumns(values []FormAttributeColumn) []FormAttributeColumn {
	if values == nil {
		return nil
	}
	result := make([]FormAttributeColumn, len(values))
	for index, value := range values {
		value.Title = maps.Clone(value.Title)
		value.Types = cloneTypes(value.Types)
		value.FunctionalOptions = slices.Clone(value.FunctionalOptions)
		value.View, value.Edit = value.View.clone(), value.Edit.clone()
		result[index] = value
	}
	return result
}

func (value *FormCommandInterface) clone() *FormCommandInterface {
	if value == nil {
		return nil
	}
	lines := func(values []FormInterfaceCommand) []FormInterfaceCommand {
		result := slices.Clone(values)
		for index := range result {
			result[index].Visibility = result[index].Visibility.clone()
		}
		return result
	}
	return &FormCommandInterface{NavigationPanel: lines(value.NavigationPanel), CommandBar: lines(value.CommandBar)}
}
