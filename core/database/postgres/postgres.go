package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/maestro-go/maestro/core/database"
	"github.com/maestro-go/maestro/core/enums"
	"github.com/maestro-go/maestro/internal/migrations"
)

const schema_history_table = "schema_history"
const lock_num = 5691374

type PostgresRepository struct {
	database.Repository
	ctx       context.Context
	queriable database.Queriable
	db        database.Database
}

func NewPostgresRepository(ctx context.Context, db database.Database) *PostgresRepository {
	return &PostgresRepository{
		ctx:       ctx,
		queriable: db,
		db:        db,
	}
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
	`, schema_history_table)

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
	`, schema_history_table)

	_, err = r.queriable.ExecContext(r.ctx, query)
	if err != nil {
		return err
	}

	return nil
}

func (r *PostgresRepository) CheckSchemaHistoryTable() (bool, error) {
	query := `
		SELECT EXISTS (
			SELECT FROM information_schema.tables
			WHERE table_name = $1
		);
	`

	exists := false
	err := r.queriable.QueryRowContext(r.ctx, query, schema_history_table).Scan(&exists)
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
	`, schema_history_table)

	_, err = r.queriable.ExecContext(r.ctx, query, migration.Version, migration.Description,
		migration.Checksum, err == nil)

	if err != nil {
		errs = append(errs, err)
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

func (r *PostgresRepository) RollbackMigration(migration *migrations.Migration) []error {
	if migration.Type != enums.MIGRATION_DOWN {
		return []error{fmt.Errorf("invalid migration type: %s", migration.Type.Name())}
	}

	query := fmt.Sprintf(`
		SELECT EXISTS (
			SELECT version FROM %s WHERE version = $1
		);
	`, schema_history_table)

	exists := false
	err := r.queriable.QueryRowContext(r.ctx, query, migration.Version).Scan(&exists)
	if err != nil {
		return []error{err}
	}

	if !exists {
		return nil
	}

	errs := make([]error, 0)

	_, err = r.queriable.ExecContext(r.ctx, *migration.Content)
	if err != nil {
		errs = append(errs, err)
	}

	query = fmt.Sprintf(`
		DELETE FROM %s
		WHERE version = $1;
	`, schema_history_table)

	res, err := r.queriable.ExecContext(r.ctx, query, migration.Version)
	if err != nil {
		errs = append(errs, err)
		return errs
	}

	rowsAffected, err := res.RowsAffected()
	if err != nil {
		errs = append(errs, err)
		return errs
	}

	if rowsAffected < 1 {
		errs = append(errs, fmt.Errorf("version was not deleted from \"%s\" table", schema_history_table))
	}

	if len(errs) > 0 {
		return errs
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
            SELECT version, description, md5_checksum
            FROM %s
            WHERE version = $1;
        `, schema_history_table)

		var storedVersion uint16
		var storedDescription, storedChecksum string

		err := r.queriable.QueryRowContext(r.ctx, query, migration.Version).Scan(&storedVersion, &storedDescription, &storedChecksum)
		if err != nil {
			if err == sql.ErrNoRows {
				// Upsert the migration if it does not exist
				insertQuery := fmt.Sprintf(`
                    INSERT INTO %s (version, description, md5_checksum, success, repaired_at)
                    VALUES ($1, $2, $3, true, NOW())
                    ON CONFLICT (version) DO UPDATE
                    SET description = $2, md5_checksum = $3, repaired_at = NOW();
                `, schema_history_table)

				_, err := r.queriable.ExecContext(r.ctx, insertQuery, migration.Version, migration.Description, *migration.Checksum)
				if err != nil {
					errs = append(errs, err)
				}
			} else {
				errs = append(errs, err)
			}
			continue
		}

		if storedDescription != migration.Description || storedChecksum != *migration.Checksum {
			updateQuery := fmt.Sprintf(`
                UPDATE %s
                SET description = $2, md5_checksum = $3, repaired_at = NOW()
                WHERE version = $1;
            `, schema_history_table)

			_, err := r.queriable.ExecContext(r.ctx, updateQuery, migration.Version, migration.Description, *migration.Checksum)
			if err != nil {
				errs = append(errs, err)
			}
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
