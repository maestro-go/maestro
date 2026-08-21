# Microsoft SQL Server Driver

This driver provides support for Microsoft SQL Server.

## Configuration

To use the MSSQL driver, set the driver type to `mssql` or `sqlserver` in your configuration.

### Connection String

The driver uses the `github.com/microsoft/go-mssqldb` driver. The connection details are provided via the `maestro` configuration:

- `host`: The hostname or IP address of the SQL Server.
- `port`: The port number (default is 1433).
- `user`: The database user.
- `password`: The user's password.
- `database`: The name of the database.

## Implementation Details

### Schema History Table

The driver manages a schema history table (default name: `schema_history`) with the following structure:

| Column | Type | Description |
| :--- | :--- | :--- |
| `version` | `SMALLINT` | Migration version (Primary Key) |
| `description` | `NVARCHAR(255)` | Migration description |
| `md5_checksum` | `CHAR(32)` | MD5 checksum of the migration content |
| `success` | `BIT` | Execution status (1 for success, 0 for failure) |
| `executed_at` | `DATETIME2` | Timestamp of execution |
| `repaired_at` | `DATETIME2` | Timestamp of the last repair |

### Locking Mechanism

To ensure safe concurrent migrations, the driver uses application-level locks via `sp_getapplock` and `sp_releaseapplock` with a session-level scope.

### Transactions

The driver supports executing migrations within transactions. If enabled, each migration will be wrapped in a transaction, and the schema history table will be updated within the same transaction.
