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

type OracleContainer struct {
	testcontainers.Container
	URI      string
	Username string
	Password string
	Database string
	Port     string
}

func SetupOracle(t *testing.T) *OracleContainer {
	ctx := context.Background()
	database := "FREEPDB1"
	username := "SYSTEM"
	password := "password"
	req := testcontainers.ContainerRequest{
		Image:        "gvenzl/oracle-free:23.4-slim",
		ExposedPorts: []string{"1521/tcp"},
		WaitingFor:   wait.ForLog("DATABASE IS READY TO USE!"),
		Env: map[string]string{
			"ORACLE_PASSWORD": password,
		},
	}

	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	require.NoError(t, err)

	host, err := container.Host(ctx)
	require.NoError(t, err)

	port, err := container.MappedPort(ctx, "1521")
	require.NoError(t, err)

	uri := fmt.Sprintf("oracle://%s:%s@%s:%s/%s", username, password, host, port.Port(), database)

	oracle := &OracleContainer{
		Container: container,
		URI:       uri,
		Username:  username,
		Password:  password,
		Database:  database,
		Port:      port.Port(),
	}

	// Wait for container to be ready
	time.Sleep(2 * time.Second)

	return oracle
}
