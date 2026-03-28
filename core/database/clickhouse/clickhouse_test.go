package clickhouse

import (
	"context"
	"database/sql"
	"fmt"
	"testing"
	"time"

	"github.com/maestro-go/maestro/core/enums"
	"github.com/maestro-go/maestro/internal/migrations"
	testUtils "github.com/maestro-go/maestro/internal/utils/testing"
	"github.com/stretchr/testify/suite"

	_ "github.com/ClickHouse/clickhouse-go/v2"
)

type ClickHouseTestSuite struct {
	suite.Suite
	clickhouse *testUtils.ClickHouseContainer
	suiteDb    *sql.DB

	ctx context.Context

	repository *ClickHouseRepository
}

func (s *ClickHouseTestSuite) SetupSuite() {
	s.ctx = context.Background()

	s.clickhouse = testUtils.SetupClickHouse(s.T())

	db, err := sql.Open("clickhouse", s.clickhouse.URI)
	s.Assert().NoError(err)

	s.suiteDb = db

	s.repository = NewClickHouseRepository(s.ctx, db, testUtils.ToPtr(default_history_table))
}

func (s *ClickHouseTestSuite) TearDownTest() {
	if s.clickhouse != nil {
		// Drop all tables
		rows, err := s.suiteDb.Query("SHOW TABLES")
		if err == nil {
			defer rows.Close()
			for rows.Next() {
				var tableName string
				if err := rows.Scan(&tableName); err == nil {
					_, _ = s.suiteDb.Exec(fmt.Sprintf("DROP TABLE IF EXISTS %s", tableName))
				}
			}
		}
	}
}

func (s *ClickHouseTestSuite) checkTableExists(table string, shouldExist bool) {
	s.T().Helper()

	query := `
		SELECT count()
		FROM system.tables
		WHERE database = currentDatabase() AND name = ?
	`

	var count uint64
	err := s.suiteDb.QueryRowContext(s.ctx, query, table).Scan(&count)
	s.Assert().NoError(err)
	s.Assert().Equal(shouldExist, count > 0)
}

func TestClickHouseSuite(t *testing.T) {
	suite.Run(t, new(ClickHouseTestSuite))
}

func (s *ClickHouseTestSuite) TestNewClickHouseRepository() {
	repo := NewClickHouseRepository(s.ctx, s.suiteDb, nil)
	s.Assert().Equal(default_history_table, repo.history_table)
}

func (s *ClickHouseTestSuite) TestAssertSchemaHistoryTable() {
	err := s.repository.AssertSchemaHistoryTable()
	s.Assert().NoError(err)

	s.checkTableExists(default_history_table, true)

	// Test already exists
	err = s.repository.AssertSchemaHistoryTable()
	s.Assert().NoError(err)
}

func (s *ClickHouseTestSuite) TestCheckSchemaHistoryTable() {
	tableExists, err := s.repository.CheckSchemaHistoryTable()
	s.Assert().NoError(err)
	s.Assert().False(tableExists)

	err = s.repository.AssertSchemaHistoryTable()
	s.Assert().NoError(err)

	tableExists, err = s.repository.CheckSchemaHistoryTable()
	s.Assert().NoError(err)
	s.Assert().True(tableExists)
}

func (s *ClickHouseTestSuite) TestGetLatestMigration() {
	// No table
	version, err := s.repository.GetLatestMigration()
	s.Assert().NoError(err)
	s.Assert().Equal(uint16(0), version)

	err = s.repository.AssertSchemaHistoryTable()
	s.Assert().NoError(err)

	// Empty table
	version, err = s.repository.GetLatestMigration()
	s.Assert().NoError(err)
	s.Assert().Equal(uint16(0), version)

	query := fmt.Sprintf(`
		INSERT INTO %s (version, description, md5_checksum, success) VALUES
			(1, 't', '0a52730597fb4ffa01fc117d9e71e3a9', 1),
			(5, 't', '0a52730597fb4ffa01fc117d9e71e3a9', 1),
			(7, 't', '0a52730597fb4ffa01fc117d9e71e3a9', 0);
	`, default_history_table)

	_, err = s.suiteDb.Exec(query)
	s.Assert().NoError(err)

	version, err = s.repository.GetLatestMigration()
	s.Assert().NoError(err)
	s.Assert().Equal(uint16(5), version)
}

func (s *ClickHouseTestSuite) TestValidateMigrations() {
	// Empty list
	errs := s.repository.ValidateMigrations(nil)
	s.Assert().Nil(errs)

	checksums := []string{"0a52730597fb4ffa01fc117d9e71e3a9", "3d41c8443df34e73867adb149efbb2ea"}
	contents := []string{"EXAMPLE CONTENT 1", "EXAMPLE CONTENT 2"}
	migrationsList := []*migrations.Migration{
		{
			Version:     1,
			Description: "abcd",
			Type:        enums.MIGRATION_UP,
			Checksum:    &checksums[0],
			Content:     &contents[0],
		},
		{
			Version:     2,
			Description: "abcd",
			Type:        enums.MIGRATION_UP,
			Checksum:    &checksums[1],
			Content:     &contents[1],
		},
	}

	// No table
	errs = s.repository.ValidateMigrations(migrationsList)
	s.Assert().Nil(errs)

	err := s.repository.AssertSchemaHistoryTable()
	s.Assert().NoError(err)

	// Invalid migration type
	migrationsList[0].Type = enums.MIGRATION_DOWN
	errs = s.repository.ValidateMigrations(migrationsList)
	s.Assert().Len(errs, 1)
	s.Assert().Contains(errs[0].Error(), "invalid migration type")
	migrationsList[0].Type = enums.MIGRATION_UP

	errs = s.repository.ValidateMigrations(migrationsList)
	s.Assert().Nil(errs)

	query := fmt.Sprintf(`
		INSERT INTO %s (version, description, md5_checksum, success) VALUES
			(?, ?, ?, 1);
	`, default_history_table)

	_, err = s.suiteDb.ExecContext(s.ctx, query, migrationsList[1].Version,
		migrationsList[1].Description, *migrationsList[1].Checksum)
	s.Assert().NoError(err)

	// Gap check: DB has version 2, but not version 1
	errs = s.repository.ValidateMigrations(migrationsList)
	s.Assert().Len(errs, 1)
	s.Assert().Contains(errs[0].Error(), "missing version 1")

	_, err = s.suiteDb.ExecContext(s.ctx, query, migrationsList[0].Version,
		migrationsList[0].Description, *migrationsList[0].Checksum)
	s.Assert().NoError(err)

	errs = s.repository.ValidateMigrations(migrationsList)
	s.Assert().Nil(errs)

	// ClickHouse ReplacingMergeTree: insert new row with same version to "update"
	_, err = s.suiteDb.ExecContext(s.ctx, `
		INSERT INTO schema_history (version, description, md5_checksum, success, executed_at)
		VALUES (?, ?, ?, 1, now() + INTERVAL 1 MINUTE);
	`, migrationsList[1].Version, migrationsList[1].Description, checksums[0])
	s.Assert().NoError(err)

	errs = s.repository.ValidateMigrations(migrationsList)
	s.Assert().Len(errs, 1)
}

func (s *ClickHouseTestSuite) TestExecuteMigration() {
	checksum := "0a52730597fb4ffa01fc117d9e71e3a9"
	content := "INVALID SQL"
	migration := &migrations.Migration{
		Version:     1,
		Description: "abcd",
		Type:        enums.MIGRATION_UP,
		Checksum:    &checksum,
		Content:     &content,
	}

	// Invalid type
	migration.Type = enums.MIGRATION_DOWN
	errs := s.repository.ExecuteMigration(migration)
	s.Assert().Len(errs, 1)
	s.Assert().Contains(errs[0].Error(), "invalid migration type")
	migration.Type = enums.MIGRATION_UP

	// Invalid SQL
	errs = s.repository.ExecuteMigration(migration)
	s.Assert().Len(errs, 2)

	*migration.Content = "CREATE TABLE test (id UInt32) ENGINE = Memory;"

	// No schema table
	errs = s.repository.ExecuteMigration(migration)
	s.Assert().Len(errs, 1)

	err := s.repository.AssertSchemaHistoryTable()
	s.Assert().NoError(err)

	*migration.Content = "CREATE TABLE test2 (id UInt32) ENGINE = Memory;"

	errs = s.repository.ExecuteMigration(migration)
	s.Assert().Nil(errs)

	s.checkTableExists("test2", true)

	query := fmt.Sprintf(`SELECT version, description, md5_checksum FROM %s FINAL;`, default_history_table)
	version := uint16(0)
	description := ""
	md5Checksum := ""
	err = s.suiteDb.QueryRowContext(s.ctx, query).Scan(&version, &description, &md5Checksum)
	s.Assert().NoError(err)
	s.Assert().Equal(migration.Version, version)
	s.Assert().Equal(migration.Description, description)
	s.Assert().Equal(*migration.Checksum, md5Checksum)
}

func (s *ClickHouseTestSuite) TestExecuteHook() {
	content := "INVALID SQL"
	hook := &migrations.Hook{
		Order:   1,
		Content: &content,
		Type:    enums.HOOK_AFTER_EACH,
	}

	err := s.repository.ExecuteHook(hook)
	s.Assert().Error(err)

	*hook.Content = "CREATE TABLE test3 (id UInt32) ENGINE = Memory;"

	err = s.repository.ExecuteHook(hook)
	s.Assert().NoError(err)

	s.checkTableExists("test3", true)
}

func (s *ClickHouseTestSuite) TestRollbackMigration() {
	content := "INVALID SQL"
	migration := &migrations.Migration{
		Version:     1,
		Description: "abcd",
		Type:        enums.MIGRATION_DOWN,
		Content:     &content,
	}

	// Invalid type
	migration.Type = enums.MIGRATION_UP
	err := s.repository.RollbackMigration(migration)
	s.Assert().Error(err)
	s.Assert().Contains(err.Error(), "invalid migration type")
	migration.Type = enums.MIGRATION_DOWN

	err = s.repository.RollbackMigration(migration)
	s.Assert().Error(err)

	err = s.repository.AssertSchemaHistoryTable()
	s.Assert().NoError(err)

	// Version doesn't exist
	err = s.repository.RollbackMigration(migration)
	s.Assert().NoError(err)

	_, err = s.suiteDb.ExecContext(s.ctx, "CREATE TABLE test4 (id UInt32) ENGINE = Memory;")
	s.Assert().NoError(err)
	_, err = s.suiteDb.ExecContext(s.ctx, fmt.Sprintf(`
		INSERT INTO %s (version, description, md5_checksum, success)
		VALUES (1, 'abcd', '0a52730597fb4ffa01fc117d9e71e3a9', 1);
	`, default_history_table))
	s.Assert().NoError(err)

	s.checkTableExists("test4", true)

	*migration.Content = "DROP TABLE test4;"
	err = s.repository.RollbackMigration(migration)
	s.Assert().NoError(err)

	s.checkTableExists("test4", false)
}

func (s *ClickHouseTestSuite) TestDoInTransaction() {
	// ClickHouse doesn't support transactions, but we should verify it doesn't break
	content := "CREATE TABLE test1 (id UInt32) ENGINE = Memory;"
	checksum := "0a52730597fb4ffa01fc117d9e71e3a9"
	migration := &migrations.Migration{
		Version:     1,
		Description: "abcd",
		Type:        enums.MIGRATION_UP,
		Checksum:    &checksum,
		Content:     &content,
	}

	err := s.repository.AssertSchemaHistoryTable()
	s.Assert().NoError(err)

	// Since there are no transactions, it will NOT roll back
	err = s.repository.DoInTransaction(func() error {
		errs := s.repository.ExecuteMigration(migration)
		s.Assert().Nil(errs)

		return fmt.Errorf("example error")
	})
	s.Assert().Error(err)
	s.checkTableExists("test1", true) 
}

func (s *ClickHouseTestSuite) TestDoInLock() {
	err := s.repository.AssertSchemaHistoryTable()
	s.Assert().NoError(err)

	// Open another session
	db2, err := sql.Open("clickhouse", s.clickhouse.URI)
	s.Assert().NoError(err)
	defer db2.Close()

	repo2 := NewClickHouseRepository(s.ctx, db2, testUtils.ToPtr(default_history_table))

	// Acquire lock in first session
	lockAcquired := make(chan bool)
	releaseLock := make(chan bool)
	errChan := make(chan error)

	go func() {
		err := s.repository.DoInLock(func() error {
			lockAcquired <- true
			<-releaseLock
			return nil
		})
		if err != nil {
			errChan <- err
		}
	}()

	<-lockAcquired

	// Try to acquire lock in second session (should fail/timeout)
	// We'll use a shorter context to avoid waiting 60s
	ctx, cancel := context.WithTimeout(s.ctx, 2*time.Second)
	defer cancel()
	
	repo2.ctx = ctx
	err = repo2.DoInLock(func() error {
		return nil
	})

	s.Assert().Error(err)
	s.Assert().Contains(err.Error(), "context deadline exceeded")

	releaseLock <- true
	
	// Success with error in fn
	err = s.repository.DoInLock(func() error {
		return fmt.Errorf("error")
	})
	s.Assert().Error(err)
}

func (s *ClickHouseTestSuite) TestRepair() {
	checksums := []string{"0a52730597fb4ffa01fc117d9e71e3a9", "3d41c8443df34e73867adb149efbb2ea"}
	contents := []string{"EXAMPLE CONTENT 1", "EXAMPLE CONTENT 2"}
	migrationsList := []*migrations.Migration{
		{
			Version:     1,
			Description: "abcd",
			Type:        enums.MIGRATION_UP,
			Checksum:    &checksums[0],
			Content:     &contents[0],
		},
	}

	// No table
	errs := s.repository.Repair(migrationsList)
	s.Assert().Nil(errs)

	err := s.repository.AssertSchemaHistoryTable()
	s.Assert().NoError(err)

	query := fmt.Sprintf(`
        INSERT INTO %s (version, description, md5_checksum, success, executed_at) VALUES
            (?, ?, ?, 1, now() - INTERVAL 2 MINUTE);
    `, default_history_table)

	_, err = s.suiteDb.ExecContext(s.ctx, query, migrationsList[0].Version, migrationsList[0].Description, *migrationsList[0].Checksum)
	s.Assert().NoError(err)

	// "Update" checksum by inserting new row (ReplacingMergeTree)
	newChecksum := "d41d8cd98f00b204e9800998ecf8427e"
	_, err = s.suiteDb.ExecContext(s.ctx, fmt.Sprintf(`
        INSERT INTO %s (version, description, md5_checksum, success, executed_at)
		VALUES (?, ?, ?, 1, now() - INTERVAL 1 MINUTE);
    `, default_history_table), migrationsList[0].Version, migrationsList[0].Description, newChecksum)
	s.Assert().NoError(err)

	errs = s.repository.Repair(migrationsList)
	s.Assert().Nil(errs)

	query = fmt.Sprintf(`
        SELECT md5_checksum FROM %s FINAL WHERE version = ?;
    `, default_history_table)

	var repairedChecksum string
	err = s.suiteDb.QueryRowContext(s.ctx, query, migrationsList[0].Version).Scan(&repairedChecksum)
	s.Assert().NoError(err)
	s.Assert().Equal(*migrationsList[0].Checksum, repairedChecksum)
}

func (s *ClickHouseTestSuite) TestGetFailingMigrations() {
	// No table
	failingMigrations, err := s.repository.GetFailingMigrations()
	s.Assert().NoError(err)
	s.Assert().Nil(failingMigrations)

	err = s.repository.AssertSchemaHistoryTable()
	s.Assert().NoError(err)

	query := fmt.Sprintf(`
		INSERT INTO %s (version, description, md5_checksum, success) VALUES
			(1, 't', '0a52730597fb4ffa01fc117d9e71e3a9', 0),
			(2, 't', '0a52730597fb4ffa01fc117d9e71e3a9', 1),
			(3, 't', '0a52730597fb4ffa01fc117d9e71e3a9', 0);
	`, default_history_table)

	_, err = s.suiteDb.Exec(query)
	s.Assert().NoError(err)

	failingMigrations, err = s.repository.GetFailingMigrations()
	s.Assert().NoError(err)
	s.Assert().Len(failingMigrations, 2)
}
