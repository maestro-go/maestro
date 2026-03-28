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

type MSSQLContainer struct {
	testcontainers.Container
	URI      string
	Username string
	Password string
	Database string
	Port     string
}

func SetupMSSQL(t *testing.T) *MSSQLContainer {
	ctx := context.Background()
	database := "master"
	username := "sa"
	password := "Password123!"
	req := testcontainers.ContainerRequest{
		Image:        "mcr.microsoft.com/mssql/server:2022-latest",
		ExposedPorts: []string{"1433/tcp"},
		WaitingFor:   wait.ForLog("SQL Server is now ready for client connections"),
		Env: map[string]string{
			"ACCEPT_EULA":       "Y",
			"MSSQL_SA_PASSWORD": password,
		},
	}

	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	require.NoError(t, err)

	host, err := container.Host(ctx)
	require.NoError(t, err)

	port, err := container.MappedPort(ctx, "1433")
	require.NoError(t, err)

	uri := fmt.Sprintf("sqlserver://%s:%s@%s:%s?database=%s", username, password, host, port.Port(), database)

	mssql := &MSSQLContainer{
		Container: container,
		URI:       uri,
		Username:  username,
		Password:  password,
		Database:  database,
		Port:      port.Port(),
	}

	// Wait for container to be ready
	time.Sleep(5 * time.Second)

	return mssql
}
