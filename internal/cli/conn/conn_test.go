package conn_test

import (
	"context"
	"database/sql"
	"strconv"
	"testing"

	"github.com/creasty/defaults"
	"github.com/maestro-go/maestro/core/conf"
	"github.com/maestro-go/maestro/core/enums"
	"github.com/maestro-go/maestro/internal/cli/conn"
	testUtils "github.com/maestro-go/maestro/internal/utils/testing"
	"github.com/stretchr/testify/suite"

	_ "github.com/lib/pq"
	_ "github.com/mattn/go-sqlite3"
)

type ConnTestSuite struct {
	suite.Suite
	postgres        *testUtils.PostgresContainer
	postgresSuiteDB *sql.DB

	ctx context.Context
}

func (s *ConnTestSuite) SetupSuite() {
	s.ctx = context.Background()

	s.postgres = testUtils.SetupPostgres(s.T())

	pqDB, err := sql.Open("postgres", s.postgres.URI)
	s.Assert().NoError(err)

	s.postgresSuiteDB = pqDB
}

func (s *ConnTestSuite) TearDownSuite() {
	if s.postgresSuiteDB != nil {
		s.postgresSuiteDB.Close()
	}
}

func TestCliTestSuite(t *testing.T) {
	suite.Run(t, new(ConnTestSuite))
}

func (s *ConnTestSuite) TestConn() {
	s.Run("should connect to postgres", func() {
		config := &conf.ProjectConfig{}
		defaults.MustSet(config)
		port, err := strconv.ParseUint(s.postgres.Port, 10, 16)
		s.Require().NoError(err)
		config.Port = uint16(port)
		config.Database = s.postgres.Database
		config.User = s.postgres.Username
		config.Password = s.postgres.Password

		repo, cleanup, err := conn.ConnectToDatabase(s.ctx, config, enums.DRIVER_POSTGRES)
		s.Assert().NoError(err)
		s.Assert().NotNil(repo)
		s.Assert().NotNil(cleanup)
		cleanup()
	})

	s.Run("should connect to sqlite3", func() {
		config := &conf.ProjectConfig{}
		defaults.MustSet(config)
		config.Database = ":memory:"

		repo, cleanup, err := conn.ConnectToDatabase(s.ctx, config, enums.DRIVER_SQLITE3)
		s.Assert().NoError(err)
		s.Assert().NotNil(repo)
		s.Assert().NotNil(cleanup)
		cleanup()
	})

	s.Run("should return error for unsupported driver", func() {
		config := &conf.ProjectConfig{}
		defaults.MustSet(config)

		repo, cleanup, err := conn.ConnectToDatabase(s.ctx, config, 99)
		s.Assert().Error(err)
		s.Assert().Nil(repo)
		s.Assert().Nil(cleanup)
	})
}
