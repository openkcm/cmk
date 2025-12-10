package keyversion

import (
	"github.tools.sap/kms/cmk/internal/api/cmkapi"
	"github.tools.sap/kms/cmk/internal/model"
	"github.tools.sap/kms/cmk/utils/ptr"
	"github.tools.sap/kms/cmk/utils/sanitise"
)

// ToAPI converts KeyVersion db model to a KeyVersion api model
func ToAPI(kv model.KeyVersion) (*cmkapi.KeyVersion, error) {
	err := sanitise.Stringlikes(&kv)
	if err != nil {
		return nil, err
	}

	var nativeID *string

	if kv.NativeID != nil {
		nativeID = kv.NativeID
	}

	return &cmkapi.KeyVersion{
		IsPrimary: &kv.IsPrimary,
		Version:   &kv.Version,
		Metadata: ptr.PointTo(cmkapi.KeyVersionMetadata{
			CreatedAt: ptr.PointTo(kv.CreatedAt),
			UpdatedAt: ptr.PointTo(kv.UpdatedAt),
		}),
		NativeID: nativeID,
	}, nil
}
