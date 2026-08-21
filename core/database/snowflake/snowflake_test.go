package snowflake

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"testing"

	"github.com/maestro-go/maestro/core/enums"
	"github.com/maestro-go/maestro/internal/migrations"
	testUtils "github.com/maestro-go/maestro/internal/utils/testing"
	"github.com/stretchr/testify/suite"

	_ "github.com/snowflakedb/gosnowflake"
)

type SnowflakeMigrationTestSuite struct {
	suite.Suite
	dsn     string
	suiteDb *sql.DB

	ctx context.Context

	repository *SnowflakeRepository
}

func (s *SnowflakeMigrationTestSuite) SetupSuite() {
	s.ctx = context.Background()

	if dsn := os.Getenv("SNOWFLAKE_DSN"); dsn != "" {
		s.dsn = dsn
	} else if os.Getenv("LOCALSTACK_AUTH_TOKEN") != "" {
		container := testUtils.SetupSnowflake(s.T())
		if container == nil {
			s.T().Skip("Snowflake emulator is not available; skipping Snowflake tests")
		}
		s.dsn = container.DSN
	} else {
		s.T().Skip("SNOWFLAKE_DSN not set; skipping Snowflake tests")
	}

	db, err := sql.Open("snowflake", s.dsn)
	s.Assert().NoError(err)

	s.suiteDb = db

	s.repository = NewSnowflakeRepository(s.ctx, db, testUtils.ToPtr(default_history_table))
}

func (s *SnowflakeMigrationTestSuite) TearDownTest() {
	if s.suiteDb != nil {
		rows, err := s.suiteDb.Query(`
			SELECT TABLE_NAME
			FROM information_schema.tables
			WHERE table_catalog = CURRENT_DATABASE()
			  AND table_schema = CURRENT_SCHEMA()
		`)
		s.Require().NoError(err)
		defer rows.Close()

		for rows.Next() {
			var tableName string
			if err := rows.Scan(&tableName); err != nil {
				s.Require().NoError(err)
			}

			_, err := s.suiteDb.Exec(fmt.Sprintf("DROP TABLE IF EXISTS %s", tableName))
			s.Require().NoError(err)
		}
		s.Require().NoError(rows.Err())
	}
}

func (s *SnowflakeMigrationTestSuite) checkTableExists(table string, shouldExist bool) {
	s.T().Helper()

	query := `
		SELECT COUNT(*)
		FROM information_schema.tables
		WHERE table_catalog = CURRENT_DATABASE()
		  AND table_schema = CURRENT_SCHEMA()
		  AND table_name = ?
	`

	count := 0
	err := s.suiteDb.QueryRowContext(s.ctx, query, table).Scan(&count)
	s.Assert().NoError(err)
	exists := count > 0
	s.Assert().Equal(shouldExist, exists)
}

func TestSnowflakeMigrationSuite(t *testing.T) {
	suite.Run(t, new(SnowflakeMigrationTestSuite))
}

func (s *SnowflakeMigrationTestSuite) TestNewSnowflakeRepository() {
	repo := NewSnowflakeRepository(s.ctx, s.suiteDb, nil)
	s.Assert().Equal(default_history_table, repo.history_table)
}

func (s *SnowflakeMigrationTestSuite) TestAssertSchemaHistoryTable() {
	err := s.repository.AssertSchemaHistoryTable()
	s.Assert().NoError(err)

	s.checkTableExists(default_history_table, true)

	err = s.repository.AssertSchemaHistoryTable()
	s.Assert().NoError(err)
}

func (s *SnowflakeMigrationTestSuite) TestCheckSchemaHistoryTable() {
	tableExists, err := s.repository.CheckSchemaHistoryTable()
	s.Assert().NoError(err)
	s.Assert().False(tableExists)

	err = s.repository.AssertSchemaHistoryTable()
	s.Assert().NoError(err)

	tableExists, err = s.repository.CheckSchemaHistoryTable()
	s.Assert().NoError(err)
	s.Assert().True(tableExists)
}

func (s *SnowflakeMigrationTestSuite) TestGetLatestMigration() {
	version, err := s.repository.GetLatestMigration()
	s.Assert().NoError(err)
	s.Assert().Equal(uint16(0), version)

	err = s.repository.AssertSchemaHistoryTable()
	s.Assert().NoError(err)

	version, err = s.repository.GetLatestMigration()
	s.Assert().NoError(err)
	s.Assert().Equal(uint16(0), version)

	query := fmt.Sprintf(`
		INSERT INTO %s (version, description, md5_checksum, success) VALUES
			(1, 't', '0a52730597fb4ffa01fc117d9e71e3a9', TRUE),
			(5, 't', '0a52730597fb4ffa01fc117d9e71e3a9', TRUE),
			(7, 't', '0a52730597fb4ffa01fc117d9e71e3a9', FALSE);
	`, default_history_table)

	_, err = s.suiteDb.Exec(query)
	s.Assert().NoError(err)

	version, err = s.repository.GetLatestMigration()
	s.Assert().NoError(err)
	s.Assert().Equal(uint16(5), version)
}

func (s *SnowflakeMigrationTestSuite) TestValidateMigrations() {
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

	errs = s.repository.ValidateMigrations(migrationsList)
	s.Assert().Nil(errs)

	err := s.repository.AssertSchemaHistoryTable()
	s.Assert().NoError(err)

	migrationsList[0].Type = enums.MIGRATION_DOWN
	errs = s.repository.ValidateMigrations(migrationsList)
	s.Assert().Len(errs, 1)
	s.Assert().Contains(errs[0].Error(), "invalid migration type")
	migrationsList[0].Type = enums.MIGRATION_UP

	errs = s.repository.ValidateMigrations(migrationsList)
	s.Assert().Nil(errs)

	query := fmt.Sprintf(`
		INSERT INTO %s (version, description, md5_checksum, success) VALUES
			(?, ?, ?, TRUE);
	`, default_history_table)

	_, err = s.suiteDb.ExecContext(s.ctx, query, migrationsList[1].Version,
		migrationsList[1].Description, *migrationsList[1].Checksum)
	s.Assert().NoError(err)

	errs = s.repository.ValidateMigrations(migrationsList)
	s.Assert().Len(errs, 1)
	s.Assert().Contains(errs[0].Error(), "missing version 1")

	_, err = s.suiteDb.ExecContext(s.ctx, query, migrationsList[0].Version,
		migrationsList[0].Description, *migrationsList[0].Checksum)
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

func (s *SnowflakeMigrationTestSuite) TestExecuteMigration() {
	checksum := "0a52730597fb4ffa01fc117d9e71e3a9"
	content := "INVALID SQL"
	migration := &migrations.Migration{
		Version:     1,
		Description: "abcd",
		Type:        enums.MIGRATION_UP,
		Checksum:    &checksum,
		Content:     &content,
	}

	migration.Type = enums.MIGRATION_DOWN
	errs := s.repository.ExecuteMigration(migration)
	s.Assert().Len(errs, 1)
	s.Assert().Contains(errs[0].Error(), "invalid migration type")
	migration.Type = enums.MIGRATION_UP

	errs = s.repository.ExecuteMigration(migration)
	s.Assert().Len(errs, 2)

	*migration.Content = "CREATE TABLE test_table (id INT NOT NULL PRIMARY KEY);"

	errs = s.repository.ExecuteMigration(migration)
	s.Assert().Len(errs, 1)

	err := s.repository.AssertSchemaHistoryTable()
	s.Assert().NoError(err)

	*migration.Content = "CREATE TABLE test2 (id INT NOT NULL PRIMARY KEY);"

	errs = s.repository.ExecuteMigration(migration)
	s.Assert().Nil(errs)

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

func (s *SnowflakeMigrationTestSuite) TestExecuteHook() {
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

func (s *SnowflakeMigrationTestSuite) TestRollbackMigration() {
	content := "INVALID SQL"
	migration := &migrations.Migration{
		Version:     1,
		Description: "abcd",
		Type:        enums.MIGRATION_DOWN,
		Content:     &content,
	}

	migration.Type = enums.MIGRATION_UP
	err := s.repository.RollbackMigration(migration)
	s.Assert().Error(err)
	s.Assert().Contains(err.Error(), "invalid migration type")
	migration.Type = enums.MIGRATION_DOWN

	err = s.repository.RollbackMigration(migration)
	s.Assert().Error(err)

	err = s.repository.AssertSchemaHistoryTable()
	s.Assert().NoError(err)

	err = s.repository.RollbackMigration(migration)
	s.Assert().NoError(err)

	_, err = s.suiteDb.ExecContext(s.ctx, "CREATE TABLE test4 (id INT NOT NULL PRIMARY KEY);")
	s.Assert().NoError(err)
	_, err = s.suiteDb.ExecContext(s.ctx, fmt.Sprintf(`
		INSERT INTO %s (version, description, md5_checksum, success)
		VALUES (1, 'abcd', '0a52730597fb4ffa01fc117d9e71e3a9', TRUE);
	`, default_history_table))
	s.Assert().NoError(err)

	s.checkTableExists("test4", true)

	*migration.Content = "DROP TABLE test4;"
	err = s.repository.RollbackMigration(migration)
	s.Assert().NoError(err)

	s.checkTableExists("test4", false)
}

func (s *SnowflakeMigrationTestSuite) TestDoInTransaction() {
	content := "CREATE TABLE test1 (id INT NOT NULL PRIMARY KEY);"
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

	// Snowflake auto-commits DDL, so the table persists even when fn errors.
	err = s.repository.DoInTransaction(func() error {
		errs := s.repository.ExecuteMigration(migration)
		s.Assert().Nil(errs)

		return fmt.Errorf("example error")
	})
	s.Assert().Error(err)
	s.checkTableExists("test1", true)
}

func (s *SnowflakeMigrationTestSuite) TestDoInLock() {
	err := s.repository.AssertSchemaHistoryTable()
	s.Assert().NoError(err)

	err = s.repository.DoInLock(func() error {
		s.checkTableExists(lock_table, true)
		return nil
	})
	s.Assert().NoError(err)

	s.checkTableExists(lock_table, false)

	err = s.repository.DoInLock(func() error {
		return fmt.Errorf("error")
	})
	s.Assert().Error(err)
}

func (s *SnowflakeMigrationTestSuite) TestRepair() {
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

	errs := s.repository.Repair(migrationsList)
	s.Assert().Nil(errs)

	err := s.repository.AssertSchemaHistoryTable()
	s.Assert().NoError(err)

	query := fmt.Sprintf(`
		INSERT INTO %s (version, description, md5_checksum, success) VALUES
			(?, ?, ?, TRUE);
	`, default_history_table)

	_, err = s.suiteDb.ExecContext(s.ctx, query, migrationsList[0].Version, migrationsList[0].Description, *migrationsList[0].Checksum)
	s.Assert().NoError(err)

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

func (s *SnowflakeMigrationTestSuite) TestGetFailingMigrations() {
	failingMigrations, err := s.repository.GetFailingMigrations()
	s.Assert().NoError(err)
	s.Assert().Nil(failingMigrations)

	err = s.repository.AssertSchemaHistoryTable()
	s.Assert().NoError(err)

	query := fmt.Sprintf(`
		INSERT INTO %s (version, description, md5_checksum, success) VALUES
			(1, 't', '0a52730597fb4ffa01fc117d9e71e3a9', FALSE),
			(2, 't', '0a52730597fb4ffa01fc117d9e71e3a9', TRUE),
			(3, 't', '0a52730597fb4ffa01fc117d9e71e3a9', FALSE);
	`, default_history_table)

	_, err = s.suiteDb.Exec(query)
	s.Assert().NoError(err)

	failingMigrations, err = s.repository.GetFailingMigrations()
	s.Assert().NoError(err)
	s.Assert().Len(failingMigrations, 2)
}
