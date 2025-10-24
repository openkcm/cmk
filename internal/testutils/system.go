package testutils

import (
	"github.com/google/uuid"

	"github.com/openkcm/cmk-core/internal/model"
)

// SystemMutator provides a base System for testing
var SystemMutator = NewMutator(func() model.System {
	return model.System{
		ID:                 uuid.New(),
		Region:             "test-region",
		KeyConfigurationID: nil,
	}
})

// CreateTestSystem creates a test System in the database
func CreateTestSystem(mut func(*model.System)) *model.System {
	key := SystemMutator(mut)

	return &key
}
