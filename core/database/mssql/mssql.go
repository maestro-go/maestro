package mssql

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/maestro-go/maestro/core/database"
	"github.com/maestro-go/maestro/core/enums"
	"github.com/maestro-go/maestro/internal/migrations"
)

const default_history_table = "schema_history"
const lock_name = "maestro_migration_lock"

type MSSQLRepository struct {
	database.Repository
	ctx           context.Context
	queriable     database.Queriable
	db            database.Database
	history_table string
}

func NewMSSQLRepository(ctx context.Context, db database.Database, history_table *string) *MSSQLRepository {
	repo := &MSSQLRepository{
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

func (r *MSSQLRepository) GetLatestMigration() (uint16, error) {
	tableExists, err := r.CheckSchemaHistoryTable()
	if err != nil {
		return 0, err
	}

	if !tableExists {
		return 0, nil
	}

	query := fmt.Sprintf(`
		SELECT ISNULL(MAX(version), 0)
		FROM %s
		WHERE success = 1;
	`, r.history_table)

	version := uint16(0)
	err = r.queriable.QueryRowContext(r.ctx, query).Scan(&version)
	if err != nil {
		return 0, err
	}
	return version, nil
}

func (r *MSSQLRepository) AssertSchemaHistoryTable() error {
	exists, err := r.CheckSchemaHistoryTable()
	if err != nil {
		return err
	}

	if exists {
		return nil
	}

	query := fmt.Sprintf(`
		IF NOT EXISTS (SELECT * FROM INFORMATION_SCHEMA.TABLES WHERE TABLE_NAME = '%s' AND TABLE_SCHEMA = SCHEMA_NAME())
		CREATE TABLE %s (
			version SMALLINT NOT NULL PRIMARY KEY,
			description NVARCHAR(255) NOT NULL,
			md5_checksum CHAR(32) NOT NULL,
			success BIT NOT NULL DEFAULT 0,
			executed_at DATETIME2 NOT NULL DEFAULT GETDATE(),
			repaired_at DATETIME2
		);
	`, r.history_table, r.history_table)

	_, err = r.queriable.ExecContext(r.ctx, query)
	if err != nil {
		return err
	}

	return nil
}

func (r *MSSQLRepository) CheckSchemaHistoryTable() (bool, error) {
	query := `
		SELECT COUNT(*)
		FROM INFORMATION_SCHEMA.TABLES
		WHERE TABLE_NAME = @p1
		  AND TABLE_SCHEMA = SCHEMA_NAME()
	`

	count := 0
	err := r.queriable.QueryRowContext(r.ctx, query, r.history_table).Scan(&count)
	if err != nil {
		return false, err
	}

	return count > 0, nil
}

func (r *MSSQLRepository) ValidateMigrations(migrations []*migrations.Migration) []error {
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

	// MSSQL doesn't support the (a, b, c) IN ((1, 2, 3), (4, 5, 6)) syntax directly for all versions or in all contexts easily without table value constructors.
	// We'll use a temporary table or a join with values.
	
	values := make([]string, 0, len(migrations))
	params := make([]any, 0, len(migrations)*3)
	for i, migration := range migrations {
		if migration.Type != enums.MIGRATION_UP {
			return []error{fmt.Errorf("invalid migration type: %s", migration.Type.Name())}
		}
		offset := i * 3
		values = append(values, fmt.Sprintf("(@p%d, @p%d, @p%d)", offset+1, offset+2, offset+3))
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

	// Check description or checksum mismatch using a subquery with VALUES
	query = fmt.Sprintf(`
		SELECT h.version, h.description, h.md5_checksum
		FROM %s h
		WHERE h.success = 1 AND NOT EXISTS (
			SELECT 1 FROM (VALUES %s) AS e(version, description, md5_checksum)
			WHERE h.version = e.version AND h.description = e.description AND h.md5_checksum = e.md5_checksum
		);
	`, r.history_table, strings.Join(values, ", "))

	rows, err := r.queriable.QueryContext(r.ctx, query, params...)
	if err != nil {
		return []error{err}
	}
	defer rows.Close()

	for rows.Next() {
		var v uint16
		var d, c string
		err := rows.Scan(&v, &d, &c)
		if err != nil {
			errs = append(errs, err)
		}

		errs = append(errs, fmt.Errorf("invalid migration found: version: %d, description: %s, md5_checksum: %s."+
			" Please check your local migration and changes", v, d, c))
	}

	if len(errs) > 0 {
		return errs
	}
	return nil
}

func (r *MSSQLRepository) ExecuteMigration(migration *migrations.Migration) []error {
	if migration.Type != enums.MIGRATION_UP {
		return []error{fmt.Errorf("invalid migration type: %s", migration.Type.Name())}
	}

	errs := make([]error, 0)

	_, err := r.queriable.ExecContext(r.ctx, *migration.Content)
	if err != nil {
		errs = append(errs, err)
	}

	query := fmt.Sprintf(`
		MERGE INTO %s AS target
		USING (SELECT @p1 AS version, @p2 AS description, @p3 AS md5_checksum, @p4 AS success) AS source
		ON (target.version = source.version)
		WHEN MATCHED THEN
			UPDATE SET description = source.description, md5_checksum = source.md5_checksum, success = source.success, executed_at = GETDATE()
		WHEN NOT MATCHED THEN
			INSERT (version, description, md5_checksum, success)
			VALUES (source.version, source.description, source.md5_checksum, source.success);
	`, r.history_table)

	success := 0
	if err == nil {
		success = 1
	}

	_, dbErr := r.queriable.ExecContext(r.ctx, query, migration.Version, migration.Description,
		migration.Checksum, success)

	if dbErr != nil {
		errs = append(errs, fmt.Errorf("migration %d: %w", migration.Version, dbErr))
	}

	if len(errs) > 0 {
		return errs
	}

	return nil
}

func (r *MSSQLRepository) ExecuteHook(hook *migrations.Hook) error {
	_, err := r.queriable.ExecContext(r.ctx, *hook.Content)
	if err != nil {
		return err
	}

	return nil
}

func (r *MSSQLRepository) RollbackMigration(migration *migrations.Migration) error {
	if migration.Type != enums.MIGRATION_DOWN {
		return fmt.Errorf("invalid migration type: %s", migration.Type.Name())
	}

	query := fmt.Sprintf(`
		SELECT COUNT(*) FROM %s WHERE version = @p1
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
		WHERE version = @p1;
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

func (r *MSSQLRepository) DoInTransaction(fn func() error) error {
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

func (r *MSSQLRepository) DoInLock(fn func() error) error {
	// sp_getapplock [ @Resource = ] 'resource_name'
	//     , [ @LockMode = ] 'lock_mode'
	//     , [ @LockOwner = ] 'lock_owner'
	//     , [ @LockTimeout = ] 'value'
	
	// LockMode: 'Exclusive'
	// LockOwner: 'Session'
	// LockTimeout: 60000 (60 seconds)

	db, ok := r.db.(*sql.DB)
	if !ok {
		return fmt.Errorf("database is not *sql.DB")
	}

	conn, err := db.Conn(r.ctx)
	if err != nil {
		return fmt.Errorf("failed to get dedicated connection: %w", err)
	}
	defer conn.Close()

	query := "DECLARE @res INT; EXEC @res = sp_getapplock @Resource = @p1, @LockMode = 'Exclusive', @LockOwner = 'Session', @LockTimeout = 60000; SELECT @res"
	var returnCode int
	err = conn.QueryRowContext(r.ctx, query, lock_name).Scan(&returnCode)
	if err != nil {
		return fmt.Errorf("failed to acquire advisory lock: %w", err)
	}
	if returnCode < 0 {
		return fmt.Errorf("failed to acquire advisory lock: return code %d", returnCode)
	}

	defer func() {
		query := "DECLARE @res INT; EXEC @res = sp_releaseapplock @Resource = @p1, @LockOwner = 'Session'; SELECT @res"
		_, err = conn.ExecContext(r.ctx, query, lock_name)
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

func (r *MSSQLRepository) Repair(migrations []*migrations.Migration) []error {
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
			MERGE INTO %s AS target
			USING (SELECT @p1 AS version, @p2 AS description, @p3 AS md5_checksum) AS source
			ON (target.version = source.version)
			WHEN MATCHED THEN
				UPDATE SET 
					repaired_at = CASE
						WHEN target.description <> source.description OR target.md5_checksum <> source.md5_checksum
						THEN GETDATE()
						ELSE target.repaired_at
					END,
					description = source.description,
					md5_checksum = source.md5_checksum,
					success = 1
			WHEN NOT MATCHED THEN
				INSERT (version, description, md5_checksum, success, repaired_at)
				VALUES (source.version, source.description, source.md5_checksum, 1, GETDATE());
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

func (r *MSSQLRepository) GetFailingMigrations() ([]*migrations.Migration, error) {
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
        WHERE success = 0;
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
