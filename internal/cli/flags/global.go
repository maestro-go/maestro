package flags

import (
	"github.com/maestro-go/maestro/core/conf"
	"github.com/spf13/cobra"
)

type globalFlags struct {
	Location           string
	MigrationLocations []string
}

func SetupGlobalFlags(cmd *cobra.Command) {
	cmd.PersistentFlags().StringP("location", "l", ".", "Project directory.")
	cmd.PersistentFlags().StringArrayP("migrations", "m", []string{"./migrations"}, "Migrations directories.")
}

func ExtractGlobalFlags(cmd *cobra.Command) (*globalFlags, error) {
	flags := &globalFlags{}
	err := (error)(nil)

	flags.Location, err = cmd.Flags().GetString("location")
	if err != nil {
		return nil, err
	}

	flags.MigrationLocations, err = cmd.Flags().GetStringArray("migrations")
	if err != nil {
		return nil, err
	}

	return flags, nil
}

func MergeMigrationLocations(cmd *cobra.Command, config *conf.MigrationConfig) error {
	err := (error)(nil)

	if cmd.Flags().Changed("migrations") {
		config.Locations, err = cmd.Flags().GetStringArray("migrations")
		if err != nil {
			return err
		}
	}

	return nil
}
