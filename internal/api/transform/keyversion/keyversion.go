package keyversion

import (
	"github.com/openkcm/cmk-core/internal/api/cmkapi"
	"github.com/openkcm/cmk-core/internal/api/transform"
	"github.com/openkcm/cmk-core/internal/model"
	"github.com/openkcm/cmk-core/utils/ptr"
)

// ToAPI converts KeyVersion db model to a KeyVersion api model
func ToAPI(kv model.KeyVersion) (*cmkapi.KeyVersion, error) {
	var nativeID *string

	if kv.NativeID != nil {
		nativeID = kv.NativeID
	}

	return &cmkapi.KeyVersion{
		IsPrimary: &kv.IsPrimary,
		Version:   &kv.Version,
		Metadata: ptr.PointTo(cmkapi.KeyVersionMetadata{
			CreatedAt: ptr.PointTo(kv.CreatedAt.Format(transform.DefTimeFormat)),
			UpdatedAt: ptr.PointTo(kv.UpdatedAt.Format(transform.DefTimeFormat)),
		}),
		NativeID: nativeID,
	}, nil
}
