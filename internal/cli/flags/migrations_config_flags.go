package flags

import (
	"github.com/maestro-go/maestro/core/conf"
	"github.com/spf13/cobra"
)

func SetupMigrationConfigFlags(cmd *cobra.Command) {
	cmd.Flags().Bool("validate", true, "Validate migrations before executing.")
	cmd.Flags().Bool("down", false, "Run migrations in the down direction.")
	cmd.Flags().Bool("in-transaction", true, "Run migrations within a transaction.")
	cmd.Flags().Uint16("destination", 0, "Target migration version.")
	cmd.Flags().Bool("force", false, "Continue executing migrations even if errors occur.")
	cmd.Flags().Bool("use-repeatable", true, "Execute repeatable migrations.")
	cmd.Flags().Bool("use-before", true, "Execute before-all hooks.")
	cmd.Flags().Bool("use-after", true, "Execute after-all hooks.")
	cmd.Flags().Bool("use-before-each", true, "Execute before-each hooks.")
	cmd.Flags().Bool("use-after-each", true, "Execute after-each hooks.")
	cmd.Flags().Bool("use-before-version", true, "Execute before-version hooks.")
	cmd.Flags().Bool("use-after-version", true, "Execute after-version hooks.")
}

func ExtractMigrationConfigFlags(cmd *cobra.Command, config *conf.MigrationConfig) error {
	var err error

	config.Validate, err = cmd.Flags().GetBool("validate")
	if err != nil {
		return err
	}

	config.Down, err = cmd.Flags().GetBool("down")
	if err != nil {
		return err
	}

	config.InTransaction, err = cmd.Flags().GetBool("in-transaction")
	if err != nil {
		return err
	}

	destination, err := cmd.Flags().GetUint16("destination")
	if err != nil {
		return err
	}
	if destination != 0 { // Only set if the flag is explicitly provided
		config.Destination = &destination
	}

	config.Force, err = cmd.Flags().GetBool("force")
	if err != nil {
		return err
	}

	config.UseRepeatable, err = cmd.Flags().GetBool("use-repeatable")
	if err != nil {
		return err
	}

	config.UseBefore, err = cmd.Flags().GetBool("use-before")
	if err != nil {
		return err
	}

	config.UseAfter, err = cmd.Flags().GetBool("use-after")
	if err != nil {
		return err
	}

	config.UseBeforeEach, err = cmd.Flags().GetBool("use-before-each")
	if err != nil {
		return err
	}

	config.UseAfterEach, err = cmd.Flags().GetBool("use-after-each")
	if err != nil {
		return err
	}

	config.UseBeforeVersion, err = cmd.Flags().GetBool("use-before-version")
	if err != nil {
		return err
	}

	config.UseAfterVersion, err = cmd.Flags().GetBool("use-after-version")
	if err != nil {
		return err
	}

	return nil
}

func MergeMigrationsConfigFlags(cmd *cobra.Command, config *conf.MigrationConfig) error {
	var err error

	if cmd.Flags().Changed("validate") {
		config.Validate, err = cmd.Flags().GetBool("validate")
		if err != nil {
			return err
		}
	}
	if cmd.Flags().Changed("down") {
		config.Down, err = cmd.Flags().GetBool("down")
		if err != nil {
			return err
		}
	}
	if cmd.Flags().Changed("in-transaction") {
		config.InTransaction, err = cmd.Flags().GetBool("in-transaction")
		if err != nil {
			return err
		}
	}
	if cmd.Flags().Changed("destination") {
		destination, err := cmd.Flags().GetUint16("destination")
		if err != nil {
			return err
		}
		config.Destination = &destination // Only set if explicitly provided
	}
	if cmd.Flags().Changed("force") {
		config.Force, err = cmd.Flags().GetBool("force")
		if err != nil {
			return err
		}
	}
	if cmd.Flags().Changed("use-repeatable") {
		config.UseRepeatable, err = cmd.Flags().GetBool("use-repeatable")
		if err != nil {
			return err
		}
	}
	if cmd.Flags().Changed("use-before") {
		config.UseBefore, err = cmd.Flags().GetBool("use-before")
		if err != nil {
			return err
		}
	}
	if cmd.Flags().Changed("use-after") {
		config.UseAfter, err = cmd.Flags().GetBool("use-after")
		if err != nil {
			return err
		}
	}
	if cmd.Flags().Changed("use-before-each") {
		config.UseBeforeEach, err = cmd.Flags().GetBool("use-before-each")
		if err != nil {
			return err
		}
	}
	if cmd.Flags().Changed("use-after-each") {
		config.UseAfterEach, err = cmd.Flags().GetBool("use-after-each")
		if err != nil {
			return err
		}
	}
	if cmd.Flags().Changed("use-before-version") {
		config.UseBeforeVersion, err = cmd.Flags().GetBool("use-before-version")
		if err != nil {
			return err
		}
	}
	if cmd.Flags().Changed("use-after-version") {
		config.UseAfterVersion, err = cmd.Flags().GetBool("use-after-version")
		if err != nil {
			return err
		}
	}

	return nil
}
