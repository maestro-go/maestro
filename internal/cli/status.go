package cli

import (
	"context"
	"log"
	"path/filepath"

	_ "github.com/lib/pq"
	"github.com/maestro-go/maestro/core/conf"
	"github.com/maestro-go/maestro/core/enums"
	"github.com/maestro-go/maestro/internal/cli/conn"
	"github.com/maestro-go/maestro/internal/cli/flags"
	internalConf "github.com/maestro-go/maestro/internal/conf"
	"github.com/maestro-go/maestro/internal/filesystem"
	"github.com/maestro-go/maestro/internal/pkg/logger"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
)

func SetupStatusCommand() *cobra.Command {
	statusCmd := &cobra.Command{
		Use:   "status",
		Short: "Show the status of migrations",
		Long:  `Show the status of migrations including the latest migration, validation errors, and failing migrations.`,
		Run:   runStatusCommand,
	}

	statusCmd.Flags().SortFlags = false
	flags.SetupDBConfigFlags(statusCmd)

	return statusCmd
}

func runStatusCommand(cmd *cobra.Command, args []string) {
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

	} else {
		err = flags.ExtractDBConfigFlags(cmd, projectConfig)
		if err != nil {
			logger.Error("error extracting database config flags", zap.Error(err))
			return
		}
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

	// Log the latest migration
	latestMigration, err := repo.GetLatestMigration()
	if err != nil {
		logger.Error("error getting latest migration", zap.Error(err))
		return
	}
	logger.Info("Latest migration", zap.Uint16("version", latestMigration))

	// Load migrations
	migrations, _, errs := filesystem.LoadObjectsFromFiles(&projectConfig.Migration)
	if len(errs) > 0 {
		for _, err := range errs {
			logger.Error("error loading migrations", zap.Error(err))
		}
		return
	}

	// Validate migrations
	validationErrors := repo.ValidateMigrations(migrations[enums.MIGRATION_UP])
	if len(validationErrors) > 0 {
		for _, err := range validationErrors {
			logger.Warn("validation error", zap.Error(err))
		}
	}

	// Log failing migrations
	failingMigrations, err := repo.GetFailingMigrations()
	if err != nil {
		logger.Error("error getting failing migrations", zap.Error(err))
		return
	}

	for _, migration := range failingMigrations {
		logger.Info("Failing migration", zap.Uint16("version", migration.Version), zap.String("description", migration.Description))
	}
}
