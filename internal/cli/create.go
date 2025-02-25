package cli

import (
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/maestro-go/maestro/core/conf"
	"github.com/maestro-go/maestro/internal/cli/flags"
	internalConf "github.com/maestro-go/maestro/internal/conf"
	"github.com/maestro-go/maestro/internal/filesystem"
	"github.com/maestro-go/maestro/internal/pkg/logger"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
)

func SetupCreateCommand() *cobra.Command {
	createCmd := &cobra.Command{
		Use:   "create [migration_name]",
		Short: "Create a new migration file",
		Long: `Create a new migration file with the specified name.

This command performs the following:
1. Determines the next version number by scanning existing migration files in the configured migration directories.
2. Creates a new placeholder migration file, in the first given migration location, with the format "VXXX_migration_name.sql", where XXX is the next version number.`,
		Args: cobra.ExactArgs(1),
		RunE: runCreateCommand,
	}

	createCmd.Flags().SortFlags = false

	createCmd.Flags().BoolP("with-down", "d", false, "Generates a down migration too.")

	return createCmd
}

func runCreateCommand(cmd *cobra.Command, args []string) error {
	logger, err := logger.NewLogger()
	if err != nil {
		log.Fatal(err)
		return err
	}

	migrationName := args[0]
	if migrationName == "" {
		logger.Error("migration name must not be empty")
		return errors.New("migration name must not be empty")
	}

	globalFlags, err := flags.ExtractGlobalFlags(cmd)
	if err != nil {
		logger.Error("error extracting global flags", zap.Error(err))
		return err
	}

	configFilePath := filepath.Join(globalFlags.Location, internalConf.DEFAULT_PROJECT_FILE)
	configExists, err := filesystem.CheckFSObject(configFilePath)
	if err != nil {
		logger.Error("error checking file", zap.Error(err))
		return err
	}

	projectConfig := &conf.ProjectConfig{}
	if configExists {
		err := conf.LoadConfigFromFile(configFilePath, projectConfig)
		if err != nil {
			logger.Error("error loading config from file", zap.Error(err))
			return err
		}

		err = flags.MergeMigrationLocations(cmd, &projectConfig.Migration)
		if err != nil {
			logger.Error("error merging migrations location", zap.Error(err))
			return err
		}
	} else {
		projectConfig.Migration.Locations = globalFlags.MigrationLocations
	}

	latestVersion, err := filesystem.GetLatestVersionFromFiles(projectConfig.Migration.Locations)
	if err != nil {
		logger.Error("error getting latest version in files", zap.Error(err))
		return err
	}

	newMigrationPath := filepath.Join(projectConfig.Migration.Locations[0],
		fmt.Sprintf("V%.3d_%s.sql", latestVersion+1, migrationName))

	err = os.WriteFile(newMigrationPath, []byte(internalConf.NEW_MIGRATION_PLACEHOLDER), os.ModePerm)
	if err != nil {
		logger.Error("error writing migration", zap.Error(err))
		return err
	}

	withDown, err := cmd.Flags().GetBool("with-down")
	if err != nil {
		logger.Error("error reading with-down flag", zap.Error(err))
		return err
	}

	if withDown {
		newDownMigrationPath := filepath.Join(projectConfig.Migration.Locations[0],
			fmt.Sprintf("V%.3d_%s.down.sql", latestVersion+1, migrationName))

		err = os.WriteFile(newDownMigrationPath, []byte(internalConf.NEW_MIGRATION_PLACEHOLDER), os.ModePerm)
		if err != nil {
			logger.Error("error writing down migration", zap.Error(err))
			return err
		}
	}

	logger.Info("migration created successfully", zap.Uint16("version", latestVersion+1),
		zap.String("name", migrationName))

	return nil
}
