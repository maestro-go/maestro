package testing

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/cockroachdb"
)

type CockroachContainer struct {
	testcontainers.Container
	URI string // Connection URI
}

func SetupCockroach(t *testing.T) *CockroachContainer {
	ctx := context.Background()

	container, err := cockroachdb.Run(ctx, "cockroachdb/cockroach:latest-v23.1",
		cockroachdb.WithInsecure(), cockroachdb.WithDatabase("test_db"),
		cockroachdb.WithUser("test_user"))
	require.NoError(t, err)

	host, err := container.Host(ctx)
	require.NoError(t, err)

	port, err := container.MappedPort(ctx, "26257")
	require.NoError(t, err)

	uri := fmt.Sprintf("postgres://test_user:@%s:%s/test_db?sslmode=disable", host, port.Port())

	cockroach := &CockroachContainer{
		Container: container,
		URI:       uri,
	}

	return cockroach
}
