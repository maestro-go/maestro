package cli

import (
	"github.com/maestro-go/maestro/internal/cli/flags"
	"github.com/spf13/cobra"
)

func SetupRootCommand() *cobra.Command {
	rootCmd := &cobra.Command{
		Use:   "maestro",
		Short: "Maestro is a powerful database migration tool for Go.",
		Long: `Maestro is a comprehensive database migration tool designed for Go applications.
It provides a robust set of commands to manage database schema changes, including initialization,
migration creation, applying migrations, repairing migrations, and checking migration status.
With Maestro, you can ensure your database schema evolves smoothly and consistently across all environments.`,
	}

	rootCmd.CompletionOptions.DisableDefaultCmd = true
	rootCmd.Flags().SortFlags = false

	flags.SetupGlobalFlags(rootCmd)

	initCmd := SetupInitCommand()
	createCmd := SetupCreateCommand()
	migrateCmd := SetupMigrateCommand()
	repairCmd := SetupRepairCommand()
	statusCmd := SetupStatusCommand()

	rootCmd.AddCommand(initCmd, createCmd, migrateCmd, repairCmd, statusCmd)

	return rootCmd
}
