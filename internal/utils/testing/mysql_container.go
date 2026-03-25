package testing

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/mysql"
)

type MySQLContainer struct {
	*mysql.MySQLContainer
	ConnectionString string
}

func SetupMySQLContainer(ctx context.Context) (*MySQLContainer, error) {
	dbName := "testdb"
	dbUser := "testuser"
	dbPassword := "testpassword"

	mysqlContainer, err := mysql.Run(ctx,
		"mysql:8.0",
		mysql.WithDatabase(dbName),
		mysql.WithUsername(dbUser),
		mysql.WithPassword(dbPassword),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to start container: %w", err)
	}

	connStr, err := mysqlContainer.ConnectionString(ctx, "parseTime=true")
	if err != nil {
		return nil, fmt.Errorf("failed to get connection string: %w", err)
	}

	return &MySQLContainer{
		MySQLContainer:   mysqlContainer,
		ConnectionString: connStr,
	}, nil
}

func (c *MySQLContainer) GetDB() (*sql.DB, error) {
	db, err := sql.Open("mysql", c.ConnectionString)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := db.PingContext(ctx); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	return db, nil
}

func (c *MySQLContainer) Teardown(ctx context.Context) error {
	return testcontainers.TerminateContainer(c.MySQLContainer)
}
