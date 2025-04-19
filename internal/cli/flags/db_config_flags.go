package flags

import (
	"github.com/maestro-go/maestro/core/conf"
	"github.com/spf13/cobra"
)

func SetupDBConfigFlags(cmd *cobra.Command) {
	// ProjectConfig flags
	cmd.Flags().String("driver", "postgres", "Database driver (e.g., postgres).")
	cmd.Flags().String("host", "localhost", "Database host.")
	cmd.Flags().Uint16("port", 5432, "Database port.")
	cmd.Flags().String("database", "postgres", "Database name.")
	cmd.Flags().String("user", "postgres", "Database user.")
	cmd.Flags().String("password", "postgres", "Database password.")
	cmd.Flags().String("schema", "public", "Database schema.")
	cmd.Flags().String("history-table", "schema_history", "Schema history table name")

	// SSLConfig flags
	cmd.Flags().String("sslmode", "disable", "SSL mode for the database connection.")
	cmd.Flags().String("sslrootcert", "", "Path to the SSL root certificate.")
}

func ExtractDBConfigFlags(cmd *cobra.Command, config *conf.ProjectConfig) error {
	var err error

	// Extract ProjectConfig flags
	config.Driver, err = cmd.Flags().GetString("driver")
	if err != nil {
		return err
	}

	config.Host, err = cmd.Flags().GetString("host")
	if err != nil {
		return err
	}

	config.Port, err = cmd.Flags().GetUint16("port")
	if err != nil {
		return err
	}

	config.Database, err = cmd.Flags().GetString("database")
	if err != nil {
		return err
	}

	config.User, err = cmd.Flags().GetString("user")
	if err != nil {
		return err
	}

	config.Password, err = cmd.Flags().GetString("password")
	if err != nil {
		return err
	}

	config.Schema, err = cmd.Flags().GetString("schema")
	if err != nil {
		return err
	}

	config.HistoryTable, err = cmd.Flags().GetString("history-table")
	if err != nil {
		return err
	}

	// Extract SSLConfig flags
	config.SSL.SSLMode, err = cmd.Flags().GetString("sslmode")
	if err != nil {
		return err
	}

	config.SSL.SSLRootCert, err = cmd.Flags().GetString("sslrootcert")
	if err != nil {
		return err
	}

	return nil
}

func MergeDBConfigFlags(cmd *cobra.Command, config *conf.ProjectConfig) error {
	var err error

	// Extract and override DB-related flags
	if cmd.Flags().Changed("driver") {
		config.Driver, err = cmd.Flags().GetString("driver")
		if err != nil {
			return err
		}
	}
	if cmd.Flags().Changed("host") {
		config.Host, err = cmd.Flags().GetString("host")
		if err != nil {
			return err
		}
	}
	if cmd.Flags().Changed("port") {
		config.Port, err = cmd.Flags().GetUint16("port")
		if err != nil {
			return err
		}
	}
	if cmd.Flags().Changed("database") {
		config.Database, err = cmd.Flags().GetString("database")
		if err != nil {
			return err
		}
	}
	if cmd.Flags().Changed("user") {
		config.User, err = cmd.Flags().GetString("user")
		if err != nil {
			return err
		}
	}
	if cmd.Flags().Changed("password") {
		config.Password, err = cmd.Flags().GetString("password")
		if err != nil {
			return err
		}
	}
	if cmd.Flags().Changed("schema") {
		config.Schema, err = cmd.Flags().GetString("schema")
		if err != nil {
			return err
		}
	}

	// Extract and override SSL-related flags
	if cmd.Flags().Changed("sslmode") {
		config.SSL.SSLMode, err = cmd.Flags().GetString("sslmode")
		if err != nil {
			return err
		}
	}
	if cmd.Flags().Changed("sslrootcert") {
		config.SSL.SSLRootCert, err = cmd.Flags().GetString("sslrootcert")
		if err != nil {
			return err
		}
	}

	return nil
}
