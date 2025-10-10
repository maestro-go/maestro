# 💾 SQLite3

> [!WARNING]
> Our library is specifically designed to be used with the **`sqlite3`** driver (`github.com/mattn/go-sqlite3`). We strongly recommend this driver because it fully supports **query parameter binding**, a crucial security feature that prevents **SQL injection**. Other SQLite drivers may not offer this protection and will not work as expected.
>
> If you choose to use a different driver, please ensure it supports parameter binding. Regardless of the driver used, you must pass **`"sqlite3"`** as the driver name to our library.

---

## ⚙️ Configuration

The following configuration options are available for the SQLite3 driver.

### Required

- `database`: The file path to your SQLite database. (e.g., `./test.db`)
- `driver`: Must be set to `sqlite3`.

### Ignored

- `host`, `port`, `schema`, `user`, `password`: These parameters are not applicable for a file-based database like SQLite and will be ignored.

---

## 🚀 Examples

### CLI

To use the CLI, ensure you specify the correct database path and driver name.

```bash
maestro migrate --database=./test.db --driver sqlite3
```

### Go library

```go
import (
    "context"
    "database/sql"
    "log"
    "go.uber.org/zap"

   _ "github.com/mattn/go-sqlite3"
    "github.com/maestro-go/maestro/core/conf"
    "github.com/maestro-go/maestro/core/database/sqlite3"
    "github.com/maestro-go/maestro/core/migrator"
)

func main() {
    ctx := context.Background()
    logger, _ := zap.NewProduction()
    defer logger.Sync()

    config := createConfig()

    db, err := sql.Open("sqlite3", "your-sqlite-database-location")
    if err != nil {
        log.Fatal(err)
    }

    // Initializes a new SQLite3 repository instance.
    // You can pass a value for the third parameter (history table name), but in this case, it will use the default (schema_history).
  repo := sqlite3.NewSQLiteRepository(ctx, db, nil)
    migrator := migrator.NewMigrator(logger, repo, config)

    err = migrator.Migrate()
    if err != nil {
        log.Fatal(err)
    }

    log.Println("Migrations applied successfully")
}
```
