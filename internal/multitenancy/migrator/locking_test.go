package migrator_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/openkcm/cmk/internal/multitenancy/migrator"
)

func TestGenerateLockKey(t *testing.T) {
	// The FNV-1a-derived key must remain stable: a change here would mean a
	// rolling deploy could take a different advisory lock than older binaries.
	tests := []struct {
		name string
		text string
		want int64
	}{
		{name: "test", text: "test", want: -439409999022904539},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, migrator.GenerateLockKey(tt.text))
		})
	}
}
