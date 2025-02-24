package testing

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

type PostgresContainer struct {
	testcontainers.Container
	URI      string
	Username string
	Password string
	Database string
	Port     string
}

func SetupPostgres(t *testing.T) *PostgresContainer {
	ctx := context.Background()
	database := "test_db"
	username := "test_user"
	password := "password"
	req := testcontainers.ContainerRequest{
		Image:        "postgres:17-alpine",
		ExposedPorts: []string{"5432/tcp"},
		WaitingFor:   wait.ForLog("database system is ready to accept connections"),
		Env: map[string]string{
			"POSTGRES_DB":       database,
			"POSTGRES_USER":     username,
			"POSTGRES_PASSWORD": password,
		},
	}

	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	require.NoError(t, err)

	host, err := container.Host(ctx)
	require.NoError(t, err)

	port, err := container.MappedPort(ctx, "5432")
	require.NoError(t, err)

	uri := fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=disable", username, password, host, port.Port(), database)

	postgres := &PostgresContainer{
		Container: container,
		URI:       uri,
		Username:  username,
		Password:  password,
		Database:  database,
		Port:      port.Port(),
	}

	// Wait for container to be ready
	time.Sleep(2 * time.Second)

	return postgres
}
