package cli

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	_ "github.com/lib/pq"
	"github.com/maestro-go/maestro/core/database"
	"github.com/maestro-go/maestro/core/database/postgres"
	"github.com/maestro-go/maestro/internal/cli/flags"
	testUtils "github.com/maestro-go/maestro/internal/pkg/testing"
	"github.com/stretchr/testify/suite"
)

type MigrateTestSuite struct {
	suite.Suite
	postgres *testUtils.PostgresContainer
	suiteDb  *sql.DB

	ctx context.Context

	repository database.Repository
}

func (s *MigrateTestSuite) SetupSuite() {
	s.ctx = context.Background()

	s.postgres = testUtils.SetupPostgres(s.T())

	db, err := sql.Open("postgres", s.postgres.URI)
	s.Assert().NoError(err)

	s.suiteDb = db

	s.repository = postgres.NewPostgresRepository(s.ctx, db)
}

func (s *MigrateTestSuite) TearDownTest() {
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

func (s *MigrateTestSuite) TearDownSuite() {
	if s.suiteDb != nil {
		s.suiteDb.Close()
	}
}

func (s *MigrateTestSuite) insertMigration(dir string, version uint16, description, content string) {
	migrationFile := filepath.Join(dir, fmt.Sprintf("V%.3d_%s.sql", version, description))
	err := os.WriteFile(migrationFile, []byte(content), os.ModePerm)
	s.Require().NoError(err)
}

func (s *MigrateTestSuite) checkTableExists(table string, shouldExist bool) {
	s.T().Helper()

	query := `
        SELECT EXISTS (
            SELECT 1
            FROM information_schema.tables
            WHERE table_schema = 'public'
            AND table_name = $1
        );
    `

	exists := false
	err := s.suiteDb.QueryRowContext(s.ctx, query, table).Scan(&exists)
	s.Assert().NoError(err)
	s.Assert().Equal(shouldExist, exists)
}

func (s *MigrateTestSuite) TestMigrateUp() {
	migrationsDir := s.T().TempDir()

	upContent1 := "CREATE TABLE test1 (id SERIAL PRIMARY KEY);"
	upContent2 := "CREATE TABLE test2 (id SERIAL PRIMARY KEY);"

	s.insertMigration(migrationsDir, 1, "test1", upContent1)
	s.insertMigration(migrationsDir, 2, "test2", upContent2)

	// Setup migrate command
	migrateCmd := SetupMigrateCommand()
	flags.SetupGlobalFlags(migrateCmd)

	// Run migrate command
	migrateCmd.SetArgs([]string{"-m", migrationsDir, "--driver", "postgres", "--user", s.postgres.Username,
		"--password", s.postgres.Password, "--host", "localhost", "--database", s.postgres.Database, "--port", s.postgres.Port})
	migrateCmd.Execute()

	// Check if the tables are created
	s.checkTableExists("test1", true)
	s.checkTableExists("test2", true)
}

func TestMigrateTestSuite(t *testing.T) {
	suite.Run(t, new(MigrateTestSuite))
}
