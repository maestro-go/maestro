package cli

import (
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/maestro-go/maestro/internal/cli/flags"
	"github.com/maestro-go/maestro/internal/conf"
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
		Run:  runCreateCommand,
	}

	createCmd.Flags().SortFlags = false

	createCmd.Flags().BoolP("with-down", "d", false, "Generates a down migration too.")

	return createCmd
}

func runCreateCommand(cmd *cobra.Command, args []string) {
	logger, err := logger.NewLogger()
	if err != nil {
		log.Fatal(err)
		return
	}

	migrationName := args[0]
	if migrationName == "" {
		logger.Error("migration name must not be empty")
		return
	}

	globalFlags, err := flags.ExtractGlobalFlags(cmd)
	if err != nil {
		logger.Error("error extracting global flags", zap.Error(err))
		return
	}

	latestVersion, err := filesystem.GetLatestVersionFromFiles(globalFlags.MigrationLocations)
	if err != nil {
		logger.Error("error getting latest version in files", zap.Error(err))
		return
	}

	newMigrationPath := filepath.Join(globalFlags.MigrationLocations[0],
		fmt.Sprintf("V%.3d_%s.sql", latestVersion+1, migrationName))

	err = os.WriteFile(newMigrationPath, []byte(conf.NEW_MIGRATION_PLACEHOLDER), os.ModePerm)
	if err != nil {
		logger.Error("error writing migration", zap.Error(err))
		return
	}

	withDown, err := cmd.Flags().GetBool("with-down")
	if err != nil {
		logger.Error("error reading with-down flag", zap.Error(err))
		return
	}

	if withDown {
		newDownMigrationPath := filepath.Join(globalFlags.MigrationLocations[0],
			fmt.Sprintf("V%.3d_%s.down.sql", latestVersion+1, migrationName))

		err = os.WriteFile(newDownMigrationPath, []byte(conf.NEW_MIGRATION_PLACEHOLDER), os.ModePerm)
		if err != nil {
			logger.Error("error writing down migration", zap.Error(err))
			return
		}
	}

	logger.Info("migration created successfully", zap.Uint16("version", latestVersion+1),
		zap.String("name", migrationName))
}
