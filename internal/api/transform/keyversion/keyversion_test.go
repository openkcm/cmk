package keyversion_test

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"

	"github.com/openkcm/cmk/internal/api/transform/keyversion"
	"github.com/openkcm/cmk/internal/model"
	"github.com/openkcm/cmk/internal/testutils"
	"github.com/openkcm/cmk/utils/ptr"
)

func TestTransformKeyVersion_ToAPI(t *testing.T) {
	now := time.Now()
	versionID := uuid.New()

	key1 := model.Key{
		ID:        uuid.New(),
		Name:      "key1",
		Provider:  "TEST",
		Algorithm: "AES256",
		Region:    "us-west-2",
	}

	modelKeyVersionMut := testutils.NewMutator(func() model.KeyVersion {
		return model.KeyVersion{
			ID:    versionID,
			KeyID: key1.ID,
			AutoTimeModel: model.AutoTimeModel{
				CreatedAt: now,
			},
			RotatedAt: ptr.PointTo(now),
			NativeID:  "arn:aws:kms:us-west-2:111122223333:alias/my-key-alias",
		}
	})

	tests := []struct {
		name            string
		modelKeyVersion model.KeyVersion
		err             error
	}{
		{
			name:            "KeyVersionToAPI_Success",
			modelKeyVersion: modelKeyVersionMut(),
			err:             nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			keyVersion, err := keyversion.ToAPI(tt.modelKeyVersion)
			if tt.err != nil {
				assert.ErrorIs(t, err, tt.err)
				assert.Nil(t, keyVersion)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.modelKeyVersion.ID.String(), keyVersion.Id.String())
				assert.Equal(t, tt.modelKeyVersion.NativeID, *keyVersion.NativeID)
				// IsPrimary is not set by ToAPI - controller sets it
				assert.Nil(t, keyVersion.IsPrimary)
				// State is not set by ToAPI - controller sets it from parent Key
				assert.Nil(t, keyVersion.State)
			}
		})
	}
}
