package conn

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/maestro-go/maestro/core/conf"
	"github.com/maestro-go/maestro/core/database"
	"github.com/maestro-go/maestro/core/database/cockroachdb"
	"github.com/maestro-go/maestro/core/database/postgres"
	"github.com/maestro-go/maestro/core/enums"
)

// ConnectToDatabase establishes a connection to a database based on the provided configuration and driver type.
// It returns a repository interface for database operations, a cleanup function to release resources, and an error if any.
func ConnectToDatabase(ctx context.Context, config *conf.ProjectConfig, driver enums.DriverType) (database.Repository, func(), error) {
	repo := (database.Repository)(nil)
	db := (*sql.DB)(nil)

	switch driver {
	case enums.DRIVER_POSTGRES, enums.DRIVER_COCKROACHDB:
		var err error
		db, err = connectToPostgres(config)
		if err != nil {
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

	default:
		return nil, nil, fmt.Errorf("unsupported driver type: %d", driver)
	}

	cleanup := func() {
		db.Close()
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
