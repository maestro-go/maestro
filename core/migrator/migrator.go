package migrator

import (
	"context"
	"errors"

	"github.com/maestro-go/maestro/core/conf"
	"github.com/maestro-go/maestro/core/database"
	"github.com/maestro-go/maestro/core/enums"
	"github.com/maestro-go/maestro/internal/filesystem"
	"github.com/maestro-go/maestro/internal/migrations"
	"go.uber.org/zap"
)

type Migrator struct {
	ctx    context.Context
	logger *zap.Logger

	repository database.Repository

	config *conf.MigrationConfig
}

func NewMigrator(ctx context.Context, logger *zap.Logger, repository database.Repository, config *conf.MigrationConfig) *Migrator {
	return &Migrator{
		ctx:        ctx,
		logger:     logger,
		repository: repository,
		config:     config,
	}
}

// Migrate performs database migrations based on the configuration and current state of the database.
// It handles both "up" and "down" migrations, validates migrations if configured, and ensures
// the schema history table exists. The function operates within a distributed lock to prevent
// concurrent migrations.
//
// Steps:
//  1. Load migrations and hooks from the filesystem.
//  2. Ensure the schema history table exists.
//  3. Determine the latest applied migration version.
//  4. Validate migrations (if enabled in the configuration).
//  5. Determine the target migration version (destination).
//  6. Execute the appropriate migration (up or down) based on the configuration.
//  7. Log errors and warnings throughout the process.
func (m *Migrator) Migrate() error {
	return m.repository.DoInLock(func() error {
		migrationsMap, hooksMap, errs := filesystem.LoadObjectsFromFiles(m.config)
		if len(errs) > 0 {
			if m.logger != nil {
				for _, err := range errs {
					m.logger.Error("error loading migrations and hooks", zap.Error(err))
				}
			}
			return errors.Join(errs...)
		}

		err := m.repository.AssertSchemaHistoryTable()
		if err != nil {
			if m.logger != nil {
				m.logger.Error("error asserting schema history table", zap.Error(err))
			}
			return err
		}

		latestMigration, err := m.repository.GetLatestMigration()
		if err != nil {
			return err
		}

		// Fix up migration destination to latest local version
		if !m.config.Down && m.config.Destination == nil {
			m.config.Destination = &migrationsMap[enums.MIGRATION_UP][len(migrationsMap[enums.MIGRATION_UP])-1].Version
		}

		// Fix down migration destination to 0
		if m.config.Down && m.config.Destination == nil {
			zero := uint16(0)
			m.config.Destination = &zero
		}

		if m.config.Validate {
			errs = migrations.ValidateMigrations(migrationsMap[enums.MIGRATION_UP])
			if len(errs) > 0 {
				if m.logger != nil {
					for _, err := range errs {
						m.logger.Error("validate local migrations error", zap.Error(err))
					}
				}
				return errors.Join(errs...)
			}

			errs = m.repository.ValidateMigrations(migrationsMap[enums.MIGRATION_UP])
			if len(errs) > 0 {
				if m.logger != nil {
					for _, err := range errs {
						m.logger.Error("validate database migrations error", zap.Error(err))
					}
				}
				return errors.Join(errs...)
			}
		}

		if latestMigration == *m.config.Destination {
			if m.logger != nil {
				m.logger.Info("Database is up to date")
			}
			return nil
		}

		if !m.config.Down && *m.config.Destination < latestMigration {
			if m.logger != nil {
				m.logger.Warn("trying to up migrate to a previous version")
			}
			return nil
		}

		if m.config.Down && *m.config.Destination > latestMigration {
			if m.logger != nil {
				m.logger.Warn("trying to down migrate to a latest version")
			}
			return nil
		}

		migrate := func() error {
			if m.config.Down {
				errs := m.migrateDown(migrationsMap[enums.MIGRATION_DOWN], hooksMap, latestMigration, *m.config.Destination)
				if len(errs) > 0 {
					if m.logger != nil {
						for _, err := range errs {
							m.logger.Error("error migrating down", zap.Error(err))
						}
					}
					return errors.Join(errs...)
				}
				return nil
			}

			errs := m.migrateUp(migrationsMap[enums.MIGRATION_UP], hooksMap, latestMigration+1, *m.config.Destination)
			if len(errs) > 0 {
				if m.logger != nil {
					for _, err := range errs {
						m.logger.Error("error migrating up", zap.Error(err))
					}
				}
				return errors.Join(errs...)
			}
			return nil
		}

		if m.config.InTransaction {
			return m.repository.DoInTransaction(func() error {
				return migrate()
			})
		}

		return migrate()
	})
}

func (m *Migrator) migrateUp(migrations []*migrations.Migration, hooks map[enums.HookType][]*migrations.Hook, from uint16, to uint16) []error {
	errs := make([]error, 0)

	if m.config.UseBefore {
		hErrs := m.executeHooks(hooks[enums.HOOK_BEFORE])
		if len(hErrs) > 0 {
			errs = append(errs, hErrs...)
			if !m.config.Force {
				return errs
			}
		}
	}

	for _, migration := range migrations {
		if migration.Version < from || migration.Version > to {
			continue
		}

		if m.config.UseBeforeEach {
			hErrs := m.executeHooks(hooks[enums.HOOK_BEFORE_EACH])
			if hErrs != nil {
				errs = append(errs, hErrs...)
				if !m.config.Force {
					return errs
				}
			}
		}

		if m.config.UseBeforeVersion {
			hErrs := m.executeVersionedHooks(migration.Version, hooks[enums.HOOK_BEFORE_VERSION])
			if len(hErrs) > 0 {
				errs = append(errs, hErrs...)
				if !m.config.Force {
					return errs
				}
			}
		}

		if m.logger != nil {
			m.logger.Info("Migrating", zap.Uint16("version", migration.Version),
				zap.String("description", migration.Description))
		}
		mErrs := m.repository.ExecuteMigration(migration)
		if len(mErrs) > 0 {
			errs = append(errs, mErrs...)
			if !m.config.Force {
				return errs
			}
		}

		if m.config.UseAfterVersion {
			hErrs := m.executeVersionedHooks(migration.Version, hooks[enums.HOOK_AFTER_VERSION])
			if len(hErrs) > 0 {
				errs = append(errs, hErrs...)
				if !m.config.Force {
					return errs
				}
			}
		}

		if m.config.UseAfterEach {
			hErrs := m.executeHooks(hooks[enums.HOOK_AFTER_EACH])
			if hErrs != nil {
				errs = append(errs, hErrs...)
				if !m.config.Force {
					return errs
				}
			}
		}

		// Do not execute repeatable after last migration
		if m.config.UseRepeatable && migration.Version < to {
			hErrs := m.executeHooks(hooks[enums.HOOK_REPEATABLE])
			if len(hErrs) > 0 {
				errs = append(errs, hErrs...)
				if !m.config.Force {
					return errs
				}
			}
		}
	}

	if m.config.UseAfter {
		hErrs := m.executeHooks(hooks[enums.HOOK_AFTER])
		if len(hErrs) > 0 {
			errs = append(errs, hErrs...)
			if !m.config.Force {
				return errs
			}
		}
	}

	if len(errs) > 0 {
		return errs
	}
	return nil
}

func (m *Migrator) migrateDown(migrations []*migrations.Migration, hooks map[enums.HookType][]*migrations.Hook, from uint16, to uint16) []error {
	errs := make([]error, 0)

	for _, migration := range migrations {
		if from < migration.Version || to > migration.Version {
			continue
		}

		if m.logger != nil {
			m.logger.Info("Rolling back", zap.Uint16("version", migration.Version),
				zap.String("name", migration.Description))
		}
		mErrs := m.repository.RollbackMigration(migration)
		if len(mErrs) > 0 {
			errs = append(errs, mErrs...)
			if !m.config.Force {
				return errs
			}
		}

		// Do not execute repeatable after last migration
		if m.config.UseRepeatable && migration.Version > to {
			hErrs := m.executeHooks(hooks[enums.HOOK_REPEATABLE_DOWN])
			if len(hErrs) > 0 {
				errs = append(errs, hErrs...)
				if !m.config.Force {
					return errs
				}
			}
		}
	}

	if len(errs) > 0 {
		return errs
	}
	return nil
}

func (m *Migrator) executeHooks(hooks []*migrations.Hook) []error {
	errs := make([]error, 0)
	for _, hook := range hooks {
		err := m.repository.ExecuteHook(hook)
		if err != nil {
			errs = append(errs, err)
			if !m.config.Force {
				return errs
			}
		}
	}

	if len(errs) > 0 {
		return errs
	}
	return nil
}

func (m *Migrator) executeVersionedHooks(version uint16, hooks []*migrations.Hook) []error {
	errs := make([]error, 0)
	for _, hook := range hooks {
		if version == hook.Version {
			err := m.repository.ExecuteHook(hook)
			if err != nil {
				errs = append(errs, err)
				if !m.config.Force {
					return errs
				}
			}
		}
	}

	if len(errs) > 0 {
		return errs
	}
	return nil
}
