package kinds

// The words of a form's module beyond its attributes: the properties and the
// methods of the form itself (help 8.3.27, ФормаКлиентскогоПриложения), which
// its code names without the form - Заголовок, Модифицированность, Закрыть().
// The help gives the form 50 properties and 41 methods besides one property
// per attribute (<Имя реквизита>); the attributes are the form's own and come
// from its description (metadata.ModuleNamer).
//
// A few belong to the mobile client alone (НачатьРедактированиеЭлемента,
// ПереоткрытьСДругогоСервера, СворачиваниеЭлементовПоВажности), which ML does
// not build. They are named all the same: the prototype compiles them in any
// form, and erp and sb call НачатьРедактированиеЭлемента in a common form
// under #Если МобильноеПриложениеКлиент, a branch only the preprocessor drops.

// formProperties are the properties of ФормаКлиентскогоПриложения, Элементы
// and ЭтотОбъект among them.
var formProperties = []Words{
	{"АвтоЗаголовок", "AutoTitle"}, {"АвтоНавигационнаяСсылка", "AutoURL"},
	{"АвтоматическоеСохранениеДанныхВНастройках", "AutoSaveDataInSettings"},
	{"ВертикальнаяПрокрутка", "VerticalScroll"}, {"ВертикальноеПоложениеПодчиненных", "ChildItemsVerticalAlign"},
	{"ВертикальныйИнтервал", "VerticalSpacing"}, {"ВладелецФормы", "FormOwner"},
	{"ВыравниваниеЭлементовИЗаголовков", "ItemsAndTitlesAlign"}, {"Высота", "Height"},
	{"ГоризонтальноеПоложениеПодчиненных", "ChildItemsHorizontalAlign"}, {"ГоризонтальныйИнтервал", "HorizontalSpacing"},
	{"Группировка", "Group"}, {"Доступность", "Enabled"}, {"Заголовок", "Title"},
	{"ЗакрыватьПриВыборе", "CloseOnChoice"}, {"ЗакрыватьПриЗакрытииВладельца", "CloseOnOwnerClose"},
	{"ИмяФормы", "FormName"}, {"ИспользуемыйСерверФормы", "UsedFormServer"},
	{"КлючНазначенияИспользования", "PurposeUseKey"}, {"КлючСохраненияПоложенияОкна", "WindowOptionsKey"},
	{"КлючУникальности", "UniqueKey"}, {"КоманднаяПанель", "CommandBar"}, {"Команды", "Commands"},
	{"Масштаб", "Scale"}, {"МодальныйРежим", "ModalMode"}, {"Модифицированность", "Modified"},
	{"НавигационнаяСсылка", "URL"}, {"Окно", "Window"}, {"ОписаниеОповещенияОЗакрытии", "CallbackDescriptionOnClose"},
	{"ОтображатьЗаголовок", "ShowTitle"}, {"ОтображатьКнопкуЗакрытия", "ShowCloseButton"},
	{"ОтображениеОбсуждений", "ConversationsRepresentation"}, {"Параметры", "Parameters"},
	{"ПоведениеКлавишиEnter", "EnterKeyBehavior"}, {"ПодчиненныеЭлементы", "ChildItems"},
	{"ПоложениеКоманднойПанели", "CommandBarLocation"}, {"ПроверятьЗаполнениеАвтоматически", "AutoFillCheck"},
	{"РежимОткрытияОкнаФормы", "FormWindowOpeningMode"}, {"СворачиваниеЭлементовПоВажности", "CollapseItemsByImportance"},
	{"СоставКоманднойПанелиНаМобильномУстройстве", "MobileDeviceCommandBarContent"},
	{"СохранениеДанныхВНастройках", "SaveDataInSettings"},
	{"СохраняемыеВНастройкахДанныеМодифицированы", "SavedInSettingsDataModified"},
	{"ТекущийЭлемент", "CurrentItem"}, {"ТолькоПросмотр", "ReadOnly"}, {"УникальныйИдентификатор", "UUID"},
	{"УсловноеОформление", "ConditionalAppearance"}, {"Ширина", "Width"},
	{"ШиринаПодчиненныхЭлементов", "SlaveItemsWidth"}, Items, ThisObject,
}

// formMethods are the methods of ФормаКлиентскогоПриложения.
var formMethods = []Words{
	{"Активизировать", "Activate"}, {"ВводДоступен", "IsInputAvailable"},
	{"ВыбратьИзМеню", "ChooseFromMenu"}, {"ВыбратьИзМенюАсинх", "ChooseFromMenuAsync"},
	{"ВыбратьИзСписка", "ChooseFromList"}, {"ВыбратьИзСпискаАсинх", "ChooseFromListAsync"},
	{"ВыполнитьПереход", "ExecuteNavigation"}, {"ЗаблокироватьДанныеФормыДляРедактирования", "LockFormDataForEdit"},
	{"Закрыть", "Close"}, {"ЗаполнитьПриПереоткрытииСДругогоСервера", "FillOnReopenFromOtherServer"},
	{"ЗначениеВРеквизитФормы", "ValueToFormAttribute"}, {"ИзменитьРеквизиты", "ChangeAttributes"},
	{"НачатьПолучениеОбсуждения", "BeginGetConversation"}, {"НачатьРедактированиеЭлемента", "BeginEditingItem"},
	{"ОбновитьОтображениеДанных", "RefreshDataRepresentation"}, {"ОповеститьОВыборе", "NotifyChoice"},
	{"ОповеститьОЗаписиНового", "NotifyWritingNew"}, {"ОповеститьОбАктивизации", "NotifyActivate"},
	{"ОтключитьОбработчикОжидания", "DetachIdleHandler"}, {"ОтключитьОтметкуНезаполненного", "ClearMarkIncomplete"},
	{"Открыта", "IsOpen"}, {"Открыть", "Open"}, {"ОткрытьМодально", "DoModal"},
	{"ОткрытьСправкуФормы", "OpenFormHelp"}, {"ОтобразитьИзменениеДанных", "RepresentDataChange"},
	{"ПереоткрытьСДругогоСервера", "ReopenFromOtherServer"}, {"ПодключитьОбработчикОжидания", "AttachIdleHandler"},
	{"ПоказатьВыборИзМеню", "ShowChooseFromMenu"}, {"ПоказатьВыборИзСписка", "ShowChooseFromList"},
	{"ПолучитьДействие", "GetAction"}, {"ПолучитьОбсуждениеАсинх", "GetConversationAsync"},
	{"ПолучитьПараметрыФункциональныхОпцийФормы", "GetFormFunctionalOptionParameters"},
	{"ПолучитьРеквизиты", "GetAttributes"}, {"ПолучитьФункциональнуюОпциюФормы", "GetFormFunctionalOption"},
	{"ПроверитьЗаполнение", "CheckFilling"}, {"РазблокироватьДанныеФормыДляРедактирования", "UnlockFormDataForEdit"},
	{"РеквизитФормыВЗначение", "FormAttributeToValue"},
	{"УстановитьВыполнениеОбработчиковСобытия", "SetEventHandlersExecution"},
	{"УстановитьВыполнениеПослеОбработчиковСобытия", "SetExecutionAfterEventHandlers"},
	{"УстановитьДействие", "SetAction"},
	{"УстановитьПараметрыФункциональныхОпцийФормы", "SetFormFunctionalOptionParameters"},
}
