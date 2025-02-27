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
	"github.com/maestro-go/maestro/internal/pkg/logger"
	"github.com/spf13/cobra"
)

func SetupRepairCommand() *cobra.Command {
	repairCmd := &cobra.Command{
		Use:   "repair",
		Short: "Repair migration checksums and descriptions",
		Long:  `Repair migration checksums and descriptions in the schema history table.`,
		RunE:  runRepairCommand,
	}

	repairCmd.Flags().SortFlags = false
	flags.SetupDBConfigFlags(repairCmd)

	return repairCmd
}

func runRepairCommand(cmd *cobra.Command, args []string) error {
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

	migrations, _, errs := filesystem.LoadObjectsFromFiles(&projectConfig.Migration)
	if len(errs) > 0 {
		logErrors(logger, ErrLoadMigrations, errs)
		return errors.Join(errs...)
	}

	errs = repo.Repair(migrations[enums.MIGRATION_UP])
	if len(errs) > 0 {
		logErrors(logger, ErrRepairMigration, errs)
		return errors.Join(errs...)
	}

	logger.Info("Migrations repaired successfully")

	return nil
}
