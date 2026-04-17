package keyversion_test

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"

	"github.com/openkcm/cmk/internal/api/cmkapi"
	"github.com/openkcm/cmk/internal/api/transform/keyversion"
	"github.com/openkcm/cmk/internal/model"
	"github.com/openkcm/cmk/internal/testutils"
)

func TestTransformKeyVersion_ToAPI(t *testing.T) {
	now := time.Now()
	versionID := uuid.New()
	latestVersionID := uuid.New()

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
			RotatedAt: now,
			NativeID:  "arn:aws:kms:us-west-2:111122223333:alias/my-key-alias",
		}
	})

	tests := []struct {
		name              string
		modelKeyVersion   model.KeyVersion
		latestVersionID   uuid.UUID
		keyState          cmkapi.KeyState
		expectedIsPrimary bool
		err               error
	}{
		{
			name:              "KeyVersionToAPI_Success_IsPrimary",
			modelKeyVersion:   modelKeyVersionMut(),
			latestVersionID:   versionID, // Same as version ID, so isPrimary=true
			keyState:          cmkapi.KeyStateENABLED,
			expectedIsPrimary: true,
			err:               nil,
		},
		{
			name:              "KeyVersionToAPI_Success_NotPrimary",
			modelKeyVersion:   modelKeyVersionMut(),
			latestVersionID:   latestVersionID, // Different from version ID, so isPrimary=false
			keyState:          cmkapi.KeyStateDISABLED,
			expectedIsPrimary: false,
			err:               nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			keyVersion, err := keyversion.ToAPI(tt.modelKeyVersion, tt.latestVersionID, tt.keyState)
			if tt.err != nil {
				assert.ErrorIs(t, err, tt.err)
				assert.Nil(t, keyVersion)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.modelKeyVersion.ID.String(), keyVersion.Id.String())
				assert.Equal(t, tt.modelKeyVersion.NativeID, *keyVersion.NativeID)
				assert.NotNil(t, keyVersion.Metadata)
				assert.NotNil(t, keyVersion.Metadata.RotatedAt)
				assert.Equal(t, tt.modelKeyVersion.RotatedAt, *keyVersion.Metadata.RotatedAt)
				// IsPrimary is set by ToAPI based on latestVersionID
				assert.NotNil(t, keyVersion.IsPrimary)
				assert.Equal(t, tt.expectedIsPrimary, *keyVersion.IsPrimary)
				// State is set by ToAPI from keyState parameter
				assert.NotNil(t, keyVersion.State)
				assert.Equal(t, tt.keyState, *keyVersion.State)
			}
		})
	}
}
