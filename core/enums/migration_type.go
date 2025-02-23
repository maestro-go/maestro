package enums

import "github.com/maestro-go/maestro/internal/conf"

type MigrationType int8

const (
	MIGRATION_UP MigrationType = iota
	MIGRATION_DOWN
)

func (m *MigrationType) Name() string {
	return []string{"UP", "DOWN"}[*m]
}

var MapMigrationTypeToRegex = map[MigrationType]string{
	MIGRATION_UP:   conf.MIGRATION_REGEX,
	MIGRATION_DOWN: conf.MIGRATION_DOWN_REGEX,
}
