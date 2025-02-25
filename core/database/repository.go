package database

import (
	"context"
	"database/sql"

	"github.com/maestro-go/maestro/internal/migrations"
)

// Queriable abstract the either database or transaction
type Queriable interface {
	Exec(query string, args ...any) (sql.Result, error)
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	Prepare(query string) (*sql.Stmt, error)
	PrepareContext(ctx context.Context, query string) (*sql.Stmt, error)
	Query(query string, args ...any) (*sql.Rows, error)
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRow(query string, args ...any) *sql.Row
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

// Database abstract the database connection
type Database interface {
	Queriable
	Begin() (*sql.Tx, error)
	BeginTx(ctx context.Context, opts *sql.TxOptions) (*sql.Tx, error)
}

type Repository interface {

	// GetLatestMigration retrieves the highest successfully executed migration version
	// from the schema history table. If the schema history table does not exist, it returns 0.
	// Returns an error if there is an issue querying the database.
	GetLatestMigration() (uint16, error)

	// AssertSchemaHistoryTable ensures that the schema history table exists.
	// If it does not exist, the method creates it.
	// Returns an error if there is an issue creating the table.
	AssertSchemaHistoryTable() error

	// CheckSchemaHistoryTable verifies whether the schema history table exists in the database.
	// Returns true if the table exists, false otherwise.
	// Returns an error if there is an issue querying the database.
	CheckSchemaHistoryTable() (bool, error)

	// ValidateMigrations compares the versions of the provided migrations with their respective
	// checksums stored in the schema history table. If a mismatch is found or if a migration
	// version is missing from the table, an error is returned.
	// Returns a slice of errors if there are validation issues.
	ValidateMigrations(migrations []*migrations.Migration) []error

	// ExecuteMigration applies the specified UP migration to the database.
	// If the migration is already recorded in the schema history table, its status is updated.
	// If the migration fails, it is marked as unsuccessful in the schema history table.
	// Returns a slice of errors if there are issues executing the migration.
	ExecuteMigration(migration *migrations.Migration) []error

	// ExecuteHook runs the specified hook script. This method is used for executing hooks such
	// as before/after migration scripts.
	// Returns an error if there is an issue executing the hook.
	ExecuteHook(hook *migrations.Hook) error

	// RollbackMigration executes the specified DOWN migration to revert changes made by a previous
	// migration. After successful execution, the corresponding version is removed from the schema
	// history table.
	// Returns an error if there is an issue executing the rollback.
	RollbackMigration(migration *migrations.Migration) error

	// Repair updates the md5 checksums, descriptions, or versions of migrations that mismatch
	// the stored values in the schema history table. Updates the repaired_at timestamp to now.
	// Returns a list of errors for any failed repairs.
	Repair(migrations []*migrations.Migration) []error

	// GetFailingMigrations retrieves migrations that have failed (success = false).
	// Returns a slice of migrations and an error if there is an issue querying the database.
	GetFailingMigrations() ([]*migrations.Migration, error)

	// DoInTransaction initializes a database transaction. All queries executed within the callback
	// function are performed within this transaction. If the callback function returns an error,
	// the transaction is rolled back.
	// Returns an error if there is an issue starting the transaction or if the callback returns an error.
	DoInTransaction(fn func() error) error

	// DoInLock acquires a lock on the database to prevent concurrent execution of
	// migrations. This ensures that migrations are applied sequentially and avoids duplication.
	// Returns an error if there is an issue acquiring or releasing the lock, or if the callback returns an error.
	DoInLock(fn func() error) error
}
