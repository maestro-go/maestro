package migrations

import (
	"fmt"

	"github.com/maestro-go/maestro/core/enums"
)

type Migration struct {
	Version     uint16
	Description string
	Type        enums.MigrationType
	Checksum    *string // Only used in migrations up
	Content     *string
}

func ValidateMigrations(migrations []*Migration) []error {
	errs := make([]error, 0)

	expectedVersion := uint16(1)
	for _, migration := range migrations {
		if migration.Version != expectedVersion {
			errs = append(errs, fmt.Errorf("expected version %d got %d", expectedVersion, migration.Version))
		}
		expectedVersion = migration.Version + 1
	}

	if len(errs) > 0 {
		return errs
	}
	return nil
}
