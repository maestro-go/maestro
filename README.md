![GitHub Release](https://img.shields.io/github/v/release/maestro-go/maestro?color=red)
![Supported Go Versions](https://img.shields.io/badge/Go-1.22%2C%201.23-blue.svg)
![Go Report Card](https://goreportcard.com/badge/github.com/maestro-go/maestro)

<p align="center">
<img src="./.github/assets/imgs/logo.png" alt="Maestro Logo" width="300">
</p>

# Maestro

Maestro is a powerful **Go library** and **CLI tool** designed for seamless database migration management.

Just as a maestro orchestrates a symphony, `maestro` orchestrates your database changes, conducting migrations with precision and grace. It ensures your database schema evolves smoothly, keeping all your environments in perfect harmony while maintaining a clear record of every change.

## Quick Start

```bash
# Install as CLI tool
go install github.com/maestro-go/maestro@latest

# Create a new migration
maestro create add_users_table -m ./migrations --with-down

# Run migrations
maestro migrate
```

## Index

- [📦 Installation](#installation)
- [🗄️ Supported Databases](#supported-databases)
- [✨ Key Features](#key-features)
- [📁 Migrations](#migrations)
  - [📄 Migration Files](#migrations-files)
  - [📍 Migration Destination](#migrations-destination)
  - [⬇️ Migrating Down](#migrating-down)
  - [🔧 Repair Migrations](#migrations-repair)
  - [🔍 Check Status](#migrations-status)
  - [📑 Templates](#templates)
- [⚠️ Warnings](#warnings)
- [📚 Documentation](#documentation)
- [🤝 Contributing](#contributing)
- [📜 License](#license)

## Installation

### CLI Tool
```bash
go install github.com/maestro-go/maestro@latest
```

### Go Library
```bash
go get github.com/maestro-go/maestro/core
```

## Supported Databases

### Currently Supported
- ✅ [PostgreSQL](https://www.postgresql.org)  
- ✅ [CockroachDB](https://www.cockroachlabs.com)

### In Progress
- 🚧 MySQL  
- 🚧 SQLite  
- 🚧 ClickHouse

## Key Features

- ✨ Manage up/down migrations effortlessly
- 🛠️ Repair migrations seamlessly
- 🔒 Validate migrations with MD5 checksums
- 🪝 Utilize a flexible hooks system
- 📝 Track migration history clearly

### Upcoming Features

- 🔑 Built-in SSH tunnel support

## Migrations

### Migrations Files

Maestro uses a simple naming convention for migration files:

```
📁 migrations/
├── 📄 V001_create_users.sql         # Up migration
├── 📄 V001_create_users.down.sql    # Down migration (optional)
├── 📄 V002_add_email_column.sql
└── 📄 V002_add_email_column.down.sql
```

If you're using hooks, the recommended folder structure is:

```
📁 migrations/
├── 📄 V001_example.sql
├── 📄 V001_example.down.sql
├── 📁 before/
├── 📁 beforeEach/
├── 📁 beforeVersion/
├── 📁 afterVersion/
├── 📁 afterEach/
├── 📁 after/
├── 📁 repeatable/
├── 📁 repeatableDown/
```

Create new migrations using the CLI:
```bash
maestro create add_users_table -m ./migrations --with-down
```

### Migrations Destination

Control which migrations to run using destination:

```bash
# Run migrations up to 10
maestro migrate --destination 10

# Run migrations down to 5
maestro migrate --down --destination 5
```

### Migrating down

When performing a downward migration, ensure that each upward migration has a corresponding downward migration file.
Failure to do so may result in inconsistencies.

### Migrations Repair

If you encounter checksum mismatches or other issues with your migration history, you can use the `repair` command to fix them. This command recalculates and updates the checksums of your migration files, ensuring that the recorded checksums match the actual files.

```bash
maestro repair
```

**Note:** Using `repair` is not recommended as the primary fix. However, if you need to change old migrations and hooks cannot solve the problem, the `repair` command can be used to maintain the integrity of your migration history.

### Migrations Status

Check the current migrations status, like latest applied migration and failed migrations:

```bash
maestro status
```

### Templates
Maestro supports the use of templates to simplify and standardize your migration files. Templates allow you to define reusable content that can be dynamically replaced with specific values during migration execution.

To use templates, create a template file in your migrations directory with the `.template.sql` extension. For example:

```
📁 migrations/
├── 📄 V001_create_users.sql
├── 📄 V001_create_users.down.sql
└── 📄 table_template.template.sql
```

In your migration files, you can reference the template using the `{{template_name, value1, value2}}` syntax. Maestro will replace the template placeholders with the provided values.

Example template file (`table_template.template.sql`):
```sql
CREATE TABLE $1 (
  id SERIAL PRIMARY KEY,
  $2 VARCHAR(255) NOT NULL,
  $3 VARCHAR(255) UNIQUE NOT NULL,
  created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
```

Example migration file using the template:
```
{{table_template, users, name, email}}
```

Maestro will replace `{{table_template, users, name, email}}` with the content of `table_template.template.sql`, resulting in:
```sql
CREATE TABLE users (
  id SERIAL PRIMARY KEY,
  name VARCHAR(255) NOT NULL,
  email VARCHAR(255) UNIQUE NOT NULL,
  created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
```

## Warnings

### Force

You can force migrations using the `force` flag/config. However, it is not compatible with the `in-transaction` flag/config. When using transactions, forcing a migration that encounters an error will result in the entire transaction being rolled back.

## Documentation

Detailed documentation is available:
- [CLI Tool Documentation](./.github/assets/docs/CLI.md)
- [Go Library Documentation](./.github/assets/docs/LIBRARY.md)
- [Hooks Documentation](./.github/assets/docs/HOOKS.md)

## Contributing

We welcome contributions! Please read our:
- [Code of Conduct](./CODE_OF_CONDUCT.md)
- [Contributing Guide](./CONTRIBUTING.md)

## License

This project is licensed under the MIT License - see the [LICENSE](./LICENSE) file for details.
