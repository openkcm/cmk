package manager_test

import (
	"context"
	"crypto/rsa"
	"crypto/x509/pkix"
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"

	multitenancy "github.com/bartventer/gorm-multitenancy/v8"
	certificate_issuerv1 "github.com/openkcm/plugin-sdk/proto/plugin/certificate_issuer/v1"

	"github.com/openkcm/cmk/internal/config"
	"github.com/openkcm/cmk/internal/grpc/catalog"
	"github.com/openkcm/cmk/internal/manager"
	"github.com/openkcm/cmk/internal/model"
	"github.com/openkcm/cmk/internal/repo/sql"
	"github.com/openkcm/cmk/internal/testutils"
	cmkcontext "github.com/openkcm/cmk/utils/context"
	"github.com/openkcm/cmk/utils/crypto"
	"github.com/openkcm/cmk/utils/ptr"
)

var (
	ErrMockCertificationServiceResponse = errors.New("mock certification service response")
	ErrMockCertificateChain             = errors.New("mock certificate chain")
)

type CertificateIssuerMock struct {
	NewCertificateChain func() string
}

func (c CertificateIssuerMock) GetCertificate(
	_ context.Context,
	_ *certificate_issuerv1.GetCertificateRequest,
	_ ...grpc.CallOption,
) (*certificate_issuerv1.GetCertificateResponse, error) {
	return &certificate_issuerv1.GetCertificateResponse{
		CertificateChain: c.NewCertificateChain(),
	}, nil
}

func SetupCertificateManager(
	t *testing.T,
) (*manager.CertificateManager, *multitenancy.DB, string) {
	t.Helper()

	db, tenants, _ := testutils.NewTestDB(t, testutils.TestDBConfig{
		CreateDatabase: true,
	})

	dbRepository := sql.NewRepository(db)
	cfg := &config.Config{Plugins: testutils.SetupMockPlugins(testutils.CertIssuer)}

	catalog, err := catalog.New(t.Context(), cfg)
	assert.NoError(t, err)

	m := manager.NewCertificateManager(
		t.Context(),
		dbRepository,
		catalog,
		&cfg.Certificates,
	)

	return m, db, tenants[0]
}

func TestCertificate_UpdateCertificate(t *testing.T) {
	m, db, tenant := SetupCertificateManager(t)
	assert.NotNil(t, db)
	ctx := cmkcontext.CreateTenantContext(t.Context(), tenant)
	r := sql.NewRepository(db)

	now := time.Now()

	expected := []*model.Certificate{
		{
			ID:             uuid.New(),
			Fingerprint:    "Fingerprint1",
			CommonName:     "TestCommonName1",
			State:          model.CertificateStateActive,
			Purpose:        model.CertificatePurposeGeneric,
			CreationDate:   now,
			ExpirationDate: now.AddDate(0, 0, 3),
		},
		{
			ID:             uuid.New(),
			Fingerprint:    "Fingerprint2",
			CommonName:     "TestCommonName2",
			State:          model.CertificateStateActive,
			Purpose:        model.CertificatePurposeGeneric,
			CreationDate:   now,
			ExpirationDate: now.AddDate(0, 0, 3),
		},
	}

	testutils.CreateTestEntities(ctx, t, r, expected[0], expected[1])

	t.Run("Should update certificate", func(t *testing.T) {
		result, err := m.UpdateCertificate(testutils.CreateCtxWithTenant(tenant), &expected[0].ID, false)
		assert.NoError(t, err)
		assert.NotNil(t, result.AutoRotate)

		res, err := m.GetCertificate(testutils.CreateCtxWithTenant(tenant), &expected[0].ID)
		assert.NoError(t, err)
		assert.False(t, res.AutoRotate)

		res, err = m.GetCertificate(testutils.CreateCtxWithTenant(tenant), &expected[1].ID)
		assert.NoError(t, err)
		assert.True(t, res.AutoRotate)
	})
}

func TestCertificateManager_RequestNewCertificate(t *testing.T) {
	tests := []struct {
		name                string
		commonName          string
		validationDateUnit  certificate_issuerv1.ValidityType
		validationDateValue int
		purpose             model.CertificatePurpose
		request2time        bool
		statusCode          int
		expectedErr         bool
	}{
		{
			name:                "RequestNewCertificate Purpose Generic Success",
			validationDateUnit:  certificate_issuerv1.ValidityType_VALIDITY_TYPE_DAYS,
			validationDateValue: 6,
			purpose:             model.CertificatePurposeGeneric,
			request2time:        false,
			statusCode:          http.StatusOK,
			expectedErr:         false,
		},
		{
			name:                "RequestNewCertificate Purpose Tenant Default Success",
			validationDateUnit:  certificate_issuerv1.ValidityType_VALIDITY_TYPE_DAYS,
			validationDateValue: 6,
			purpose:             model.CertificatePurposeTenantDefault,
			request2time:        false,
			statusCode:          http.StatusOK,
			expectedErr:         false,
		},
		{
			name:                "RequestNewCertificate Purpose Tenant Default Error not available",
			validationDateUnit:  certificate_issuerv1.ValidityType_VALIDITY_TYPE_DAYS,
			validationDateValue: 6,
			purpose:             model.CertificatePurposeTenantDefault,
			request2time:        true,
			statusCode:          http.StatusOK,
			expectedErr:         true,
		},
	}

	privateKey, err := crypto.GeneratePrivateKey(manager.DefaultKeyBitSize)
	assert.NoError(t, err)
	m, _, tenant := SetupCertificateManager(t)

	m.SetClient(CertificateIssuerMock{NewCertificateChain: func() string {
		return testutils.CreateCertificateChain(t, pkix.Name{
			Country:            []string{"test"},
			Organization:       []string{"test"},
			OrganizationalUnit: []string{"test"},
			Locality:           []string{"test"},
			CommonName:         "test",
		}, privateKey)
	}})

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cert, privateKey, err := m.RequestNewCertificate(
				testutils.CreateCtxWithTenant(tenant),
				privateKey,
				model.RequestCertArgs{
					CertPurpose: tt.purpose,
					Supersedes:  nil,
					CommonName:  "MyCert",
					Locality:    []string{"locality"},
				})

			if tt.request2time {
				cert, privateKey, err = m.RequestNewCertificate(
					testutils.CreateCtxWithTenant(tenant),
					privateKey,
					model.RequestCertArgs{
						CertPurpose: tt.purpose,
						Supersedes:  nil,
						CommonName:  "MyCert",
						Locality:    []string{"locality"},
					})
			}

			if tt.expectedErr {
				assert.Error(t, err)
				assert.Nil(t, cert)
				assert.Nil(t, privateKey)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, cert)
				assert.NotNil(t, privateKey)
			}
		})
	}
}

func TestCertificateManager_RotateCertificate(t *testing.T) {
	privateKey, err := crypto.GeneratePrivateKey(manager.DefaultKeyBitSize)
	assert.NoError(t, err)
	m, _, tenant := SetupCertificateManager(t)

	m.SetPrivateKeyGenerator(func() (*rsa.PrivateKey, error) {
		return privateKey, nil
	})

	m.SetClient(CertificateIssuerMock{NewCertificateChain: func() string {
		return testutils.CreateCertificateChain(t, pkix.Name{
			Country:            []string{"test"},
			Organization:       []string{"test"},
			OrganizationalUnit: []string{"test"},
			Locality:           []string{"test"},
			CommonName:         "test",
		}, privateKey)
	}})

	ctx := testutils.CreateCtxWithTenant(tenant)

	origCert, _, err := m.RequestNewCertificate(ctx, privateKey,
		model.RequestCertArgs{
			CertPurpose: model.CertificatePurposeTenantDefault,
			Supersedes:  nil,
			CommonName:  "MyCert",
			Locality:    []string{"locality"},
		})
	// Sometimes this fails due to tenant not found, but I cant find why it happens
	// Change it to require so the test fails before panicing trying to access the pointer
	// so it is retried
	require.NoError(t, err)

	gotOrigCert, err := m.GetCertificate(ctx, ptr.PointTo(origCert.ID))
	assert.NoError(t, err)
	assert.True(t, gotOrigCert.AutoRotate)

	m.SetRotationThreshold(9999) // Want to catch all for testing auto rotate
	rotCerts, err := m.GetCertificatesForRotation(ctx)
	assert.NoError(t, err)
	assert.Equal(t, gotOrigCert.ID, rotCerts[0].ID)

	// Do first rotation
	rot1Cert, _, err := m.RotateCertificate(ctx,
		model.RequestCertArgs{
			CertPurpose: model.CertificatePurposeTenantDefault,
			Supersedes:  ptr.PointTo(origCert.ID),
			CommonName:  "MyCert",
			Locality:    []string{"locality"},
		})
	assert.NoError(t, err)

	gotOrigCert2, err := m.GetCertificate(ctx, ptr.PointTo(origCert.ID))
	assert.NoError(t, err)
	assert.False(t, gotOrigCert2.AutoRotate)

	gotRot1Cert, err := m.GetCertificate(ctx, ptr.PointTo(rot1Cert.ID))
	assert.NoError(t, err)
	assert.True(t, gotRot1Cert.AutoRotate)

	rotCerts, err = m.GetCertificatesForRotation(ctx)
	assert.NoError(t, err)
	assert.Equal(t, gotRot1Cert.ID, rotCerts[0].ID)

	// Do second rotation
	rot2Cert, _, err := m.RotateCertificate(ctx,
		model.RequestCertArgs{
			CertPurpose: model.CertificatePurposeTenantDefault,
			Supersedes:  ptr.PointTo(rot1Cert.ID),
			CommonName:  "MyCert",
			Locality:    []string{"locality"},
		})
	assert.NoError(t, err)

	gotOrigCert3, err := m.GetCertificate(ctx, ptr.PointTo(origCert.ID))
	assert.NoError(t, err)
	assert.False(t, gotOrigCert3.AutoRotate)

	gotRot1Cert2, err := m.GetCertificate(ctx, ptr.PointTo(rot1Cert.ID))
	assert.NoError(t, err)
	assert.False(t, gotRot1Cert2.AutoRotate)

	gotRot2Cert, err := m.GetCertificate(ctx, ptr.PointTo(rot2Cert.ID))
	assert.NoError(t, err)
	assert.True(t, gotRot2Cert.AutoRotate)

	rotCerts, err = m.GetCertificatesForRotation(ctx)
	assert.NoError(t, err)
	assert.Equal(t, gotRot2Cert.ID, rotCerts[0].ID)
}

func TestGetCertificateByPurpose(t *testing.T) {
	m, db, tenant := SetupCertificateManager(t)
	ctx := cmkcontext.CreateTenantContext(t.Context(), tenant)
	r := sql.NewRepository(db)

	t.Run("Should return false on cert not exist", func(t *testing.T) {
		_, exist, err := m.GetCertificateByPurpose(ctx, model.CertificatePurposeTenantDefault)
		assert.NoError(t, err)
		assert.False(t, exist)
	})

	t.Run("Should return true if cert exists", func(t *testing.T) {
		cert := testutils.NewCertificate(func(c *model.Certificate) {
			c.Purpose = model.CertificatePurposeTenantDefault
		})
		testutils.CreateTestEntities(ctx, t, r, cert)
		res, exist, err := m.GetCertificateByPurpose(ctx, model.CertificatePurposeTenantDefault)
		assert.NoError(t, err)
		assert.Equal(t, cert.ID, res.ID)
		assert.True(t, exist)
	})
}

func TestCertificateManager_GetDefaultClientCert(t *testing.T) {
	m, db, tenant := SetupCertificateManager(t)
	r := sql.NewRepository(db)

	privateKey, err := crypto.GeneratePrivateKey(manager.DefaultKeyBitSize)
	assert.NoError(t, err)

	m.SetPrivateKeyGenerator(func() (*rsa.PrivateKey, error) {
		return privateKey, nil
	})
	m.SetClient(CertificateIssuerMock{NewCertificateChain: func() string {
		return testutils.CreateCertificateChain(t, pkix.Name{
			Country:            []string{"test"},
			Organization:       []string{"test"},
			OrganizationalUnit: []string{"test"},
			Locality:           []string{"test"},
			CommonName:         "test",
		}, privateKey)
	}})

	ctx := testutils.CreateCtxWithTenant(tenant)

	t.Run("Should get default keystore certificate", func(t *testing.T) {
		// Act
		cert, err := m.GetDefaultKeystoreClientCert(ctx, "locality", "commonName")
		// Assert
		assert.NoError(t, err)
		assert.NotNil(t, cert)
	})

	t.Run("Failed to get default keystore certificate with out tenant ID", func(t *testing.T) {
		// Act
		cert, err := m.GetDefaultKeystoreClientCert(t.Context(), "locality", "commonName")
		// Assert
		assert.Error(t, err)
		assert.ErrorIs(t, err, manager.ErrGetDefaultKeystoreCertificate)
		assert.Nil(t, cert)
	})

	t.Run("Failed to get default keystore certificate with DB error", func(t *testing.T) {
		forced := testutils.NewDBErrorForced(db, ErrForced)

		forced.Register()
		defer forced.Unregister()
		// Act
		cert, err := m.GetDefaultKeystoreClientCert(ctx, "locality", "commonName")
		// Assert
		assert.Error(t, err)
		assert.Nil(t, cert)
	})

	t.Run("Should get default HYOK certificate", func(t *testing.T) {
		// Act
		cert, err := m.GetDefaultHYOKClientCert(ctx)
		// Assert
		assert.NoError(t, err)
		assert.NotNil(t, cert)
	})

	t.Run("Should rotate default HYOK certificate if invalid", func(t *testing.T) {
		certTime := time.Now().Add(-1 * time.Hour)
		oldCert := testutils.NewCertificate(func(c *model.Certificate) {
			c.Purpose = model.CertificatePurposeTenantDefault
			c.CreationDate = certTime
			c.ExpirationDate = certTime
		})
		testutils.CreateTestEntities(
			ctx,
			t,
			r,
			oldCert,
		)
		cert, err := m.GetDefaultHYOKClientCert(ctx)
		assert.NoError(t, err)
		assert.LessOrEqual(t, oldCert.ExpirationDate, cert.ExpirationDate)
	})

	t.Run("Failed to get default HYOK certificate with DB error", func(t *testing.T) {
		forced := testutils.NewDBErrorForced(db, ErrForced)

		forced.Register()
		defer forced.Unregister()
		// Act
		cert, err := m.GetDefaultHYOKClientCert(ctx)
		// Assert
		assert.Error(t, err)
		assert.Nil(t, cert)
	})
}
