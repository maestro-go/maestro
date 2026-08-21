package snowflake

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/maestro-go/maestro/core/database"
	"github.com/maestro-go/maestro/core/enums"
	"github.com/maestro-go/maestro/internal/migrations"
)

const default_history_table = "SCHEMA_HISTORY"
const lock_table = "SCHEMA_LOCK"

type SnowflakeRepository struct {
	database.Repository
	ctx           context.Context
	queriable     database.Queriable
	db            database.Database
	history_table string
}

func NewSnowflakeRepository(ctx context.Context, db database.Database, history_table *string) *SnowflakeRepository {
	repo := &SnowflakeRepository{
		ctx:       ctx,
		queriable: db,
		db:        db,
	}

	if history_table != nil {
		repo.history_table = strings.ToUpper(*history_table)
	} else {
		repo.history_table = default_history_table
	}

	return repo
}

func (r *SnowflakeRepository) GetLatestMigration() (uint16, error) {
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
		WHERE success = TRUE;
	`, r.history_table)

	version := uint16(0)
	err = r.queriable.QueryRowContext(r.ctx, query).Scan(&version)
	if err != nil {
		return 0, err
	}
	return version, nil
}

func (r *SnowflakeRepository) AssertSchemaHistoryTable() error {
	exists, err := r.CheckSchemaHistoryTable()
	if err != nil {
		return err
	}

	if exists {
		return nil
	}

	query := fmt.Sprintf(`
		CREATE TABLE IF NOT EXISTS %s (
			version NUMBER(5, 0) NOT NULL PRIMARY KEY,
			description VARCHAR(255) NOT NULL,
			md5_checksum VARCHAR(32) NOT NULL,
			success BOOLEAN NOT NULL DEFAULT FALSE,
			executed_at TIMESTAMP_NTZ DEFAULT CURRENT_TIMESTAMP(),
			repaired_at TIMESTAMP_NTZ
		);
	`, r.history_table)

	_, err = r.queriable.ExecContext(r.ctx, query)
	if err != nil {
		return err
	}

	return nil
}

func (r *SnowflakeRepository) CheckSchemaHistoryTable() (bool, error) {
	query := `
		SELECT COUNT(*)
		FROM information_schema.tables
		WHERE table_catalog = CURRENT_DATABASE()
		  AND table_schema = CURRENT_SCHEMA()
		  AND table_name = ?
	`

	count := 0
	err := r.queriable.QueryRowContext(r.ctx, query, r.history_table).Scan(&count)
	if err != nil {
		return false, err
	}

	return count > 0, nil
}

func (r *SnowflakeRepository) ValidateMigrations(migs []*migrations.Migration) []error {
	if len(migs) < 1 {
		return nil
	}

	tableExists, err := r.CheckSchemaHistoryTable()
	if err != nil {
		return []error{err}
	}

	if !tableExists {
		return nil
	}

	for _, migration := range migs {
		if migration.Type != enums.MIGRATION_UP {
			return []error{fmt.Errorf("invalid migration type: %s", migration.Type.Name())}
		}
	}

	// Check gaps
	query := fmt.Sprintf(`
		SELECT version FROM %s ORDER BY version ASC;
	`, r.history_table)

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

		for expectedVersion < actualVersion {
			errs = append(errs, fmt.Errorf("missing version %d", expectedVersion))
			expectedVersion++
		}

		expectedVersion = actualVersion + 1
	}

	// Check description or checksum mismatch
	query = fmt.Sprintf(`
		SELECT version, description, md5_checksum
		FROM %s
		WHERE success = TRUE;
	`, r.history_table)

	rows, err := r.queriable.QueryContext(r.ctx, query)
	if err != nil {
		return []error{err}
	}
	defer rows.Close()

	historyMap := make(map[uint16]*migrations.Migration)
	for rows.Next() {
		var v uint16
		var d, c string
		if err := rows.Scan(&v, &d, &c); err != nil {
			return []error{err}
		}
		historyMap[v] = &migrations.Migration{
			Version:     v,
			Description: d,
			Checksum:    &c,
		}
	}

	for _, m := range migs {
		if h, ok := historyMap[m.Version]; ok {
			if h.Description != m.Description || *h.Checksum != *m.Checksum {
				errs = append(errs, fmt.Errorf("invalid migration found: version: %d, description: %s, md5_checksum: %s."+
					" Please check your local migration and changes", h.Version, h.Description, *h.Checksum))
			}
		}
	}

	if len(errs) > 0 {
		return errs
	}
	return nil
}

func (r *SnowflakeRepository) ExecuteMigration(migration *migrations.Migration) []error {
	if migration.Type != enums.MIGRATION_UP {
		return []error{fmt.Errorf("invalid migration type: %s", migration.Type.Name())}
	}

	errs := make([]error, 0)

	_, err := r.queriable.ExecContext(r.ctx, *migration.Content)
	if err != nil {
		errs = append(errs, err)
	}

	query := fmt.Sprintf(`
		MERGE INTO %s t
		USING (SELECT ? AS version, ? AS description, ? AS md5_checksum, ? AS success) s
		ON (t.version = s.version)
		WHEN MATCHED THEN
			UPDATE SET t.description = s.description, t.md5_checksum = s.md5_checksum, t.success = s.success, t.executed_at = CURRENT_TIMESTAMP()
		WHEN NOT MATCHED THEN
			INSERT (version, description, md5_checksum, success)
			VALUES (s.version, s.description, s.md5_checksum, s.success);
	`, r.history_table)

	_, dbErr := r.queriable.ExecContext(r.ctx, query, migration.Version, migration.Description,
		migration.Checksum, err == nil)

	if dbErr != nil {
		errs = append(errs, fmt.Errorf("migration %d: %w", migration.Version, dbErr))
	}

	if len(errs) > 0 {
		return errs
	}

	return nil
}

func (r *SnowflakeRepository) ExecuteHook(hook *migrations.Hook) error {
	_, err := r.queriable.ExecContext(r.ctx, *hook.Content)
	if err != nil {
		return err
	}

	return nil
}

func (r *SnowflakeRepository) RollbackMigration(migration *migrations.Migration) error {
	if migration.Type != enums.MIGRATION_DOWN {
		return fmt.Errorf("invalid migration type: %s", migration.Type.Name())
	}

	query := fmt.Sprintf(`
		SELECT COUNT(*) FROM %s WHERE version = ?
	`, r.history_table)

	count := 0
	err := r.queriable.QueryRowContext(r.ctx, query, migration.Version).Scan(&count)
	if err != nil {
		return err
	}

	if count == 0 {
		return nil
	}

	_, err = r.queriable.ExecContext(r.ctx, *migration.Content)
	if err != nil {
		return err
	}

	query = fmt.Sprintf(`
		DELETE FROM %s
		WHERE version = ?
	`, r.history_table)

	res, err := r.queriable.ExecContext(r.ctx, query, migration.Version)
	if err != nil {
		return err
	}

	rowsAffected, err := res.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected < 1 {
		return fmt.Errorf("version was not deleted from \"%s\" table", r.history_table)
	}

	return nil
}

func (r *SnowflakeRepository) DoInTransaction(fn func() error) error {
	// Snowflake doesn't support transactional DDL: each DDL statement
	// implicitly commits any active transaction and executes on its own.
	return fn()
}

func (r *SnowflakeRepository) DoInLock(fn func() error) error {
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
func (r *SnowflakeRepository) lock() error {
	query := `
		SELECT COUNT(*)
		FROM information_schema.tables
		WHERE table_catalog = CURRENT_DATABASE()
		  AND table_schema = CURRENT_SCHEMA()
		  AND table_name = ?
	`

	success := false
	for range 12 {
		count := 0
		err := r.db.QueryRowContext(r.ctx, query, lock_table).Scan(&count)
		if err != nil {
			return err
		}

		if count == 0 {
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

func (r *SnowflakeRepository) unlock() error {
	_, err := r.db.ExecContext(r.ctx, fmt.Sprintf("DROP TABLE IF EXISTS %s;", lock_table))
	if err != nil {
		return err
	}

	return nil
}

func (r *SnowflakeRepository) Repair(migrations []*migrations.Migration) []error {
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
			MERGE INTO %s t
			USING (SELECT ? AS version, ? AS description, ? AS md5_checksum) s
			ON (t.version = s.version)
			WHEN MATCHED THEN
				UPDATE SET
					repaired_at = CASE
						WHEN t.description <> s.description OR t.md5_checksum <> s.md5_checksum
						THEN CURRENT_TIMESTAMP()
						ELSE t.repaired_at
					END,
					t.description = s.description,
					t.md5_checksum = s.md5_checksum,
					t.success = TRUE
			WHEN NOT MATCHED THEN
				INSERT (version, description, md5_checksum, success, repaired_at)
				VALUES (s.version, s.description, s.md5_checksum, TRUE, CURRENT_TIMESTAMP());
		`, r.history_table)

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

func (r *SnowflakeRepository) GetFailingMigrations() ([]*migrations.Migration, error) {
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
		WHERE success = FALSE;
	`, r.history_table)

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
