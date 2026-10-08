package keyversion

import (
	"github.com/google/uuid"

	cmkapi "github.com/openkcm/cmk/internal/api/cmk/generated"
	"github.com/openkcm/cmk/internal/model"
	"github.com/openkcm/cmk/utils/sanitise"
)

// ToAPI converts KeyVersion db model to a KeyVersion api model.
// Requires latestVersionID (from GetLatestVersion) and keyState (from parent Key)
// to correctly set IsPrimary and State fields regardless of pagination.
func ToAPI(kv model.KeyVersion, latestVersionID uuid.UUID, keyState cmkapi.KeyState) (*cmkapi.KeyVersion, error) {
	err := sanitise.Sanitize(&kv)
	if err != nil {
		return nil, err
	}

	isPrimary := kv.ID == latestVersionID

	return &cmkapi.KeyVersion{
		Id:        &kv.ID,
		IsPrimary: &isPrimary,
		State:     &keyState,
		Metadata: &cmkapi.KeyVersionMetadata{
			CreatedAt: new(kv.CreatedAt),
			UpdatedAt: new(kv.UpdatedAt),
			RotatedAt: new(kv.RotatedAt),
		},
		NativeID: &kv.NativeID,
	}, nil
}
