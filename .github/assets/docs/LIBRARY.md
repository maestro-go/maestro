# Maestro Go Library Documentation

Maestro is a powerful Go library designed for seamless database migration management.
This documentation provides detailed instructions on how to use the `core` package of Maestro as a **Go library**.

## Installation

To include Maestro in your Go project, run:

```bash
go get github.com/maestro-go/maestro/core
```

## Usage

### Importing the Library

```go
import (
    "github.com/maestro-go/maestro/core/conf"
    "github.com/maestro-go/maestro/core/database/postgres" // Use your driver here
    "github.com/maestro-go/maestro/core/migrator"
)
```

### Configuration

You can configure Maestro either by loading the configuration from a file or by writing it directly in your code.

#### Loading Configuration from File

To load the configuration from the `maestro.yaml` file, use the `LoadConfigFromFile` function:

```go
import (
    "github.com/maestro-go/maestro/core/conf"
)

func loadConfig() (*conf.MigrationConfig, error) {
    config := &conf.ProjectConfig{}
    err := conf.LoadConfigFromFile("path/to/maestro.yaml", config)
    if err != nil {
        return nil, err
    }
    return &config.Migration, nil
}
```

#### Writing Configuration Directly

You can also write the configuration directly in your code:

```go
import (
    "github.com/maestro-go/maestro/core/conf"
)

func createConfig() *conf.MigrationConfig {
    return &conf.MigrationConfig{
        Locations:        []string{"./migrations"},
        Validate:         true,
        Down:             false,
        InTransaction:    true,
        UseRepeatable:    true,
        UseBefore:        true,
        UseAfter:         true,
        UseBeforeEach:    true,
        UseAfterEach:     true,
        UseBeforeVersion: true,
        UseAfterVersion:  true,
    }
}
```

### Migrating

To create a new migrator, you need to initialize the repository using your `*sql.DB` connection, and then create the migrator itself.
Here is an example using PostgreSQL:

```go
import (
    "context"
    "database/sql"
    "log"
    "go.uber.org/zap"

    _ "github.com/lib/pq"
    "github.com/maestro-go/maestro/core/conf"
    "github.com/maestro-go/maestro/core/database/postgres"
    "github.com/maestro-go/maestro/core/migrator"
)

func main() {
    ctx := context.Background()
    logger, _ := zap.NewProduction()
    defer logger.Sync()

    config := createConfig()

    db, err := sql.Open("postgres", "your-postgres-connection-string")
    if err != nil {
        log.Fatal(err)
    }

    // Initializes a new Postgres repository instance.
    // You can pass a value for the third parameter (history table name), but in this case, it will use the default (schema_history).
    repo := postgres.NewPostgresRepository(ctx, db, nil)
    migrator := migrator.NewMigrator(logger, repo, config)

    err = migrator.Migrate()
    if err != nil {
        log.Fatal(err)
    }

    log.Println("Migrations applied successfully")
}
```

#### Zap Logger

You can pass a [zap logger](https://github.com/uber-go/zap) to the `NewMigrator` function to enable logging.
If you prefer not to log anything, you can pass `nil` instead.

## Repository

Maestro allows direct interaction with the repository for tasks such as repairing migrations, debugging or custom logging.
For more details, refer to the [database folder](../../../core/database).

### Custom Repository

If you need to use a database that is not supported by Maestro, you can implement a custom repository.
This involves creating a new repository type that satisfies the [`Repository` interface](../../../core/database/repository.go) defined in the library.
