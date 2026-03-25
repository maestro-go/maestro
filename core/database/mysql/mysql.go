package mysql

import (
	"context"
	"fmt"
	"strings"

	"github.com/maestro-go/maestro/core/database"
	"github.com/maestro-go/maestro/core/enums"
	"github.com/maestro-go/maestro/internal/migrations"
)

const default_history_table = "schema_history"
const lock_name = "maestro_migration_lock"

type MySQLRepository struct {
	database.Repository
	ctx           context.Context
	queriable     database.Queriable
	db            database.Database
	history_table string
}

func NewMySQLRepository(ctx context.Context, db database.Database, history_table *string) *MySQLRepository {
	repo := &MySQLRepository{
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

func (r *MySQLRepository) GetLatestMigration() (uint16, error) {
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

func (r *MySQLRepository) AssertSchemaHistoryTable() error {
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
			executed_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
			repaired_at TIMESTAMP NULL
		);
	`, r.history_table)

	_, err = r.queriable.ExecContext(r.ctx, query)
	if err != nil {
		return err
	}

	return nil
}

func (r *MySQLRepository) CheckSchemaHistoryTable() (bool, error) {
	query := `
		SELECT COUNT(*)
		FROM information_schema.tables
		WHERE table_schema = DATABASE() AND table_name = ?
	`

	count := 0
	err := r.queriable.QueryRowContext(r.ctx, query, r.history_table).Scan(&count)
	if err != nil {
		return false, err
	}

	return count > 0, nil
}

func (r *MySQLRepository) ValidateMigrations(migrations []*migrations.Migration) []error {
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
	for _, migration := range migrations {

		if migration.Type != enums.MIGRATION_UP {
			return []error{fmt.Errorf("invalid migration type: %s", migration.Type.Name())}
		}

		tuples = append(tuples, "(?, ?, ?)")
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

func (r *MySQLRepository) ExecuteMigration(migration *migrations.Migration) []error {
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
		VALUES (?, ?, ?, ?)
		ON DUPLICATE KEY UPDATE 
            description = VALUES(description), 
            md5_checksum = VALUES(md5_checksum), 
            success = VALUES(success), 
            executed_at = CURRENT_TIMESTAMP;
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

func (r *MySQLRepository) ExecuteHook(hook *migrations.Hook) error {
	_, err := r.queriable.ExecContext(r.ctx, *hook.Content)
	if err != nil {
		return err
	}

	return nil
}

func (r *MySQLRepository) RollbackMigration(migration *migrations.Migration) error {
	if migration.Type != enums.MIGRATION_DOWN {
		return fmt.Errorf("invalid migration type: %s", migration.Type.Name())
	}

	query := fmt.Sprintf(`
		SELECT EXISTS (
			SELECT version FROM %s WHERE version = ?
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
		WHERE version = ?;
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

func (r *MySQLRepository) DoInTransaction(fn func() error) error {
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

func (r *MySQLRepository) DoInLock(fn func() error) error {
	var lockResult int
	query := "SELECT GET_LOCK(?, 60)"
	err := r.db.QueryRowContext(r.ctx, query, lock_name).Scan(&lockResult)
	if err != nil {
		return fmt.Errorf("failed to acquire advisory lock: %w", err)
	}
	if lockResult != 1 {
		return fmt.Errorf("failed to acquire advisory lock: timeout or error")
	}

	defer func() {
		query := "SELECT RELEASE_LOCK(?)"
		_, err = r.db.ExecContext(r.ctx, query, lock_name)
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

func (r *MySQLRepository) Repair(migrations []*migrations.Migration) []error {
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
			VALUES (?, ?, ?, true, CURRENT_TIMESTAMP)
			ON DUPLICATE KEY UPDATE
			repaired_at = CASE
				WHEN description <> VALUES(description) OR md5_checksum <> VALUES(md5_checksum)
				THEN CURRENT_TIMESTAMP
				ELSE repaired_at
			END,
			description = VALUES(description),
			md5_checksum = VALUES(md5_checksum),
			success = true;
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

func (r *MySQLRepository) GetFailingMigrations() ([]*migrations.Migration, error) {
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
