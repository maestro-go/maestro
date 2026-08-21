# Snowflake Driver

This driver provides support for Snowflake.

## Configuration

To use the Snowflake driver, set the driver type to `snowflake` in your configuration.

### Connection String

The driver uses the `github.com/snowflakedb/gosnowflake` driver. The connection details are provided via the `maestro` configuration:

- `host`: The hostname of the Snowflake endpoint. Leave empty to use the default Snowflake service.
- `port`: The port number (default is 443).
- `account`: The Snowflake account identifier.
- `user`: The database user.
- `password`: The user's password.
- `database`: The name of the database.
- `schema`: The name of the schema (default is `public`).
- `warehouse`: The name of the virtual warehouse (optional).
- `role`: The name of the role (optional).

### Local Development

Snowflake is a fully managed cloud service and does not ship a local container image. For local development and testing, the [LocalStack Snowflake emulator](https://docs.localstack.cloud/snowflake/) can be used. Set `LOCALSTACK_AUTH_TOKEN` and configure `host`, `port` and `account` matching your emulator setup.

## Implementation Details

### Schema History Table

The driver manages a schema history table (default name: `SCHEMA_HISTORY`) with the following structure:

| Column | Type | Description |
| :--- | :--- | :--- |
| `version` | `NUMBER(5,0)` | Migration version (Primary Key) |
| `description` | `VARCHAR(255)` | Migration description |
| `md5_checksum` | `VARCHAR(32)` | MD5 checksum of the migration content |
| `success` | `BOOLEAN` | Execution status (TRUE for success, FALSE for failure) |
| `executed_at` | `TIMESTAMP_NTZ` | Timestamp of execution |
| `repaired_at` | `TIMESTAMP_NTZ` | Timestamp of the last repair |

Snowflake object identifiers are case-insensitive and stored in uppercase, so the history table name is uppercased when configured.

### Locking Mechanism

To ensure safe concurrent migrations, the driver uses a lock table (default name: `SCHEMA_LOCK`). The table is created to acquire the lock and dropped when the migration process finishes; other instances wait up to one minute for it to be released.

### Transactions

Snowflake does not support transactional DDL: each DDL statement implicitly commits any active transaction and executes on its own. As a result, migrations are not executed within a transaction.

## Testing

Snowflake tests require a live endpoint and are skipped unless one is available:

- Set `SNOWFLAKE_DSN` to a gosnowflake DSN string (e.g. `user:password@account/database/schema?warehouse=...`), or
- Set `LOCALSTACK_AUTH_TOKEN` to run against the LocalStack Snowflake emulator.