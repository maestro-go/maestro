package migrator

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
	testUtils "github.com/maestro-go/maestro/internal/pkg/testing"
	"github.com/stretchr/testify/suite"
	"go.uber.org/zap"
)

type MigrationTestSuite struct {
	suite.Suite
	postgres *testUtils.PostgresContainer
	suiteDb  *sql.DB

	ctx context.Context

	repository database.Repository
}

func (s *MigrationTestSuite) SetupSuite() {
	s.ctx = context.Background()

	s.postgres = testUtils.SetupPostgres(s.T())

	db, err := sql.Open("postgres", s.postgres.URI)
	s.Assert().NoError(err)

	s.suiteDb = db

	s.repository = postgres.NewPostgresRepository(s.ctx, db)
}

func (s *MigrationTestSuite) TearDownTest() {
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

func (s *MigrationTestSuite) checkTableExists(table string, shouldExist bool) {
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

func (s *MigrationTestSuite) checkTableRecordsCount(table string, count int) {
	s.T().Helper()

	query := fmt.Sprintf("SELECT COUNT(*) FROM %s;", table)

	var actualCount int
	err := s.suiteDb.QueryRowContext(s.ctx, query).Scan(&actualCount)
	s.Assert().NoError(err)
	s.Assert().Equal(count, actualCount)
}

func (s *MigrationTestSuite) insertMigration(dir string, version uint16, name string, content *string, down bool) {
	s.T().Helper()

	migrationName := fmt.Sprintf("V%.3d_%s", version, name)
	if down {
		migrationName += ".down"
	}
	migrationName += ".sql"

	err := os.WriteFile(filepath.Join(dir, migrationName), []byte(*content), os.ModePerm)
	s.Assert().NoError(err)
}

func (s *MigrationTestSuite) insertHook(dir string, order uint16, version uint16, name string, content *string, hType enums.HookType) {
	migrationName := ""
	switch hType {
	case enums.HOOK_BEFORE:
		migrationName = fmt.Sprintf("B%.3d_%s.sql", order, name)
	case enums.HOOK_BEFORE_EACH:
		migrationName = fmt.Sprintf("BE%.3d_%s.sql", order, name)
	case enums.HOOK_BEFORE_VERSION:
		migrationName = fmt.Sprintf("BV%.3d_%.3d_%s.sql", order, version, name)
	case enums.HOOK_AFTER:
		migrationName = fmt.Sprintf("A%.3d_%s.sql", order, name)
	case enums.HOOK_AFTER_EACH:
		migrationName = fmt.Sprintf("AE%.3d_%s.sql", order, name)
	case enums.HOOK_AFTER_VERSION:
		migrationName = fmt.Sprintf("AV%.3d_%.3d_%s.sql", order, version, name)
	case enums.HOOK_REPEATABLE:
		migrationName = fmt.Sprintf("R%.3d_%s.sql", order, name)
	case enums.HOOK_REPEATABLE_DOWN:
		migrationName = fmt.Sprintf("R%.3d_%s.down.sql", order, name)
	}

	err := os.WriteFile(filepath.Join(dir, migrationName), []byte(*content), os.ModePerm)
	s.Assert().NoError(err)
}

func TestMigrationSuite(t *testing.T) {
	suite.Run(t, new(MigrationTestSuite))
}

func (s *MigrationTestSuite) TestMigrateUp() {
	migrationsDir := s.T().TempDir()

	upContent1 := "CREATE TABLE test1 (id SERIAL PRIMARY KEY);"
	upContent2 := "CREATE TABLE test2 (id SERIAL PRIMARY KEY);"

	s.insertMigration(migrationsDir, 1, "test1", &upContent1, false)
	s.insertMigration(migrationsDir, 2, "test2", &upContent2, false)

	before1Content := "CREATE TABLE before (id SERIAL PRIMARY KEY, name varchar(255));"
	before2Content := "CREATE TABLE after_each (id SERIAL PRIMARY KEY, name varchar(255));"
	beforeEachContent := "INSERT INTO before (name) VALUES ('test');"
	beforeVersionContent := "CREATE TABLE before_version2 (id SERIAL PRIMARY KEY);"
	afterEachContent := "INSERT INTO after_each (name) VALUES ('test');"
	afterVersionContent := "CREATE TABLE after_version2 (id SERIAL PRIMARY KEY);"
	afterContent := "CREATE TABLE after (id SERIAL PRIMARY KEY);"
	repeatableContent := "CREATE TABLE repeatable (id SERIAL PRIMARY KEY);"

	s.insertHook(migrationsDir, 1, 0, "test", &before1Content, enums.HOOK_BEFORE)
	s.insertHook(migrationsDir, 2, 0, "test", &before2Content, enums.HOOK_BEFORE)
	s.insertHook(migrationsDir, 1, 0, "test", &beforeEachContent, enums.HOOK_BEFORE_EACH)
	s.insertHook(migrationsDir, 1, 2, "test", &beforeVersionContent, enums.HOOK_BEFORE_VERSION)
	s.insertHook(migrationsDir, 1, 0, "test", &afterContent, enums.HOOK_AFTER)
	s.insertHook(migrationsDir, 1, 0, "test", &afterEachContent, enums.HOOK_AFTER_EACH)
	s.insertHook(migrationsDir, 1, 2, "test", &afterVersionContent, enums.HOOK_AFTER_VERSION)
	s.insertHook(migrationsDir, 1, 0, "test", &repeatableContent, enums.HOOK_REPEATABLE)

	migrator := NewMigrator(zap.NewNop(), s.repository, &conf.MigrationConfig{
		Locations:        []string{migrationsDir},
		Validate:         true,
		Down:             false,
		InTransaction:    true,
		UseRepeatable:    true,
		UseBefore:        true,
		UseAfter:         true,
		UseBeforeEach:    true,
		UseAfterEach:     true,
		UseBeforeVersion: true,
		UseAfterVersion:  true,
	})

	err := migrator.Migrate()
	s.Assert().NoError(err)

	s.checkTableExists("test1", true)
	s.checkTableExists("test2", true)
	s.checkTableExists("before", true)
	s.checkTableExists("after_each", true)
	s.checkTableExists("before_version2", true)
	s.checkTableExists("after_version2", true)
	s.checkTableExists("after", true)
	s.checkTableExists("repeatable", true)

	s.checkTableRecordsCount("before", 2)
	s.checkTableRecordsCount("after_each", 2)
}

func (s *MigrationTestSuite) TestMigrateDown() {
	migrationsDir := s.T().TempDir()

	upContent1 := "CREATE TABLE test1 (id SERIAL PRIMARY KEY);"
	upContent2 := "CREATE TABLE test2 (id SERIAL PRIMARY KEY);"
	downContent1 := "DROP TABLE test1;"
	downContent2 := "DROP TABLE test2;"

	s.insertMigration(migrationsDir, 1, "test1", &upContent1, false)
	s.insertMigration(migrationsDir, 2, "test2", &upContent2, false)
	s.insertMigration(migrationsDir, 1, "test1", &downContent1, true)
	s.insertMigration(migrationsDir, 2, "test2", &downContent2, true)

	beforeContent := "CREATE TABLE repeatable_down (id SERIAL PRIMARY KEY, name varchar(255));"
	repeatableDownContent := "INSERT INTO repeatable_down (name) VALUES ('test');"

	s.insertHook(migrationsDir, 1, 0, "test", &beforeContent, enums.HOOK_BEFORE)
	s.insertHook(migrationsDir, 1, 0, "test", &repeatableDownContent, enums.HOOK_REPEATABLE_DOWN)

	migrator := NewMigrator(zap.NewNop(), s.repository, &conf.MigrationConfig{
		Locations:     []string{migrationsDir},
		Validate:      true,
		Down:          false,
		InTransaction: true,
		UseBefore:     true,
		UseRepeatable: true,
	})

	err := migrator.Migrate()
	s.Assert().NoError(err)

	s.checkTableExists("test1", true)
	s.checkTableExists("test2", true)
	s.checkTableExists("repeatable_down", true)

	migrator.config.Down = true
	migrator.config.Destination = testUtils.ToPtr(uint16(0)) // Reset destination
	err = migrator.Migrate()
	s.Assert().NoError(err)

	s.checkTableExists("test1", false)
	s.checkTableExists("test2", false)
	s.checkTableExists("repeatable_down", true)

	s.checkTableRecordsCount("repeatable_down", 1)
}

func (s *MigrationTestSuite) TestErrors() {
	migrationsDir := s.T().TempDir()

	invalidSql := "INVALID SQL"

	s.insertMigration(migrationsDir, 1, "test1", &invalidSql, false)
	s.insertMigration(migrationsDir, 2, "test2", &invalidSql, false)

	s.insertHook(migrationsDir, 1, 0, "test", &invalidSql, enums.HOOK_BEFORE)
	s.insertHook(migrationsDir, 2, 0, "test", &invalidSql, enums.HOOK_BEFORE)
	s.insertHook(migrationsDir, 1, 0, "test", &invalidSql, enums.HOOK_BEFORE_EACH)
	s.insertHook(migrationsDir, 1, 2, "test", &invalidSql, enums.HOOK_BEFORE_VERSION)
	s.insertHook(migrationsDir, 1, 0, "test", &invalidSql, enums.HOOK_AFTER)
	s.insertHook(migrationsDir, 1, 0, "test", &invalidSql, enums.HOOK_AFTER_EACH)
	s.insertHook(migrationsDir, 1, 2, "test", &invalidSql, enums.HOOK_AFTER_VERSION)
	s.insertHook(migrationsDir, 1, 0, "test", &invalidSql, enums.HOOK_REPEATABLE)

	migrator := NewMigrator(zap.NewNop(), s.repository, &conf.MigrationConfig{
		Locations:        []string{migrationsDir},
		Validate:         true,
		Down:             false,
		InTransaction:    false,
		Force:            true,
		UseRepeatable:    true,
		UseBefore:        true,
		UseAfter:         true,
		UseBeforeEach:    true,
		UseAfterEach:     true,
		UseBeforeVersion: true,
		UseAfterVersion:  true,
	})

	err := migrator.Migrate()
	s.Assert().Error(err)
}
