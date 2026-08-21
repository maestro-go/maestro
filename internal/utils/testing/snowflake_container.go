package testing

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

type SnowflakeContainer struct {
	testcontainers.Container
	DSN      string
	Username string
	Password string
	Account  string
	Database string
	Port     string
}

// SetupSnowflake starts the LocalStack Snowflake emulator. It requires a
// LocalStack auth token (LOCALSTACK_AUTH_TOKEN) to pull and run the image.
// It returns nil when the token is not configured so tests can skip.
func SetupSnowflake(t *testing.T) *SnowflakeContainer {
	ctx := context.Background()
	if os.Getenv("LOCALSTACK_AUTH_TOKEN") == "" {
		return nil
	}

	username := "test"
	password := "test"
	account := "test"
	database := "test"
	req := testcontainers.ContainerRequest{
		Image:        "localstack/snowflake:latest",
		ExposedPorts: []string{"4566/tcp"},
		WaitingFor:   wait.ForListeningPort("4566/tcp"),
		Env: map[string]string{
			"LOCALSTACK_AUTH_TOKEN": os.Getenv("LOCALSTACK_AUTH_TOKEN"),
		},
	}

	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	require.NoError(t, err)

	host, err := container.Host(ctx)
	require.NoError(t, err)

	port, err := container.MappedPort(ctx, "4566")
	require.NoError(t, err)

	dsn := fmt.Sprintf("%s:%s@%s:%s/%s/PUBLIC?account=%s&protocol=http",
		username, password, host, port.Port(), database, account)

	snowflake := &SnowflakeContainer{
		Container: container,
		DSN:       dsn,
		Username:  username,
		Password:  password,
		Account:   account,
		Database:  database,
		Port:      port.Port(),
	}

	// Wait for container to be ready
	time.Sleep(5 * time.Second)

	return snowflake
}
