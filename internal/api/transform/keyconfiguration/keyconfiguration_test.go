package keyconfiguration_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/openkcm/common-sdk/pkg/auth"
	"github.com/stretchr/testify/assert"

	"github.com/openkcm/cmk/internal/api/cmkapi"
	"github.com/openkcm/cmk/internal/api/transform/keyconfiguration"
	"github.com/openkcm/cmk/internal/apierrors"
	"github.com/openkcm/cmk/internal/errs"
	"github.com/openkcm/cmk/internal/manager"
	"github.com/openkcm/cmk/internal/model"
	"github.com/openkcm/cmk/internal/pluginregistry/service/api/identitymanagement"
	"github.com/openkcm/cmk/internal/testutils"
	"github.com/openkcm/cmk/internal/testutils/testplugins"
	cmkcontext "github.com/openkcm/cmk/utils/context"
)

type mockKeyConfigManager struct{}

var _ manager.KeyConfigurationAPI = (*mockKeyConfigManager)(nil)

func (m *mockKeyConfigManager) GetKeyConfigurations(_ context.Context, _ manager.KeyConfigFilter) ([]*model.KeyConfiguration, int, error) {
	return nil, 0, nil
}

func (m *mockKeyConfigManager) PostKeyConfigurations(_ context.Context, _ *model.KeyConfiguration) (*model.KeyConfiguration, error) {
	return &model.KeyConfiguration{}, nil
}

func (m *mockKeyConfigManager) DeleteKeyConfigurationByID(_ context.Context, _ uuid.UUID) error {
	return nil
}

func (m *mockKeyConfigManager) GetKeyConfigurationByID(_ context.Context, _ uuid.UUID) (*model.KeyConfiguration, error) {
	return &model.KeyConfiguration{}, nil
}

func (m *mockKeyConfigManager) UpdateKeyConfigurationByID(_ context.Context, _ uuid.UUID, _ cmkapi.KeyConfigurationPatch) (*model.KeyConfiguration, error) {
	return &model.KeyConfiguration{}, nil
}

func (m *mockKeyConfigManager) GetClientCertificates(_ context.Context) (model.ClientCertificates, error) {
	return model.ClientCertificates{}, nil
}

func (m *mockKeyConfigManager) CanConnectSystems(_ context.Context, keyConfig *model.KeyConfiguration) (bool, error) {
	return keyConfig.PrimaryKeyID != nil, nil
}

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
	creatorID := uuid.New().String()
	creatorName := uuid.NewString() + "@example.com"

	idm := testplugins.NewTestIdentityManagement()
	idm.PutUser(identitymanagement.User{ID: creatorID, Name: creatorName})

	primaryKeyID := uuid.New()

	modelKeyConfigMut := testutils.NewMutator(func() model.KeyConfiguration {
		return model.KeyConfiguration{
			ID:           id,
			Name:         "test-key-config",
			Description:  description,
			AdminGroupID: adminGroupID,
			CreatorID:    creatorID,
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
				CreatedAt:    new(time.Time{}),
				UpdatedAt:    new(time.Time{}),
				CreatorID:    &creatorID,
				CreatorName:  &creatorName,
				TotalKeys:    new(0),
				TotalSystems: new(0),
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
			ctx := cmkcontext.InjectBusinessUserData(t.Context(), &auth.ClientData{Identifier: "User-ID"}, nil)
			apiConf, err := keyconfiguration.ToAPI(ctx, &tt.conf, &mockKeyConfigManager{}, idm)
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

	t.Run("Should have nil creator id and name on invalid creator id", func(t *testing.T) {
		apiConf, err := keyconfiguration.ToAPI(t.Context(), testutils.NewKeyConfig(func(kc *model.KeyConfiguration) {
			kc.CreatorID = uuid.Nil.String()
		}), &mockKeyConfigManager{}, nil)
		assert.NoError(t, err)
		assert.Nil(t, apiConf.Metadata.CreatorID)
		assert.Nil(t, apiConf.Metadata.CreatorName)
	})
}
