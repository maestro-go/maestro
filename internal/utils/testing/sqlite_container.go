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

type SQLiteContainer struct {
	testcontainers.Container
	URI string
}

func SetupSQLite(t *testing.T) *SQLiteContainer {
	ctx := context.Background()

	dbDir, err := os.MkdirTemp("", "sqlite-test")
	require.NoError(t, err)

	req := testcontainers.ContainerRequest{
		Image:        "nouchka/sqlite3:latest",
		ExposedPorts: []string{"8080/tcp"},
		WaitingFor:   wait.ForListeningPort("8080/tcp").WithStartupTimeout(20 * time.Second),
		Env: map[string]string{
			"DB_FILE": fmt.Sprintf("%s/test.db", dbDir),
		},
	}

	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	require.NoError(t, err)

	host, err := container.Host(ctx)
	require.NoError(t, err)

	port, err := container.MappedPort(ctx, "8080")
	require.NoError(t, err)

	uri := fmt.Sprintf("http://%s:%s", host, port.Port())

	sqlite := &SQLiteContainer{
		Container: container,
		URI:       uri,
	}

	return sqlite
}
