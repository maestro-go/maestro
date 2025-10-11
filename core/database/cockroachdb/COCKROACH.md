# 🪳 CockroachDB

> [!NOTE]
> You must pass **`"cockroachdb"`** as the driver name to our library.

> [!WARNING]
> This driver uses a lock table (`schema_lock`) to manage concurrent migrations. If the migration process is interrupted unexpectedly, this table may not be cleaned up automatically. If you encounter a `timeout while waiting for schema_lock deletion` error, you will need to manually drop the `schema_lock` table from your database to proceed with new migrations.

---

## ⚙️ Configuration

The following configuration options are available for the CockroachDB driver.

### Required

- `database`: The name of the database to connect to.
- `driver`: Must be set to `cockroachdb`.
- `host`: The server host or IP address.
- `port`: The server port.
- `user`: The username for authentication.
- `password`: The password for authentication.

### Optional

- `schema`: The default schema to use. Defaults to `public`.
- `sslmode`: The SSL mode. Can be `disable`, `allow`, `prefer`, `require`, `verify-ca`, or `verify-full`.
- `sslrootcert`: The path to the SSL root certificate file.

---

## 🚀 Examples

### CLI

To use the CLI, ensure you specify the correct database connection details.

```bash
maestro migrate --database=mydb --driver=cockroachdb --host=localhost --port=26257 --user=myuser --password=mypassword --schema=public --sslmode=disable
```

### Go library

```go
import (
    "context"
    "database/sql"
    "log"
    "go.uber.org/zap"

    _ "github.com/lib/pq"
    "github.com/maestro-go/maestro/core/conf"
    "github.com/maestro-go/maestro/core/database/cockroachdb"
    "github.com/maestro-go/maestro/core/migrator"
)

func main() {
    ctx := context.Background()
    logger, _ := zap.NewProduction()
    defer logger.Sync()

    config := createConfig()

    db, err := sql.Open("postgres", "host=localhost port=26257 user=myuser password=mypassword dbname=mydb sslmode=disable")
    if err != nil {
        log.Fatal(err)
    }

    // Initializes a new CockroachDB repository instance.
    // You can pass a value for the third parameter (history table name), but in this case, it will use the default (schema_history).
    repo := cockroachdb.NewCockroachRepository(ctx, db, nil)
    migrator := migrator.NewMigrator(logger, repo, config)

    err = migrator.Migrate()
    if err != nil {
        log.Fatal(err)
    }

    log.Println("Migrations applied successfully")
}
```
