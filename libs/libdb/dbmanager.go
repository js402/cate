package libdb

import (
	"context"
	"database/sql"
	"errors"
)

// Predefined errors for common database interaction scenarios.
// Using these allows application code to check for specific conditions
// using errors.Is without relying on driver-specific error types or codes.
var (
	// ErrNotFound is returned by Scan when sql.ErrNoRows is encountered.
	ErrNotFound = errors.New("libdb: not found")

	// ErrTxFailed indicates a failure during transaction finalization (Commit or Rollback).
	ErrTxFailed = errors.New("libdb: transaction failed")

	// ErrMaxRowsReached indicates that the maximum number of rows on a given table has been reached.
	// This error should be thrown when attempting to create a new entry would lead to exceeding the maximum capacity.
	// It implies that enforcing a reasonable maximum row count is necessary to prevent operations like bulk update from failing.
	ErrMaxRowsReached = errors.New("max row count reached")

	// --- Constraint Violations ---

	// ErrUniqueViolation corresponds to unique key constraint errors (e.g., PostgreSQL code 23505).
	ErrUniqueViolation = errors.New("libdb: unique constraint violation")
	// ErrForeignKeyViolation corresponds to foreign key constraint errors (e.g., PostgreSQL code 23503).
	ErrForeignKeyViolation = errors.New("libdb: foreign key violation")
	// ErrNotNullViolation corresponds to not-null constraint errors (e.g., PostgreSQL code 23502).
	ErrNotNullViolation = errors.New("libdb: not null constraint violation")
	// ErrCheckViolation corresponds to check constraint errors (e.g., PostgreSQL code 23514).
	ErrCheckViolation = errors.New("libdb: check constraint violation")
	// ErrConstraintViolation is a generic error for constraint violations not specifically mapped.
	ErrConstraintViolation = errors.New("libdb: constraint violation")

	// --- Operational Errors ---

	// ErrDeadlockDetected corresponds to deadlock errors (e.g., PostgreSQL code 40P01).
	ErrDeadlockDetected = errors.New("libdb: deadlock detected")
	// ErrSerializationFailure corresponds to serialization failures (e.g., PostgreSQL code 40001).
	ErrSerializationFailure = errors.New("libdb: serialization failure")
	// ErrLockNotAvailable corresponds to lock acquisition failures (e.g., PostgreSQL code 55P03).
	ErrLockNotAvailable = errors.New("libdb: lock not available")
	// ErrQueryCanceled corresponds to query cancellation (e.g., PostgreSQL code 57014 or context cancellation).
	ErrQueryCanceled = errors.New("libdb: query canceled")

	// --- Data Errors ---

	// ErrDataTruncation corresponds to data truncation errors (e.g., PostgreSQL code 22001).
	ErrDataTruncation = errors.New("libdb: data truncation error")
	// ErrNumericOutOfRange corresponds to numeric overflow errors (e.g., PostgreSQL code 22003).
	ErrNumericOutOfRange = errors.New("libdb: numeric value out of range")
	// ErrInvalidInputSyntax corresponds to syntax errors in data representation (e.g., PostgreSQL code 22P02).
	ErrInvalidInputSyntax = errors.New("libdb: invalid input syntax")

	// --- Schema Errors ---

	// ErrUndefinedColumn corresponds to referencing an unknown column (e.g., PostgreSQL code 42703).
	ErrUndefinedColumn = errors.New("libdb: undefined column")
	// ErrUndefinedTable corresponds to referencing an unknown table (e.g., PostgreSQL code 42P01).
	ErrUndefinedTable = errors.New("libdb: undefined table")
)

// DBManager defines the interface for obtaining database executors and managing
// the database connection lifecycle. It serves as the main entry point for database interactions.
type DBManager interface {
	// WithoutTransaction returns an executor that operates directly on the underlying
	// database connection pool (i.e., outside of an explicit transaction).
	// Each operation may run on a different connection.
	WithoutTransaction() Exec

	// WithTransaction starts a new database transaction and returns an executor
	// bound to that transaction, a function to commit the transaction, and a function
	// to release (roll back) the transaction.
	//
	// The returned ReleaseTx function is designed to be deferred to ensure the transaction
	// is always finalized (rolled back on error/panic, or a no-op after successful commit)
	// and the connection is released. See package documentation for usage patterns.
	WithTransaction(ctx context.Context) (Exec, CommitTx, ReleaseTx, error)

	// Close terminates the underlying database connection pool.
	// It should be called when the application is shutting down.
	Close() error
}

// Exec defines the common interface for executing database operations,
// whether within a transaction or directly on the connection pool.
// Errors returned by methods implementing this interface should be translated
// into the package's predefined Err* variables where applicable.
type Exec interface {
	// ExecContext executes a query without returning any rows.
	// The args are for any placeholder parameters in the query.
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)

	// QueryContext executes a query that returns rows, typically a SELECT.
	// The args are for any placeholder parameters in the query.
	// Callers should check rows.Err() after iterating.
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)

	// QueryRowContext executes a query that is expected to return at most one row.
	// QueryRowContext always returns a non-nil value. Errors are deferred until
	// QueryRower's Scan method is called.
	QueryRowContext(ctx context.Context, query string, args ...any) QueryRower
}

// QueryRower provides the Scan method, typically implemented by wrapping *sql.Row.
// Using this interface allows Scan errors (like sql.ErrNoRows) to be translated
// consistently by the library.
type QueryRower interface {
	// Scan copies the columns from the matched row into the values pointed at by dest.
	// If no rows were found, it returns ErrNotFound. Other scan errors are translated.
	Scan(dest ...any) error
}

// CommitTx is a function type responsible for attempting to commit a transaction.
// It should typically only be called on the success path of a transactional operation.
// It may check the context before committing and returns ErrTxFailed or a translated
// database error if the commit fails.
type CommitTx func(ctx context.Context) error

// ReleaseTx is a function type responsible for rolling back a transaction, ensuring
// its resources are released. It is designed to be idempotent (safe to call multiple times
// or after a commit) and is ideal for use with `defer` to guarantee cleanup.
// It returns ErrTxFailed or a translated database error if the rollback fails
// (and the transaction wasn't already finalized).
type ReleaseTx func() error
