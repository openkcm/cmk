package keyconfiguration_test

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"

	"github.com/openkcm/cmk-core/internal/api/cmkapi"
	"github.com/openkcm/cmk-core/internal/api/transform/keyconfiguration"
	"github.com/openkcm/cmk-core/internal/apierrors"
	"github.com/openkcm/cmk-core/internal/errs"
	"github.com/openkcm/cmk-core/internal/model"
	"github.com/openkcm/cmk-core/internal/testutils"
	"github.com/openkcm/cmk-core/utils/ptr"
)

func TestTransformKeyConfiguration_FromAPI(t *testing.T) {
	description := "Test key configuration"
	adminGroupID := uuid.New()

	apiKeyConfigMut := testutils.NewMutator(func() cmkapi.KeyConfiguration {
		return cmkapi.KeyConfiguration{
			Name:         "test-key-config",
			Description:  &description,
			AdminGroupID: adminGroupID,
		}
	})

	modelKeyConfigMut := testutils.NewMutator(func() model.KeyConfiguration {
		return model.KeyConfiguration{
			Name:         "test-key-config",
			Description:  description,
			AdminGroupID: adminGroupID,
		}
	})

	tests := []struct {
		name     string
		apiConf  cmkapi.KeyConfiguration
		expected model.KeyConfiguration
		err      error
	}{
		{
			name:     "KeyConfigFromAPI_Success",
			apiConf:  apiKeyConfigMut(),
			expected: modelKeyConfigMut(),
			err:      nil,
		},
		{
			name: "KeyConfigFromAPI_NoDescription",
			apiConf: apiKeyConfigMut(func(k *cmkapi.KeyConfiguration) {
				k.Description = nil
			}),
			expected: modelKeyConfigMut(func(k *model.KeyConfiguration) {
				k.Description = ""
			}),
			err: nil,
		},
		{
			name: "KeyConfigFromAPI_MissingName",
			apiConf: apiKeyConfigMut(func(k *cmkapi.KeyConfiguration) {
				k.Name = ""
			}),
			expected: model.KeyConfiguration{},
			err:      errs.Wrapf(apierrors.ErrNameFieldMissingProperty, "name"),
		},
		{
			name: "KeyConfigFromAPI_MissingAdminGroupID",
			apiConf: apiKeyConfigMut(func(k *cmkapi.KeyConfiguration) {
				k.AdminGroupID = uuid.Nil
			}),
			expected: model.KeyConfiguration{},
			err:      errs.Wrapf(apierrors.ErrNameFieldMissingProperty, "adminGroupID"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			conf, err := keyconfiguration.FromAPI(tt.apiConf)
			if tt.err != nil {
				assert.EqualError(t, err, tt.err.Error())
				assert.Nil(t, conf)
			} else {
				assert.NotEmpty(t, conf.ID)
				assert.Equal(t, tt.expected.Name, conf.Name)
				assert.Equal(t, tt.expected.Description, conf.Description)
				assert.Equal(t, tt.expected.AdminGroupID, conf.AdminGroupID)
			}
		})
	}
}

func TestTransformKeyConfiguration_ToAPI(t *testing.T) {
	description := "Test key configuration"
	id := uuid.New()
	adminGroupID := uuid.New()
	creatorID := uuid.New()
	creatorName := "test-creator"

	primaryKeyID := uuid.New()

	modelKeyConfigMut := testutils.NewMutator(func() model.KeyConfiguration {
		return model.KeyConfiguration{
			ID:           id,
			Name:         "test-key-config",
			Description:  description,
			AdminGroupID: adminGroupID,
			CreatorID:    creatorID,
			CreatorName:  creatorName,
		}
	})

	apiKeyConfigMut := testutils.NewMutator(func() cmkapi.KeyConfiguration {
		connect := false

		return cmkapi.KeyConfiguration{
			Id:           &id,
			Name:         "test-key-config",
			Description:  &description,
			AdminGroupID: adminGroupID,
			Metadata: &cmkapi.KeyConfigurationMetadata{
				CreatedAt:    ptr.PointTo(time.Time{}.Format(time.RFC3339)),
				UpdatedAt:    ptr.PointTo(time.Time{}.Format(time.RFC3339)),
				CreatorID:    &creatorID,
				CreatorName:  &creatorName,
				TotalKeys:    ptr.PointTo(0),
				TotalSystems: ptr.PointTo(0),
			},
			CanConnectSystems: &connect,
		}
	})

	tests := []struct {
		name      string
		conf      model.KeyConfiguration
		expected  cmkapi.KeyConfiguration
		expectErr bool
		err       error
	}{
		{
			name:     "KeyConfigToAPI_Success",
			conf:     modelKeyConfigMut(),
			expected: apiKeyConfigMut(),
		},
		{
			name: "KeyConfigToAPI_WithPrimaryKey",
			conf: modelKeyConfigMut(func(k *model.KeyConfiguration) {
				k.PrimaryKeyID = &primaryKeyID
			}),
			expected: apiKeyConfigMut(func(k *cmkapi.KeyConfiguration) {
				con := true
				k.CanConnectSystems = &con
				k.PrimaryKeyID = &primaryKeyID
			}),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			apiConf, err := keyconfiguration.ToAPI(tt.conf)
			if tt.expectErr {
				assert.Error(t, err)
				assert.ErrorIs(t, err, tt.err)
				assert.Nil(t, apiConf)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expected, *apiConf)
			}
		})
	}
}
