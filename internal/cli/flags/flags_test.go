package flags_test

import (
	"testing"

	"github.com/creasty/defaults"
	"github.com/maestro-go/maestro/core/conf"
	"github.com/maestro-go/maestro/internal/cli/flags"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExtractAllFlags(t *testing.T) {
	cmd := &cobra.Command{}
	flags.SetupDBConfigFlags(cmd)
	flags.SetupGlobalFlags(cmd)
	flags.SetupMigrationConfigFlags(cmd)

	err := cmd.ParseFlags([]string{
		"--driver", "postgres",
		"--host", "localhost",
		"--port", "5432",
		"--database", "testdb",
		"--user", "testuser",
		"--password", "testpass",
		"--schema", "public",
		"--history-table", "history",
		"--sslmode", "disable",
		"--sslrootcert", "/path/to/cert",
		"--location", "/tmp",
		"--migrations", "/migrations",
		"--validate=false",
		"--down=true",
		"--in-transaction=false",
		"--destination", "1",
		"--force=true",
		"--use-repeatable=false",
		"--use-before=false",
		"--use-after=false",
		"--use-before-each=false",
		"--use-after-each=false",
		"--use-before-version=false",
		"--use-after-version=false",
	})
	require.NoError(t, err)

	projectConfig := &conf.ProjectConfig{}
	defaults.MustSet(projectConfig)

	err = flags.ExtractDBConfigFlags(cmd, projectConfig)
	require.NoError(t, err)

	globalFlags, err := flags.ExtractGlobalFlags(cmd)
	require.NoError(t, err)

	err = flags.ExtractMigrationConfigFlags(cmd, &projectConfig.Migration)
	require.NoError(t, err)

	assert.Equal(t, "postgres", projectConfig.Driver)
	assert.Equal(t, "localhost", projectConfig.Host)
	assert.Equal(t, uint16(5432), projectConfig.Port)
	assert.Equal(t, "testdb", projectConfig.Database)
	assert.Equal(t, "testuser", projectConfig.User)
	assert.Equal(t, "testpass", projectConfig.Password)
	assert.Equal(t, "public", projectConfig.Schema)
	assert.Equal(t, "history", projectConfig.HistoryTable)
	assert.Equal(t, "disable", projectConfig.SSL.SSLMode)
	assert.Equal(t, "/path/to/cert", projectConfig.SSL.SSLRootCert)
	assert.Equal(t, "/tmp", globalFlags.Location)
	assert.Equal(t, []string{"/migrations"}, globalFlags.MigrationLocations)
	assert.False(t, projectConfig.Migration.Validate)
	assert.True(t, projectConfig.Migration.Down)
	assert.False(t, projectConfig.Migration.InTransaction)
	assert.Equal(t, uint16(1), *projectConfig.Migration.Destination)
	assert.True(t, projectConfig.Migration.Force)
	assert.False(t, projectConfig.Migration.UseRepeatable)
	assert.False(t, projectConfig.Migration.UseBefore)
	assert.False(t, projectConfig.Migration.UseAfter)
	assert.False(t, projectConfig.Migration.UseBeforeEach)
	assert.False(t, projectConfig.Migration.UseAfterEach)
	assert.False(t, projectConfig.Migration.UseBeforeVersion)
	assert.False(t, projectConfig.Migration.UseAfterVersion)
}

func TestMergeFlagsIfChanged(t *testing.T) {
	cmd := &cobra.Command{}
	flags.SetupDBConfigFlags(cmd)
	flags.SetupGlobalFlags(cmd)
	flags.SetupMigrationConfigFlags(cmd)

	err := cmd.ParseFlags([]string{
		"--driver", "sqlite3",
		"--migrations", "/new/migrations",
		"--destination", "2",
	})
	require.NoError(t, err)

	projectConfig := &conf.ProjectConfig{}
	defaults.MustSet(projectConfig)
	projectConfig.Driver = "postgres"
	projectConfig.Migration.Locations = []string{"/old/migrations"}

	err = flags.MergeDBConfigFlags(cmd, projectConfig)
	require.NoError(t, err)

	err = flags.MergeMigrationLocations(cmd, &projectConfig.Migration)
	require.NoError(t, err)

	err = flags.MergeMigrationsConfigFlags(cmd, &projectConfig.Migration)
	require.NoError(t, err)

	assert.Equal(t, "sqlite3", projectConfig.Driver)
	assert.Equal(t, []string{"/new/migrations"}, projectConfig.Migration.Locations)
	assert.Equal(t, uint16(2), *projectConfig.Migration.Destination)
}
