package cli

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	_ "github.com/lib/pq"
	"github.com/maestro-go/maestro/core/conf"
	"github.com/maestro-go/maestro/core/database"
	"github.com/maestro-go/maestro/core/database/postgres"
	"github.com/maestro-go/maestro/core/enums"
	"github.com/maestro-go/maestro/internal/cli/flags"
	"github.com/maestro-go/maestro/internal/filesystem"
	testUtils "github.com/maestro-go/maestro/internal/pkg/testing"
	"github.com/stretchr/testify/suite"
)

type StatusTestSuite struct {
	suite.Suite
	postgres *testUtils.PostgresContainer
	suiteDb  *sql.DB

	ctx context.Context

	repository database.Repository
}

func (s *StatusTestSuite) SetupSuite() {
	s.ctx = context.Background()

	s.postgres = testUtils.SetupPostgres(s.T())

	db, err := sql.Open("postgres", s.postgres.URI)
	s.Assert().NoError(err)

	s.suiteDb = db

	s.repository = postgres.NewPostgresRepository(s.ctx, db)
}

func (s *StatusTestSuite) TearDownTest() {
	if s.postgres != nil {
		// Drop all tables before terminating
		db, err := sql.Open("postgres", s.postgres.URI)
		if err == nil {
			defer db.Close()

			// Drop all tables in public schema
			_, err = db.Exec(`
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
}

func (s *StatusTestSuite) TearDownSuite() {
	if s.suiteDb != nil {
		s.suiteDb.Close()
	}
}

func (s *StatusTestSuite) insertMigration(dir string, version uint16, description, content string) {
	migrationFile := filepath.Join(dir, fmt.Sprintf("V%.3d_%s.sql", version, description))
	err := os.WriteFile(migrationFile, []byte(content), os.ModePerm)
	s.Require().NoError(err)
}

func (s *StatusTestSuite) TestStatusCommand() {
	migrationsDir := s.T().TempDir()

	upContent1 := "CREATE TABLE test1 (id SERIAL PRIMARY KEY);"
	upContent2 := "CREATE TABLE test2 (id SERIAL PRIMARY KEY);"

	s.insertMigration(migrationsDir, 1, "test1", upContent1)
	s.insertMigration(migrationsDir, 2, "test2", upContent2)

	migrationsMap, _, errs := filesystem.LoadObjectsFromFiles(&conf.MigrationConfig{
		Locations: []string{migrationsDir},
	})
	s.Assert().Empty(errs)
	s.Assert().Len(migrationsMap[enums.MIGRATION_UP], 2)

	err := s.repository.AssertSchemaHistoryTable()
	s.Assert().NoError(err)

	errs = s.repository.ExecuteMigration(migrationsMap[enums.MIGRATION_UP][0])
	s.Assert().Empty(errs)

	errs = s.repository.ExecuteMigration(migrationsMap[enums.MIGRATION_UP][1])
	s.Assert().Empty(errs)

	// Setup status command
	statusCmd := SetupStatusCommand()
	flags.SetupGlobalFlags(statusCmd)

	// Run status command
	statusCmd.SetArgs([]string{"-m", migrationsDir, "--driver", "postgres", "--user", s.postgres.Username,
		"--password", s.postgres.Password, "--host", "localhost", "--database", s.postgres.Database, "--port", s.postgres.Port})
	err = statusCmd.Execute()

	// Assert no error
	s.Assert().NoError(err)
}

func TestStatusTestSuite(t *testing.T) {
	suite.Run(t, new(StatusTestSuite))
}
