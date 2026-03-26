package migrations

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestValidateMigrations(t *testing.T) {
	tests := []struct {
		name       string
		migrations []*Migration
		wantErr    bool
	}{
		{
			name:       "valid migrations",
			migrations: []*Migration{{Version: 1}, {Version: 2}},
			wantErr:    false,
		},
		{
			name:       "gap in versions",
			migrations: []*Migration{{Version: 1}, {Version: 3}},
			wantErr:    true,
		},
		{
			name:       "starts from wrong version",
			migrations: []*Migration{{Version: 2}},
			wantErr:    true,
		},
		{
			name:       "empty migrations",
			migrations: []*Migration{},
			wantErr:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errs := ValidateMigrations(tt.migrations)
			if tt.wantErr {
				assert.NotEmpty(t, errs)
			} else {
				assert.Empty(t, errs)
			}
		})
	}
}
