package oracle

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/maestro-go/maestro/core/database"
	"github.com/maestro-go/maestro/core/enums"
	"github.com/maestro-go/maestro/internal/migrations"
)

const default_history_table = "SCHEMA_HISTORY"

type OracleRepository struct {
	database.Repository
	ctx           context.Context
	queriable     database.Queriable
	db            database.Database
	history_table string
}

func NewOracleRepository(ctx context.Context, db database.Database, history_table *string) *OracleRepository {
	repo := &OracleRepository{
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

func (r *OracleRepository) GetLatestMigration() (uint16, error) {
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
		WHERE success = 1
	`, r.history_table)

	version := uint16(0)
	err = r.queriable.QueryRowContext(r.ctx, query).Scan(&version)
	if err != nil {
		return 0, err
	}
	return version, nil
}

func (r *OracleRepository) AssertSchemaHistoryTable() error {
	exists, err := r.CheckSchemaHistoryTable()
	if err != nil {
		return err
	}

	if exists {
		return nil
	}

	query := fmt.Sprintf(`
		CREATE TABLE %s (
			version NUMBER(5) NOT NULL PRIMARY KEY,
			description VARCHAR2(255) NOT NULL,
			md5_checksum CHAR(32) NOT NULL,
			success NUMBER(1) DEFAULT 0 NOT NULL,
			executed_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP NOT NULL,
			repaired_at TIMESTAMP
		)
	`, r.history_table)

	_, err = r.queriable.ExecContext(r.ctx, query)
	if err != nil {
		return err
	}

	return nil
}

func (r *OracleRepository) CheckSchemaHistoryTable() (bool, error) {
	query := `
		SELECT COUNT(*)
		FROM user_tables
		WHERE table_name = :1
	`

	count := 0
	err := r.queriable.QueryRowContext(r.ctx, query, r.history_table).Scan(&count)
	if err != nil {
		return false, err
	}

	return count > 0, nil
}

func (r *OracleRepository) ValidateMigrations(migs []*migrations.Migration) []error {
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
		SELECT version FROM %s ORDER BY version ASC
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

	query = fmt.Sprintf(`
		SELECT version, description, md5_checksum
		FROM %s
		WHERE success = 1
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

func (r *OracleRepository) ExecuteMigration(migration *migrations.Migration) []error {
	if migration.Type != enums.MIGRATION_UP {
		return []error{fmt.Errorf("invalid migration type: %s", migration.Type.Name())}
	}

	errs := make([]error, 0)

	_, err := r.queriable.ExecContext(r.ctx, *migration.Content)
	if err != nil {
		errs = append(errs, err)
	}

	success := 0
	if err == nil {
		success = 1
	}

	query := fmt.Sprintf(`
		MERGE INTO %s t
		USING (SELECT :1 AS version, :2 AS description, :3 AS md5_checksum, :4 AS success FROM dual) s
		ON (t.version = s.version)
		WHEN MATCHED THEN
			UPDATE SET t.description = s.description, t.md5_checksum = s.md5_checksum, t.success = s.success, t.executed_at = CURRENT_TIMESTAMP
		WHEN NOT MATCHED THEN
			INSERT (version, description, md5_checksum, success)
			VALUES (s.version, s.description, s.md5_checksum, s.success)
	`, r.history_table)

	_, dbErr := r.queriable.ExecContext(r.ctx, query, migration.Version, migration.Description,
		migration.Checksum, success)

	if dbErr != nil {
		errs = append(errs, fmt.Errorf("migration %d history update failed: %w", migration.Version, dbErr))
	}

	if len(errs) > 0 {
		return errs
	}

	return nil
}

func (r *OracleRepository) ExecuteHook(hook *migrations.Hook) error {
	_, err := r.queriable.ExecContext(r.ctx, *hook.Content)
	if err != nil {
		return err
	}

	return nil
}

func (r *OracleRepository) RollbackMigration(migration *migrations.Migration) error {
	if migration.Type != enums.MIGRATION_DOWN {
		return fmt.Errorf("invalid migration type: %s", migration.Type.Name())
	}

	query := fmt.Sprintf(`
		SELECT COUNT(*) FROM %s WHERE version = :1
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
		WHERE version = :1
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

func (r *OracleRepository) DoInTransaction(fn func() error) error {
	tx, err := r.db.BeginTx(r.ctx, nil)
	if err != nil {
		return err
	}
	defer func() {
		_ = tx.Rollback()
		r.queriable = r.db // Always reset queriable to db
	}()

	r.queriable = tx

	err = fn()
	if err != nil {
		return err
	}

	return tx.Commit()
}

func (r *OracleRepository) DoInLock(fn func() error) error {
	lockTable := "MAESTRO_LOCK"

	// Ensure lock table exists
	checkQuery := "SELECT COUNT(*) FROM user_tables WHERE table_name = :1"
	var count int
	err := r.db.QueryRowContext(r.ctx, checkQuery, lockTable).Scan(&count)
	if err != nil {
		return fmt.Errorf("failed to check lock table: %w", err)
	}

	if count == 0 {
		_, err = r.db.ExecContext(r.ctx, fmt.Sprintf("CREATE TABLE %s (id NUMBER PRIMARY KEY)", lockTable))
		if err != nil {
			// Ignore error if someone else created it simultaneously
			if !strings.Contains(err.Error(), "ORA-00955") {
				return fmt.Errorf("failed to create lock table: %w", err)
			}
		}
		_, _ = r.db.ExecContext(r.ctx, fmt.Sprintf("INSERT INTO %s (id) VALUES (1)", lockTable))
	}

	// We need a dedicated connection for the lock to ensure it's held throughout the function
	conn, err := r.db.(*sql.DB).Conn(r.ctx)
	if err != nil {
		return fmt.Errorf("failed to get connection for lock: %w", err)
	}
	defer conn.Close()

	// Start a transaction on this connection for the lock
	tx, err := conn.BeginTx(r.ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to start transaction for lock: %w", err)
	}
	defer tx.Rollback()

	// Acquire lock
	_, err = tx.ExecContext(r.ctx, fmt.Sprintf("SELECT id FROM %s WHERE id = 1 FOR UPDATE", lockTable))
	if err != nil {
		return fmt.Errorf("failed to acquire lock: %w", err)
	}

	err = fn()
	if err != nil {
		return err
	}

	return tx.Commit()
}

func (r *OracleRepository) Repair(migs []*migrations.Migration) []error {
	tableExists, err := r.CheckSchemaHistoryTable()
	if err != nil {
		return []error{err}
	}

	if !tableExists {
		return nil
	}

	errs := make([]error, 0)

	for _, migration := range migs {
		query := fmt.Sprintf(`
			MERGE INTO %s t
			USING (SELECT :1 AS version, :2 AS description, :3 AS md5_checksum FROM dual) s
			ON (t.version = s.version)
			WHEN MATCHED THEN
				UPDATE SET 
					repaired_at = CASE
						WHEN t.description <> s.description OR t.md5_checksum <> s.md5_checksum
						THEN CURRENT_TIMESTAMP
						ELSE t.repaired_at
					END,
					t.description = s.description,
					t.md5_checksum = s.md5_checksum,
					t.success = 1
			WHEN NOT MATCHED THEN
				INSERT (version, description, md5_checksum, success, repaired_at)
				VALUES (s.version, s.description, s.md5_checksum, 1, CURRENT_TIMESTAMP)
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

func (r *OracleRepository) GetFailingMigrations() ([]*migrations.Migration, error) {
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
        WHERE success = 0
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
