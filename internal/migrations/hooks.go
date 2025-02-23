package migrations

import "github.com/maestro-go/maestro/core/enums"

type Hook struct {
	Order   uint8
	Version uint16 // Only used in hooks with order and version
	Content *string
	Type    enums.HookType
}
