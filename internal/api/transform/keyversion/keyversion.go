package keyversion

import (
	"github.com/openkcm/cmk/internal/api/cmkapi"
	"github.com/openkcm/cmk/internal/model"
	"github.com/openkcm/cmk/utils/ptr"
	"github.com/openkcm/cmk/utils/sanitise"
)

// ToAPI converts KeyVersion db model to a KeyVersion api model.
// Note: IsPrimary and State fields are set by the caller (controller) as they require
// context of the parent Key and all versions.
func ToAPI(kv model.KeyVersion) (*cmkapi.KeyVersion, error) {
	err := sanitise.Sanitize(&kv)
	if err != nil {
		return nil, err
	}

	return &cmkapi.KeyVersion{
		Id: &kv.ID,
		// IsPrimary is set by the controller
		// State is set by the controller (from parent Key)
		Metadata: ptr.PointTo(cmkapi.KeyVersionMetadata{
			CreatedAt: ptr.PointTo(kv.CreatedAt),
			UpdatedAt: ptr.PointTo(kv.UpdatedAt),
			RotatedAt: kv.RotatedAt,
		}),
		NativeID: &kv.NativeID,
	}, nil
}
