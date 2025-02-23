package cli

import (
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/creasty/defaults"

	"github.com/maestro-go/maestro/core/conf"
	"github.com/maestro-go/maestro/internal/cli/flags"
	internalConf "github.com/maestro-go/maestro/internal/conf"
	"github.com/maestro-go/maestro/internal/filesystem"
	"github.com/maestro-go/maestro/internal/pkg/logger"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
	"gopkg.in/yaml.v3"
)

func SetupInitCommand() *cobra.Command {
	initCmd := &cobra.Command{
		Use:   "init",
		Short: "Initialize a new Maestro project",
		Long: `Initialize a new Maestro project by creating the required configuration file and migration folders.

This command performs the following steps:
1. Creates a project configuration file (maestro.yaml) in the specified location.
2. Sets up migration directories based on the provided locations or default values.
3. Generates example migration files within each migration directory.

If the configuration file already exists, the command will warn the user and exit without making changes.`,
		Run: runInitCommand,
	}

	return initCmd
}

func runInitCommand(cmd *cobra.Command, args []string) {
	logger, err := logger.NewLogger()
	if err != nil {
		log.Fatal(err)
		return
	}

	globalFlags, err := flags.ExtractGlobalFlags(cmd)
	if err != nil {
		logger.Error("error extracting global flags", zap.Error(err))
		return
	}

	configFilePath := filepath.Join(globalFlags.Location, internalConf.DEFAULT_PROJECT_FILE)

	exists, err := filesystem.CheckFSObject(configFilePath)
	if err != nil {
		logger.Error("error checking file", zap.Error(err))
		return
	}

	if exists {
		logger.Warn("project already initialized", zap.String("location", configFilePath))
		return
	}

	err = insertConfigFile(configFilePath, globalFlags.MigrationLocations)
	if err != nil {
		logger.Error("error creating config file", zap.Error(err))
		return
	}

	errs := insertMigrationFolders(globalFlags.MigrationLocations)
	if len(errs) > 0 {
		for _, err = range errs {
			logger.Error("error creating migration", zap.Error(err))
		}
		os.RemoveAll(configFilePath) // Rollback
		return
	}
}

func insertConfigFile(configFilePath string, migrations []string) error {
	// Default config
	config := conf.ProjectConfig{}
	err := defaults.Set(&config)
	if err != nil {
		return err
	}
	config.Migration.Locations = migrations

	content, err := yaml.Marshal(config)
	if err != nil {
		return err
	}

	err = os.WriteFile(configFilePath, content, os.ModePerm)
	if err != nil {
		return err
	}

	return nil
}

func insertMigrationFolders(migrationDirs []string) []error {
	errs := make([]error, 0)

	for i, migrationDir := range migrationDirs {
		err := os.MkdirAll(migrationDir, os.ModePerm)
		if err != nil {
			errs = append(errs, err)
			continue
		}

		migrationPath := filepath.Join(migrationDir, fmt.Sprintf("V%.3d_example.sql", i+1))

		err = os.WriteFile(migrationPath, []byte(internalConf.NEW_MIGRATION_PLACEHOLDER), os.ModePerm)
		if err != nil {
			errs = append(errs, err)
		}
	}

	if len(errs) > 0 {
		// Rollback
		for _, migrationDir := range migrationDirs {
			os.RemoveAll(migrationDir)
		}
		return errs
	}

	return nil
}
