package filesystem

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/maestro-go/maestro/core/conf"
	"github.com/maestro-go/maestro/core/enums"
	"github.com/stretchr/testify/assert"
)

func TestLoadObjectsFromFiles(t *testing.T) {
	// Setup test
	migrationsDir1 := t.TempDir()
	migrationsDir2 := t.TempDir()

	config := &conf.MigrationConfig{
		Down:          false,
		UseRepeatable: true,
		UseBefore:     true,
		UseBeforeEach: true,
		Locations:     []string{migrationsDir1, migrationsDir2},
	}

	migration1Content := "SAMPLE CONTENT"
	migration2Content := "SAMPLE CONTENT WITH TEMPLATE {{ test, 10 }}"

	repeatable1Content := "SAMPLE REPEATABLE CONTENT"
	before1Content := "SAMPLE BEFORE CONTENT"
	beforeEach1Content := "SAMPLE BEFORE EACH CONTENT"

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

	err = os.WriteFile(filepath.Join(migrationsDir2, "test.template.sql"), []byte(templateTestContent), os.ModePerm)
	assert.NoError(t, err)

	// Assert setup
	entries1, err := os.ReadDir(migrationsDir1)
	assert.NoError(t, err)
	assert.Len(t, entries1, 3)
	entries2, err := os.ReadDir(migrationsDir2)
	assert.NoError(t, err)
	assert.Len(t, entries2, 3)

	// Assert test
	migrations, hooks, errs := LoadObjectsFromFiles(config)
	assert.Len(t, errs, 0)
	assert.Len(t, migrations[enums.MIGRATION_UP], 2)
	assert.Len(t, hooks[enums.HOOK_REPEATABLE], 1)
	assert.Len(t, hooks[enums.HOOK_BEFORE], 1)
	assert.Len(t, hooks[enums.HOOK_BEFORE_EACH], 1)

	assert.Equal(t, "test1", migrations[enums.MIGRATION_UP][0].Description)
	assert.Equal(t, migration1Content, *migrations[enums.MIGRATION_UP][0].Content)
	assert.NotEmpty(t, migrations[enums.MIGRATION_UP][0].Checksum)

	assert.Equal(t, repeatable1Content, *hooks[enums.HOOK_REPEATABLE][0].Content)
	assert.Equal(t, before1Content, *hooks[enums.HOOK_BEFORE][0].Content)
	assert.Equal(t, beforeEach1Content, *hooks[enums.HOOK_BEFORE_EACH][0].Content)

	assert.Equal(t, "SAMPLE CONTENT WITH TEMPLATE TEST TEMPLATE 10 CONTENT", *migrations[enums.MIGRATION_UP][1].Content) // Assert template
}
