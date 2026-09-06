package metadata

import (
	"context"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/k33alexey/MetaLab/internal/bsl/compiler"
	"github.com/k33alexey/MetaLab/internal/bsl/vm"
	"github.com/k33alexey/MetaLab/internal/uuid"
)

func TestConstantRepositoryIntegration(t *testing.T) {
	databaseURL := os.Getenv("ML_TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("ML_TEST_DATABASE_URL is not set")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	if err := EnsureConstantStorage(ctx, pool); err != nil {
		t.Fatal(err)
	}
	id, actor := uuid.MustNew(), uuid.MustNew()
	catalog := &Catalog{
		Constants:      []Constant{{ID: id, Name: "Курс", Types: []Type{{Kind: NumberType, Precision: 8, Scale: 4}}}},
		constantByName: map[string]int{"курс": 0}, constantByID: map[uuid.UUID]int{id: 0},
	}
	repository, err := NewConstantRepository(pool, catalog)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cleanupCancel()
		_, _ = pool.Exec(cleanupCtx, "DELETE FROM ml_core.constant_values WHERE constant_id = $1", id.String())
	})
	first, err := repository.Set(ctx, "курс", Value{Kind: NumberType, Data: "40.2500"}, &actor)
	if err != nil || first.Revision != 1 || first.Value.Data != "40.25" || first.ChangedBy == nil || *first.ChangedBy != actor {
		t.Fatalf("first = %+v, error=%v", first, err)
	}
	second, err := repository.Set(ctx, "Курс", Value{Kind: NumberType, Data: "41.5"}, nil)
	if err != nil || second.Revision != 2 || second.ChangedBy != nil {
		t.Fatalf("second = %+v, error=%v", second, err)
	}
	loaded, err := repository.Get(ctx, "КУРС")
	if err != nil || loaded.Revision != second.Revision || loaded.Value != second.Value {
		t.Fatalf("loaded = %+v, error=%v", loaded, err)
	}
	if _, err := repository.Get(ctx, "Неизвестная"); err == nil || err.Error() != `unknown constant "Неизвестная"` {
		t.Fatalf("unknown constant error = %v", err)
	}
	runtime, err := NewRuntime(repository, catalog, &actor)
	if err != nil {
		t.Fatal(err)
	}
	program, diagnostics := compiler.CompileSource("constant.bsl", `&AtServer
Function Run()
Constants.Курс.Set(42.75);
Return Constants.Курс.Get();
EndFunction`)
	if len(diagnostics) != 0 {
		t.Fatal(diagnostics)
	}
	machine, err := vm.New(program)
	if err != nil {
		t.Fatal(err)
	}
	result, err := machine.NewContextWithMetadata(runtime).CallContext(ctx, "Run")
	if err != nil || result.String() != "42.75" {
		t.Fatalf("BSL constant result=%v error=%v", result, err)
	}
	const workers = 10
	start := make(chan struct{})
	errorsChannel := make(chan error, workers)
	for index := range workers {
		go func() {
			<-start
			_, updateErr := repository.Set(ctx, "Курс", Value{Kind: NumberType, Data: fmt.Sprint(43 + index)}, nil)
			errorsChannel <- updateErr
		}()
	}
	close(start)
	for range workers {
		if err := <-errorsChannel; err != nil {
			t.Fatal(err)
		}
	}
	concurrent, err := repository.Get(ctx, "Курс")
	if err != nil || concurrent.Revision != 13 {
		t.Fatalf("concurrent constant = %+v, error=%v", concurrent, err)
	}
	if _, err := pool.Exec(ctx, "DELETE FROM ml_core.constant_values WHERE constant_id = $1", id.String()); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.Get(ctx, "Курс"); !errors.Is(err, ErrConstantValueNotFound) {
		t.Fatalf("missing constant error = %v", err)
	}
}
