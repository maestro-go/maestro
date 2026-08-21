# 🐬 MySQL

> [!NOTE]
> You must pass **`"mysql"`** or **`"mariadb"`** as the driver name to our library.

---

## ⚙️ Configuration

The following configuration options are available for the MySQL driver.

### Required

- `database`: The name of the database to connect to.
- `driver`: Must be set to `mysql`.
- `host`: The server host or IP address.
- `port`: The server port.
- `user`: The username for authentication.
- `password`: The password for authentication.

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
maestro migrate --database=mydb --driver=mysql --host=localhost --port=3306 --user=myuser --password=mypassword
```

### Go library

```go
import (
    "context"
    "database/sql"
    "log"
    "go.uber.org/zap"

    _ "github.com/go-sql-driver/mysql"
    "github.com/maestro-go/maestro/core/conf"
    "github.com/maestro-go/maestro/core/database/mysql"
    "github.com/maestro-go/maestro/core/migrator"
)

func main() {
    ctx := context.Background()
    logger, _ := zap.NewProduction()
    defer logger.Sync()

    config := createConfig()

    db, err := sql.Open("mysql", "myuser:mypassword@tcp(localhost:3306)/mydb?parseTime=true")
    if err != nil {
        log.Fatal(err)
    }

    // Initializes a new MySQL repository instance.
    // You can pass a value for the third parameter (history table name), but in this case, it will use the default (schema_history).
    repo := mysql.NewMySQLRepository(ctx, db, nil)
    migrator := migrator.NewMigrator(logger, repo, config)

    err = migrator.Migrate()
    if err != nil {
        log.Fatal(err)
    }

    log.Println("Migrations applied successfully")
}
```

---

## ⚠️ Important Considerations

### DDL Transactions

MySQL does **not** support DDL transactions (e.g., `CREATE TABLE`, `ALTER TABLE`, `DROP TABLE`). Any DDL statement will cause an **implicit commit**, meaning that if a migration containing both DDL and DML (or multiple DDLs) fails, the changes made by preceding statements will NOT be rolled back.

It is recommended to keep each migration script atomic and focused on a single logical change to minimize the risk of partial migration states.
