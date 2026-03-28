package conn_test

import (
	"context"
	"strconv"
	"testing"

	"github.com/creasty/defaults"
	"github.com/maestro-go/maestro/core/conf"
	"github.com/maestro-go/maestro/core/enums"
	"github.com/maestro-go/maestro/internal/cli/conn"
	testUtils "github.com/maestro-go/maestro/internal/utils/testing"
	"github.com/stretchr/testify/suite"
	"go.uber.org/zap"

	_ "github.com/go-sql-driver/mysql"
	_ "github.com/lib/pq"
	_ "github.com/mattn/go-sqlite3"
)

type ConnTestSuite struct {
	suite.Suite
	postgres *testUtils.PostgresContainer
	mysql    *testUtils.MySQLContainer
	ctx      context.Context
	logger   *zap.Logger
}

func (s *ConnTestSuite) SetupSuite() {
	s.ctx = context.Background()
	s.logger = zap.NewNop()
	s.postgres = testUtils.SetupPostgres(s.T())

	var err error
	s.mysql, err = testUtils.SetupMySQLContainer(s.ctx)
	s.Require().NoError(err)
}

func (s *ConnTestSuite) TearDownSuite() {
	if s.mysql != nil {
		s.mysql.Teardown(s.ctx)
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
		config.SSL.SSLMode = "disable"

		repo, cleanup, err := conn.ConnectToDatabase(s.ctx, s.logger, config, enums.DRIVER_POSTGRES)
		s.Assert().NoError(err)
		s.Assert().NotNil(repo)
		s.Assert().NotNil(cleanup)
		cleanup()
	})

	s.Run("should connect to mysql", func() {
		config := &conf.ProjectConfig{}
		defaults.MustSet(config)
		
		host, err := s.mysql.Host(s.ctx)
		s.Require().NoError(err)
		port, err := s.mysql.MappedPort(s.ctx, "3306")
		s.Require().NoError(err)
		
		config.Host = host
		config.Port = uint16(port.Int())
		config.Database = "testdb"
		config.User = "testuser"
		config.Password = "testpassword"

		repo, cleanup, err := conn.ConnectToDatabase(s.ctx, s.logger, config, enums.DRIVER_MYSQL)
		s.Assert().NoError(err)
		s.Assert().NotNil(repo)
		s.Assert().NotNil(cleanup)
		cleanup()
	})

	s.Run("should connect to sqlite3", func() {
		config := &conf.ProjectConfig{}
		defaults.MustSet(config)
		config.Database = ":memory:"

		repo, cleanup, err := conn.ConnectToDatabase(s.ctx, s.logger, config, enums.DRIVER_SQLITE3)
		s.Assert().NoError(err)
		s.Assert().NotNil(repo)
		s.Assert().NotNil(cleanup)
		cleanup()
	})

	s.Run("should return error for unsupported driver", func() {
		config := &conf.ProjectConfig{}
		defaults.MustSet(config)

		repo, cleanup, err := conn.ConnectToDatabase(s.ctx, s.logger, config, 99)
		s.Assert().Error(err)
		s.Assert().Nil(repo)
		s.Assert().Nil(cleanup)
	})

	s.Run("should fail to connect to postgres with wrong port", func() {
		config := &conf.ProjectConfig{}
		defaults.MustSet(config)
		config.Port = 1234
		config.SSL.SSLMode = "disable"

		repo, cleanup, err := conn.ConnectToDatabase(s.ctx, s.logger, config, enums.DRIVER_POSTGRES)
		s.Assert().Error(err)
		s.Assert().Nil(repo)
		s.Assert().Nil(cleanup)
	})

	s.Run("should fail to connect to mysql with wrong port", func() {
		config := &conf.ProjectConfig{}
		defaults.MustSet(config)
		config.Port = 1234

		repo, cleanup, err := conn.ConnectToDatabase(s.ctx, s.logger, config, enums.DRIVER_MYSQL)
		s.Assert().Error(err)
		s.Assert().Nil(repo)
		s.Assert().Nil(cleanup)
	})
}
