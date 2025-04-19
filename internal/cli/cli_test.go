package cli

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/creasty/defaults"
	_ "github.com/lib/pq"
	"github.com/maestro-go/maestro/core/conf"
	"github.com/maestro-go/maestro/core/database"
	"github.com/maestro-go/maestro/core/database/postgres"
	"github.com/maestro-go/maestro/core/enums"
	testUtils "github.com/maestro-go/maestro/internal/utils/testing"
	"github.com/stretchr/testify/suite"
	"gopkg.in/yaml.v3"
)

type CliTestSuite struct {
	suite.Suite
	postgres *testUtils.PostgresContainer
	suiteDb  *sql.DB

	ctx context.Context

	repository database.Repository
}

func (s *CliTestSuite) SetupSuite() {
	s.ctx = context.Background()

	s.postgres = testUtils.SetupPostgres(s.T())

	db, err := sql.Open("postgres", s.postgres.URI)
	s.Assert().NoError(err)

	s.suiteDb = db

	s.repository = postgres.NewPostgresRepository(s.ctx, db, testUtils.ToPtr("schema_history"))
}

func (s *CliTestSuite) TearDownSuite() {
	if s.suiteDb != nil {
		s.suiteDb.Close()
	}
}

func TestCliTestSuite(t *testing.T) {
	suite.Run(t, new(CliTestSuite))
}

func (s *CliTestSuite) insertMigration(mType enums.MigrationType, dir string, version uint16, description string, content string) {
	s.T().Helper()

	migrationFile := ""
	if mType == enums.MIGRATION_UP {
		migrationFile = filepath.Join(dir, fmt.Sprintf("V%.3d_%s.sql", version, description))
	} else {
		migrationFile = filepath.Join(dir, fmt.Sprintf("V%.3d_%s.down.sql", version, description))
	}
	err := os.WriteFile(migrationFile, []byte(content), os.ModePerm)
	s.Require().NoError(err)
}

func (s *CliTestSuite) checkTableExists(table string, shouldExists bool) {
	s.T().Helper()

	var exists bool
	err := s.suiteDb.QueryRowContext(s.ctx, `
		SELECT EXISTS (
			SELECT 1 FROM pg_tables
			WHERE tablename = $1
		);
	`, table).Scan(&exists)
	s.Require().NoError(err)
	s.Require().Equal(shouldExists, exists)
}

func (s *CliTestSuite) checkRecordsInTable(table string, expectedCount int) {
	s.T().Helper()

	var actualCount int
	err := s.suiteDb.QueryRowContext(s.ctx, fmt.Sprintf(`
		SELECT count(*) FROM %s;
	`, table)).Scan(&actualCount)
	s.Require().NoError(err)
	s.Assert().Equal(expectedCount, actualCount)
}

func (s *CliTestSuite) checkFileExists(dir string, filename string, shouldExist bool) {
	s.T().Helper()

	_, err := os.Stat(filepath.Join(dir, filename))
	if shouldExist {
		s.Assert().NoError(err)
	} else {
		s.Assert().ErrorIs(err, os.ErrNotExist)
	}
}

func (s *CliTestSuite) clearDir(dir string) {
	s.T().Helper()

	entries, err := os.ReadDir(dir)
	s.Assert().NoError(err)
	for _, entry := range entries {
		err = os.RemoveAll(filepath.Join(dir, entry.Name()))
		s.Assert().NoError(err)
	}
}

func (s *CliTestSuite) resetDatabase() {
	s.T().Helper()

	if s.suiteDb != nil {
		// Drop all tables in public schema
		_, err := s.suiteDb.Exec(`
				DO $$ DECLARE
					r RECORD;
				BEGIN
					FOR r IN (SELECT tablename FROM pg_tables WHERE schemaname = 'public') LOOP
						EXECUTE 'DROP TABLE IF EXISTS ' || quote_ident(r.tablename) || ' CASCADE';
					END LOOP;
				END $$;
			`)
		s.Require().NoError(err)
	}
}

// This test should test all commands together
func (s *CliTestSuite) TestEntirePipeline() {
	projectDir := s.T().TempDir()
	migrationsDir := filepath.Join(projectDir, "migrations")
	os.Mkdir(migrationsDir, os.ModePerm)
	defer s.clearDir(projectDir)

	s.Run("test root command", func() {
		rootCmd := SetupRootCommand()
		err := rootCmd.Execute()
		s.Assert().NoError(err)

		rootCmd.SetArgs([]string{"-V"})
		err = rootCmd.Execute()
		s.Assert().NoError(err)
	})

	s.Run("test command flags", func() {
		rootCmd := SetupRootCommand()
		s.insertMigration(enums.MIGRATION_UP, migrationsDir, 1, "test", "CREATE TABLE test1 (id SERIAL PRIMARY KEY);")

		historyTable := "test_history"

		rootCmd.SetArgs([]string{"migrate", "-l", projectDir, "-m", migrationsDir, "--user", s.postgres.Username,
			"--password", s.postgres.Password, "--port", s.postgres.Port, "--database", s.postgres.Database,
			"--history-table", historyTable})
		err := rootCmd.Execute()
		s.Require().NoError(err)

		s.checkTableExists("test1", true)
		s.checkTableExists(historyTable, true)

		s.clearDir(migrationsDir) // Clear migrations for init
		s.resetDatabase()
	})

	s.Run("test init command", func() {
		rootCmd := SetupRootCommand()
		rootCmd.SetArgs([]string{"init", "-l", projectDir, "-m", migrationsDir})
		err := rootCmd.Execute()
		s.Require().NoError(err)

		s.checkFileExists(projectDir, "maestro.yaml", true)
		s.checkFileExists(migrationsDir, "V001_example.sql", true)
	})

	s.Run("test create command", func() {
		rootCmd := SetupRootCommand()
		rootCmd.SetArgs([]string{"create", "test2", "-l", projectDir})
		err := rootCmd.Execute()
		s.Require().NoError(err)

		s.checkFileExists(migrationsDir, "V002_test2.sql", true)
	})

	s.Run("check error with invalid config file", func() {
		rootCmd := SetupRootCommand()
		rootCmd.SetArgs([]string{"status", "-l", projectDir, "-m", migrationsDir})
		err := rootCmd.Execute()
		s.Assert().Error(err) // This should fail because it is pointing to 5432 postgres default
	})

	// Fix the config in maestro.yaml
	portAsUint, err := strconv.ParseUint(s.postgres.Port, 10, 16)
	s.Require().NoError(err)
	newConfig := &conf.ProjectConfig{}
	defaults.MustSet(newConfig)
	newConfig.Port = uint16(portAsUint)
	newConfig.Database = s.postgres.Database
	newConfig.User = s.postgres.Username
	newConfig.Password = s.postgres.Password
	newConfig.Migration.Locations = []string{migrationsDir}

	newConfigContent, err := yaml.Marshal(newConfig)
	s.Require().NoError(err)
	err = os.WriteFile(filepath.Join(projectDir, "maestro.yaml"), newConfigContent, os.ModePerm)
	s.Require().NoError(err)

	s.Run("test status command with no history table", func() {
		rootCmd := SetupRootCommand()
		rootCmd.SetArgs([]string{"status", "-l", projectDir, "-m", migrationsDir})
		err := rootCmd.Execute()
		s.Assert().NoError(err)

		s.checkTableExists("schema_history", false)
	})

	s.Run("test repair command with no history table", func() {
		rootCmd := SetupRootCommand()
		rootCmd.SetArgs([]string{"repair", "-l", projectDir, "-m", migrationsDir})
		err := rootCmd.Execute()
		s.Assert().NoError(err)

		s.checkTableExists("schema_history", false)
	})

	s.Run("test migrate command with errors", func() {
		s.checkFileExists(migrationsDir, "V001_example.sql", true)
		s.insertMigration(enums.MIGRATION_UP, migrationsDir, 1, "example", "INVALID SQL")

		rootCmd := SetupRootCommand()
		rootCmd.SetArgs([]string{"migrate", "-l", projectDir})
		err := rootCmd.Execute()
		s.Assert().Error(err)

		s.checkRecordsInTable("schema_history", 0)
	})

	s.Run("test migrate command forcing", func() {
		s.checkFileExists(migrationsDir, "V002_test2.sql", true)
		s.insertMigration(enums.MIGRATION_UP, migrationsDir, 2, "test2", "CREATE TABLE test2 (id SERIAL PRIMARY KEY, name varchar(255));")

		rootCmd := SetupRootCommand()
		rootCmd.SetArgs([]string{"migrate", "-l", projectDir, "--force=true", "--in-transaction=false"})
		err := rootCmd.Execute()
		s.Assert().Error(err)

		s.checkRecordsInTable("schema_history", 2)
		s.checkTableExists("test2", true)

		failing, err := s.repository.GetFailingMigrations()
		s.Require().NoError(err)
		s.Assert().Len(failing, 1)
	})

	s.Run("test migrate command failing for having invalid migrations", func() {
		rootCmd := SetupRootCommand()
		rootCmd.SetArgs([]string{"migrate", "-l", projectDir})
		err := rootCmd.Execute()
		s.Assert().Error(err)
	})

	s.Run("test status command with failing migrations", func() {
		rootCmd := SetupRootCommand()
		rootCmd.SetArgs([]string{"status", "-l", projectDir})
		err := rootCmd.Execute()
		s.Assert().NoError(err)
	})

	// Fix migration 1
	s.checkFileExists(migrationsDir, "V001_example.sql", true)
	s.insertMigration(enums.MIGRATION_UP, migrationsDir, 1, "example", "CREATE TABLE test1 (id SERIAL PRIMARY KEY, name varchar(255))")

	s.Run("test repair command", func() {
		rootCmd := SetupRootCommand()
		rootCmd.SetArgs([]string{"repair", "-l", projectDir})
		err := rootCmd.Execute()
		s.Require().NoError(err)

		failingMigrations, err := s.repository.GetFailingMigrations()
		s.Require().NoError(err)
		s.Assert().Empty(failingMigrations)
	})

	s.Run("test migrate command failing md5 checksum", func() {
		s.checkFileExists(migrationsDir, "V001_example.sql", true)
		s.insertMigration(enums.MIGRATION_UP, migrationsDir, 1, "example", "CHANGED SQL")
		rootCmd := SetupRootCommand()
		rootCmd.SetArgs([]string{"migrate", "-l", projectDir})
		err := rootCmd.Execute()
		s.Require().Error(err)

	})

	s.Run("test status command with failing md5 checksum", func() {
		rootCmd := SetupRootCommand()
		rootCmd.SetArgs([]string{"status", "-l", projectDir})
		err := rootCmd.Execute()
		s.Require().NoError(err)
	})

	// Reset file
	s.insertMigration(enums.MIGRATION_UP, migrationsDir, 1, "example", "CREATE TABLE test1 (id SERIAL PRIMARY KEY, name varchar(255))")

	s.insertMigration(enums.MIGRATION_UP, migrationsDir, 3, "example", "CREATE TABLE test3 (id SERIAL PRIMARY KEY, name varchar(255))")
	s.insertMigration(enums.MIGRATION_DOWN, migrationsDir, 1, "example", "DROP TABLE IF EXISTS test1;")
	s.insertMigration(enums.MIGRATION_DOWN, migrationsDir, 2, "example", "DROP TABLE IF EXISTS test2;")
	s.insertMigration(enums.MIGRATION_DOWN, migrationsDir, 3, "example", "DROP TABLE IF EXISTS test3;")

	s.Run("test migrate command with new migrations", func() {
		rootCmd := SetupRootCommand()
		rootCmd.SetArgs([]string{"migrate", "-l", projectDir})
		err := rootCmd.Execute()
		s.Require().NoError(err)

		s.checkRecordsInTable("schema_history", 3)
		s.checkTableExists("test3", true)
	})

	s.Run("test migrate command down", func() {
		rootCmd := SetupRootCommand()
		rootCmd.SetArgs([]string{"migrate", "-l", projectDir, "--down"})
		err := rootCmd.Execute()
		s.Require().NoError(err)

		s.checkRecordsInTable("schema_history", 0)
		s.checkTableExists("test1", false)
		s.checkTableExists("test2", false)
		s.checkTableExists("test3", false)
	})

	s.Run("test migrate command with destination", func() {
		rootCmd := SetupRootCommand()
		rootCmd.SetArgs([]string{"migrate", "-l", projectDir, "--destination=2"})
		err := rootCmd.Execute()
		s.Require().NoError(err)

		s.checkRecordsInTable("schema_history", 2)
		s.checkTableExists("test1", true)
		s.checkTableExists("test2", true)
		s.checkTableExists("test3", false)
	})

	s.Run("test migrate command down with destination", func() {
		rootCmd := SetupRootCommand()
		rootCmd.SetArgs([]string{"migrate", "-l", projectDir, "--down", "--destination=1"})
		err := rootCmd.Execute()
		s.Require().NoError(err)

		s.checkRecordsInTable("schema_history", 1)
		s.checkTableExists("test1", true)
		s.checkTableExists("test2", false)
		s.checkTableExists("test3", false)
	})

	s.Run("test all flags merge", func() {
		rootCmd := SetupRootCommand()
		rootCmd.SetArgs([]string{"migrate", "-l", projectDir, "--validate=true", "--in-transaction=true",
			"--force=false", "--use-repeatable=true", "--use-before=true", "--use-after=true", "--use-before-each=true",
			"--use-after-each=true", "--use-before-version=true", "--use-after-version=true", "--driver=postgres",
			"--host=localhost", "--port", s.postgres.Port, "--database", s.postgres.Database, "--user", s.postgres.Username,
			"--password", s.postgres.Password, "--schema=public", "--sslmode=disable", "--sslrootcert=\"\""})
		err := rootCmd.Execute()
		s.Assert().NoError(err)
	})
}
