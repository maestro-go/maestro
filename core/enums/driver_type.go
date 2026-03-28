package enums

type DriverType int8

const (
	DRIVER_POSTGRES DriverType = iota
	DRIVER_COCKROACHDB
	DRIVER_SQLITE3
	DRIVER_MYSQL
	DRIVER_CLICKHOUSE
)

var MapStringToDriverType = map[string]DriverType{
	"postgres":    DRIVER_POSTGRES,
	"cockroachdb": DRIVER_COCKROACHDB,
	"sqlite3":     DRIVER_SQLITE3,
	"mysql":       DRIVER_MYSQL,
	"mariadb":     DRIVER_MYSQL,
	"clickhouse":  DRIVER_CLICKHOUSE,
}
