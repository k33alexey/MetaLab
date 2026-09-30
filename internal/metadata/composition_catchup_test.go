package metadata

import (
	"bytes"
	"strings"
	"testing"

	"github.com/k33alexey/MetaLab/internal/project"
	"github.com/k33alexey/MetaLab/internal/uuid"
)

// These are the properties the sweep of composition found missing, and all of
// them were the same class of gap: the requirements ask for them, the export
// writes them on every object of the kind, and our structures had nowhere to
// put them - so a file that carried one was refused and a configuration that
// used one could not be read at all.
//
// A round trip is what each test does, because that is where the defect was:
// not in the logic but in the file. What a round trip drops is gone for good.

func catchupConfiguration() project.Project {
	return project.Project{Format: 1, ID: uuid.MustNew(), Name: "Demo", Title: project.LocalizedText{"ru": "Demo"},
		DefaultLanguage: "ru", Languages: []project.Language{{Code: "ru", Name: "Русский"}}}
}

// A global module gives its exported procedures to the global context, and code
// carried over from the prototype calls them by name alone: without the flag
// that call finds nothing. Reuse of returned values is three-valued, not two -
// a function reused for the session keeps returning what it returned first,
// however the data has changed since, and that difference is visible to an
// application.
func TestCommonModuleCarriesItsExecutionContext(t *testing.T) {
	t.Parallel()
	module := CommonModuleDefinition{
		Format: CurrentFormat, ID: uuid.MustNew(), Name: "ОбщегоНазначения",
		Title: LocalizedText{"ru": "Общего назначения"}, Module: uuid.MustNew(),
		Server: true, Global: true, ExternalConnection: true,
		Comment: "Процедуры, нужные всем", ReturnValuesReuse: ReturnValuesReuseDuringSession,
	}
	var encoded bytes.Buffer
	if err := Encode(&encoded, module); err != nil {
		t.Fatal(err)
	}
	decoded, err := DecodeCommonModule("common-module.yaml", bytes.NewReader(encoded.Bytes()), catchupConfiguration())
	if err != nil {
		t.Fatal(err)
	}
	if !decoded.Global || !decoded.ExternalConnection || decoded.Comment == "" {
		t.Fatalf("module = %+v", decoded)
	}
	if decoded.ReturnValuesReuse != ReturnValuesReuseDuringSession {
		t.Fatalf("return values reuse = %q", decoded.ReturnValuesReuse)
	}

	// A fourth value would be a setting nothing can act on.
	module.ReturnValuesReuse = "иногда"
	if err := ValidateCommonModule("common-module.yaml", module, catchupConfiguration()); err == nil {
		t.Fatal("an unknown reuse of returned values was accepted")
	}
}

// Without ВключатьВКомандныйИнтерфейс a subsystem groups objects for the
// developer and shows the user nothing, so the section a configuration was
// built around would simply not appear.
func TestSubsystemCarriesItsCommandInterface(t *testing.T) {
	t.Parallel()
	subsystem := SubsystemDefinition{
		Format: CurrentFormat, ID: uuid.MustNew(), Name: "Продажи", Title: LocalizedText{"ru": "Продажи"},
		IncludeInCommandInterface: true, UseOneCommand: true,
		Picture: &PictureReference{Standard: "Продажи"},
	}
	var encoded bytes.Buffer
	if err := Encode(&encoded, subsystem); err != nil {
		t.Fatal(err)
	}
	decoded, err := DecodeSubsystem("subsystem.yaml", bytes.NewReader(encoded.Bytes()), catchupConfiguration())
	if err != nil {
		t.Fatal(err)
	}
	if !decoded.IncludeInCommandInterface || !decoded.UseOneCommand || decoded.Picture == nil {
		t.Fatalf("subsystem = %+v", decoded)
	}

	// A picture that names two sources names neither, and one that names none
	// says the section has an icon and leaves it blank.
	common := uuid.MustNew()
	subsystem.Picture = &PictureReference{Standard: "Продажи", Common: &common}
	if err := ValidateSubsystem("subsystem.yaml", subsystem, catchupConfiguration()); err == nil {
		t.Fatal("a picture naming two sources was accepted")
	}
	subsystem.Picture = &PictureReference{}
	if err := ValidateSubsystem("subsystem.yaml", subsystem, catchupConfiguration()); err == nil {
		t.Fatal("a picture naming no source was accepted")
	}
}

// The comment is the developer's own note, and four kinds had nowhere to keep
// it while the export writes it on every one of them - 739 common modules, 230
// constants, 145 event subscriptions, 73 defined types.
func TestCommentReachesTheKindsThatHadNone(t *testing.T) {
	t.Parallel()
	const note = "Заведено под разбор переноса"
	t.Run("определяемый тип", func(t *testing.T) {
		value := DefinedTypeObject{Format: CurrentFormat, ID: uuid.MustNew(), Name: "Контрагент",
			Title: LocalizedText{"ru": "Контрагент"}, Types: []Type{{Kind: StringType, Length: 100}}, Comment: note}
		var encoded bytes.Buffer
		if err := Encode(&encoded, value); err != nil {
			t.Fatal(err)
		}
		decoded, err := DecodeDefinedType("defined-type.yaml", bytes.NewReader(encoded.Bytes()), catchupConfiguration())
		if err != nil || decoded.Comment != note {
			t.Fatalf("decoded = %+v, error = %v", decoded, err)
		}
	})
	t.Run("параметр сеанса", func(t *testing.T) {
		value := SessionParameter{Format: CurrentFormat, ID: uuid.MustNew(), Name: "ТекущийСклад",
			Title: LocalizedText{"ru": "Текущий склад"}, Types: []Type{{Kind: StringType, Length: 100}}, Comment: note}
		var encoded bytes.Buffer
		if err := Encode(&encoded, value); err != nil {
			t.Fatal(err)
		}
		decoded, err := DecodeSessionParameter("session-parameter.yaml", bytes.NewReader(encoded.Bytes()), catchupConfiguration())
		if err != nil || decoded.Comment != note {
			t.Fatalf("decoded = %+v, error = %v", decoded, err)
		}
	})
	t.Run("подписка на события", func(t *testing.T) {
		value := EventSubscriptionDefinition{Format: CurrentFormat, ID: uuid.MustNew(), Name: "ПередЗаписьюТовара",
			Title: LocalizedText{"ru": "Перед записью товара"}, Objects: []uuid.UUID{uuid.MustNew()},
			Event: "before-write", Module: uuid.MustNew(), Procedure: "ПередЗаписью", Comment: note}
		var encoded bytes.Buffer
		if err := Encode(&encoded, value); err != nil {
			t.Fatal(err)
		}
		decoded, err := DecodeEventSubscription("event-subscription.yaml", bytes.NewReader(encoded.Bytes()), catchupConfiguration())
		if err != nil || decoded.Comment != note {
			t.Fatalf("decoded = %+v, error = %v", decoded, err)
		}
	})
	t.Run("последовательность", func(t *testing.T) {
		value := SequenceDefinition{Format: CurrentFormat, ID: uuid.MustNew(), Name: "ПартииТоваров",
			Title: LocalizedText{"ru": "Партии товаров"}, Documents: []uuid.UUID{uuid.MustNew()}, Comment: note}
		var encoded bytes.Buffer
		if err := Encode(&encoded, value); err != nil {
			t.Fatal(err)
		}
		decoded, err := DecodeSequence("sequence.yaml", bytes.NewReader(encoded.Bytes()), catchupConfiguration())
		if err != nil || decoded.Comment != note {
			t.Fatalf("decoded = %+v, error = %v", decoded, err)
		}
	})
}

// A report and a data processor share one structure of presentations, and
// neither had the longer name the prototype gives both. It is one field in one
// place for that reason.
func TestReportAndDataProcessorCarryTheExtendedPresentation(t *testing.T) {
	t.Parallel()
	report := ReportDefinition{Format: CurrentFormat, ID: uuid.MustNew(), Name: "ПродажиЗаПериод",
		Title: LocalizedText{"ru": "Продажи за период"}}
	report.ExtendedPresentation = LocalizedText{"ru": "Продажи за выбранный период"}
	var encoded bytes.Buffer
	if err := Encode(&encoded, report); err != nil {
		t.Fatal(err)
	}
	decodedReport, err := DecodeReport("report.yaml", bytes.NewReader(encoded.Bytes()), catchupConfiguration())
	if err != nil || decodedReport.ExtendedPresentation["ru"] == "" {
		t.Fatalf("report = %+v, error = %v", decodedReport.ExtendedPresentation, err)
	}

	processor := DataProcessorDefinition{Format: CurrentFormat, ID: uuid.MustNew(), Name: "ЗакрытиеМесяца",
		Title: LocalizedText{"ru": "Закрытие месяца"}}
	processor.ExtendedPresentation = LocalizedText{"ru": "Закрытие месяца по организации"}
	encoded.Reset()
	if err := Encode(&encoded, processor); err != nil {
		t.Fatal(err)
	}
	decodedProcessor, err := DecodeDataProcessor("data-processor.yaml", bytes.NewReader(encoded.Bytes()), catchupConfiguration())
	if err != nil || decodedProcessor.ExtendedPresentation["ru"] == "" {
		t.Fatalf("data processor = %+v, error = %v", decodedProcessor.ExtendedPresentation, err)
	}

	// Localized like every text a person reads, and checked like one: a language
	// the configuration does not have is a presentation nobody will see.
	processor.ExtendedPresentation = LocalizedText{"de": "Monatsabschluss"}
	encoded.Reset()
	if err := Encode(&encoded, processor); err != nil {
		t.Fatal(err)
	}
	_, err = DecodeDataProcessor("data-processor.yaml", bytes.NewReader(encoded.Bytes()), catchupConfiguration())
	if err == nil || !strings.Contains(err.Error(), "extended_presentation") {
		t.Fatalf("error = %v", err)
	}
}
