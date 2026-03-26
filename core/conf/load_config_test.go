package conf

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLoadConfigFromFile(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "maestro.yaml")

	t.Run("valid config", func(t *testing.T) {
		content := `
driver: postgres
database: db
host: localhost
port: 5432
`
		err := os.WriteFile(configPath, []byte(content), 0644)
		assert.NoError(t, err)

		var config ProjectConfig
		err = LoadConfigFromFile(configPath, &config)
		assert.NoError(t, err)
		assert.Equal(t, "postgres", config.Driver)
		assert.Equal(t, "db", config.Database)
	})

	t.Run("non-existent file", func(t *testing.T) {
		var config ProjectConfig
		err := LoadConfigFromFile("non-existent.yaml", &config)
		assert.Error(t, err)
	})

	t.Run("invalid yaml", func(t *testing.T) {
		err := os.WriteFile(configPath, []byte("invalid: yaml: :"), 0644)
		assert.NoError(t, err)

		var config ProjectConfig
		err = LoadConfigFromFile(configPath, &config)
		assert.Error(t, err)
	})
}
