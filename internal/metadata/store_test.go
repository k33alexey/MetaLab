package metadata

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgconn"

	"github.com/k33alexey/MetaLab/internal/uuid"
)

func TestEnsureConstantStorageUsesPlatformSchema(t *testing.T) {
	t.Parallel()
	recorder := &recordingExecutor{}
	if err := EnsureConstantStorage(context.Background(), recorder); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(recorder.statement, "ml_core.constant_values") || !strings.Contains(recorder.statement, "value jsonb NOT NULL") {
		t.Fatalf("constant storage SQL = %q", recorder.statement)
	}
	if err := EnsureConstantStorage(context.Background(), nil); err == nil {
		t.Fatal("EnsureConstantStorage accepted nil executor")
	}
	recorder.err = errors.New("database unavailable")
	if err := EnsureConstantStorage(context.Background(), recorder); err == nil || !strings.Contains(err.Error(), "database unavailable") {
		t.Fatalf("EnsureConstantStorage error = %v", err)
	}
}

func TestSyncConstantStorageUsesExactPublishedIDs(t *testing.T) {
	t.Parallel()
	recorder := &recordingExecutor{}
	id := uuid.MustNew()
	if err := SyncConstantStorage(context.Background(), recorder, []uuid.UUID{id}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(recorder.statement, "constant_id = ANY") || len(recorder.arguments) != 1 {
		t.Fatalf("sync SQL=%q arguments=%v", recorder.statement, recorder.arguments)
	}
	if err := SyncConstantStorage(context.Background(), recorder, []uuid.UUID{{}}); err == nil {
		t.Fatal("SyncConstantStorage accepted zero UUID")
	}
}

func TestDecodeStoredConstantRejectsCorruptData(t *testing.T) {
	t.Parallel()
	catalog := &Catalog{constantByID: map[uuid.UUID]int{}}
	if _, err := decodeStoredConstant(constantID, []byte(`{"kind":"boolean","data":"true"}`), 0, nil, time.Now(), catalog); err == nil {
		t.Fatal("decodeStoredConstant accepted zero revision")
	}
}

type recordingExecutor struct {
	statement string
	arguments []any
	err       error
}

func (executor *recordingExecutor) Exec(_ context.Context, statement string, arguments ...any) (pgconn.CommandTag, error) {
	executor.statement = statement
	executor.arguments = arguments
	return pgconn.NewCommandTag("CREATE TABLE"), executor.err
}
