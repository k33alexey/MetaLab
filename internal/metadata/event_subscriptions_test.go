package metadata

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/k33alexey/MetaLab/internal/uuid"
)

// eventSubscriptionFixture listens to the objects of the catalogs named.
func eventSubscriptionFixture(catalogs ...uuid.UUID) EventSubscriptionDefinition {
	source := make([]Type, 0, len(catalogs))
	for _, id := range catalogs {
		source = append(source, Type{Kind: CatalogObjectType, Reference: &id})
	}
	return EventSubscriptionDefinition{
		Format: CurrentFormat, ID: uuid.MustNew(), Name: "ЗапретПустогоНаименования", Title: LocalizedText{"ru": "Запрет пустого наименования"},
		Source: source, Event: "before-write", Module: uuid.MustNew(), Procedure: "ПередЗаписьюТовара",
	}
}

func TestDecodeEventSubscriptionStrictAndBounded(t *testing.T) {
	t.Parallel()
	subscription := eventSubscriptionFixture(uuid.MustNew(), uuid.MustNew())
	var encoded bytes.Buffer
	if err := Encode(&encoded, subscription); err != nil {
		t.Fatal(err)
	}
	decoded, err := DecodeEventSubscription("event-subscription.yaml", bytes.NewReader(encoded.Bytes()), metadataConfiguration())
	if err != nil || decoded.ID != subscription.ID || len(decoded.Source) != 2 || decoded.Procedure != subscription.Procedure {
		t.Fatalf("decode event subscription = %+v, %v", decoded, err)
	}
	tests := map[string]func(*EventSubscriptionDefinition){
		"format":        func(s *EventSubscriptionDefinition) { s.Format++ },
		"zero identity": func(s *EventSubscriptionDefinition) { s.ID = uuid.UUID{} },
		"name":          func(s *EventSubscriptionDefinition) { s.Name = "Invalid Name" },
		"language":      func(s *EventSubscriptionDefinition) { s.Title = LocalizedText{"de": "Verboten"} },
		"no source":     func(s *EventSubscriptionDefinition) { s.Source = nil },
		"zero object": func(s *EventSubscriptionDefinition) {
			s.Source = append(s.Source, Type{Kind: CatalogObjectType, Reference: &uuid.UUID{}})
		},
		"duplicate type": func(s *EventSubscriptionDefinition) {
			s.Source = append(s.Source, s.Source[0])
		},
		"a reference is no source": func(s *EventSubscriptionDefinition) {
			s.Source = append(s.Source, Type{Kind: CatalogType, Reference: s.Source[0].Reference})
		},
		"unknown event":     func(s *EventSubscriptionDefinition) { s.Event = "on-open" },
		"zero module":       func(s *EventSubscriptionDefinition) { s.Module = uuid.UUID{} },
		"invalid procedure": func(s *EventSubscriptionDefinition) { s.Procedure = "Перед Записью" },
	}
	for name, mutate := range tests {
		t.Run(name, func(t *testing.T) {
			mutated := subscription
			mutated.Source = cloneTypes(subscription.Source)
			mutate(&mutated)
			if err := ValidateEventSubscription("event-subscription.yaml", mutated, metadataConfiguration()); err == nil {
				t.Fatalf("%s: accepted invalid event subscription", name)
			}
		})
	}
	if _, err := DecodeEventSubscription("event-subscription.yaml", strings.NewReader(encoded.String()+"unknown: true\n"), metadataConfiguration()); err == nil {
		t.Fatal("accepted unknown field")
	}
}

func TestCatalogEventSubscriptionLookupReturnsIsolatedCopies(t *testing.T) {
	t.Parallel()
	target := CatalogDefinition{Format: CurrentFormat, ID: uuid.MustNew(), Name: "Товары", Title: LocalizedText{"ru": "Товары"},
		Code: CatalogCode{Type: StringType, Length: 9}, DescriptionLength: 100}
	module := commonModuleFixture()
	module.Server = true
	subscription := eventSubscriptionFixture(target.ID)
	subscription.Module = module.ID
	catalog, err := NewCatalogSnapshotWithEventSubscriptions(metadataConfiguration(), nil, nil, nil, []CatalogDefinition{target}, nil, nil, nil, nil, nil, nil, nil, []CommonModuleDefinition{module}, []EventSubscriptionDefinition{subscription})
	if err != nil {
		t.Fatal(err)
	}
	byName, ok := catalog.EventSubscription("ЗапретПустогоНаименования")
	if !ok || byName.ID != subscription.ID {
		t.Fatalf("event subscription by name = %+v, %v", byName, ok)
	}
	byID, ok := catalog.EventSubscriptionByID(subscription.ID)
	if !ok || byID.Procedure != subscription.Procedure {
		t.Fatalf("event subscription by id = %+v, %v", byID, ok)
	}
	*byID.Source[0].Reference = uuid.MustNew()
	again, _ := catalog.EventSubscriptionByID(subscription.ID)
	if *again.Source[0].Reference != target.ID {
		t.Fatal("event subscription lookup exposed mutable metadata")
	}
	found := catalog.eventSubscriptionsFor(CatalogObjectType, target.ID)
	if len(found) != 1 || found[0].ID != subscription.ID {
		t.Fatalf("eventSubscriptionsFor = %+v", found)
	}
}

func TestEventSubscriptionReferenceValidation(t *testing.T) {
	t.Parallel()
	t.Run("unknown module", func(t *testing.T) {
		subscription := eventSubscriptionFixture(uuid.MustNew())
		if _, err := NewCatalogSnapshotWithEventSubscriptions(metadataConfiguration(), nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, []EventSubscriptionDefinition{subscription}); err == nil || !strings.Contains(err.Error(), "unknown common module") {
			t.Fatalf("unknown module error = %v", err)
		}
	})
	t.Run("client only module", func(t *testing.T) {
		module := commonModuleFixture()
		module.Server, module.Client = false, true
		subscription := eventSubscriptionFixture(uuid.MustNew())
		subscription.Module = module.ID
		if _, err := NewCatalogSnapshotWithEventSubscriptions(metadataConfiguration(), nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, []CommonModuleDefinition{module}, []EventSubscriptionDefinition{subscription}); err == nil || !strings.Contains(err.Error(), "must be a server module") {
			t.Fatalf("client-only module error = %v", err)
		}
	})
	t.Run("unknown object", func(t *testing.T) {
		module := commonModuleFixture()
		module.Server = true
		subscription := eventSubscriptionFixture(uuid.MustNew())
		subscription.Module = module.ID
		if _, err := NewCatalogSnapshotWithEventSubscriptions(metadataConfiguration(), nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, []CommonModuleDefinition{module}, []EventSubscriptionDefinition{subscription}); err == nil || !strings.Contains(err.Error(), "unknown object") {
			t.Fatalf("unknown object error = %v", err)
		}
	})
	t.Run("event not valid for register objects", func(t *testing.T) {
		register := InformationRegisterDefinition{
			Format: CurrentFormat, ID: uuid.MustNew(), Name: "Цены", Title: LocalizedText{"ru": "Цены"},
			WriteMode: InformationRegisterIndependent, Periodicity: InformationRegisterPeriodNone,
		}
		module := commonModuleFixture()
		module.Server = true
		subscription := eventSubscriptionFixture()
		subscription.Source = []Type{{Kind: InformationRegisterRecordSetType, Reference: &register.ID}}
		subscription.Module = module.ID
		subscription.Event = "posting"
		if _, err := NewCatalogSnapshotWithEventSubscriptions(metadataConfiguration(), nil, nil, nil, nil, nil, []InformationRegisterDefinition{register}, nil, nil, nil, nil, nil, []CommonModuleDefinition{module}, []EventSubscriptionDefinition{subscription}); err == nil || !strings.Contains(err.Error(), "is not an event of information-register-record-set") {
			t.Fatalf("invalid event for kind error = %v", err)
		}
	})
	t.Run("valid subscription", func(t *testing.T) {
		target := CatalogDefinition{Format: CurrentFormat, ID: uuid.MustNew(), Name: "Товары", Title: LocalizedText{"ru": "Товары"},
			Code: CatalogCode{Type: StringType, Length: 9}, DescriptionLength: 100}
		module := commonModuleFixture()
		module.Server = true
		subscription := eventSubscriptionFixture(target.ID)
		subscription.Module = module.ID
		if _, err := NewCatalogSnapshotWithEventSubscriptions(metadataConfiguration(), nil, nil, nil, []CatalogDefinition{target}, nil, nil, nil, nil, nil, nil, nil, []CommonModuleDefinition{module}, []EventSubscriptionDefinition{subscription}); err != nil {
			t.Fatalf("valid event subscription rejected: %v", err)
		}
	})
}

func TestLoadValidatesEventSubscriptionReferences(t *testing.T) {
	t.Parallel()
	root := metadataProject(t)
	if err := os.MkdirAll(filepath.Join(root, "metadata", string(EventSubscriptionKind)), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "metadata", string(CommonModuleKind)), 0o755); err != nil {
		t.Fatal(err)
	}
	catalogID := uuid.MustNew()
	writeMetadata(t, root, CatalogKind, catalogID.String(), "format: 1\nid: "+catalogID.String()+"\nname: Товары\ntitle: {ru: Товары}\ncode: {type: string, length: 9}\ndescription_length: 100\n")

	moduleSourceID := uuid.MustNew()
	if err := os.WriteFile(filepath.Join(root, "modules", moduleSourceID.String()+".bsl"),
		[]byte("Процедура ПередЗаписьюТовара(Источник, Отказ) Экспорт\nКонецПроцедуры\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	commonModuleID := uuid.MustNew()
	writeMetadata(t, root, CommonModuleKind, commonModuleID.String(), "format: 1\nid: "+commonModuleID.String()+
		"\nname: ОбщегоНазначения\ntitle: {ru: Общего назначения}\nserver: true\nmodule: "+moduleSourceID.String()+"\n")

	subscriptionID := uuid.MustNew()
	writeMetadata(t, root, EventSubscriptionKind, subscriptionID.String(), "format: 1\nid: "+subscriptionID.String()+
		"\nname: ЗапретПустогоНаименования\ntitle: {ru: Запрет}\nsource: [{kind: catalog-object, reference: "+catalogID.String()+"}]\nevent: before-write\nmodule: "+commonModuleID.String()+"\nprocedure: ПередЗаписьюТовара\n")
	if _, err := load(root, true); err != nil {
		t.Fatalf("valid event subscription rejected: %v", err)
	}

	unknownObjectID := uuid.MustNew()
	writeMetadata(t, root, EventSubscriptionKind, subscriptionID.String(), "format: 1\nid: "+subscriptionID.String()+
		"\nname: ЗапретПустогоНаименования\ntitle: {ru: Запрет}\nsource: [{kind: catalog-object, reference: "+unknownObjectID.String()+"}]\nevent: before-write\nmodule: "+commonModuleID.String()+"\nprocedure: ПередЗаписьюТовара\n")
	if _, err := load(root, true); err == nil || !strings.Contains(err.Error(), "unknown object") {
		t.Fatalf("event subscription with an unknown object was accepted: %v", err)
	}

	writeMetadata(t, root, EventSubscriptionKind, subscriptionID.String(), "format: 1\nid: "+subscriptionID.String()+
		"\nname: ЗапретПустогоНаименования\ntitle: {ru: Запрет}\nsource: [{kind: catalog-object, reference: "+catalogID.String()+"}]\nevent: before-write\nmodule: "+commonModuleID.String()+"\nprocedure: ПередЗаписьюТовара\n")
	if _, err := load(root, true); err != nil {
		t.Fatalf("valid event subscription rejected after repair: %v", err)
	}
}
