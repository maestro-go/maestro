package cli

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/maestro-go/maestro/internal/cli/flags"
	"github.com/maestro-go/maestro/internal/conf"
	"github.com/stretchr/testify/assert"
)

func TestCreateCommand(t *testing.T) {
	tempDir := t.TempDir()

	createCmd := SetupCreateCommand()
	flags.SetupGlobalFlags(createCmd)

	// Run create command
	createCmd.SetArgs([]string{"test_migration", "-m", tempDir})
	createCmd.Execute()

	// Check if the migration file is created
	migrationFilePath := filepath.Join(tempDir, "V001_test_migration.sql")
	assert.FileExists(t, migrationFilePath)

	// Check if the migration file content is correct
	content, err := os.ReadFile(migrationFilePath)
	assert.NoError(t, err)
	assert.Equal(t, conf.NEW_MIGRATION_PLACEHOLDER, string(content))
}

func TestCreateCommandWithDown(t *testing.T) {
	tempDir := t.TempDir()

	createCmd := SetupCreateCommand()
	flags.SetupGlobalFlags(createCmd)

	// Run create command with --with-down flag
	createCmd.SetArgs([]string{"test_migration", "-m", tempDir, "--with-down"})
	createCmd.Execute()

	// Check if the migration file is created
	migrationFilePath := filepath.Join(tempDir, "V001_test_migration.sql")
	assert.FileExists(t, migrationFilePath)

	// Check if the down migration file is created
	downMigrationFilePath := filepath.Join(tempDir, "V001_test_migration.down.sql")
	assert.FileExists(t, downMigrationFilePath)

	// Check if the migration file content is correct
	content, err := os.ReadFile(migrationFilePath)
	assert.NoError(t, err)
	assert.Equal(t, conf.NEW_MIGRATION_PLACEHOLDER, string(content))

	// Check if the down migration file content is correct
	downContent, err := os.ReadFile(downMigrationFilePath)
	assert.NoError(t, err)
	assert.Equal(t, conf.NEW_MIGRATION_PLACEHOLDER, string(downContent))
}

func TestCreateCommandEmptyMigrationName(t *testing.T) {
	tempDir := t.TempDir()

	createCmd := SetupCreateCommand()
	flags.SetupGlobalFlags(createCmd)

	// Run create command with empty migration name
	createCmd.SetArgs([]string{"", "-m", tempDir})
	createCmd.Execute()

	// Check if no migration file is created
	migrationFiles, err := filepath.Glob(filepath.Join(tempDir, "*.sql"))
	assert.NoError(t, err)
	assert.Empty(t, migrationFiles)
}
