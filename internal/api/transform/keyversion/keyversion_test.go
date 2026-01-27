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
	"github.com/openkcm/cmk/utils/ptr"
)

func TestTransformKeyVersion_ToAPI(t *testing.T) {
	now := time.Now()

	key1 := model.Key{
		ID:        uuid.New(),
		Name:      "key1",
		Provider:  "TEST",
		Algorithm: "AES256",
		Region:    "us-west-2",
	}

	modelKeyVersionMut := testutils.NewMutator(func() model.KeyVersion {
		return model.KeyVersion{
			ExternalID: uuid.New().String(),
			KeyID:      key1.ID,
			AutoTimeModel: model.AutoTimeModel{
				CreatedAt: now,
			},
			Version:   1,
			IsPrimary: true,
			NativeID:  ptr.PointTo("arn:aws:kms:us-west-2:111122223333:alias/<alias-name>"),
		}
	})

	apiKeyVersionMut := testutils.NewMutator(func() cmkapi.KeyVersion {
		return cmkapi.KeyVersion{
			IsPrimary: ptr.PointTo(true),
			Version:   ptr.PointTo(1),
			Metadata: ptr.PointTo(cmkapi.KeyVersionMetadata{
				CreatedAt: ptr.PointTo(now),
				UpdatedAt: ptr.PointTo(now),
			}),
			NativeID: ptr.PointTo("arn:aws:kms:us-west-2:111122223333:alias/<alias-name>"),
		}
	})

	tests := []struct {
		name            string
		modelKeyVersion model.KeyVersion
		expected        cmkapi.KeyVersion
		err             error
	}{
		{
			name:            "KeyVersionToAPI_Success",
			modelKeyVersion: modelKeyVersionMut(),
			expected:        apiKeyVersionMut(),
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
				assert.Equal(t, tt.expected.Version, keyVersion.Version)
			}
		})
	}
}
