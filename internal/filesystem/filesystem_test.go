package filesystem_test

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/maestro-go/maestro/core/conf"
	"github.com/maestro-go/maestro/core/enums"
	"github.com/maestro-go/maestro/internal/filesystem"
	"github.com/stretchr/testify/assert"
)

func TestGetLatestVersionFromFiles(t *testing.T) {
	migrationsDir1 := t.TempDir()
	migrationsDir2 := t.TempDir()

	err := os.WriteFile(filepath.Join(migrationsDir1, "V001_test1.sql"), []byte(""), os.ModePerm)
	assert.NoError(t, err)

	err = os.WriteFile(filepath.Join(migrationsDir2, "V002_test2.sql"), []byte(""), os.ModePerm)
	assert.NoError(t, err)

	latest, err := filesystem.GetLatestVersionFromFiles([]string{migrationsDir1, migrationsDir2})
	assert.NoError(t, err)
	assert.Equal(t, uint16(2), latest)
}

func TestCheckFSObject(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "test_file.txt")

	// Check for a non-existent file
	exists, err := filesystem.CheckFSObject(filePath)
	assert.NoError(t, err)
	assert.False(t, exists)

	// Create the file
	err = os.WriteFile(filePath, []byte("test"), 0644)
	assert.NoError(t, err)

	// Check for an existent file
	exists, err = filesystem.CheckFSObject(filePath)
	assert.NoError(t, err)
	assert.True(t, exists)

	// Check for a non-existent file in the same folder
	exists, err = filesystem.CheckFSObject(fmt.Sprintf("%s/test_file2.txt", dir))
	assert.NoError(t, err)
	assert.False(t, exists)
}

func TestLoadObjectsFromFiles(t *testing.T) {
	// Setup test
	migrationsDir1 := t.TempDir()
	migrationsDir2 := t.TempDir()

	config := &conf.MigrationConfig{
		Down:             false,
		UseRepeatable:    true,
		UseBefore:        true,
		UseBeforeEach:    true,
		UseBeforeVersion: true,
		UseAfter:         true,
		UseAfterEach:     true,
		UseAfterVersion:  true,
		Locations:        []string{migrationsDir1, migrationsDir2},
	}

	migration1Content := "SAMPLE CONTENT"
	migration2Content := "SAMPLE CONTENT WITH TEMPLATE {{ test, 10 }}"

	repeatable1Content := "SAMPLE REPEATABLE CONTENT"
	before1Content := "SAMPLE BEFORE CONTENT"
	beforeEach1Content := "SAMPLE BEFORE EACH CONTENT"
	beforeVersion1Content := "SAMPLE BEFORE VERSION CONTENT"
	after1Content := "SAMPLE AFTER CONTENT"
	afterEach1Content := "SAMPLE AFTER EACH CONTENT"
	afterVersion1Content := "SAMPLE AFTER VERSION CONTENT"

	templateTestContent := "TEST TEMPLATE $1 CONTENT"

	err := os.WriteFile(filepath.Join(migrationsDir1, "V001_test1.sql"), []byte(migration1Content), os.ModePerm)
	assert.NoError(t, err)

	err = os.WriteFile(filepath.Join(migrationsDir2, "V002_test2.sql"), []byte(migration2Content), os.ModePerm)
	assert.NoError(t, err)

	err = os.WriteFile(filepath.Join(migrationsDir1, "R001_test1_repeatable.sql"), []byte(repeatable1Content), os.ModePerm)
	assert.NoError(t, err)

	err = os.WriteFile(filepath.Join(migrationsDir2, "B001_test1_before.sql"), []byte(before1Content), os.ModePerm)
	assert.NoError(t, err)

	err = os.WriteFile(filepath.Join(migrationsDir1, "BE001_test1_before_each.sql"), []byte(beforeEach1Content), os.ModePerm)
	assert.NoError(t, err)

	err = os.WriteFile(filepath.Join(migrationsDir1, "BV001_001_test1_before_version.sql"), []byte(beforeVersion1Content), os.ModePerm)
	assert.NoError(t, err)

	err = os.WriteFile(filepath.Join(migrationsDir1, "A001_test1_after.sql"), []byte(after1Content), os.ModePerm)
	assert.NoError(t, err)

	err = os.WriteFile(filepath.Join(migrationsDir1, "AE001_test1_after_each.sql"), []byte(afterEach1Content), os.ModePerm)
	assert.NoError(t, err)

	err = os.WriteFile(filepath.Join(migrationsDir1, "AV001_001_test1_after_version.sql"), []byte(afterVersion1Content), os.ModePerm)
	assert.NoError(t, err)

	err = os.WriteFile(filepath.Join(migrationsDir2, "test.template.sql"), []byte(templateTestContent), os.ModePerm)
	assert.NoError(t, err)

	// Assert setup
	entries1, err := os.ReadDir(migrationsDir1)
	assert.NoError(t, err)
	assert.Len(t, entries1, 7)
	entries2, err := os.ReadDir(migrationsDir2)
	assert.NoError(t, err)
	assert.Len(t, entries2, 3)

	// Assert test
	migrations, hooks, errs := filesystem.LoadObjectsFromFiles(config)
	assert.Len(t, errs, 0)
	assert.Len(t, migrations[enums.MIGRATION_UP], 2)
	assert.Len(t, hooks[enums.HOOK_REPEATABLE], 1)
	assert.Len(t, hooks[enums.HOOK_BEFORE], 1)
	assert.Len(t, hooks[enums.HOOK_BEFORE_EACH], 1)
	assert.Len(t, hooks[enums.HOOK_BEFORE_VERSION], 1)
	assert.Len(t, hooks[enums.HOOK_AFTER], 1)
	assert.Len(t, hooks[enums.HOOK_AFTER_EACH], 1)
	assert.Len(t, hooks[enums.HOOK_AFTER_VERSION], 1)

	assert.Equal(t, "test1", migrations[enums.MIGRATION_UP][0].Description)
	assert.Equal(t, migration1Content, *migrations[enums.MIGRATION_UP][0].Content)
	assert.NotEmpty(t, migrations[enums.MIGRATION_UP][0].Checksum)

	assert.Equal(t, repeatable1Content, *hooks[enums.HOOK_REPEATABLE][0].Content)
	assert.Equal(t, before1Content, *hooks[enums.HOOK_BEFORE][0].Content)
	assert.Equal(t, beforeEach1Content, *hooks[enums.HOOK_BEFORE_EACH][0].Content)
	assert.Equal(t, beforeVersion1Content, *hooks[enums.HOOK_BEFORE_VERSION][0].Content)
	assert.Equal(t, uint16(1), hooks[enums.HOOK_BEFORE_VERSION][0].Version)

	assert.Equal(t, "SAMPLE CONTENT WITH TEMPLATE TEST TEMPLATE 10 CONTENT", *migrations[enums.MIGRATION_UP][1].Content) // Assert template
}

func TestLoadObjectsFromFilesDown(t *testing.T) {
	migrationsDir := t.TempDir()

	config := &conf.MigrationConfig{
		Down:          true,
		UseRepeatable: true,
		Locations:     []string{migrationsDir},
	}

	migration1Content := "SAMPLE CONTENT"
	repeatableDownContent := "SAMPLE REPEATABLE DOWN CONTENT"

	err := os.WriteFile(filepath.Join(migrationsDir, "V001_test1.down.sql"), []byte(migration1Content), os.ModePerm)
	assert.NoError(t, err)

	err = os.WriteFile(filepath.Join(migrationsDir, "R001_test1_repeatable.down.sql"), []byte(repeatableDownContent), os.ModePerm)
	assert.NoError(t, err)

	// Assert test
	migrations, hooks, errs := filesystem.LoadObjectsFromFiles(config)
	assert.Len(t, errs, 0)
	assert.Len(t, migrations[enums.MIGRATION_DOWN], 1)
	assert.Len(t, hooks[enums.HOOK_REPEATABLE_DOWN], 1)

	assert.Equal(t, migration1Content, *migrations[enums.MIGRATION_DOWN][0].Content)
	assert.Equal(t, repeatableDownContent, *hooks[enums.HOOK_REPEATABLE_DOWN][0].Content)
}

func TestSortHooks(t *testing.T) {
	migrationsDir := t.TempDir()

	config := &conf.MigrationConfig{
		UseBefore: true,
		Locations: []string{migrationsDir},
	}

	err := os.WriteFile(filepath.Join(migrationsDir, "B002_test2_before.sql"), []byte(""), os.ModePerm)
	assert.NoError(t, err)

	err = os.WriteFile(filepath.Join(migrationsDir, "B001_test1_before.sql"), []byte(""), os.ModePerm)
	assert.NoError(t, err)

	_, hooks, errs := filesystem.LoadObjectsFromFiles(config)
	assert.Len(t, errs, 0)
	assert.Len(t, hooks[enums.HOOK_BEFORE], 2)

	assert.Equal(t, uint8(1), hooks[enums.HOOK_BEFORE][0].Order)
	assert.Equal(t, uint8(2), hooks[enums.HOOK_BEFORE][1].Order)
}
