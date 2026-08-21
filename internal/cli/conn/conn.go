package conn

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/maestro-go/maestro/core/conf"
	"github.com/maestro-go/maestro/core/database"
	"github.com/maestro-go/maestro/core/database/clickhouse"
	"github.com/maestro-go/maestro/core/database/cockroachdb"
	"github.com/maestro-go/maestro/core/database/mssql"
	"github.com/maestro-go/maestro/core/database/mysql"
	"github.com/maestro-go/maestro/core/database/oracle"
	"github.com/maestro-go/maestro/core/database/postgres"
	"github.com/maestro-go/maestro/core/database/snowflake"
	"github.com/maestro-go/maestro/core/database/sqlite3"
	"github.com/maestro-go/maestro/core/enums"

	"github.com/snowflakedb/gosnowflake"

	"github.com/maestro-go/maestro/internal/ssh"
	"github.com/maestro-go/maestro/internal/utils/net"
	"go.uber.org/zap"
)

// ConnectToDatabase establishes a connection to a database based on the provided configuration and driver type.
// It returns a repository interface for database operations, a cleanup function to release resources, and an error if any.
func ConnectToDatabase(ctx context.Context, logger *zap.Logger, config *conf.ProjectConfig, driver enums.DriverType) (database.Repository, func(), error) {
	repo := (database.Repository)(nil)
	db := (*sql.DB)(nil)
	var tunnel *ssh.Tunnel

	if config.SSH.Host != "" {
		localPort, err := net.GetFreePort()
		if err != nil {
			return nil, nil, fmt.Errorf("failed to get free port for SSH tunnel: %w", err)
		}

		logger.Info("Starting SSH tunnel",
			zap.String("ssh_host", config.SSH.Host),
			zap.Uint16("ssh_port", config.SSH.Port),
			zap.Uint16("local_port", localPort),
			zap.String("remote_db_host", config.Host),
			zap.Uint16("remote_db_port", config.Port),
		)

		tunnel = ssh.NewTunnel(&config.SSH, config.Host, config.Port, localPort)
		if err := tunnel.Start(ctx); err != nil {
			return nil, nil, fmt.Errorf("failed to start SSH tunnel: %w", err)
		}

		// Update config to connect through the tunnel
		config.Host = "localhost"
		config.Port = localPort
	}

	switch driver {
	case enums.DRIVER_POSTGRES, enums.DRIVER_COCKROACHDB:
		var err error
		db, err = connectToPostgres(config)
		if err != nil {
			if tunnel != nil {
				tunnel.Close()
			}
			return nil, nil, err
		}

		db.SetMaxOpenConns(25)
		db.SetMaxIdleConns(25)
		db.SetConnMaxLifetime(5 * time.Minute)

		if driver == enums.DRIVER_POSTGRES {
			repo = postgres.NewPostgresRepository(ctx, db, &config.HistoryTable)
		} else {
			repo = cockroachdb.NewCockroachRepository(ctx, db, &config.HistoryTable)
		}

	case enums.DRIVER_MYSQL:
		var err error
		db, err = connectToMySQL(config)
		if err != nil {
			if tunnel != nil {
				tunnel.Close()
			}
			return nil, nil, err
		}

		db.SetMaxOpenConns(25)
		db.SetMaxIdleConns(25)
		db.SetConnMaxLifetime(5 * time.Minute)

		repo = mysql.NewMySQLRepository(ctx, db, &config.HistoryTable)

	case enums.DRIVER_CLICKHOUSE:
		var err error
		db, err = connectToClickHouse(config)
		if err != nil {
			if tunnel != nil {
				tunnel.Close()
			}
			return nil, nil, err
		}

		db.SetMaxOpenConns(25)
		db.SetMaxIdleConns(25)
		db.SetConnMaxLifetime(5 * time.Minute)

		repo = clickhouse.NewClickHouseRepository(ctx, db, &config.HistoryTable)

	case enums.DRIVER_ORACLE:
		var err error
		db, err = connectToOracle(config)
		if err != nil {
			if tunnel != nil {
				tunnel.Close()
			}
			return nil, nil, err
		}

		db.SetMaxOpenConns(25)
		db.SetMaxIdleConns(25)
		db.SetConnMaxLifetime(5 * time.Minute)

		repo = oracle.NewOracleRepository(ctx, db, &config.HistoryTable)

	case enums.DRIVER_SQLITE3:
		var err error
		if driver == enums.DRIVER_SQLITE3 {
			db, err = sql.Open("sqlite3", config.Database)
		}
		if err != nil {
			if tunnel != nil {
				tunnel.Close()
			}
			return nil, nil, err
		}

		repo = sqlite3.NewSQLiteRepository(ctx, db, &config.HistoryTable)

	case enums.DRIVER_MSSQL:
		var err error
		db, err = connectToMSSQL(config)
		if err != nil {
			if tunnel != nil {
				tunnel.Close()
			}
			return nil, nil, err
		}

		db.SetMaxOpenConns(25)
		db.SetMaxIdleConns(25)
		db.SetConnMaxLifetime(5 * time.Minute)

		repo = mssql.NewMSSQLRepository(ctx, db, &config.HistoryTable)

	case enums.DRIVER_SNOWFLAKE:
		var err error
		db, err = connectToSnowflake(config)
		if err != nil {
			if tunnel != nil {
				tunnel.Close()
			}
			return nil, nil, err
		}

		db.SetMaxOpenConns(25)
		db.SetMaxIdleConns(25)
		db.SetConnMaxLifetime(5 * time.Minute)

		repo = snowflake.NewSnowflakeRepository(ctx, db, &config.HistoryTable)

	default:
		if tunnel != nil {
			tunnel.Close()
		}
		return nil, nil, fmt.Errorf("unsupported driver type: %d", driver)
	}

	cleanup := func() {
		db.Close()
		if tunnel != nil {
			tunnel.Close()
		}
	}

	return repo, cleanup, nil
}

func connectToPostgres(config *conf.ProjectConfig) (*sql.DB, error) {
	var connStr string

	connStr = buildConnectionString(config, config.Host, config.Port)

	// Add SSL configuration if needed
	if config.SSL.SSLRootCert != "" {
		connStr += fmt.Sprintf(" sslrootcert=%s", config.SSL.SSLRootCert)
	}

	// Establish database connection
	db, err := sql.Open("postgres", connStr)
	if err != nil {
		return nil, fmt.Errorf("database connection failed: %w", err)
	}

	// Verify connection
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return nil, fmt.Errorf("database ping failed: %w", err)
	}

	return db, nil
}

func connectToMySQL(config *conf.ProjectConfig) (*sql.DB, error) {
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?parseTime=true",
		config.User,
		config.Password,
		config.Host,
		config.Port,
		config.Database,
	)

	// Establish database connection
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, fmt.Errorf("database connection failed: %w", err)
	}

	// Verify connection
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return nil, fmt.Errorf("database ping failed: %w", err)
	}

	return db, nil
}

func connectToClickHouse(config *conf.ProjectConfig) (*sql.DB, error) {
	dsn := fmt.Sprintf("clickhouse://%s:%s@%s:%d/%s",
		config.User,
		config.Password,
		config.Host,
		config.Port,
		config.Database,
	)

	// Establish database connection
	db, err := sql.Open("clickhouse", dsn)
	if err != nil {
		return nil, fmt.Errorf("database connection failed: %w", err)
	}

	// Verify connection
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return nil, fmt.Errorf("database ping failed: %w", err)
	}

	return db, nil
}

func connectToOracle(config *conf.ProjectConfig) (*sql.DB, error) {
	port := config.Port
	if port == 0 {
		port = 1521
	}

	// go-ora format: oracle://user:password@host:port/service_name
	dsn := fmt.Sprintf("oracle://%s:%s@%s:%d/%s",
		config.User,
		config.Password,
		config.Host,
		port,
		config.Database,
	)

	// Establish database connection
	db, err := sql.Open("oracle", dsn)
	if err != nil {
		return nil, fmt.Errorf("database connection failed: %w", err)
	}

	// Verify connection
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return nil, fmt.Errorf("database ping failed: %w", err)
	}

	return db, nil
}

func connectToMSSQL(config *conf.ProjectConfig) (*sql.DB, error) {
	port := config.Port
	if port == 0 {
		port = 1433
	}

	// sqlserver://user:password@host:port?database=dbname
	dsn := fmt.Sprintf("sqlserver://%s:%s@%s:%d?database=%s",
		config.User,
		config.Password,
		config.Host,
		port,
		config.Database,
	)

	// Establish database connection
	db, err := sql.Open("sqlserver", dsn)
	if err != nil {
		return nil, fmt.Errorf("database connection failed: %w", err)
	}

	// Verify connection
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return nil, fmt.Errorf("database ping failed: %w", err)
	}

	return db, nil
}

func connectToSnowflake(config *conf.ProjectConfig) (*sql.DB, error) {
	port := config.Port
	if port == 0 {
		port = 443
	}

	// For custom endpoints (e.g. the LocalStack Snowflake emulator) HTTP is used,
	// while the default Snowflake service communicates over HTTPS.
	protocol := "https"
	if port != 443 {
		protocol = "http"
	}

	cfg := &gosnowflake.Config{
		Account:   config.Account,
		User:      config.User,
		Password:  config.Password,
		Database:  config.Database,
		Schema:    config.Schema,
		Warehouse: config.Warehouse,
		Role:      config.Role,
		Host:      config.Host,
		Port:      int(port),
		Protocol:  protocol,
		Params:    make(map[string]*string),
	}

	connector := gosnowflake.NewConnector(gosnowflake.SnowflakeDriver{}, *cfg)
	db := sql.OpenDB(connector)

	// Verify connection
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return nil, fmt.Errorf("database ping failed: %w", err)
	}

	return db, nil
}

func buildConnectionString(config *conf.ProjectConfig, host string, port uint16) string {
	return fmt.Sprintf(
		"host=%s port=%d dbname=%s user=%s password=%s sslmode=%s search_path=%s",
		host,
		port,
		config.Database,
		config.User,
		config.Password,
		config.SSL.SSLMode,
		config.Schema,
	)
}
