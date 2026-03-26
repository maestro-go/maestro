package enums

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestHookTypeName(t *testing.T) {
	tests := []struct {
		name string
		h    HookType
		want string
	}{
		{name: "repeatable", h: HOOK_REPEATABLE, want: "REPEATABLE"},
		{name: "repeatable_down", h: HOOK_REPEATABLE_DOWN, want: "REPEATABLE_DOWN"},
		{name: "before", h: HOOK_BEFORE, want: "BEFORE"},
		{name: "before_each", h: HOOK_BEFORE_EACH, want: "BEFORE_EACH"},
		{name: "before_version", h: HOOK_BEFORE_VERSION, want: "BEFORE_VERSION"},
		{name: "after", h: HOOK_AFTER, want: "AFTER"},
		{name: "after_each", h: HOOK_AFTER_EACH, want: "AFTER_EACH"},
		{name: "after_version", h: HOOK_AFTER_VERSION, want: "AFTER_VERSION"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.h.Name())
		})
	}
}

func TestMigrationTypeName(t *testing.T) {
	tests := []struct {
		name string
		m    MigrationType
		want string
	}{
		{name: "up", m: MIGRATION_UP, want: "UP"},
		{name: "down", m: MIGRATION_DOWN, want: "DOWN"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.m.Name())
		})
	}
}
