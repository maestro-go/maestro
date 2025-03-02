package cli

import (
	"context"
	"errors"
	"log"
	"path/filepath"

	_ "github.com/lib/pq"
	"github.com/maestro-go/maestro/core/conf"
	"github.com/maestro-go/maestro/core/enums"
	"github.com/maestro-go/maestro/internal/cli/conn"
	"github.com/maestro-go/maestro/internal/cli/flags"
	internalConf "github.com/maestro-go/maestro/internal/conf"
	"github.com/maestro-go/maestro/internal/filesystem"
	"github.com/maestro-go/maestro/internal/utils/logger"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
)

func SetupStatusCommand() *cobra.Command {
	statusCmd := &cobra.Command{
		Use:   "status",
		Short: "Show the status of migrations",
		Long:  `Show the status of migrations including the latest migration, validation errors, and failing migrations.`,
		RunE:  runStatusCommand,
	}

	statusCmd.Flags().SortFlags = false
	flags.SetupDBConfigFlags(statusCmd)

	return statusCmd
}

func runStatusCommand(cmd *cobra.Command, args []string) error {
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
			logError(logger, ErrExtractConfigFromFile, err)
			return genError(ErrExtractConfigFromFile, err)
		}

		err = flags.MergeDBConfigFlags(cmd, projectConfig)
		if err != nil {
			logError(logger, ErrMergeDBConfigFlags, err)
			return genError(ErrMergeDBConfigFlags, err)
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

	// Log the latest migration
	latestMigration, err := repo.GetLatestMigration()
	if err != nil {
		logError(logger, ErrGetFailingMigrations, err)
		return genError(ErrGetFailingMigrations, err)
	}

	// Load migrations
	migrations, _, errs := filesystem.LoadObjectsFromFiles(&projectConfig.Migration)
	if len(errs) > 0 {
		logErrors(logger, ErrLoadMigrations, errs)
		return errors.Join(errs...)
	}

	// Validate migrations
	validationErrors := repo.ValidateMigrations(migrations[enums.MIGRATION_UP])

	// Log failing migrations
	failingMigrations, err := repo.GetFailingMigrations()
	if err != nil {
		logError(logger, ErrGetFailingMigrations, err)
		return genError(ErrGetFailingMigrations, err)
	}

	for _, validationError := range validationErrors {
		logger.Info("validation error: ", zap.String("error", validationError.Error()))
	}

	for _, migration := range failingMigrations {
		logger.Info("Failing migration", zap.Uint16("version", migration.Version), zap.String("description", migration.Description))
	}

	logger.Info("Migrations status:", zap.Uint16("latest migration", latestMigration), zap.Int("migrations mismatches",
		len(validationErrors)), zap.Int("failing migrations", len(failingMigrations)))

	return nil
}
