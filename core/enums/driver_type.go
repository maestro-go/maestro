package enums

type DriverType int8

const (
	DRIVER_POSTGRES DriverType = iota
	DRIVER_COCKROACHDB
	DRIVER_SQLITE3
	DRIVER_MYSQL
)

var MapStringToDriverType = map[string]DriverType{
	"postgres":    DRIVER_POSTGRES,
	"cockroachdb": DRIVER_COCKROACHDB,
	"sqlite3":     DRIVER_SQLITE3,
	"mysql":       DRIVER_MYSQL,
	"mariadb":     DRIVER_MYSQL,
}
