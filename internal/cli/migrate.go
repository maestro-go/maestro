package cli

import (
	"context"
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
	"go.uber.org/zap"
)

func SetupMigrateCommand() *cobra.Command {
	migrateCmd := &cobra.Command{
		Use:   "migrate",
		Short: "Run database migrations",
		Long: `The migrate command applies database migrations based on the configuration provided.
It supports both "up" and "down" migrations, validates migrations if configured, and ensures the schema history table exists.`,
		Run: runMigrateCommand,
	}

	migrateCmd.Flags().SortFlags = false
	flags.SetupDBConfigFlags(migrateCmd)
	flags.SetupMigrationConfigFlags(migrateCmd)

	return migrateCmd
}

func runMigrateCommand(cmd *cobra.Command, args []string) {
	logger, err := logger.NewLogger()
	if err != nil {
		log.Fatal(err)
		return
	}

	ctx := context.Background()

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

	projectConfig := &conf.ProjectConfig{}
	if exists {
		logger.Info("Located config file")

		err = conf.LoadConfigFromFile(configFilePath, projectConfig)
		if err != nil {
			logger.Error("error extracting config from file", zap.Error(err))
			return
		}

		err = flags.MergeDBConfigFlags(cmd, projectConfig)
		if err != nil {
			logger.Error("error merging database config flags", zap.Error(err))
			return
		}

		err = flags.MergeMigrationsConfigFlags(cmd, &projectConfig.Migration)
		if err != nil {
			logger.Error("error merging migrations config flags", zap.Error(err))
			return
		}

		err = flags.MergeMigrationLocations(cmd, &projectConfig.Migration)
		if err != nil {
			logger.Error("error merging migrations locations flag", zap.Error(err))
			return
		}

	} else {
		err = flags.ExtractDBConfigFlags(cmd, projectConfig)
		if err != nil {
			logger.Error("error extracting database config flags", zap.Error(err))
			return
		}

		err = flags.ExtractMigrationConfigFlags(cmd, &projectConfig.Migration)
		if err != nil {
			logger.Error("error extracting migrations config flags", zap.Error(err))
			return
		}

		projectConfig.Migration.Locations = globalFlags.MigrationLocations
	}

	driver, ok := enums.MapStringToDriverType[projectConfig.Driver]
	if !ok {
		logger.Error("invalid driver", zap.String("driver", projectConfig.Driver))
		return
	}

	repo, cleanup, err := conn.ConnectToDatabase(ctx, projectConfig, driver)
	if err != nil {
		logger.Error("error connecting to database", zap.Error(err))
		return
	}
	defer cleanup()

	migrator := migrator.NewMigrator(logger, repo, &projectConfig.Migration)
	err = migrator.Migrate()
	if err != nil {
		return
	}

	logger.Info("Migrations executed successfully")
}
