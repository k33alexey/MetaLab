package metadata

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

const defaultDataLockWait = 20 * time.Second

var (
	ErrTransactionNotActive    = errors.New("transaction is not active")
	ErrTransactionNotCompleted = errors.New("transaction was not completed")
	ErrTransactionDoomed       = errors.New("transaction cannot be committed after an error or rollback")
	ErrTransactionBoundary     = errors.New("transaction boundary changed during a data operation")
	ErrDeadlockDetected        = errors.New("database deadlock detected")
	ErrDataLockTimeout         = errors.New("data lock wait timeout exceeded")
	ErrSerializationFailure    = errors.New("transaction serialization failure")
	errNoDatabaseRepository    = errors.New("metadata runtime has no PostgreSQL repository")
)

type transactionContextKey struct{}

type transactionScope struct {
	runtime         *Runtime
	pool            *pgxpool.Pool
	tx              pgx.Tx
	depth           int
	rollbackOnly    bool
	implicit        bool
	rollbackActions []func()
	lockWait        time.Duration
	operationDepth  int
}

// BeginExecution creates one transaction boundary for a top-level BSL call.
// Nested VM calls inherit it from context and never finalize it independently.
func (runtime *Runtime) BeginExecution(ctx context.Context) (context.Context, func(error) error, error) {
	if ctx == nil {
		return nil, nil, fmt.Errorf("execution context is required")
	}
	if existing, ok := ctx.Value(transactionContextKey{}).(*transactionScope); ok {
		if existing.runtime != nil && existing.runtime != runtime {
			return nil, nil, fmt.Errorf("execution context belongs to another metadata runtime")
		}
		existing.runtime = runtime
		return ctx, nil, nil
	}
	pool, err := runtime.databasePool()
	if err != nil {
		if errors.Is(err, errNoDatabaseRepository) {
			return ctx, nil, nil
		}
		return nil, nil, err
	}
	scope := &transactionScope{runtime: runtime, pool: pool, lockWait: runtime.dataLockWaitTimeout()}
	scoped := context.WithValue(ctx, transactionContextKey{}, scope)
	return scoped, func(cause error) error { return scope.finish(cause) }, nil
}

// SetDataLockWaitTimeout configures how long this runtime waits for PostgreSQL locks.
func (runtime *Runtime) SetDataLockWaitTimeout(wait time.Duration) error {
	if wait < time.Millisecond || wait > 5*time.Minute {
		return fmt.Errorf("data lock wait timeout must be between 1ms and 5m")
	}
	runtime.dataLockWait.Store(int64(wait))
	return nil
}

func (runtime *Runtime) dataLockWaitTimeout() time.Duration {
	wait := time.Duration(runtime.dataLockWait.Load())
	if wait <= 0 {
		return defaultDataLockWait
	}
	return wait
}

func (runtime *Runtime) databasePool() (*pgxpool.Pool, error) {
	var pool *pgxpool.Pool
	for _, candidate := range []*pgxpool.Pool{
		poolOfConstants(runtime.repository), poolOfCatalogs(runtime.catalogRepository), poolOfDocuments(runtime.documentRepository),
	} {
		if candidate == nil {
			continue
		}
		if pool != nil && pool != candidate {
			return nil, fmt.Errorf("metadata runtime repositories use different PostgreSQL pools")
		}
		pool = candidate
	}
	if pool == nil {
		return nil, errNoDatabaseRepository
	}
	return pool, nil
}

func poolOfConstants(repository *ConstantRepository) *pgxpool.Pool {
	if repository == nil {
		return nil
	}
	return repository.pool
}

func poolOfCatalogs(repository *CatalogRepository) *pgxpool.Pool {
	if repository == nil {
		return nil
	}
	return repository.pool
}

func poolOfDocuments(repository *DocumentRepository) *pgxpool.Pool {
	if repository == nil {
		return nil
	}
	return repository.pool
}

func scopeFromContext(ctx context.Context, pool *pgxpool.Pool) (*transactionScope, bool) {
	if ctx == nil {
		return nil, false
	}
	scope, ok := ctx.Value(transactionContextKey{}).(*transactionScope)
	return scope, ok && scope.pool == pool
}

func (runtime *Runtime) BeginTransaction(ctx context.Context) error {
	pool, err := runtime.databasePool()
	if err != nil {
		return err
	}
	scope, ok := scopeFromContext(ctx, pool)
	if !ok {
		return fmt.Errorf("begin transaction requires an active BSL execution")
	}
	return scope.begin(ctx)
}

func (runtime *Runtime) CommitTransaction(ctx context.Context) error {
	pool, err := runtime.databasePool()
	if err != nil {
		return err
	}
	scope, ok := scopeFromContext(ctx, pool)
	if !ok {
		return ErrTransactionNotActive
	}
	return scope.commit(ctx)
}

func (runtime *Runtime) RollbackTransaction(ctx context.Context) error {
	pool, err := runtime.databasePool()
	if err != nil {
		return err
	}
	scope, ok := scopeFromContext(ctx, pool)
	if !ok {
		return ErrTransactionNotActive
	}
	return scope.rollback(ctx)
}

func (runtime *Runtime) TransactionActive(ctx context.Context) (bool, error) {
	pool, err := runtime.databasePool()
	if err != nil {
		return false, err
	}
	scope, ok := scopeFromContext(ctx, pool)
	return ok && scope.tx != nil && scope.depth > 0, nil
}

func (scope *transactionScope) begin(ctx context.Context) error {
	if scope.tx != nil {
		if scope.rollbackOnly {
			return ErrTransactionDoomed
		}
		scope.depth++
		return nil
	}
	tx, err := scope.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if err != nil {
		return classifyTransactionError(fmt.Errorf("begin transaction: %w", err))
	}
	if err := configureLockWait(ctx, tx, scope.lockWait); err != nil {
		_ = rollbackWithCleanupContext(tx)
		return classifyTransactionError(err)
	}
	scope.tx, scope.depth, scope.rollbackOnly = tx, 1, false
	scope.rollbackActions = nil
	return nil
}

func (scope *transactionScope) commit(ctx context.Context) error {
	if scope.tx == nil || scope.depth < 1 {
		return ErrTransactionNotActive
	}
	if scope.depth > 1 {
		scope.depth--
		if scope.rollbackOnly {
			return ErrTransactionDoomed
		}
		return nil
	}
	if scope.implicit || scope.operationDepth > 0 {
		return ErrTransactionBoundary
	}
	tx, doomed := scope.tx, scope.rollbackOnly
	if doomed {
		rollbackErr := rollbackWithCleanupContext(tx)
		scope.reset(true)
		if rollbackErr != nil {
			return errors.Join(ErrTransactionDoomed, rollbackErr)
		}
		return ErrTransactionDoomed
	}
	if err := tx.Commit(ctx); err != nil {
		_ = rollbackWithCleanupContext(tx)
		scope.reset(true)
		return classifyTransactionError(fmt.Errorf("commit transaction: %w", err))
	}
	scope.reset(false)
	return nil
}

func (scope *transactionScope) rollback(_ context.Context) error {
	if scope.tx == nil || scope.depth < 1 {
		return ErrTransactionNotActive
	}
	if scope.implicit {
		if scope.depth > 1 {
			scope.depth--
		}
		scope.rollbackOnly = true
		scope.runRollbackActions()
		return nil
	}
	if scope.depth > 1 {
		scope.depth--
		scope.rollbackOnly = true
		scope.runRollbackActions()
		return nil
	}
	tx := scope.tx
	err := rollbackWithCleanupContext(tx)
	scope.reset(true)
	if err != nil {
		return classifyTransactionError(fmt.Errorf("rollback transaction: %w", err))
	}
	return nil
}

func (scope *transactionScope) finish(cause error) error {
	if scope.tx == nil {
		return nil
	}
	doomed := scope.rollbackOnly
	err := rollbackWithCleanupContext(scope.tx)
	scope.reset(true)
	if err != nil {
		return classifyTransactionError(fmt.Errorf("cleanup unfinished transaction: %w", err))
	}
	if cause != nil {
		return nil
	}
	if doomed {
		return ErrTransactionDoomed
	}
	return ErrTransactionNotCompleted
}

func (scope *transactionScope) reset(rollback bool) {
	if rollback {
		scope.runRollbackActions()
	}
	scope.tx, scope.depth, scope.rollbackOnly, scope.implicit = nil, 0, false, false
	scope.rollbackActions = nil
	scope.operationDepth = 0
}

func (scope *transactionScope) runRollbackActions() {
	for index := len(scope.rollbackActions) - 1; index >= 0; index-- {
		scope.rollbackActions[index]()
	}
	scope.rollbackActions = nil
}

func rollbackWithCleanupContext(tx pgx.Tx) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	err := tx.Rollback(ctx)
	if errors.Is(err, pgx.ErrTxClosed) {
		return nil
	}
	return err
}

func configureLockWait(ctx context.Context, tx pgx.Tx, wait time.Duration) error {
	if wait <= 0 {
		wait = defaultDataLockWait
	}
	_, err := tx.Exec(ctx, "SELECT set_config('lock_timeout', $1, true)", wait.String())
	if err != nil {
		return fmt.Errorf("configure data lock wait: %w", err)
	}
	return nil
}

func runDataTransaction(ctx context.Context, pool *pgxpool.Pool, rollbackAction func(), operation func(context.Context, pgx.Tx) error) error {
	if pool == nil || operation == nil {
		return fmt.Errorf("data transaction requires PostgreSQL and an operation")
	}
	if scope, ok := scopeFromContext(ctx, pool); ok {
		if scope.tx != nil {
			if scope.rollbackOnly {
				return ErrTransactionDoomed
			}
			tx, depth := scope.tx, scope.depth
			scope.operationDepth++
			err := operation(ctx, tx)
			scope.operationDepth--
			if err != nil {
				scope.recordFailure(err)
				scope.rollbackOnly = true
				scope.runRollbackActions()
				return classifyTransactionError(err)
			}
			if scope.tx != tx || scope.depth != depth || scope.rollbackOnly {
				if scope.rollbackOnly {
					return ErrTransactionDoomed
				}
				return ErrTransactionBoundary
			}
			if rollbackAction != nil {
				scope.rollbackActions = append(scope.rollbackActions, rollbackAction)
			}
			return nil
		}
		return scope.runImplicit(ctx, rollbackAction, operation)
	}
	scope := &transactionScope{pool: pool, lockWait: defaultDataLockWait}
	scoped := context.WithValue(ctx, transactionContextKey{}, scope)
	return scope.runImplicit(scoped, rollbackAction, operation)
}

func (scope *transactionScope) runImplicit(ctx context.Context, rollbackAction func(), operation func(context.Context, pgx.Tx) error) error {
	tx, err := scope.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if err != nil {
		return classifyTransactionError(err)
	}
	scope.tx, scope.depth, scope.rollbackOnly, scope.implicit = tx, 1, false, true
	scope.rollbackActions = nil
	rollback := func(result error) error {
		rollbackErr := rollbackWithCleanupContext(tx)
		scope.reset(true)
		if rollbackErr != nil {
			return errors.Join(result, classifyTransactionError(rollbackErr))
		}
		return result
	}
	if err := configureLockWait(ctx, tx, scope.lockWait); err != nil {
		return rollback(classifyTransactionError(err))
	}
	if err := operation(ctx, tx); err != nil {
		scope.recordFailure(err)
		return rollback(classifyTransactionError(err))
	}
	if scope.tx != tx || scope.depth != 1 {
		return rollback(ErrTransactionBoundary)
	}
	if scope.rollbackOnly {
		return rollback(ErrTransactionDoomed)
	}
	if rollbackAction != nil {
		scope.rollbackActions = append(scope.rollbackActions, rollbackAction)
	}
	if err := tx.Commit(ctx); err != nil {
		return rollback(classifyTransactionError(err))
	}
	scope.reset(false)
	return nil
}

type dataQueryer interface {
	Query(context.Context, string, ...any) (pgx.Rows, error)
	QueryRow(context.Context, string, ...any) pgx.Row
}

func queryData(ctx context.Context, pool *pgxpool.Pool) (dataQueryer, error) {
	if scope, ok := scopeFromContext(ctx, pool); ok && scope.tx != nil {
		if scope.rollbackOnly {
			return nil, ErrTransactionDoomed
		}
		return scope.tx, nil
	}
	return pool, nil
}

func recordDataError(ctx context.Context, pool *pgxpool.Pool, err error) error {
	if err == nil {
		return nil
	}
	if scope, ok := scopeFromContext(ctx, pool); ok && scope.tx != nil {
		scope.recordFailure(err)
	}
	return classifyTransactionError(err)
}

func (scope *transactionScope) recordFailure(err error) {
	var databaseError *pgconn.PgError
	if errors.As(err, &databaseError) {
		scope.rollbackOnly = true
		scope.runRollbackActions()
	}
}

func classifyTransactionError(err error) error {
	if err == nil {
		return nil
	}
	var databaseError *pgconn.PgError
	if !errors.As(err, &databaseError) {
		return err
	}
	var classified error
	switch databaseError.Code {
	case "40P01":
		classified = ErrDeadlockDetected
	case "55P03":
		classified = ErrDataLockTimeout
	case "40001":
		classified = ErrSerializationFailure
	default:
		return err
	}
	return fmt.Errorf("%w", errors.Join(classified, err))
}
