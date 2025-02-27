package cli

import (
	"fmt"

	"go.uber.org/zap"
)

func genError(description string, err error) error {
	return fmt.Errorf("%s: %w", description, err)
}

func logError(logger *zap.Logger, description string, err error) {
	logger.Error(description, zap.Error(err))
}

func logErrors(logger *zap.Logger, description string, errs []error) {
	for _, err := range errs {
		logger.Error(description, zap.Error(err))
	}
}

var (
	ErrExtractGlobalFlags      = "Error extracting global flags"
	ErrCheckFile               = "Error checking file existence"
	ErrExtractConfigFromFile   = "Error extracting configuration from file"
	ErrLoadConfigFromFile      = "Error loading configuration from file"
	ErrMergeDBConfigFlags      = "Error merging database configuration flags"
	ErrMergeMigrationLocations = "Error merging migration locations flag"
	ErrExtractDBConfigFlags    = "Error extracting database configuration flags"
	ErrGetLatestVersion        = "Error getting the latest version from files"
	ErrWriteMigration          = "Error writing migration file"
	ErrReadWithDownFlag        = "Error reading with-down flag"
	ErrConnectToDatabase       = "Error connecting to the database"
	ErrLoadMigrations          = "Error loading migrations"
	ErrRepairMigration         = "Error repairing migration"
	ErrGetFailingMigrations    = "Error getting failing migrations"
	ErrInvalidDriver           = "Invalid database driver"
	ErrValidation              = "Validation error"
)
