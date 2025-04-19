package postgres

import (
	"context"
	"fmt"
	"strings"

	"github.com/maestro-go/maestro/core/database"
	"github.com/maestro-go/maestro/core/enums"
	"github.com/maestro-go/maestro/internal/migrations"
)

const default_history_table = "schema_history"
const lock_num = 5691374

type PostgresRepository struct {
	database.Repository
	ctx           context.Context
	queriable     database.Queriable
	db            database.Database
	history_table string
}

func NewPostgresRepository(ctx context.Context, db database.Database, history_table *string) *PostgresRepository {
	repo := &PostgresRepository{
		ctx:       ctx,
		queriable: db,
		db:        db,
	}

	if history_table != nil {
		repo.history_table = *history_table
	} else {
		repo.history_table = default_history_table
	}

	return repo
}

func (r *PostgresRepository) GetLatestMigration() (uint16, error) {
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
	`, r.history_table)

	version := uint16(0)
	err = r.queriable.QueryRowContext(r.ctx, query).Scan(&version)
	if err != nil {
		return 0, err
	}
	return version, nil
}

func (r *PostgresRepository) AssertSchemaHistoryTable() error {
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
	`, r.history_table)

	_, err = r.queriable.ExecContext(r.ctx, query)
	if err != nil {
		return err
	}

	return nil
}

func (r *PostgresRepository) CheckSchemaHistoryTable() (bool, error) {
	query := `
		SELECT EXISTS (
			SELECT 1 FROM pg_tables
			WHERE tablename = $1 AND schemaname = current_schema()
		);
	`

	exists := false
	err := r.queriable.QueryRowContext(r.ctx, query, r.history_table).Scan(&exists)
	if err != nil {
		return false, err
	}

	return exists, nil
}

func (r *PostgresRepository) ValidateMigrations(migrations []*migrations.Migration) []error {
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

	tuples := make([]string, 0, len(migrations))
	params := make([]any, 0, len(migrations)*3)
	for i, migration := range migrations {

		if migration.Type != enums.MIGRATION_UP {
			return []error{fmt.Errorf("invalid migration type: %s", migration.Type.Name())}
		}

		offset := i * 3
		tuples = append(tuples, fmt.Sprintf("($%d, $%d, $%d)", offset+1, offset+2, offset+3))
		params = append(params, migration.Version, migration.Description, *migration.Checksum)
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
	`, r.history_table, strings.Join(tuples, ", "))

	rows, err := r.queriable.QueryContext(r.ctx, query, params...)
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
			" Please check your local migration and changes", res.version, res.description, res.md5_checksum))
	}

	if len(errs) > 0 {
		return errs
	}
	return nil
}

func (r *PostgresRepository) ExecuteMigration(migration *migrations.Migration) []error {
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
	`, r.history_table)

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

func (r *PostgresRepository) ExecuteHook(hook *migrations.Hook) error {
	_, err := r.queriable.ExecContext(r.ctx, *hook.Content)
	if err != nil {
		return err
	}

	return nil
}

func (r *PostgresRepository) RollbackMigration(migration *migrations.Migration) error {
	if migration.Type != enums.MIGRATION_DOWN {
		return fmt.Errorf("invalid migration type: %s", migration.Type.Name())
	}

	query := fmt.Sprintf(`
		SELECT EXISTS (
			SELECT version FROM %s WHERE version = $1
		);
	`, r.history_table)

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

func (r *PostgresRepository) DoInTransaction(fn func() error) error {
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

func (r *PostgresRepository) DoInLock(fn func() error) error {
	_, err := r.db.ExecContext(r.ctx, "select pg_advisory_lock($1)", lock_num)
	if err != nil {
		return fmt.Errorf("failed to acquire advisory lock: %w", err)
	}
	defer func() {
		_, err = r.db.ExecContext(r.ctx, "select pg_advisory_unlock($1)", lock_num)
		if err != nil {
			panic(fmt.Errorf("failed to release advisory lock: %w", err))
		}
	}()

	err = fn()
	if err != nil {
		return err
	}

	return nil
}

func (r *PostgresRepository) Repair(migrations []*migrations.Migration) []error {
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
			SET description = EXCLUDED.description, md5_checksum = EXCLUDED.md5_checksum, success = true,
				repaired_at = CASE
					WHEN EXCLUDED.description <> %s.description OR EXCLUDED.md5_checksum <> %s.md5_checksum
					THEN NOW()
					ELSE %s.repaired_at
				END;
		`, r.history_table, r.history_table, r.history_table, r.history_table)

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

func (r *PostgresRepository) GetFailingMigrations() ([]*migrations.Migration, error) {
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
    `, r.history_table)

	rows, err := r.queriable.QueryContext(r.ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var failingMigrations []*migrations.Migration
	for rows.Next() {
		var migration migrations.Migration
		if err := rows.Scan(&migration.Version, &migration.Description, &migration.Checksum); err != nil {
			return nil, err
		}
		failingMigrations = append(failingMigrations, &migration)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return failingMigrations, nil
}
