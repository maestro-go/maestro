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

func SetupRepairCommand() *cobra.Command {
	repairCmd := &cobra.Command{
		Use:   "repair",
		Short: "Repair migration checksums and descriptions",
		Long:  `Repair migration checksums and descriptions in the schema history table.`,
		Run:   runRepairCommand,
	}

	repairCmd.Flags().SortFlags = false
	flags.SetupDBConfigFlags(repairCmd)

	return repairCmd
}

func runRepairCommand(cmd *cobra.Command, args []string) {
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

	migrations, _, errs := filesystem.LoadObjectsFromFiles(&projectConfig.Migration)
	if len(errs) > 0 {
		for _, err := range errs {
			logger.Error("error loading migrations", zap.Error(err))
		}
		return
	}

	errs = repo.Repair(migrations[enums.MIGRATION_UP])
	if len(errs) > 0 {
		for _, err := range errs {
			logger.Error("error repairing migration", zap.Error(err))
		}
		return
	}

	logger.Info("Migrations repaired successfully")
}
