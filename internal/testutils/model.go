package testutils

import (
	"database/sql"
	"encoding/json"
	"time"

	"github.com/google/uuid"

	multitenancy "github.com/bartventer/gorm-multitenancy/v8"

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

func NewKeyConfig(m func(*model.KeyConfiguration)) *model.KeyConfiguration {
	mut := NewMutator(func() model.KeyConfiguration {
		return model.KeyConfiguration{
			ID:         uuid.New(),
			Name:       uuid.NewString(),
			AdminGroup: model.Group{ID: uuid.New(), Name: uuid.NewString(), IAMIdentifier: uuid.NewString()},
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

func NewKeystoreConfig(m func(*model.KeystoreConfiguration)) *model.KeystoreConfiguration {
	mut := NewMutator(func() model.KeystoreConfiguration {
		ksConfigValue := map[string]any{
			"localityId": TestLocalityID,
			"commonName": TestDefaultKeystoreCommonName,
			"managementAccessData": map[string]string{
				"roleArn":        TestRoleArn,
				"trustAnchorArn": TestTrustAnchorArn,
				"profileArn":     TestProfileArn,
				"AccountID":      ValidKeystoreAccountInfo["AccountID"],
				"UserID":         ValidKeystoreAccountInfo["UserID"],
			},
			"supportedRegions": SupportedRegionsMap,
		}

		valueBytes, _ := json.Marshal(ksConfigValue)

		return model.KeystoreConfiguration{
			ID:       uuid.New(),
			Provider: "AWS",
			Value:    valueBytes,
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
			InitiatorID:  uuid.New(),
			ArtifactType: wfMechanism.ArtifactTypeKey.String(),
			ArtifactID:   uuid.New(),
			ActionType:   wfMechanism.ActionTypeDelete.String(),
			Approvers:    []model.WorkflowApprover{},
		}
	})

	return ptr.PointTo(mut(m))
}

func NewWorkflowApprover(m func(approver *model.WorkflowApprover)) *model.WorkflowApprover {
	mut := NewMutator(func() model.WorkflowApprover {
		return model.WorkflowApprover{
			WorkflowID: uuid.New(),
			UserID:     uuid.New(),
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
			ID:     tenantID,
			Region: "test-region",
		}
	})

	return ptr.PointTo(mut(m))
}
