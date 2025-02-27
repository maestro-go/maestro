package cli

import (
	"context"
	"errors"
	"log"
	"path/filepath"

	_ "github.com/lib/pq"
	"github.com/maestro-go/maestro/core/conf"
	"github.com/maestro-go/maestro/core/enums"
	"github.com/maestro-go/maestro/core/migrator"
	"github.com/maestro-go/maestro/internal/cli/conn"
	"github.com/maestro-go/maestro/internal/cli/flags"
	internalConf "github.com/maestro-go/maestro/internal/conf"
	"github.com/maestro-go/maestro/internal/filesystem"
	"github.com/maestro-go/maestro/internal/pkg/logger"
	"github.com/spf13/cobra"
)

func SetupMigrateCommand() *cobra.Command {
	migrateCmd := &cobra.Command{
		Use:   "migrate",
		Short: "Run database migrations",
		Long: `The migrate command applies database migrations based on the configuration provided.
It supports both "up" and "down" migrations, validates migrations if configured, and ensures the schema history table exists.`,
		RunE: runMigrateCommand,
	}

	migrateCmd.Flags().SortFlags = false
	flags.SetupDBConfigFlags(migrateCmd)
	flags.SetupMigrationConfigFlags(migrateCmd)

	return migrateCmd
}

func runMigrateCommand(cmd *cobra.Command, args []string) error {
	logger, err := logger.NewLogger()
	if err != nil {
		log.Fatal(err)
		return err
	}

	ctx := context.Background()

	globalFlags, err := flags.ExtractGlobalFlags(cmd)
	if err != nil {
		logError(logger, ErrExtractGlobalFlags, err)
		return genError(ErrExtractGlobalFlags, err)
	}

	configFilePath := filepath.Join(globalFlags.Location, internalConf.DEFAULT_PROJECT_FILE)
	exists, err := filesystem.CheckFSObject(configFilePath)
	if err != nil {
		logError(logger, ErrCheckFile, err)
		return genError(ErrCheckFile, err)
	}

	projectConfig := &conf.ProjectConfig{}
	if exists {
		logger.Info("Located config file")

		err = conf.LoadConfigFromFile(configFilePath, projectConfig)
		if err != nil {
			logError(logger, ErrLoadConfigFromFile, err)
			return genError(ErrLoadConfigFromFile, err)
		}

		err = flags.MergeDBConfigFlags(cmd, projectConfig)
		if err != nil {
			logError(logger, ErrMergeDBConfigFlags, err)
			return genError(ErrMergeDBConfigFlags, err)
		}

		err = flags.MergeMigrationsConfigFlags(cmd, &projectConfig.Migration)
		if err != nil {
			logError(logger, ErrMergeMigrationLocations, err)
			return genError(ErrMergeMigrationLocations, err)
		}

		err = flags.MergeMigrationLocations(cmd, &projectConfig.Migration)
		if err != nil {
			logError(logger, ErrMergeMigrationLocations, err)
			return genError(ErrMergeMigrationLocations, err)
		}

	} else {
		err = flags.ExtractDBConfigFlags(cmd, projectConfig)
		if err != nil {
			logError(logger, ErrExtractDBConfigFlags, err)
			return genError(ErrExtractDBConfigFlags, err)
		}

		err = flags.ExtractMigrationConfigFlags(cmd, &projectConfig.Migration)
		if err != nil {
			logError(logger, ErrExtractConfigFromFile, err)
			return genError(ErrExtractConfigFromFile, err)
		}

		projectConfig.Migration.Locations = globalFlags.MigrationLocations
	}

	driver, ok := enums.MapStringToDriverType[projectConfig.Driver]
	if !ok {
		logError(logger, ErrInvalidDriver, errors.New(projectConfig.Driver))
		return genError(ErrInvalidDriver, errors.New(projectConfig.Driver))
	}

	repo, cleanup, err := conn.ConnectToDatabase(ctx, projectConfig, driver)
	if err != nil {
		logError(logger, ErrConnectToDatabase, err)
		return genError(ErrConnectToDatabase, err)
	}
	defer cleanup()

	migrator := migrator.NewMigrator(logger, repo, &projectConfig.Migration)
	err = migrator.Migrate()
	if err != nil {
		logError(logger, ErrLoadMigrations, err)
		return genError(ErrLoadMigrations, err)
	}

	logger.Info("Migrations executed successfully")

	return nil
}
