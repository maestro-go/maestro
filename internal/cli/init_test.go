package cli

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/maestro-go/maestro/internal/cli/flags"
	internalConf "github.com/maestro-go/maestro/internal/conf"
	"github.com/stretchr/testify/assert"
)

func TestInitCommand(t *testing.T) {
	tempDir := t.TempDir()
	migrationDir1 := filepath.Join(tempDir, "migrations1")
	migrationDir2 := filepath.Join(tempDir, "migrations2")

	initCmd := SetupInitCommand()
	flags.SetupGlobalFlags(initCmd)

	// Run init command
	initCmd.SetArgs([]string{"-l", tempDir, "-m", migrationDir1, "-m", migrationDir2})
	initCmd.Execute()

	// Check if the config file is created
	configFilePath := filepath.Join(tempDir, internalConf.DEFAULT_PROJECT_FILE)
	assert.FileExists(t, configFilePath)

	// Check if the migration directories are created
	assert.DirExists(t, migrationDir1)
	assert.DirExists(t, migrationDir2)

	// Check if the example migration files are created
	exampleMigrationFile1 := filepath.Join(migrationDir1, "V001_example.sql")
	exampleMigrationFile2 := filepath.Join(migrationDir2, "V002_example.sql")
	assert.FileExists(t, exampleMigrationFile1)
	assert.FileExists(t, exampleMigrationFile2)

	// Check if the example migration file content is correct
	content1, err := os.ReadFile(exampleMigrationFile1)
	assert.NoError(t, err)
	assert.Equal(t, internalConf.NEW_MIGRATION_PLACEHOLDER, string(content1))

	content2, err := os.ReadFile(exampleMigrationFile2)
	assert.NoError(t, err)
	assert.Equal(t, internalConf.NEW_MIGRATION_PLACEHOLDER, string(content2))
}

func TestInitCommandAlreadyInitialized(t *testing.T) {
	tempDir := t.TempDir()
	configFilePath := filepath.Join(tempDir, internalConf.DEFAULT_PROJECT_FILE)
	err := os.WriteFile(configFilePath, []byte("existing content"), os.ModePerm)
	assert.NoError(t, err)

	initCmd := SetupInitCommand()
	flags.SetupGlobalFlags(initCmd)

	// Run init command
	initCmd.SetArgs([]string{"-l", tempDir})
	initCmd.Execute()

	// Check if the existing config file is not overwritten
	content, err := os.ReadFile(configFilePath)
	assert.NoError(t, err)
	assert.Equal(t, "existing content", string(content))
}
