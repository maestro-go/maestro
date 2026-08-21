package clickhouse

import (
	"context"
	"fmt"
	"time"

	"github.com/maestro-go/maestro/core/database"
	"github.com/maestro-go/maestro/core/enums"
	"github.com/maestro-go/maestro/internal/migrations"
)

const default_history_table = "schema_history"

type ClickHouseRepository struct {
	database.Repository
	ctx           context.Context
	queriable     database.Queriable
	db            database.Database
	history_table string
}

func NewClickHouseRepository(ctx context.Context, db database.Database, history_table *string) *ClickHouseRepository {
	repo := &ClickHouseRepository{
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

func (r *ClickHouseRepository) GetLatestMigration() (uint16, error) {
	tableExists, err := r.CheckSchemaHistoryTable()
	if err != nil {
		return 0, err
	}

	if !tableExists {
		return 0, nil
	}

	query := fmt.Sprintf(`
		SELECT MAX(version)
		FROM %s FINAL
		WHERE success = 1;
	`, r.history_table)

	version := uint16(0)
	err = r.queriable.QueryRowContext(r.ctx, query).Scan(&version)
	if err != nil {
		// If table is empty, MAX returns 0 or null depending on ClickHouse version and settings
		return 0, nil
	}
	return version, nil
}

func (r *ClickHouseRepository) AssertSchemaHistoryTable() error {
	exists, err := r.CheckSchemaHistoryTable()
	if err != nil {
		return err
	}

	if exists {
		return nil
	}

	query := fmt.Sprintf(`
		CREATE TABLE IF NOT EXISTS %s (
			version UInt16,
			description String,
			md5_checksum String,
			success UInt8,
			executed_at DateTime DEFAULT now(),
			repaired_at Nullable(DateTime)
		) ENGINE = ReplacingMergeTree(executed_at)
		ORDER BY version;
	`, r.history_table)

	_, err = r.queriable.ExecContext(r.ctx, query)
	if err != nil {
		return err
	}

	return nil
}

func (r *ClickHouseRepository) CheckSchemaHistoryTable() (bool, error) {
	query := `
		SELECT count()
		FROM system.tables
		WHERE database = currentDatabase() AND name = ?
	`

	var count uint64
	err := r.queriable.QueryRowContext(r.ctx, query, r.history_table).Scan(&count)
	if err != nil {
		return false, err
	}

	return count > 0, nil
}

func (r *ClickHouseRepository) ValidateMigrations(migrations []*migrations.Migration) []error {
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

	for _, migration := range migrations {
		if migration.Type != enums.MIGRATION_UP {
			return []error{fmt.Errorf("invalid migration type: %s", migration.Type.Name())}
		}
	}

	// Check gaps
	query := fmt.Sprintf(`
		SELECT version FROM %s FINAL ORDER BY version ASC;
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

	query = fmt.Sprintf(`
		SELECT version, description, md5_checksum
		FROM %s FINAL
		WHERE success = 1;
	`, r.history_table)

	rows, err := r.queriable.QueryContext(r.ctx, query)
	if err != nil {
		return []error{err}
	}
	defer rows.Close()

	dbMigrations := make(map[uint16]struct {
		description string
		checksum    string
	})

	for rows.Next() {
		var v uint16
		var d, c string
		if err := rows.Scan(&v, &d, &c); err != nil {
			return []error{err}
		}
		dbMigrations[v] = struct {
			description string
			checksum    string
		}{d, c}
	}

	for _, m := range migrations {
		if dbM, ok := dbMigrations[m.Version]; ok {
			if dbM.description != m.Description || dbM.checksum != *m.Checksum {
				errs = append(errs, fmt.Errorf("invalid migration found: version: %d, description: %s, md5_checksum: %s."+
					" Please check your local migration and changes", m.Version, dbM.description, dbM.checksum))
			}
		}
	}

	if len(errs) > 0 {
		return errs
	}
	return nil
}

func (r *ClickHouseRepository) ExecuteMigration(migration *migrations.Migration) []error {
	if migration.Type != enums.MIGRATION_UP {
		return []error{fmt.Errorf("invalid migration type: %s", migration.Type.Name())}
	}

	errs := make([]error, 0)

	_, err := r.queriable.ExecContext(r.ctx, *migration.Content)
	if err != nil {
		errs = append(errs, err)
	}

	success := uint8(1)
	if err != nil {
		success = 0
	}

	query := fmt.Sprintf(`
		INSERT INTO %s (version, description, md5_checksum, success, executed_at)
		VALUES (?, ?, ?, ?, now());
	`, r.history_table)

	_, insertErr := r.queriable.ExecContext(r.ctx, query, migration.Version, migration.Description,
		migration.Checksum, success)

	if insertErr != nil {
		errs = append(errs, fmt.Errorf("migration %d: %w", migration.Version, insertErr))
	}

	if len(errs) > 0 {
		return errs
	}

	return nil
}

func (r *ClickHouseRepository) ExecuteHook(hook *migrations.Hook) error {
	_, err := r.queriable.ExecContext(r.ctx, *hook.Content)
	if err != nil {
		return err
	}

	return nil
}

func (r *ClickHouseRepository) RollbackMigration(migration *migrations.Migration) error {
	if migration.Type != enums.MIGRATION_DOWN {
		return fmt.Errorf("invalid migration type: %s", migration.Type.Name())
	}

	query := fmt.Sprintf(`
		SELECT count() FROM %s FINAL WHERE version = ?;
	`, r.history_table)

	var count uint64
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
		ALTER TABLE %s DELETE WHERE version = ?;
	`, r.history_table)

	_, err = r.queriable.ExecContext(r.ctx, query, migration.Version)
	if err != nil {
		return err
	}

	return nil
}

func (r *ClickHouseRepository) DoInTransaction(fn func() error) error {
	// ClickHouse doesn't support transactions in the standard way.
	return fn()
}

func (r *ClickHouseRepository) DoInLock(fn func() error) error {
	lockTable := r.history_table + "_lock"
	maxRetries := 60
	retryInterval := 1 * time.Second
	lockTimeout := 10 * time.Minute

	for i := 0; i < maxRetries; i++ {
		query := fmt.Sprintf(`
			CREATE TABLE %s
			ENGINE = Memory
			AS SELECT now() AS created_at, 'maestro' AS owner;
		`, lockTable)

		_, err := r.db.ExecContext(r.ctx, query)
		if err == nil {
			defer func() {
				_, _ = r.db.ExecContext(r.ctx, fmt.Sprintf("DROP TABLE IF EXISTS %s", lockTable))
			}()
			return fn()
		}

		var createdAt time.Time
		checkQuery := fmt.Sprintf("SELECT created_at FROM %s LIMIT 1", lockTable)
		err = r.db.QueryRowContext(r.ctx, checkQuery).Scan(&createdAt)
		if err == nil {
			if time.Since(createdAt) > lockTimeout {
				// Lock is stale, try to drop it.
				_, _ = r.db.ExecContext(r.ctx, fmt.Sprintf("DROP TABLE IF EXISTS %s", lockTable))
				continue
			}
		}

		select {
		case <-r.ctx.Done():
			return r.ctx.Err()
		case <-time.After(retryInterval):
		}
	}

	return fmt.Errorf("failed to acquire ClickHouse migration lock on table %s after %d retries", lockTable, maxRetries)
}

func (r *ClickHouseRepository) Repair(migrations []*migrations.Migration) []error {
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
			INSERT INTO %s (version, description, md5_checksum, success, repaired_at, executed_at)
			VALUES (?, ?, ?, 1, now(), now());
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

func (r *ClickHouseRepository) GetFailingMigrations() ([]*migrations.Migration, error) {
	exists, err := r.CheckSchemaHistoryTable()
	if err != nil {
		return nil, err
	}

	if !exists {
		return nil, nil
	}

	query := fmt.Sprintf(`
        SELECT version, description, md5_checksum
        FROM %s FINAL
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
