# 🦅 Oracle

> [!NOTE]
> You must pass **`"oracle"`** as the driver name to our library.

---

## ⚙️ Configuration

The following configuration options are available for the Oracle driver.

### Required

- `database`: The Service Name or SID of the Oracle database.
- `driver`: Must be set to `oracle`.
- `host`: The server host or IP address.
- `port`: The server port (defaults to `1521`).
- `user`: The username for authentication.
- `password`: The password for authentication.

### Optional

- `history_table`: The name of the table used to track migration history. Defaults to `SCHEMA_HISTORY`.

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
maestro migrate --database=FREEPDB1 --driver=oracle --host=localhost --port=1521 --user=SYSTEM --password=password
```

### Go library

```go
import (
    "context"
    "database/sql"
    "log"
    "go.uber.org/zap"

    _ "github.com/sijms/go-ora/v2"
    "github.com/maestro-go/maestro/core/conf"
    "github.com/maestro-go/maestro/core/database/oracle"
    "github.com/maestro-go/maestro/core/migrator"
)

func main() {
    ctx := context.Background()
    logger, _ := zap.NewProduction()
    defer logger.Sync()

    config := createConfig()

    // go-ora format: oracle://user:password@host:port/service_name
    db, err := sql.Open("oracle", "oracle://SYSTEM:password@localhost:1521/FREEPDB1")
    if err != nil {
        log.Fatal(err)
    }

    // Initializes a new Oracle repository instance.
    // You can pass a value for the third parameter (history table name), but in this case, it will use the default (SCHEMA_HISTORY).
    repo := oracle.NewOracleRepository(ctx, db, nil)
    migrator := migrator.NewMigrator(logger, repo, config)

    err = migrator.Migrate()
    if err != nil {
        log.Fatal(err)
    }

    log.Println("Migrations applied successfully")
}
```

---

## 💡 Oracle Specifics

### Case Sensitivity

By default, Oracle converts all unquoted identifiers to uppercase. Maestro follows this convention for the schema history table (`SCHEMA_HISTORY`) and the lock table (`MAESTRO_LOCK`).

### Locking Mechanism

Maestro implements a locking mechanism to prevent concurrent migrations. In Oracle, it creates a `MAESTRO_LOCK` table and uses `SELECT ... FOR UPDATE` on a dedicated connection to acquire a lock. This ensures that even if a migration script performs an implicit commit (common with DDL in Oracle), the migration lock remains held until the entire migration process for that version is complete.

### Schema History Table

The schema history table uses `NUMBER(1)` to represent boolean success status (0 for false, 1 for true) and `TIMESTAMP` for execution and repair times.

### DDL and Transactions

Like many other databases, Oracle performs an implicit commit before and after any DDL statement (e.g., `CREATE TABLE`, `ALTER TABLE`). While Maestro supports running migrations in transactions, be aware that DDL statements will effectively split the transaction.

## Connection DSN

The driver uses the following DSN format provided by `go-ora`:
`oracle://user:password@host:port/service_name`
