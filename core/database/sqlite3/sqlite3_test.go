package sqlite3

import (
	"context"
	"database/sql"
	"fmt"
	"testing"

	"github.com/maestro-go/maestro/core/enums"
	"github.com/maestro-go/maestro/internal/migrations"
	testUtils "github.com/maestro-go/maestro/internal/utils/testing"
	_ "github.com/mattn/go-sqlite3"
	"github.com/stretchr/testify/suite"
)

type MigrationTestSuite struct {
	suite.Suite
	suiteDB *sql.DB

	ctx context.Context

	repository *SQLiteRepository
}

func (s *MigrationTestSuite) SetupSuite() {
	s.ctx = context.Background()

	db, err := sql.Open("sqlite3", ":memory:")
	s.Require().NoError(err)

	s.suiteDB = db

	s.repository = NewSQLiteRepository(s.ctx, s.suiteDB, testUtils.ToPtr(default_history_table))
}

func (s *MigrationTestSuite) TearDownTest() {
	tx, err := s.suiteDB.BeginTx(s.ctx, nil)
	s.NoError(err)

	rows, err := tx.QueryContext(s.ctx, "SELECT name FROM sqlite_schema WHERE type = 'table';")
	s.NoError(err)
	defer rows.Close()

	tables := make([]string, 0)

	for rows.Next() {
		table := ""
		err := rows.Scan(&table)
		s.NoError(err)

		tables = append(tables, table)
	}

	for _, t := range tables {
		_, err = tx.ExecContext(s.ctx, "DROP TABLE IF EXISTS "+t+";")
		s.NoError(err)
	}

	err = tx.Commit()
	s.NoError(err)
}

func (s *MigrationTestSuite) TearDownSuite() {
	s.suiteDB.Close()
}

func (s *MigrationTestSuite) checkTableExists(table string, shouldExist bool) {
	s.T().Helper()

	query := `
		SELECT name
		FROM sqlite_schema
		WHERE type='table' AND name=?;
	`

	name := ""
	err := s.suiteDB.QueryRowContext(s.ctx, query, table).Scan(&name)
	if err != nil {
		s.Equal(shouldExist, err != sql.ErrNoRows)
	} else {
		s.NoError(err)
		s.Equal(shouldExist, true)
	}
}

func TestMigrationSuite(t *testing.T) {
	suite.Run(t, new(MigrationTestSuite))
}

func (s *MigrationTestSuite) TestAssertSchemaHistoryTable() {
	err := s.repository.AssertSchemaHistoryTable()
	s.Assert().NoError(err)

	s.checkTableExists(default_history_table, true)
}

func (s *MigrationTestSuite) TestCheckSchemaHistoryTable() {
	s.checkTableExists(default_history_table, false)

	err := s.repository.AssertSchemaHistoryTable()
	s.Assert().NoError(err)

	s.checkTableExists(default_history_table, true)
}

func (s *MigrationTestSuite) TestGetLatestMigration() {
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
			(1, 't', '0a52730597fb4ffa01fc117d9e71e3a9', true),
			(5, 't', '0a52730597fb4ffa01fc117d9e71e3a9', true),
			(7, 't', '0a52730597fb4ffa01fc117d9e71e3a9', false);
	`, default_history_table)

	_, err = s.suiteDB.Exec(query)
	s.Assert().NoError(err)

	version, err = s.repository.GetLatestMigration()
	s.Assert().NoError(err)
	s.Assert().Equal(uint16(5), version)
}

func (s *MigrationTestSuite) TestValidateMigrations() {
	checksums := []string{"0a52730597fb4ffa01fc117d9e71e3a9", "3d41c8443df34e73867adb149efbb2ea"}
	contents := []string{"EXAMPLE CONTENT 1", "EXAMPLE CONTENT 2"}
	migrations := []*migrations.Migration{
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

	errs := s.repository.ValidateMigrations(migrations)
	s.Assert().Nil(errs)

	err := s.repository.AssertSchemaHistoryTable()
	s.Assert().NoError(err)

	errs = s.repository.ValidateMigrations(migrations)
	s.Assert().Nil(errs)

	query := fmt.Sprintf(`
		INSERT INTO %s (version, description, md5_checksum, success) VALUES
			($1, $2, $3, true);
	`, default_history_table)

	_, err = s.suiteDB.ExecContext(s.ctx, query, migrations[1].Version,
		migrations[1].Description, migrations[1].Checksum)
	s.Assert().NoError(err)

	errs = s.repository.ValidateMigrations(migrations)
	s.Assert().Len(errs, 1)

	_, err = s.suiteDB.ExecContext(s.ctx, query, migrations[0].Version,
		migrations[0].Description, migrations[0].Checksum)
	s.Assert().NoError(err)

	errs = s.repository.ValidateMigrations(migrations)
	s.Assert().Nil(errs)

	query = fmt.Sprintf(`
		UPDATE %s SET md5_checksum = $1 WHERE version = $2;
	`, default_history_table)

	_, err = s.suiteDB.ExecContext(s.ctx, query, checksums[0], migrations[1].Version)
	s.Assert().NoError(err)

	errs = s.repository.ValidateMigrations(migrations)
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

	// Invalid SQL
	errs := s.repository.ExecuteMigration(migration)
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
	err = s.suiteDB.QueryRowContext(s.ctx, query).Scan(&version, &description, &md5Checksum)
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

	err := s.repository.RollbackMigration(migration)
	s.Assert().Error(err)

	*migration.Content = "DROP TABLE IF EXISTS test4;"

	err = s.repository.RollbackMigration(migration)
	s.Assert().Error(err)

	err = s.repository.AssertSchemaHistoryTable()
	s.Assert().NoError(err)

	_, err = s.suiteDB.ExecContext(s.ctx, "CREATE TABLE test4 (id INT NOT NULL PRIMARY KEY);")
	s.Assert().NoError(err)
	_, err = s.suiteDB.ExecContext(s.ctx, fmt.Sprintf(`
		INSERT INTO %s (version, description, md5_checksum, success)
		VALUES (1, 'abcd', '0a52730597fb4ffa01fc117d9e71e3a9', true);
	`, default_history_table))
	s.Assert().NoError(err)

	query2 := fmt.Sprintf(`
		SELECT EXISTS (
			SELECT version FROM %s WHERE version = $1
		);
	`, default_history_table)

	s.checkTableExists("test4", true)

	exists := false
	err = s.suiteDB.QueryRowContext(s.ctx, query2, 1).Scan(&exists)
	s.Assert().NoError(err)
	s.Assert().True(exists)

	err = s.repository.RollbackMigration(migration)
	s.Assert().NoError(err)

	s.checkTableExists("test4", false)

	err = s.suiteDB.QueryRowContext(s.ctx, query2, 1).Scan(&exists)
	s.Assert().NoError(err)
	s.Assert().False(exists)
}

func (s *MigrationTestSuite) TestDoInTransaction() {
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

	err = s.repository.DoInTransaction(func() error {
		errs := s.repository.ExecuteMigration(migration)
		s.Assert().Nil(errs)

		return fmt.Errorf("example error")
	})
	s.Assert().Error(err)

	s.checkTableExists("test1", false)
}

func (s *MigrationTestSuite) TestDoInLock() {
	err := s.repository.AssertSchemaHistoryTable()
	s.Assert().NoError(err)

	s.checkTableExists(lock_table, false)

	err = s.repository.DoInLock(func() error {
		s.checkTableExists(lock_table, true)
		return nil
	})
	s.Assert().NoError(err)

	s.checkTableExists(lock_table, false)
}

func (s *MigrationTestSuite) TestRepair() {
	checksums := []string{"0a52730597fb4ffa01fc117d9e71e3a9", "3d41c8443df34e73867adb149efbb2ea"}
	contents := []string{"EXAMPLE CONTENT 1", "EXAMPLE CONTENT 2"}
	migrations := []*migrations.Migration{
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

	err := s.repository.AssertSchemaHistoryTable()
	s.Assert().NoError(err)

	query := fmt.Sprintf(`
        INSERT INTO %s (version, description, md5_checksum, success) VALUES
            ($1, $2, $3, false);
    `, default_history_table)

	_, err = s.suiteDB.ExecContext(s.ctx, query, migrations[0].Version, migrations[0].Description, migrations[0].Checksum)
	s.Assert().NoError(err)

	// Change the checksum to simulate a mismatch
	newChecksum := "d41d8cd98f00b204e9800998ecf8427e"
	_, err = s.suiteDB.ExecContext(s.ctx, fmt.Sprintf(`
        UPDATE %s SET md5_checksum = $1 WHERE version = $2;
    `, default_history_table), newChecksum, migrations[0].Version)
	s.Assert().NoError(err)

	errs := s.repository.Repair(migrations)
	s.Assert().Nil(errs)

	query = fmt.Sprintf(`
        SELECT md5_checksum FROM %s WHERE version = $1;
    `, default_history_table)

	var repairedChecksum string
	err = s.suiteDB.QueryRowContext(s.ctx, query, migrations[0].Version).Scan(&repairedChecksum)
	s.Assert().NoError(err)
	s.Assert().Equal(*migrations[0].Checksum, repairedChecksum)

	// Test upsert for non-existing migration
	errs = s.repository.Repair(migrations[1:])
	s.Assert().Nil(errs)

	query = fmt.Sprintf(`
        SELECT md5_checksum FROM %s WHERE version = $1;
    `, default_history_table)

	err = s.suiteDB.QueryRowContext(s.ctx, query, migrations[1].Version).Scan(&repairedChecksum)
	s.Assert().NoError(err)
	s.Assert().Equal(*migrations[1].Checksum, repairedChecksum)
}

func (s *MigrationTestSuite) TestGetFailingMigrations() {
	err := s.repository.AssertSchemaHistoryTable()
	s.Assert().NoError(err)

	query := fmt.Sprintf(`
		INSERT INTO %s (version, description, md5_checksum, success) VALUES
			(1, 't', '0a52730597fb4ffa01fc117d9e71e3a9', false),
			(2, 't', '0a52730597fb4ffa01fc117d9e71e3a9', true),
			(3, 't', '0a52730597fb4ffa01fc117d9e71e3a9', false);
	`, default_history_table)

	_, err = s.suiteDB.Exec(query)
	s.Assert().NoError(err)

	failingMigrations, err := s.repository.GetFailingMigrations()
	s.Assert().NoError(err)
	s.Assert().Len(failingMigrations, 2)
	s.Assert().Equal(uint16(1), failingMigrations[0].Version)
	s.Assert().Equal(uint16(3), failingMigrations[1].Version)
}
