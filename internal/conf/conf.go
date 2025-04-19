package conf

// Lib version
const VERSION = "v1.0.2"

// Default values
const (
	DEFAULT_PROJECT_FILE   = "maestro.yaml"
	DEFAULT_MIGRATIONS_DIR = "./migrations"
)

const NEW_MIGRATION_PLACEHOLDER = `/*
Insert here your migration
Be sure to use 'BEGIN' and 'COMMIT' if not using transactions so your database don't get incosistent.
*/`

// Regexes
const (
	MIGRATION_REGEX      = `^V(\d+)_([^.]+)\.sql$`
	MIGRATION_DOWN_REGEX = `^V(\d+)_([^.]+)\.down\.sql$`

	HOOK_REPEATABLE_REGEX      = `^R(\d+)_([^.]+)\.sql$`
	HOOK_REPEATABLE_DOWN_REGEX = `^R(\d+)_([^.]+)\.down\.sql$`

	HOOK_BEFORE_REGEX         = `^B(\d+)_([^.]+)\.sql$`
	HOOK_BEFORE_EACH_REGEX    = `^BE(\d+)_([^.]+)\.sql$`
	HOOK_BEFORE_VERSION_REGEX = `^BV(\d+)_(\d+)_([^.]+)\.sql$`

	HOOK_AFTER_REGEX         = `^A(\d+)_([^.]+)\.sql$`
	HOOK_AFTER_EACH_REGEX    = `^AE(\d+)_([^.]+)\.sql$`
	HOOK_AFTER_VERSION_REGEX = `^AV(\d+)_(\d+)_([^.]+)\.sql$`

	TEMPLATE_REGEX = `^([^.]+)\.template\.sql$`
)
