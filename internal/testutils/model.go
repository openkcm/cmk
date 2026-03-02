package testutils

import (
	"database/sql"
	"encoding/json"
	"time"

	"github.com/google/uuid"

	multitenancy "github.com/bartventer/gorm-multitenancy/v8"

	"github.com/openkcm/cmk/internal/api/cmkapi"
	"github.com/openkcm/cmk/internal/config"
	"github.com/openkcm/cmk/internal/constants"
	"github.com/openkcm/cmk/internal/manager"
	"github.com/openkcm/cmk/internal/model"
	wfMechanism "github.com/openkcm/cmk/internal/workflow"
	"github.com/openkcm/cmk/utils/ptr"
)

const (
	DaysToExpiration              = 7
	TestLocalityID                = "12345678-90ab-cdef-1234-567890abcdef"
	TestDefaultKeystoreCommonName = "default.kms.cmk"
	TestRoleArn                   = "arn:aws:iam::123456789012:role/ExampleRole"
	TestTrustAnchorArn            = "arn:aws:rolesanywhere:eu-west-2:123456789012:trust-anchor/12345678-90ab-cdef-1234"
	TestProfileArn                = "arn:aws:rolesanywhere:eu-west-2:123456789012:profile/12345678-90ab-cdef-1234"
)

var SupportedRegions = []config.Region{
	{Name: "Region 1", TechnicalName: "region-1"},
	{Name: "Region 2", TechnicalName: "region-2"},
}

var SupportedRegionsMap = RegionsToMapSlice(SupportedRegions)

func RegionsToMapSlice(regions []config.Region) []map[string]string {
	result := make([]map[string]string, 0, len(regions))
	for _, region := range regions {
		result = append(result, map[string]string{
			"name":          region.Name,
			"technicalName": region.TechnicalName,
		})
	}

	return result
}

func NewSystem(m func(*model.System)) *model.System {
	mut := NewMutator(func() model.System {
		return model.System{
			ID:         uuid.New(),
			Identifier: uuid.NewString(),
			Region:     uuid.NewString(),
			Properties: make(map[string]string),
		}
	})

	return ptr.PointTo(mut(m))
}

type KeyConfigOpt func(*model.KeyConfiguration)

func NewKeyConfig(m func(*model.KeyConfiguration),
	opts ...KeyConfigOpt) *model.KeyConfiguration {
	keyConfig := model.KeyConfiguration{
		ID:         uuid.New(),
		Name:       uuid.NewString(),
		AdminGroup: *NewGroup(func(*model.Group) {}),
		CreatorID:  uuid.NewString(),
	}

	for _, o := range opts {
		o(&keyConfig)
	}

	mut := NewMutator(func() model.KeyConfiguration {
		return keyConfig
	})

	return ptr.PointTo(mut(m))
}

func NewTag(m func(*model.Tag)) *model.Tag {
	mut := NewMutator(func() model.Tag {
		return model.Tag{
			ID:     uuid.New(),
			Values: []byte(""),
		}
	})

	return ptr.PointTo(mut(m))
}

func NewKey(m func(*model.Key)) *model.Key {
	mut := NewMutator(func() model.Key {
		return model.Key{
			ID:      uuid.New(),
			KeyType: constants.KeyTypeSystemManaged,
			Name:    uuid.NewString(),
			State:   string(cmkapi.KeyStateENABLED),
		}
	})

	return ptr.PointTo(mut(m))
}

func NewKeyVersion(m func(*model.KeyVersion)) *model.KeyVersion {
	mut := NewMutator(func() model.KeyVersion {
		return model.KeyVersion{
			ExternalID: uuid.NewString(),
			Key:        *NewKey(func(_ *model.Key) {}),
			IsPrimary:  true,
			Version:    1,
		}
	})

	return ptr.PointTo(mut(m))
}

func NewGroup(m func(*model.Group)) *model.Group {
	mut := NewMutator(func() model.Group {
		return model.Group{
			ID:            uuid.New(),
			Name:          uuid.NewString(),
			IAMIdentifier: uuid.NewString(),
			Role:          constants.KeyAdminRole,
		}
	})

	return ptr.PointTo(mut(m))
}

func NewKeystoreConfig(m func(*model.KeystoreConfig)) *model.KeystoreConfig {
	mut := NewMutator(func() model.KeystoreConfig {
		return model.KeystoreConfig{
			LocalityID: TestLocalityID,
			CommonName: TestDefaultKeystoreCommonName,
			ManagementAccessData: map[string]any{
				"roleArn":        TestRoleArn,
				"trustAnchorArn": TestTrustAnchorArn,
				"profileArn":     TestProfileArn,
				"AccountID":      ValidKeystoreAccountInfo["AccountID"],
				"UserID":         ValidKeystoreAccountInfo["UserID"],
			},
			SupportedRegions: SupportedRegions,
		}
	})

	return ptr.PointTo(mut(m))
}

func NewKeystore(m func(*model.Keystore)) *model.Keystore {
	mut := NewMutator(func() model.Keystore {
		keystore := NewKeystoreConfig(func(_ *model.KeystoreConfig) {})
		ksBytes, _ := json.Marshal(keystore)

		return model.Keystore{
			ID:       uuid.New(),
			Provider: "AWS",
			Config:   ksBytes,
		}
	})

	return ptr.PointTo(mut(m))
}

func NewCertificate(m func(*model.Certificate)) *model.Certificate {
	now := time.Now()
	mut := NewMutator(func() model.Certificate {
		return model.Certificate{
			ID:             uuid.New(),
			Purpose:        model.CertificatePurposeTenantDefault,
			CommonName:     manager.DefaultHYOKCertCommonName,
			State:          model.CertificateStateActive,
			CreationDate:   now,
			ExpirationDate: now.AddDate(0, 0, DaysToExpiration),
			CertPEM:        "test-cert-pem-base64",
			PrivateKeyPEM:  "test-private-key-pem-base64",
		}
	})

	return ptr.PointTo(mut(m))
}

func NewImportParams(m func(*model.ImportParams)) *model.ImportParams {
	mut := NewMutator(func() model.ImportParams {
		return model.ImportParams{
			KeyID:              uuid.New(),
			WrappingAlg:        "CKM_RSA_AES_KEY_WRAP",
			HashFunction:       "SHA256",
			Expires:            ptr.PointTo(time.Now().Add(1 * time.Hour)),
			ProviderParameters: json.RawMessage{},
		}
	})

	return ptr.PointTo(mut(m))
}

func NewWorkflow(m func(*model.Workflow)) *model.Workflow {
	mut := NewMutator(func() model.Workflow {
		return model.Workflow{
			ID:           uuid.New(),
			State:        wfMechanism.StateInitial.String(),
			InitiatorID:  uuid.NewString(),
			ArtifactType: wfMechanism.ArtifactTypeKey.String(),
			ArtifactID:   uuid.New(),
			ActionType:   wfMechanism.ActionTypeDelete.String(),
			Approvers:    []model.WorkflowApprover{{UserID: uuid.NewString()}},
		}
	})

	return ptr.PointTo(mut(m))
}

func NewWorkflowApprover(m func(approver *model.WorkflowApprover)) *model.WorkflowApprover {
	mut := NewMutator(func() model.WorkflowApprover {
		return model.WorkflowApprover{
			WorkflowID: uuid.New(),
			UserID:     uuid.NewString(),
			UserName:   uuid.New().String(),
			Workflow:   model.Workflow{},
			Approved:   sql.NullBool{},
		}
	})

	return ptr.PointTo(mut(m))
}

func NewKeyLabel(m func(l *model.KeyLabel)) *model.KeyLabel {
	mut := NewMutator(func() model.KeyLabel {
		return model.KeyLabel{
			BaseLabel: model.BaseLabel{
				ID:    uuid.New(),
				Value: uuid.NewString(),
				Key:   uuid.NewString(),
			},
		}
	})

	return ptr.PointTo(mut(m))
}

func NewTenant(m func(t *model.Tenant)) *model.Tenant {
	tenantID := uuid.NewString()
	mut := NewMutator(func() model.Tenant {
		return model.Tenant{
			TenantModel: multitenancy.TenantModel{
				SchemaName: tenantID,
				DomainURL:  tenantID,
			},
			ID:        tenantID,
			Region:    "test-region",
			Status:    "STATUS_ACTIVE",
			Role:      "ROLE_LIVE",
			OwnerID:   tenantID + "-owner-id",
			OwnerType: "owner-type",
			Name:      tenantID,
		}
	})

	return ptr.PointTo(mut(m))
}

func NewWorkflowConfig(m func(m *model.TenantConfig)) *model.TenantConfig {
	retentionPeriodDays := 30
	wc := model.WorkflowConfig{
		Enabled:             true,
		MinimumApprovals:    1,
		RetentionPeriodDays: retentionPeriodDays,
	}
	//nolint:errchkjson
	configValue, _ := json.Marshal(wc)
	mut := NewMutator(func() model.TenantConfig {
		return model.TenantConfig{
			Key:   constants.WorkflowConfigKey,
			Value: configValue,
		}
	})

	return ptr.PointTo(mut(m))
}

// NewDefaultWorkflowConfig creates a default WorkflowConfig for testing
// with commonly used default values
func NewDefaultWorkflowConfig(enabled bool) *model.WorkflowConfig {
	return &model.WorkflowConfig{
		Enabled:                 enabled,
		MinimumApprovals:        constants.DefaultMinimumApprovalCount,
		RetentionPeriodDays:     constants.DefaultRetentionPeriodDays,
		DefaultExpiryPeriodDays: constants.DefaultExpiryPeriodDays,
		MaxExpiryPeriodDays:     constants.DefaultMaxExpiryPeriodDays,
	}
}
