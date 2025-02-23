package migrator

import (
	"context"
	"database/sql"
	"testing"

	_ "github.com/lib/pq"
	"github.com/maestro-go/maestro/core/conf"
	"github.com/maestro-go/maestro/core/database"
	"github.com/maestro-go/maestro/core/database/postgres"
	"github.com/maestro-go/maestro/core/enums"
	"github.com/maestro-go/maestro/internal/migrations"
	testUtils "github.com/maestro-go/maestro/internal/pkg/testing"
	"github.com/stretchr/testify/suite"
	"go.uber.org/zap"
)

type MigratorTestSuite struct {
	suite.Suite
	postgres   *testUtils.PostgresContainer
	suiteDb    *sql.DB
	ctx        context.Context
	repository database.Repository
	logger     *zap.Logger
	config     *conf.MigrationConfig
	migrator   *Migrator
}

func (s *MigratorTestSuite) SetupSuite() {
	s.ctx = context.Background()
	s.postgres = testUtils.SetupPostgres(s.T())

	db, err := sql.Open("postgres", s.postgres.URI)
	s.Require().NoError(err)
	s.suiteDb = db

	s.repository = postgres.NewPostgresRepository(s.ctx, db)
	s.logger, _ = zap.NewDevelopment()
	s.config = &conf.MigrationConfig{
		Down:          false,
		Validate:      true,
		InTransaction: true,
		UseBefore:     true,
		UseAfter:      true,
		UseBeforeEach: true,
		UseAfterEach:  true,
		UseRepeatable: true,
		Force:         false,
	}
	s.migrator = NewMigrator(s.logger, s.repository, s.config)
}

func (s *MigratorTestSuite) TearDownSuite() {
	if s.postgres != nil {
		s.postgres.Terminate(s.ctx)
	}
}

func (s *MigratorTestSuite) TestMigrateUp() {
	err := s.repository.AssertSchemaHistoryTable()
	s.Require().NoError(err)

	migrationsMap := []*migrations.Migration{
		{
			Version:     1,
			Description: "Create users table",
			Type:        enums.MIGRATION_UP,
			Content:     testUtils.ToPtr("CREATE TABLE users (id SERIAL PRIMARY KEY, name TEXT);"),
			Checksum:    testUtils.ToPtr("d41d8cd98f00b204e9800998ecf8427e"),
		},
		{
			Version:     2,
			Description: "Add email to users table",
			Type:        enums.MIGRATION_UP,
			Content:     testUtils.ToPtr("ALTER TABLE users ADD COLUMN email TEXT;"),
			Checksum:    testUtils.ToPtr("d41d8cd98f00b204e9800998ecf8427e"),
		},
	}

	hooks := map[enums.HookType][]*migrations.Hook{
		enums.HOOK_BEFORE: {
			{
				Order:   1,
				Content: testUtils.ToPtr("SELECT 1;"),
				Type:    enums.HOOK_BEFORE,
			},
		},
		enums.HOOK_AFTER: {
			{
				Order:   1,
				Content: testUtils.ToPtr("SELECT 1;"),
				Type:    enums.HOOK_AFTER,
			},
		},
	}

	errs := s.migrator.migrateUp(migrationsMap, hooks, 1, 2)
	s.Require().Nil(errs)

	s.checkTableExists("users", true)
}

func (s *MigratorTestSuite) TestMigrateDown() {
	err := s.repository.AssertSchemaHistoryTable()
	s.Require().NoError(err)

	migrationsMap := []*migrations.Migration{
		{
			Version:     1,
			Description: "Drop users table",
			Type:        enums.MIGRATION_DOWN,
			Content:     testUtils.ToPtr("DROP TABLE IF EXISTS users;"),
		},
	}

	hooks := map[enums.HookType][]*migrations.Hook{
		enums.HOOK_BEFORE: {
			{
				Order:   1,
				Content: testUtils.ToPtr("SELECT 1;"),
				Type:    enums.HOOK_BEFORE,
			},
		},
		enums.HOOK_AFTER: {
			{
				Order:   1,
				Content: testUtils.ToPtr("SELECT 1;"),
				Type:    enums.HOOK_AFTER,
			},
		},
	}

	errs := s.migrator.migrateDown(migrationsMap, hooks, 1, 0)
	s.Require().Nil(errs)

	s.checkTableExists("users", false)
}

func (s *MigratorTestSuite) TestExecuteHooks() {
	hooks := []*migrations.Hook{
		{
			Order:   1,
			Content: testUtils.ToPtr("SELECT 1;"),
			Type:    enums.HOOK_BEFORE,
		},
	}

	errs := s.migrator.executeHooks(hooks)
	s.Require().Nil(errs)
}

func (s *MigratorTestSuite) TestExecuteVersionedHooks() {
	hooks := []*migrations.Hook{
		{
			Version: 1,
			Order:   1,
			Content: testUtils.ToPtr("SELECT 1;"),
			Type:    enums.HOOK_BEFORE_VERSION,
		},
	}

	errs := s.migrator.executeVersionedHooks(1, hooks)
	s.Require().Nil(errs)
}

func (s *MigratorTestSuite) checkTableExists(table string, shouldExist bool) {
	s.T().Helper()

	query := `
		SELECT EXISTS (
			SELECT FROM information_schema.tables
			WHERE table_name = $1
		);
	`

	exists := false
	err := s.suiteDb.QueryRowContext(s.ctx, query, table).Scan(&exists)
	s.Require().NoError(err)
	s.Require().Equal(shouldExist, exists)
}

func TestMigratorSuite(t *testing.T) {
	suite.Run(t, new(MigratorTestSuite))
}
