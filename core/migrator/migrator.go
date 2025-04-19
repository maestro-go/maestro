package migrator

import (
	"errors"
	"fmt"

	"github.com/maestro-go/maestro/core/conf"
	"github.com/maestro-go/maestro/core/database"
	"github.com/maestro-go/maestro/core/enums"
	"github.com/maestro-go/maestro/internal/filesystem"
	"github.com/maestro-go/maestro/internal/migrations"
	"go.uber.org/zap"
)

type Migrator struct {
	logger *zap.Logger

	repository database.Repository

	config *conf.MigrationConfig
}

func NewMigrator(logger *zap.Logger, repository database.Repository, config *conf.MigrationConfig) *Migrator {
	return &Migrator{
		logger:     logger,
		repository: repository,
		config:     config,
	}
}

// Migrate performs database migrations based on the configuration and current state of the database.
func (m *Migrator) Migrate() error {
	return m.repository.DoInLock(func() error {

		// Load migrations and hooks to memory
		migrationsMap, hooksMap, errs := filesystem.LoadObjectsFromFiles(m.config)
		if len(errs) > 0 {
			if m.logger != nil {
				for _, err := range errs {
					m.logger.Error("Error loading migrations and hooks", zap.Error(err))
				}
			}
			return errors.Join(errs...)
		}

		// Assert that schema history table exists
		err := m.repository.AssertSchemaHistoryTable()
		if err != nil {
			if m.logger != nil {
				m.logger.Error("Error asserting schema history table", zap.Error(err))
			}
			return err
		}

		latestMigration, err := m.repository.GetLatestMigration()
		if err != nil {
			return fmt.Errorf("error getting latest migration: %w", err)
		}

		if (!m.config.Down && len(migrationsMap[enums.MIGRATION_UP]) < 1) ||
			(m.config.Down && len(migrationsMap[enums.MIGRATION_DOWN]) < 1) {
			if m.logger != nil {
				m.logger.Warn("No migrations found in the specified directories")
			}
			return nil
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

			// Assert that there are no unsucceeded migrations in database
			failingMigrations, err := m.repository.GetFailingMigrations()
			if err != nil {
				return fmt.Errorf("error getting failing migrations: %w", err)
			}

			if len(failingMigrations) > 0 {
				errs = make([]error, 0)
				for _, failingMigration := range failingMigrations {
					if m.logger != nil {
						m.logger.Error("Found an unsucceeded migration", zap.Uint16("version", failingMigration.Version))
					}
					errs = append(errs, fmt.Errorf("found an unsucceeded migration: %d", failingMigration.Version))
				}
				return errors.Join(errs...)
			}

			// Validate local migrations
			errs = migrations.ValidateMigrations(migrationsMap[enums.MIGRATION_UP])
			if len(errs) > 0 {
				if m.logger != nil {
					for _, err := range errs {
						m.logger.Error("Validate local migrations error", zap.Error(err))
					}
				}
				return errors.Join(errs...)
			}

			// Validate local <-> remote migrations
			errs = m.repository.ValidateMigrations(migrationsMap[enums.MIGRATION_UP])
			if len(errs) > 0 {
				if m.logger != nil {
					for _, err := range errs {
						m.logger.Error("Validate database migrations error", zap.Error(err))
					}
				}
				return errors.Join(errs...)
			}
		}

		if latestMigration == *m.config.Destination {
			if m.logger != nil {
				m.logger.Info("Database is up to date", zap.Uint16("version", latestMigration))
			}
			return nil
		}

		if !m.config.Down && *m.config.Destination < latestMigration {
			if m.logger != nil {
				m.logger.Warn("Trying to up migrate to a previous version", zap.Uint16("current", latestMigration), zap.Uint16("target", *m.config.Destination))
			}
			return nil
		}

		if m.config.Down && *m.config.Destination > latestMigration {
			if m.logger != nil {
				m.logger.Warn("Trying to down migrate to a later version", zap.Uint16("current", latestMigration), zap.Uint16("target", *m.config.Destination))
			}
			return nil
		}

		// Define the migrate function to handle the migration process, either within a transaction or not
		migrate := func() error {
			if m.config.Down {
				errs := m.migrateDown(migrationsMap[enums.MIGRATION_DOWN], hooksMap, latestMigration, *m.config.Destination)
				if len(errs) > 0 {
					if m.logger != nil {
						for _, err := range errs {
							m.logger.Error("Error migrating down", zap.Error(err))
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
						m.logger.Error("Error migrating up", zap.Error(err))
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

		// Do not execute repeatable before first migration
		if m.config.UseRepeatable && migration.Version > 1 {
			hErrs := m.executeHooks(hooks[enums.HOOK_REPEATABLE])
			if len(hErrs) > 0 {
				errs = append(errs, hErrs...)
				if !m.config.Force {
					return errs
				}
			}
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
			m.logger.Info("Migrating up", zap.Uint16("version", migration.Version),
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
		if from < migration.Version || to >= migration.Version {
			continue
		}

		if m.logger != nil {
			m.logger.Info("Rolling back", zap.Uint16("version", migration.Version),
				zap.String("description", migration.Description))
		}
		err := m.repository.RollbackMigration(migration)
		if err != nil {
			errs = append(errs, fmt.Errorf("error rolling back migration %d: %w", migration.Version, err))
			if !m.config.Force {
				return errs
			}
		}

		// Do not execute repeatable after last migration
		if m.config.UseRepeatable && migration.Version > to+1 {
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
		if m.logger != nil {
			m.logger.Info("Executing hook", zap.Uint8("order", hook.Order), zap.String("type", hook.Type.Name()))
		}
		err := m.repository.ExecuteHook(hook)
		if err != nil {
			errs = append(errs, fmt.Errorf("error executing hook %d_%s: %w", hook.Order, hook.Type.Name(), err))
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
			if m.logger != nil {
				m.logger.Info("Executing versioned hook", zap.Uint8("order", hook.Order), zap.Uint16("version", hook.Version),
					zap.String("type", hook.Type.Name()))
			}
			err := m.repository.ExecuteHook(hook)
			if err != nil {
				errs = append(errs, fmt.Errorf("error executing versioned hook %d_%d_%s: %w", hook.Order,
					hook.Version, hook.Type.Name(), err))
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
