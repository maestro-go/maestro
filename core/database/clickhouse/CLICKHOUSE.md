# 🗄️ ClickHouse

> [!NOTE]
> You must pass **`"clickhouse"`** as the driver name to our library.

---

## ⚙️ Configuration

The following configuration options are available for the ClickHouse driver.

### Required

- `database`: The name of the database to connect to.
- `driver`: Must be set to `clickhouse`.
- `host`: The server host or IP address.
- `port`: The server port.
- `user`: The username for authentication.
- `password`: The password for authentication.

### Optional

- `history_table`: The name of the table used to track migration history. Defaults to `schema_history`.

### SSH (CLI only)

- `ssh-host`: The SSH server host.
- `ssh-port`: The SSH server port. Defaults to `22`.
- `ssh-user`: The SSH username.
- `ssh-password`: The SSH password.
- `ssh-key-path`: The path to the SSH private key file.
- `ssh-passphrase`: The passphrase for the SSH private key.

---

## 🚀 Examples

### CLI

To use the CLI, ensure you specify the correct database connection details.

```bash
maestro migrate --database=test_db --driver=clickhouse --host=localhost --port=9000 --user=default --password=password
```

### Go library

```go
import (
    "context"
    "database/sql"
    "log"
    "go.uber.org/zap"

    _ "github.com/ClickHouse/clickhouse-go/v2"
    "github.com/maestro-go/maestro/core/conf"
    "github.com/maestro-go/maestro/core/database/clickhouse"
    "github.com/maestro-go/maestro/core/migrator"
)

func main() {
    ctx := context.Background()
    logger, _ := zap.NewProduction()
    defer logger.Sync()

    config := createConfig()

    db, err := sql.Open("clickhouse", "clickhouse://default:password@localhost:9000/test_db")
    if err != nil {
        log.Fatal(err)
    }

    // Initializes a new ClickHouse repository instance.
    // You can pass a value for the third parameter (history table name), but in this case, it will use the default (schema_history).
    repo := clickhouse.NewClickHouseRepository(ctx, db, nil)
    migrator := migrator.NewMigrator(logger, repo, config)

    err = migrator.Migrate()
    if err != nil {
        log.Fatal(err)
    }

    log.Println("Migrations applied successfully")
}
```

---

## 💡 ClickHouse Specifics

### Transactions

ClickHouse does not support multi-statement transactions in the traditional sense for all engine types. The `DoInTransaction` method in this driver is currently a no-op that simply executes the provided function. If you need atomicity, ensure your migration scripts are designed to be idempotent or use the Atomic database engine if supported by your ClickHouse version.

### Locking

ClickHouse does not have built-in advisory locks. This driver implements a locking mechanism using a temporary `Memory` engine table (e.g., `schema_history_lock`). 

The `DoInLock` method attempts to create this table atomically. If the table already exists, it will retry for up to 60 seconds. If the lock table exists for more than 10 minutes, it is considered stale and will be automatically cleared.

### Schema History Table

The schema history table uses the `ReplacingMergeTree` engine, ordered by `version` and versioned by `executed_at`. This allows `maestro` to "update" migration status and checksums by inserting new rows. Queries to this table use the `FINAL` modifier to ensure the latest state is retrieved.

### DDL and Mutations

ClickHouse DDL statements (like `CREATE TABLE`) and mutations (like `ALTER TABLE ... DELETE`) have different behaviors:
- DDL statements are generally synchronous.
- Mutations are asynchronous. The `RollbackMigration` method uses `ALTER TABLE ... DELETE` which might take some time to complete in the background.

## Connection DSN

The driver uses the following DSN format:
`clickhouse://user:password@host:port/database`
