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

type ClickHouseContainer struct {
	testcontainers.Container
	URI      string
	Username string
	Password string
	Database string
	Port     string
	Host     string
}

func SetupClickHouse(t *testing.T) *ClickHouseContainer {
	ctx := context.Background()
	database := "test_db"
	username := "default"
	password := "password"
	req := testcontainers.ContainerRequest{
		Image:        "clickhouse/clickhouse-server:latest",
		ExposedPorts: []string{"9000/tcp", "8123/tcp"},
		WaitingFor:   wait.ForHTTP("/").WithPort("8123/tcp"),
		Env: map[string]string{
			"CLICKHOUSE_DB":       database,
			"CLICKHOUSE_USER":     username,
			"CLICKHOUSE_PASSWORD": password,
			"CLICKHOUSE_DEFAULT_ACCESS_MANAGEMENT": "1",
		},
	}

	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	require.NoError(t, err)

	host, err := container.Host(ctx)
	require.NoError(t, err)

	port, err := container.MappedPort(ctx, "9000")
	require.NoError(t, err)

	uri := fmt.Sprintf("clickhouse://%s:%s@%s:%s/%s", username, password, host, port.Port(), database)

	clickhouse := &ClickHouseContainer{
		Container: container,
		URI:       uri,
		Username:  username,
		Password:  password,
		Database:  database,
		Port:      port.Port(),
		Host:      host,
	}

	// Wait for container to be ready
	time.Sleep(2 * time.Second)

	return clickhouse
}
