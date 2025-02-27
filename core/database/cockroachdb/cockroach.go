package cockroachdb

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/maestro-go/maestro/core/database"
	"github.com/maestro-go/maestro/core/enums"
	"github.com/maestro-go/maestro/internal/migrations"
)

const schema_history_table = "schema_history"
const lock_table = "schema_lock"

type CockroachRepository struct {
	database.Repository
	ctx       context.Context
	queriable database.Queriable
	db        database.Database
}

func NewCockroachRepository(ctx context.Context, db database.Database) *CockroachRepository {
	return &CockroachRepository{
		ctx:       ctx,
		queriable: db,
		db:        db,
	}
}

func (r *CockroachRepository) GetLatestMigration() (uint16, error) {
	tableExists, err := r.CheckSchemaHistoryTable()
	if err != nil {
		return 0, err
	}

	if !tableExists {
		return 0, nil
	}

	query := fmt.Sprintf(`
		SELECT COALESCE(MAX(version), 0)
		FROM %s
		WHERE success = true;
	`, schema_history_table)

	version := uint16(0)
	err = r.queriable.QueryRowContext(r.ctx, query).Scan(&version)
	if err != nil {
		return 0, err
	}
	return version, nil
}

func (r *CockroachRepository) AssertSchemaHistoryTable() error {
	exists, err := r.CheckSchemaHistoryTable()
	if err != nil {
		return err
	}

	if exists {
		return nil
	}

	query := fmt.Sprintf(`
		CREATE TABLE IF NOT EXISTS %s (
			version SMALLINT NOT NULL PRIMARY KEY,
			description VARCHAR(255) NOT NULL,
			md5_checksum CHAR(32) NOT NULL,
			success BOOLEAN NOT NULL DEFAULT false,
			executed_at TIMESTAMP NOT NULL DEFAULT NOW(),
			repaired_at TIMESTAMP
		);
	`, schema_history_table)

	_, err = r.queriable.ExecContext(r.ctx, query)
	if err != nil {
		return err
	}

	return nil
}

func (r *CockroachRepository) CheckSchemaHistoryTable() (bool, error) {
	query := `
		SELECT EXISTS (
			SELECT 1 FROM pg_tables
			WHERE tablename = $1 AND schemaname = current_schema()
		);
	`

	exists := false
	err := r.queriable.QueryRowContext(r.ctx, query, schema_history_table).Scan(&exists)
	if err != nil {
		return false, err
	}

	return exists, nil
}

func (r *CockroachRepository) ValidateMigrations(migrations []*migrations.Migration) []error {
	if len(migrations) < 1 {
		return nil
	}

	tableExists, err := r.CheckSchemaHistoryTable()
	if err != nil {
		return []error{err}
	}

	if !tableExists {
		return nil
	}

	migrationsTuples := make([]string, 0)
	for _, migration := range migrations {

		if migration.Type != enums.MIGRATION_UP {
			return []error{fmt.Errorf("invalid migration type: %s", migration.Type.Name())}
		}

		migrationsTuples = append(migrationsTuples,
			fmt.Sprintf("(%d, '%s', '%s')", migration.Version, migration.Description, *migration.Checksum))
	}

	// Check gaps
	query := fmt.Sprintf(`
		SELECT version FROM %s ORDER BY version ASC;
	`, schema_history_table)

	versionsRows, err := r.queriable.QueryContext(r.ctx, query)
	if err != nil {
		return []error{err}
	}
	defer versionsRows.Close()

	errs := make([]error, 0)

	expectedVersion := uint16(1)
	actualVersion := uint16(0)
	for versionsRows.Next() {
		err = versionsRows.Scan(&actualVersion)
		if err != nil {
			return []error{err}
		}

		if expectedVersion != actualVersion {
			errs = append(errs, fmt.Errorf("missing version %d", expectedVersion))
		}

		expectedVersion = actualVersion + 1
	}

	// Check description or checksum mismatch
	query = fmt.Sprintf(`
		SELECT version, description, md5_checksum
		FROM %s
		WHERE success = true AND (version, description, md5_checksum) NOT IN (%s);
	`, schema_history_table, strings.Join(migrationsTuples, ", "))

	rows, err := r.queriable.QueryContext(r.ctx, query)
	if err != nil {
		return []error{err}
	}
	defer rows.Close()

	type resStruct struct {
		version      uint16
		description  string
		md5_checksum string
	}

	for rows.Next() {
		res := new(resStruct)
		err := rows.Scan(&res.version, &res.description, &res.md5_checksum)
		if err != nil {
			errs = append(errs, err)
		}

		errs = append(errs, fmt.Errorf("invalid migration found: version: %d, description: %s, md5_checksum: %s."+
			" please check your local migration and changes", res.version, res.description, res.md5_checksum))
	}

	if len(errs) > 0 {
		return errs
	}
	return nil
}

func (r *CockroachRepository) ExecuteMigration(migration *migrations.Migration) []error {
	if migration.Type != enums.MIGRATION_UP {
		return []error{fmt.Errorf("invalid migration type: %s", migration.Type.Name())}
	}

	errs := make([]error, 0)

	_, err := r.queriable.ExecContext(r.ctx, *migration.Content)
	if err != nil {
		errs = append(errs, err)
	}

	query := fmt.Sprintf(`
		INSERT INTO %s (version, description, md5_checksum, success)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (version)
		DO UPDATE SET description = $2, md5_checksum = $3, success = $4, executed_at = NOW();
	`, schema_history_table)

	_, err = r.queriable.ExecContext(r.ctx, query, migration.Version, migration.Description,
		migration.Checksum, err == nil)

	if err != nil {
		errs = append(errs, fmt.Errorf("migration %d: %w", migration.Version, err))
	}

	if len(errs) > 0 {
		return errs
	}

	return nil
}

func (r *CockroachRepository) ExecuteHook(hook *migrations.Hook) error {
	_, err := r.queriable.ExecContext(r.ctx, *hook.Content)
	if err != nil {
		return err
	}

	return nil
}

func (r *CockroachRepository) RollbackMigration(migration *migrations.Migration) error {
	if migration.Type != enums.MIGRATION_DOWN {
		return fmt.Errorf("invalid migration type: %s", migration.Type.Name())
	}

	query := fmt.Sprintf(`
		SELECT EXISTS (
			SELECT version FROM %s WHERE version = $1
		);
	`, schema_history_table)

	exists := false
	err := r.queriable.QueryRowContext(r.ctx, query, migration.Version).Scan(&exists)
	if err != nil {
		return err
	}

	if !exists {
		return nil
	}

	_, err = r.queriable.ExecContext(r.ctx, *migration.Content)
	if err != nil {
		return err
	}

	query = fmt.Sprintf(`
		DELETE FROM %s
		WHERE version = $1;
	`, schema_history_table)

	res, err := r.queriable.ExecContext(r.ctx, query, migration.Version)
	if err != nil {
		return err
	}

	rowsAffected, err := res.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected < 1 {
		return fmt.Errorf("version was not deleted from \"%s\" table", schema_history_table)
	}

	return nil
}

func (r *CockroachRepository) DoInTransaction(fn func() error) error {
	tx, err := r.db.BeginTx(r.ctx, nil)
	if err != nil {
		return err
	}
	defer func() {
		tx.Rollback()
		r.queriable = r.db // Always reset queriable to db
	}()

	r.queriable = tx

	err = fn()
	if err != nil {
		return err
	}

	tx.Commit()

	return nil
}

func (r *CockroachRepository) DoInLock(fn func() error) error {
	err := r.lock()
	if err != nil {
		return err
	}
	defer func() {
		err = r.unlock()
		if err != nil {
			panic(fmt.Errorf("failed to delete lock table: %w", err))
		}
	}()

	err = fn()
	if err != nil {
		return err
	}

	return nil
}

// This function ensures that only one instance of the application can perform schema migrations at a time.
// It achieves this by creating a lock table if it doesn't already exist. If the table exists,
// it waits for up to 1 minute for the table to be deleted by another instance, indicating that the migration
// process has completed.
func (r *CockroachRepository) lock() error {
	query := `
		SELECT EXISTS (
			SELECT table_name FROM information_schema.tables
			WHERE table_name = $1
		);
	`

	success := false
	for range 12 {
		exists := false
		err := r.db.QueryRowContext(r.ctx, query, lock_table).Scan(&exists)
		if err != nil {
			return err
		}

		if !exists {
			_, err = r.db.ExecContext(r.ctx, fmt.Sprintf(`
				CREATE TABLE IF NOT EXISTS %s (
					unused INT NOT NULL PRIMARY KEY
				);
			`, lock_table))
			if err != nil {
				return err
			}

			success = true
			break
		}

		time.Sleep(time.Second * 5) // Delays 5 seconds
	}

	if !success {
		return fmt.Errorf("timeout while waiting for schema_lock deletion")
	}

	return nil
}

func (r *CockroachRepository) unlock() error {
	_, err := r.db.ExecContext(r.ctx, fmt.Sprintf("DROP TABLE IF EXISTS %s;", lock_table))
	if err != nil {
		return err
	}

	return nil
}

func (r *CockroachRepository) Repair(migrations []*migrations.Migration) []error {
	tableExists, err := r.CheckSchemaHistoryTable()
	if err != nil {
		return []error{err}
	}

	if !tableExists {
		return nil
	}

	errs := make([]error, 0)

	for _, migration := range migrations {
		query := fmt.Sprintf(`
			INSERT INTO %s (version, description, md5_checksum, success, repaired_at)
			VALUES ($1, $2, $3, true, NOW())
			ON CONFLICT (version) DO UPDATE
			SET description = EXCLUDED.description, md5_checksum = EXCLUDED.md5_checksum,
				repaired_at = CASE
					WHEN EXCLUDED.description <> %s.description OR EXCLUDED.md5_checksum <> %s.md5_checksum
					THEN NOW()
					ELSE %s.repaired_at
				END;
		`, schema_history_table, schema_history_table, schema_history_table, schema_history_table)

		_, err := r.queriable.ExecContext(r.ctx, query, migration.Version, migration.Description, *migration.Checksum)
		if err != nil {
			errs = append(errs, err)
		}
	}

	if len(errs) > 0 {
		return errs
	}
	return nil
}

func (r *CockroachRepository) GetFailingMigrations() ([]*migrations.Migration, error) {
	exists, err := r.CheckSchemaHistoryTable()
	if err != nil {
		return nil, err
	}

	if !exists {
		return nil, nil
	}

	query := fmt.Sprintf(`
        SELECT version, description, md5_checksum
        FROM %s
        WHERE success = false;
    `, schema_history_table)

	rows, err := r.queriable.QueryContext(r.ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var failingMigrations []*migrations.Migration
	for rows.Next() {
		var migration migrations.Migration
		var checksum string
		if err := rows.Scan(&migration.Version, &migration.Description, &checksum); err != nil {
			return nil, err
		}
		migration.Checksum = &checksum
		failingMigrations = append(failingMigrations, &migration)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return failingMigrations, nil
}
