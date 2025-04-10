/*
Package libdb provides an interface for interacting with
a SQL database, currently with a specific implementation for PostgreSQL using lib/pq.

Key Features:

 1. Abstraction: Defines interfaces (`DBManager`, `Exec`, `QueryRower`) to decouple
    application code from specific database driver details.

 2. Simplified Transaction Management: The `DBManager.WithTransaction` method
    provides a clear pattern for handling database transactions, returning
    separate functions for committing (`CommitTx`) and releasing/rolling back
    (`ReleaseTx`). The `ReleaseTx` function is designed for use with `defer`
    to ensure transactions are always finalized and connections are released,
    even in cases of errors or panics.

 3. Centralized Error Translation: Maps common low-level database errors
    (like sql.ErrNoRows or PostgreSQL-specific pq.Error codes) to a consistent
    set of exported package errors (e.g., ErrNotFound, ErrUniqueViolation,
    ErrDeadlockDetected). This simplifies error handling in application code.

Usage Example (Transaction):

	func handleRequest(ctx context.Context, mgr libdb.DBManager) error {
	    // Start transaction, get executor and commit/release functions
	    exec, commit, release, err := mgr.WithTransaction(ctx)
	    if err != nil {
	        return fmt.Errorf("failed to start transaction: %w", err)
	    }
	    // Always defer release() to ensure cleanup (rollback on error/panic, no-op after commit)
	    defer release()

	    // --- Do work using exec ---
	    _, err = exec.ExecContext(ctx, "UPDATE settings SET value = $1 WHERE key = $2", "new_value", "setting_key")
	    if err != nil {
	        // Error occurred - no need to call release explicitly, defer handles it.
	        return fmt.Errorf("failed to update setting: %w", err)
	    }

	    // --- Success ---
	    // Attempt to commit; if it fails, the deferred release() still runs.
	    if err = commit(ctx); err != nil {
	        return fmt.Errorf("transaction commit failed: %w", err)
	    }

	    // Commit successful. The deferred release() will run but do nothing (idempotent).
	    return nil
	}
*/
package libdb
