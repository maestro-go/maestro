# Hooks

Maestro provides various hook points to extend functionality during the migration process. Hooks allow you to execute custom SQL scripts at specific points in the migration lifecycle, useful for tasks such as setting up preconditions, cleaning up after migrations, or performing additional operations.

## Hook Types

Maestro supports the following types of hooks:

| Hook Type           | File Pattern                          | Description                                                                 |
|---------------------|---------------------------------------|-----------------------------------------------------------------------------|
| **Before**          | `B{number}_description.sql`           | Runs before all migrations.                                                 |
| **After**           | `A{number}_description.sql`           | Runs after all migrations.                                                  |
| **Before Each**     | `BE{number}_description.sql`          | Runs before each migration.                                                 |
| **After Each**      | `AE{number}_description.sql`          | Runs after each migration.                                                  |
| **Before Version**  | `BV{number}_{version}_description.sql` | Runs before a specific migration version.                                    |
| **After Version**   | `AV{number}_{version}_description.sql` | Runs after a specific migration version.                                     |
| **Repeatable**      | `R{number}_description.sql`           | Runs between migrations.                                                     |
| **Repeatable Down** | `R{number}_description.down.sql`      | Runs between down migrations.                                                |

> Note: The `{number}` in hook files determines the execution order and is not related to migration versions.

## Hook Execution Order

Hooks are executed in the following order based on their type and the `{number}` in their file name:

1. **Before Hooks**
2. **Before Each Hooks**
3. **Before Version Hooks**
4. **Migration Scripts**
5. **After Version Hooks**
6. **After Each Hooks**
7. **Repeatable Hooks** (except after last migration)
8. **After Hooks**

## Hook File Naming

The naming convention for hook files determines their execution order. The file name pattern includes a `{number}` specifying the order and a `description` of the hook's purpose.

### Example Hook Files

- `B01_initialize.sql`: Runs before all migrations to initialize settings.
- `BE01_before_each_migration.sql`: Runs before each migration to set up preconditions.
- `BV01_001_add_column.sql`: Runs before migration version 001 to prepare for adding a column.
- `AV01_001_add_column.sql`: Runs after migration version 001 to verify the column addition.
- `AE01_after_each_migration.sql`: Runs after each migration to clean up temporary data.
- `A01_finalize.sql`: Runs after all migrations to finalize the process.
- `R01_repeatable_task.sql`: Runs between migrations to perform a repeatable task.
- `R01_repeatable_task.down.sql`: Runs between down migrations to undo the repeatable task.

## Configuring Hooks

Hooks can be configured in the Maestro configuration file (`maestro.yaml`). The configuration allows you to enable or disable specific types of hooks and set other related options.

```yaml
migration:
  useBefore: true
  useAfter: true
  useBeforeEach: true
  useAfterEach: true
  useBeforeVersion: true
  useAfterVersion: true
  useRepeatable: true
  useRepeatableDown: true
```
