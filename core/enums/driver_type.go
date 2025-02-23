package enums

type DriverType int8

const (
	DRIVER_POSTGRES DriverType = iota
	DRIVER_COCKROACHDB
)

var MapStringToDriverType = map[string]DriverType{
	"postgres":    DRIVER_POSTGRES,
	"cockroachdb": DRIVER_COCKROACHDB,
}
