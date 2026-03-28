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
		"--ssh-host", "ssh-host",
		"--ssh-port", "22",
		"--ssh-user", "ssh-user",
		"--ssh-password", "ssh-pass",
		"--ssh-key-path", "/path/to/key",
		"--ssh-passphrase", "ssh-passphrase",
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
	assert.Equal(t, "ssh-host", projectConfig.SSH.Host)
	assert.Equal(t, uint16(22), projectConfig.SSH.Port)
	assert.Equal(t, "ssh-user", projectConfig.SSH.User)
	assert.Equal(t, "ssh-pass", projectConfig.SSH.Password)
	assert.Equal(t, "/path/to/key", projectConfig.SSH.KeyPath)
	assert.Equal(t, "ssh-passphrase", projectConfig.SSH.Passphrase)
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
		"--host", "remotehost",
		"--port", "3306",
		"--database", "newdb",
		"--user", "newuser",
		"--password", "newpass",
		"--schema", "private",
		"--sslmode", "require",
		"--sslrootcert", "/new/cert",
		"--ssh-host", "new-ssh-host",
		"--ssh-port", "2222",
		"--ssh-user", "new-ssh-user",
		"--ssh-password", "new-ssh-pass",
		"--ssh-key-path", "/new/ssh/key",
		"--ssh-passphrase", "new-ssh-passphrase",
		"--migrations", "/new/migrations",
		"--validate=false",
		"--down=true",
		"--in-transaction=false",
		"--destination", "2",
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
	projectConfig.Driver = "postgres"
	projectConfig.Migration.Locations = []string{"/old/migrations"}

	err = flags.MergeDBConfigFlags(cmd, projectConfig)
	require.NoError(t, err)

	err = flags.MergeMigrationLocations(cmd, &projectConfig.Migration)
	require.NoError(t, err)

	err = flags.MergeMigrationsConfigFlags(cmd, &projectConfig.Migration)
	require.NoError(t, err)

	assert.Equal(t, "sqlite3", projectConfig.Driver)
	assert.Equal(t, "remotehost", projectConfig.Host)
	assert.Equal(t, uint16(3306), projectConfig.Port)
	assert.Equal(t, "newdb", projectConfig.Database)
	assert.Equal(t, "newuser", projectConfig.User)
	assert.Equal(t, "newpass", projectConfig.Password)
	assert.Equal(t, "private", projectConfig.Schema)
	assert.Equal(t, "require", projectConfig.SSL.SSLMode)
	assert.Equal(t, "/new/cert", projectConfig.SSL.SSLRootCert)
	assert.Equal(t, "new-ssh-host", projectConfig.SSH.Host)
	assert.Equal(t, uint16(2222), projectConfig.SSH.Port)
	assert.Equal(t, "new-ssh-user", projectConfig.SSH.User)
	assert.Equal(t, "new-ssh-pass", projectConfig.SSH.Password)
	assert.Equal(t, "/new/ssh/key", projectConfig.SSH.KeyPath)
	assert.Equal(t, "new-ssh-passphrase", projectConfig.SSH.Passphrase)
	assert.Equal(t, []string{"/new/migrations"}, projectConfig.Migration.Locations)
	assert.False(t, projectConfig.Migration.Validate)
	assert.True(t, projectConfig.Migration.Down)
	assert.False(t, projectConfig.Migration.InTransaction)
	assert.Equal(t, uint16(2), *projectConfig.Migration.Destination)
	assert.True(t, projectConfig.Migration.Force)
	assert.False(t, projectConfig.Migration.UseRepeatable)
	assert.False(t, projectConfig.Migration.UseBefore)
	assert.False(t, projectConfig.Migration.UseAfter)
	assert.False(t, projectConfig.Migration.UseBeforeEach)
	assert.False(t, projectConfig.Migration.UseAfterEach)
	assert.False(t, projectConfig.Migration.UseBeforeVersion)
	assert.False(t, projectConfig.Migration.UseAfterVersion)
}
