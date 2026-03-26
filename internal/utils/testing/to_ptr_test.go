package testing

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestToPtr(t *testing.T) {
	v := "test"
	p := ToPtr(v)
	assert.Equal(t, v, *p)

	v2 := 123
	p2 := ToPtr(v2)
	assert.Equal(t, v2, *p2)
}
