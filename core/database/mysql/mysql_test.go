package mysql

import (
	"context"
	"database/sql"
	"fmt"
	"testing"

	"github.com/maestro-go/maestro/core/enums"
	"github.com/maestro-go/maestro/internal/migrations"
	testUtils "github.com/maestro-go/maestro/internal/utils/testing"
	"github.com/stretchr/testify/suite"

	_ "github.com/go-sql-driver/mysql"
)

type MigrationTestSuite struct {
	suite.Suite
	mysql   *testUtils.MySQLContainer
	suiteDb *sql.DB

	ctx context.Context

	repository *MySQLRepository
}

func (s *MigrationTestSuite) SetupSuite() {
	s.ctx = context.Background()

	var err error
	s.mysql, err = testUtils.SetupMySQLContainer(s.ctx)
	s.Require().NoError(err)

	db, err := s.mysql.GetDB()
	s.Require().NoError(err)

	s.suiteDb = db

	s.repository = NewMySQLRepository(s.ctx, db, testUtils.ToPtr(default_history_table))
}

func (s *MigrationTestSuite) TearDownSuite() {
	if s.mysql != nil {
		s.mysql.Teardown(s.ctx)
	}
}

func (s *MigrationTestSuite) TearDownTest() {
	// Drop all tables
	rows, err := s.suiteDb.QueryContext(s.ctx, "SHOW TABLES")
	s.Require().NoError(err)
	defer rows.Close()

	var tables []string
	for rows.Next() {
		var table string
		s.Require().NoError(rows.Scan(&table))
		tables = append(tables, table)
	}

	if len(tables) > 0 {
		_, err = s.suiteDb.ExecContext(s.ctx, "SET FOREIGN_KEY_CHECKS = 0")
		s.Require().NoError(err)
		for _, table := range tables {
			_, err = s.suiteDb.ExecContext(s.ctx, fmt.Sprintf("DROP TABLE IF EXISTS %s", table))
			s.Require().NoError(err)
		}
		_, err = s.suiteDb.ExecContext(s.ctx, "SET FOREIGN_KEY_CHECKS = 1")
		s.Require().NoError(err)
	}
}

func (s *MigrationTestSuite) checkTableExists(table string, shouldExist bool) {
	s.T().Helper()

	query := `
		SELECT COUNT(*)
		FROM information_schema.tables
		WHERE table_schema = DATABASE() AND table_name = ?
	`

	count := 0
	err := s.suiteDb.QueryRowContext(s.ctx, query, table).Scan(&count)
	s.Assert().NoError(err)
	s.Assert().Equal(shouldExist, count > 0)
}

func TestMigrationSuite(t *testing.T) {
	suite.Run(t, new(MigrationTestSuite))
}

func (s *MigrationTestSuite) TestNewMySQLRepository() {
	repo := NewMySQLRepository(s.ctx, s.suiteDb, nil)
	s.Assert().Equal(default_history_table, repo.history_table)
}

func (s *MigrationTestSuite) TestAssertSchemaHistoryTable() {
	err := s.repository.AssertSchemaHistoryTable()
	s.Assert().NoError(err)

	s.checkTableExists(default_history_table, true)

	// Test already exists
	err = s.repository.AssertSchemaHistoryTable()
	s.Assert().NoError(err)
}

func (s *MigrationTestSuite) TestCheckSchemaHistoryTable() {
	tableExists, err := s.repository.CheckSchemaHistoryTable()
	s.Assert().NoError(err)
	s.Assert().False(tableExists)

	err = s.repository.AssertSchemaHistoryTable()
	s.Assert().NoError(err)

	tableExists, err = s.repository.CheckSchemaHistoryTable()
	s.Assert().NoError(err)
	s.Assert().True(tableExists)
}

func (s *MigrationTestSuite) TestGetLatestMigration() {
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
			(1, 't', '0a52730597fb4ffa01fc117d9e71e3a9', true),
			(5, 't', '0a52730597fb4ffa01fc117d9e71e3a9', true),
			(7, 't', '0a52730597fb4ffa01fc117d9e71e3a9', false);
	`, default_history_table)

	_, err = s.suiteDb.Exec(query)
	s.Assert().NoError(err)

	version, err = s.repository.GetLatestMigration()
	s.Assert().NoError(err)
	s.Assert().Equal(uint16(5), version)
}

func (s *MigrationTestSuite) TestValidateMigrations() {
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
			(?, ?, ?, true);
	`, default_history_table)

	_, err = s.suiteDb.ExecContext(s.ctx, query, migrationsList[1].Version,
		migrationsList[1].Description, migrationsList[1].Checksum)
	s.Assert().NoError(err)

	// Gap check: DB has version 2, but not version 1
	errs = s.repository.ValidateMigrations(migrationsList)
	s.Assert().Len(errs, 1)
	s.Assert().Contains(errs[0].Error(), "missing version 1")

	_, err = s.suiteDb.ExecContext(s.ctx, query, migrationsList[0].Version,
		migrationsList[0].Description, migrationsList[0].Checksum)
	s.Assert().NoError(err)

	errs = s.repository.ValidateMigrations(migrationsList)
	s.Assert().Nil(errs)

	query = fmt.Sprintf(`
		UPDATE %s SET md5_checksum = ? WHERE version = ?;
	`, default_history_table)

	_, err = s.suiteDb.ExecContext(s.ctx, query, checksums[0], migrationsList[1].Version)
	s.Assert().NoError(err)

	errs = s.repository.ValidateMigrations(migrationsList)
	s.Assert().Len(errs, 1)
}

func (s *MigrationTestSuite) TestExecuteMigration() {
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

	*migration.Content = "CREATE TABLE test (id INT NOT NULL PRIMARY KEY);"

	// No schema table
	errs = s.repository.ExecuteMigration(migration)
	s.Assert().Len(errs, 1)

	err := s.repository.AssertSchemaHistoryTable()
	s.Assert().NoError(err)

	*migration.Content = "CREATE TABLE test2 (id INT NOT NULL PRIMARY KEY);"

	errs = s.repository.ExecuteMigration(migration)
	s.Assert().Nil(errs)

	s.checkTableExists(default_history_table, true)
	s.checkTableExists("test2", true)

	query := fmt.Sprintf(`SELECT version, description, md5_checksum FROM %s;`, default_history_table)
	version := uint16(0)
	description := ""
	md5Checksum := ""
	err = s.suiteDb.QueryRowContext(s.ctx, query).Scan(&version, &description, &md5Checksum)
	s.Assert().NoError(err)
	s.Assert().Equal(migration.Version, version)
	s.Assert().Equal(migration.Description, description)
	s.Assert().Equal(*migration.Checksum, md5Checksum)
}

func (s *MigrationTestSuite) TestExecuteHook() {
	content := "INVALID SQL"
	hook := &migrations.Hook{
		Order:   1,
		Content: &content,
		Type:    enums.HOOK_AFTER_EACH,
	}

	err := s.repository.ExecuteHook(hook)
	s.Assert().Error(err)

	*hook.Content = "CREATE TABLE test3 (id INT NOT NULL PRIMARY KEY);"

	err = s.repository.ExecuteHook(hook)
	s.Assert().NoError(err)

	s.checkTableExists("test3", true)
}

func (s *MigrationTestSuite) TestRollbackMigration() {
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

	_, err = s.suiteDb.ExecContext(s.ctx, "CREATE TABLE test4 (id INT NOT NULL PRIMARY KEY);")
	s.Assert().NoError(err)
	_, err = s.suiteDb.ExecContext(s.ctx, fmt.Sprintf(`
		INSERT INTO %s (version, description, md5_checksum, success)
		VALUES (1, 'abcd', '0a52730597fb4ffa01fc117d9e71e3a9', true);
	`, default_history_table))
	s.Assert().NoError(err)

	s.checkTableExists("test4", true)

	migration.Content = testUtils.ToPtr("DROP TABLE test4;")
	err = s.repository.RollbackMigration(migration)
	s.Assert().NoError(err)

	s.checkTableExists("test4", false)
}

func (s *MigrationTestSuite) TestDoInTransaction() {
	// Using DML instead of DDL because MySQL doesn't support DDL rollback
	_, err := s.suiteDb.ExecContext(s.ctx, "CREATE TABLE test_dml (id INT NOT NULL PRIMARY KEY);")
	s.Assert().NoError(err)

	err = s.repository.AssertSchemaHistoryTable()
	s.Assert().NoError(err)

	content := "INSERT INTO test_dml (id) VALUES (1);"
	checksum := "0a52730597fb4ffa01fc117d9e71e3a9"
	migration := &migrations.Migration{
		Version:     1,
		Description: "abcd",
		Type:        enums.MIGRATION_UP,
		Checksum:    &checksum,
		Content:     &content,
	}

	// Fail case
	err = s.repository.DoInTransaction(func() error {
		errs := s.repository.ExecuteMigration(migration)
		s.Assert().Nil(errs)

		return fmt.Errorf("example error")
	})
	s.Assert().Error(err)

	// Check that DML was rolled back
	count := 0
	err = s.suiteDb.QueryRowContext(s.ctx, "SELECT COUNT(*) FROM test_dml WHERE id = 1").Scan(&count)
	s.Assert().NoError(err)
	s.Assert().Equal(0, count)

	// Success case
	err = s.repository.DoInTransaction(func() error {
		errs := s.repository.ExecuteMigration(migration)
		s.Assert().Nil(errs)
		return nil
	})
	s.Assert().NoError(err)
	err = s.suiteDb.QueryRowContext(s.ctx, "SELECT COUNT(*) FROM test_dml WHERE id = 1").Scan(&count)
	s.Assert().NoError(err)
	s.Assert().Equal(1, count)
}

func (s *MigrationTestSuite) TestDoInLock() {
	err := s.repository.AssertSchemaHistoryTable()
	s.Assert().NoError(err)

	// Open another session
	db2, err := s.mysql.GetDB()
	s.Assert().NoError(err)
	defer db2.Close()

	err = s.repository.DoInLock(func() error {
		var lockResult int
		// IS_FREE_LOCK returns 1 if the lock is free, 0 if it's used
		err = db2.QueryRowContext(s.ctx, "SELECT IS_FREE_LOCK(?)", lock_name).Scan(&lockResult)
		s.Assert().NoError(err)
		s.Assert().Equal(0, lockResult)
		return nil
	})

	s.Assert().NoError(err)

	// Success with error in fn
	err = s.repository.DoInLock(func() error {
		return fmt.Errorf("error")
	})
	s.Assert().Error(err)
}

func (s *MigrationTestSuite) TestRepair() {
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
        INSERT INTO %s (version, description, md5_checksum, success) VALUES
            (?, ?, ?, true);
    `, default_history_table)

	_, err = s.suiteDb.ExecContext(s.ctx, query, migrationsList[0].Version, migrationsList[0].Description, migrationsList[0].Checksum)
	s.Assert().NoError(err)

	// Change the checksum to simulate a mismatch
	newChecksum := "d41d8cd98f00b204e9800998ecf8427e"
	_, err = s.suiteDb.ExecContext(s.ctx, fmt.Sprintf(`
        UPDATE %s SET md5_checksum = ? WHERE version = ?;
    `, default_history_table), newChecksum, migrationsList[0].Version)
	s.Assert().NoError(err)

	errs = s.repository.Repair(migrationsList)
	s.Assert().Nil(errs)

	query = fmt.Sprintf(`
        SELECT md5_checksum FROM %s WHERE version = ?;
    `, default_history_table)

	var repairedChecksum string
	err = s.suiteDb.QueryRowContext(s.ctx, query, migrationsList[0].Version).Scan(&repairedChecksum)
	s.Assert().NoError(err)
	s.Assert().Equal(*migrationsList[0].Checksum, repairedChecksum)
}

func (s *MigrationTestSuite) TestGetFailingMigrations() {
	// No table
	failingMigrations, err := s.repository.GetFailingMigrations()
	s.Assert().NoError(err)
	s.Assert().Nil(failingMigrations)

	err = s.repository.AssertSchemaHistoryTable()
	s.Assert().NoError(err)

	query := fmt.Sprintf(`
		INSERT INTO %s (version, description, md5_checksum, success) VALUES
			(1, 't', '0a52730597fb4ffa01fc117d9e71e3a9', false),
			(2, 't', '0a52730597fb4ffa01fc117d9e71e3a9', true),
			(3, 't', '0a52730597fb4ffa01fc117d9e71e3a9', false);
	`, default_history_table)

	_, err = s.suiteDb.Exec(query)
	s.Assert().NoError(err)

	failingMigrations, err = s.repository.GetFailingMigrations()
	s.Assert().NoError(err)
	s.Assert().Len(failingMigrations, 2)
}
