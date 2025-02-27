# Maestro CLI Documentation

## Overview

The Maestro CLI is a powerful tool for managing database migrations. It provides commands to create, apply, repair, and check the status of migrations. This documentation covers the available commands and their usage. For more information about each command, use the `-h` or `--help` flag.

## Installation

To install the Maestro CLI tool, run the following command:

```bash
go install github.com/maestro-go/maestro@latest
```

## Commands

### `init`

Initializes a new project with migration templates and configuration.

```bash
maestro init
```

This command performs the following steps:
1. Creates a project configuration file (`maestro.yaml`) in the specified location.
2. Sets up migration directories based on the provided locations or default values.
3. Generates example migration files within each migration directory.

> Note: The `maestro.yaml` file is not required. You can use the flags to specify configurations directly.

#### Flags

- `--location, -l`: Specifies the project directory. Default is the current directory.
- `--migrations, -m`: Specifies the migrations directories. Default is `./migrations`.

### `create`

Creates a new migration file with the specified name.

```bash
maestro create [migration_name] -m ./migrations --with-down
```

This command performs the following:
1. Determines the next version number by scanning existing migration files in the configured migration directories.
2. Creates a new placeholder migration file in the first given migration location with the format `VXXX_migration_name.sql`, where `XXX` is the next version number.

#### Flags

- `--with-down, -d`: Generates a down migration file as well.

### `migrate`

Applies the migrations to the database.

```bash
maestro migrate --destination [version]
```

This command performs the following:
1. Connects to the database using the provided configuration.
2. Applies the migrations up to the specified version or the latest version if no version is specified.

#### Flags

- `--destination`: Specifies the target migration version. Default is the latest version.
- `--validate`: Validates migrations before executing. Default is `true`.
- `--down`: Runs migrations in the down direction. Default is `false`.
- `--in-transaction`: Runs migrations within a transaction. Default is `true`.
- `--force`: Continues executing migrations even if errors occur. Default is `false`.
- `--use-repeatable`: Executes repeatable migrations. Default is `true`.
- `--use-before`: Executes before-all hooks. Default is `true`.
- `--use-after`: Executes after-all hooks. Default is `true`.
- `--use-before-each`: Executes before-each hooks. Default is `true`.
- `--use-after-each`: Executes after-each hooks. Default is `true`.
- `--use-before-version`: Executes before-version hooks. Default is `true`.
- `--use-after-version`: Executes after-version hooks. Default is `true`.

### `repair`

Repairs the migration history by recalculating and updating the checksums of migration files.

```bash
maestro repair
```

This command performs the following:
1. Connects to the database using the provided configuration.
2. Recalculates and updates the checksums of migration files in the schema history table.
3. Sets all migrations to `succeeded = true`.

> Note: This is only recommended if you have already run the migration manually, as it sets `succeeded = true`.

### `status`

Shows the status of migrations including the latest migration, validation errors, and failing migrations.

```bash
maestro status
```

This command performs the following:
1. Connects to the database using the provided configuration.
2. Displays the latest migration version.
3. Validates the migrations and displays any validation errors.
4. Displays any failing migrations.

## Global Flags

### `--location, -l`

Specifies the project directory. Default is the current directory.

### `--migrations, -m`

Specifies the migrations directories. Default is `./migrations`.

## Examples

### Initialize a Project

```bash
maestro init
```

### Create a New Migration

```bash
maestro create add_users_table -m ./migrations --with-down
```

### Apply Migrations

```bash
maestro migrate
```

### Repair Migrations

```bash
maestro repair
```

### Check Migration Status

```bash
maestro status
```

For more information about each command, use the `-h` or `--help` flag.

