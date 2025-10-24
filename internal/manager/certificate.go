package manager

import (
	"context"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/hex"
	"encoding/pem"
	"errors"
	"time"

	"github.com/fullsailor/pkcs7"
	"github.com/google/uuid"

	plugincatalog "github.com/openkcm/plugin-sdk/pkg/catalog"
	certissuerv1 "github.com/openkcm/plugin-sdk/proto/plugin/certificate_issuer/v1"

	"github.com/openkcm/cmk-core/internal/config"
	"github.com/openkcm/cmk-core/internal/errs"
	"github.com/openkcm/cmk-core/internal/log"
	"github.com/openkcm/cmk-core/internal/model"
	"github.com/openkcm/cmk-core/internal/repo"
	cmkcontext "github.com/openkcm/cmk-core/utils/context"
	"github.com/openkcm/cmk-core/utils/crypto"
)

var (
	ErrInvalidP7CertNoParse  = errors.New("returned invalid p7 cert: could not parse pkcs7")
	ErrInvalidCertEmptyChain = errors.New("empty certificate chain")

	ErrCertificateManager   = errors.New("certificate manager error")
	ErrCertificatePublicKey = errors.New("could not find a certificate with given public key")
	ErrCannotRotateOldCerts = errors.New("cannot rotate old tenant default certificates")

	ErrDefaultTenantCertificateAlreadyExists = errors.New(
		"default tenant certificate already exists; only one is allowed per tenant",
	)
	ErrDefaultTenantError = errors.New("default tenant cert error")
)

const (
	CertificateIssuerPluginName = "CERT_ISSUER"
	DefaultKeyBitSize           = 3076
)

type CertificateManager struct {
	repo                repo.Repo
	certIssuerClient    certissuerv1.CertificateIssuerServiceClient
	cfg                 *config.Certificates
	privateKeyGenerator func() (*rsa.PrivateKey, error)
}

func NewCertificateManager(
	ctx context.Context,
	repo repo.Repo,
	catalog *plugincatalog.Catalog,
	cfg *config.Certificates,
) *CertificateManager {
	client, err := createCertificateIssuerClient(catalog)
	if err != nil {
		log.Error(ctx, "failed creating certificate issuer client", err)
	}

	return &CertificateManager{
		repo:             repo,
		certIssuerClient: client,
		cfg:              cfg,
	}
}

func (m *CertificateManager) GetAllCertificates(
	ctx context.Context,
	certificateID *uuid.UUID,
) ([]*model.Certificate, int, error) {
	var certificates []*model.Certificate

	if certificateID != nil {
		cert := &model.Certificate{ID: *certificateID}

		_, err := m.repo.First(
			ctx,
			cert,
			*repo.NewQuery())
		if err != nil {
			return nil, 0, errs.Wrap(ErrCertificateManager, err)
		}

		return []*model.Certificate{cert}, 1, nil
	}

	count, err := m.repo.List(ctx, &model.Certificate{}, &certificates, *repo.NewQuery())
	if err != nil {
		return nil, 0, errs.Wrap(ErrCertificateManager, err)
	}

	return certificates, count, nil
}

func (m *CertificateManager) GetCertificate(
	ctx context.Context,
	certificateID *uuid.UUID,
) (*model.Certificate, error) {
	certificates, _, err := m.GetAllCertificates(ctx, certificateID)
	if err != nil {
		return nil, err
	}

	return certificates[0], nil
}

func (m *CertificateManager) GetCertificatesForRotation(ctx context.Context,
) ([]*model.Certificate, int, error) {
	certificates := []*model.Certificate{}
	rotateDate := time.Now().AddDate(0, 0, m.cfg.RotationThresholdDays)
	compositeKey := repo.NewCompositeKey().Where(
		repo.AutoRotateField, true).Where(repo.ExpirationDateField, rotateDate, repo.Lt)
	query := repo.NewQuery().Where(repo.NewCompositeKeyGroup(compositeKey))

	count, err := m.repo.List(
		ctx,
		model.Certificate{},
		&certificates,
		*query,
	)
	if err != nil {
		return nil, 0, errs.Wrap(ErrCertificateManager, err)
	}

	return certificates, count, nil
}

func (m *CertificateManager) UpdateCertificate(ctx context.Context, certificateID *uuid.UUID,
	autoRotate bool,
) (*model.Certificate, error) {
	cert, err := m.GetCertificate(ctx, certificateID)
	if err != nil {
		return nil, errs.Wrap(ErrCertificateManager, err)
	}

	// Get the latest Tenant/Keystore default cert
	// And prevent turning on auto rotate for expired certificates
	if autoRotate {
		var defaultCert *model.Certificate

		if cert.Purpose == model.CertificatePurposeTenantDefault {
			defaultCert, _, err = m.GetDefaultTenantCertificate(ctx)
		} else {
			defaultCert, _, err = m.GetDefaultKeystoreCertificate(ctx)
		}

		if err != nil {
			return nil, errs.Wrap(ErrCertificateManager, err)
		}

		// Check that we are only turning on autorotate for the latest default certificate
		if defaultCert != nil && defaultCert.ID != *certificateID {
			return nil, errs.Wrap(ErrCertificateManager, ErrCannotRotateOldCerts)
		}
	}

	cert.AutoRotate = autoRotate

	err = m.repo.Transaction(ctx, func(ctx context.Context, r repo.Repo) error {
		_, err := r.Patch(ctx, cert, *repo.NewQuery().UpdateAll(true))
		if err != nil {
			return errs.Wrap(ErrCertificateManager, err)
		}

		return nil
	})
	if err != nil {
		return nil, errs.Wrap(ErrCertificateManager, err)
	}

	return cert, nil
}

func (m *CertificateManager) GeneratePrivateKey() (*rsa.PrivateKey, error) {
	if m.privateKeyGenerator != nil {
		return m.privateKeyGenerator()
	}

	privateKey, err := crypto.GeneratePrivateKey(DefaultKeyBitSize)
	if err != nil {
		return nil, errs.Wrap(ErrCertificateManager, err)
	}

	return privateKey, nil
}

type RequestNewCertArgs struct {
	CertPurpose model.CertificatePurpose
	Supersedes  *uuid.UUID
	CommonName  string
	Locality    []string
}

func (m *CertificateManager) RequestNewCertificate(
	ctx context.Context,
	privateKey *rsa.PrivateKey,
	args RequestNewCertArgs,
) (*model.Certificate, *rsa.PrivateKey, error) {
	if args.CertPurpose == model.CertificatePurposeTenantDefault {
		exist, err := m.IsTenantDefaultCertExist(ctx)
		if err != nil {
			return nil, nil, errs.Wrap(ErrCertificateManager, err)
		}

		if exist {
			return nil, nil, ErrDefaultTenantCertificateAlreadyExists
		}
	}

	return m.getNewCertificate(ctx, privateKey, args)
}

func (m *CertificateManager) RotateCertificate(ctx context.Context,
	args RequestNewCertArgs,
) (*model.Certificate, *rsa.PrivateKey, error) {
	cert, pk, err := m.getNewCertificate(ctx, nil, args)
	if err != nil {
		return nil, nil, errs.Wrap(ErrCertificateManager, err)
	}

	_, err = m.UpdateCertificate(ctx, &cert.ID, false)
	if err != nil {
		return nil, nil, errs.Wrap(ErrCertificateManager, err)
	}

	return cert, pk, nil
}

func getFingerprint(cert *x509.Certificate) string {
	hash := sha256.Sum256(cert.Raw)
	return hex.EncodeToString(hash[:])
}

func buildCertificateModel(
	cert *x509.Certificate,
	pKeyPem []byte,
	purpose model.CertificatePurpose,
	clientCertChain []byte,
	supersedes *uuid.UUID,
) *model.Certificate {
	return &model.Certificate{
		ID:             uuid.New(),
		CommonName:     cert.Subject.CommonName,
		Fingerprint:    getFingerprint(cert),
		State:          model.CertificateStateActive,
		Purpose:        purpose,
		CreationDate:   cert.NotBefore,
		ExpirationDate: cert.NotAfter,
		PrivateKeyPEM:  string(pKeyPem),
		CertPEM:        string(clientCertChain),
		SupersedesID:   supersedes,
	}
}

func extractPublicKeyFromChain(certificateChainPEM []byte, privateKey *rsa.PrivateKey) (*x509.Certificate, error) {
	var certs []*x509.Certificate

	for {
		block, rest := pem.Decode(certificateChainPEM)
		if block == nil {
			break
		}

		if block.Type == "CERTIFICATE" {
			cert, err := x509.ParseCertificate(block.Bytes)
			if err != nil {
				return nil, errs.Wrap(ErrCertificateManager, err)
			}

			certs = append(certs, cert)
		}

		certificateChainPEM = rest
	}

	for _, cert := range certs {
		if verifyCertificateWithPrivateKey(cert, privateKey) {
			return cert, nil
		}
	}

	return nil, ErrCertificatePublicKey
}

func verifyCertificateWithPrivateKey(cert *x509.Certificate, privateKey *rsa.PrivateKey) bool {
	pubKey := cert.PublicKey
	switch pub := pubKey.(type) {
	case *rsa.PublicKey:
		return pub.N.Cmp(privateKey.N) == 0 && pub.E == privateKey.E
	default:
		return false
	}
}

func DecodeCertificateChain(certificationChain []byte) ([]*x509.Certificate, []byte, error) {
	// we expect 1 PEM block to be returned
	p7DER, _ := pem.Decode(certificationChain)
	if p7DER == nil || len(p7DER.Bytes) == 0 {
		return nil, nil, ErrInvalidP7CertNoParse
	}

	// convert pkcs7 to pem certs
	p7, parseErr := pkcs7.Parse(p7DER.Bytes)
	if parseErr != nil {
		return nil, nil, errs.Wrap(ErrCertificateManager, parseErr)
	}

	if len(p7.Certificates) == 0 {
		return nil, nil, ErrInvalidCertEmptyChain
	}

	var clientCertChain []byte

	for _, cert := range p7.Certificates {
		pemCert := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: cert.Raw})
		clientCertChain = append(clientCertChain, pemCert...)
	}

	return p7.Certificates, clientCertChain, nil
}

func (m *CertificateManager) IsTenantDefaultCertExist(ctx context.Context) (bool, error) {
	compositeKey := repo.NewCompositeKey().Where(repo.PurposeField,
		model.CertificatePurposeTenantDefault)

	var certs []model.Certificate

	count, err := m.repo.List(
		ctx,
		model.Certificate{}, &certs, *repo.NewQuery().Where(repo.NewCompositeKeyGroup(compositeKey)),
	)
	if err != nil {
		return false, errs.Wrap(ErrDefaultTenantError, err)
	}

	return count > 0, nil
}

//nolint:ireturn
func createCertificateIssuerClient(
	catalog *plugincatalog.Catalog,
) (certissuerv1.CertificateIssuerServiceClient, error) {
	certIssuer := catalog.LookupByTypeAndName(certissuerv1.Type, CertificateIssuerPluginName)
	if certIssuer == nil {
		return nil, ErrNoPluginInCatalog
	}

	return certissuerv1.NewCertificateIssuerServiceClient(certIssuer.ClientConnection()), nil
}

func (m *CertificateManager) GetDefaultTenantCertificate(ctx context.Context) (*model.Certificate, bool, error) {
	return m.getCertificateByPurpose(ctx, model.CertificatePurposeTenantDefault)
}

func (m *CertificateManager) GetDefaultKeystoreCertificate(ctx context.Context) (*model.Certificate, bool, error) {
	return m.getCertificateByPurpose(ctx, model.CertificatePurposeKeystoreDefault)
}

func (m *CertificateManager) getCertificateByPurpose(
	ctx context.Context,
	purpose model.CertificatePurpose,
) (*model.Certificate, bool, error) {
	compositeKey := repo.NewCompositeKey().Where(repo.PurposeField, purpose)

	var cert *model.Certificate

	count, err := m.repo.List(ctx, model.Certificate{}, &cert, *repo.NewQuery().Where(repo.NewCompositeKeyGroup(
		compositeKey)).Order(repo.OrderField{
		Field:     repo.CreationDateField,
		Direction: repo.Desc,
	}).SetLimit(1))
	if err != nil {
		return nil, false, errs.Wrap(ErrCertificateManager, err)
	}

	if count == 0 {
		return nil, false, nil
	}

	return cert, true, nil
}

func (m *CertificateManager) getNewCertificate(
	ctx context.Context,
	privateKey *rsa.PrivateKey,
	args RequestNewCertArgs,
) (*model.Certificate, *rsa.PrivateKey, error) {
	if privateKey == nil {
		var pkErr error

		privateKey, pkErr = m.GeneratePrivateKey()
		if pkErr != nil {
			return nil, nil, errs.Wrap(ErrCertificateManager, pkErr)
		}
	}

	pKeyPem := pem.EncodeToMemory(&pem.Block{
		Type:  crypto.PEMArmorPKCS1RSAPrivateKey,
		Bytes: x509.MarshalPKCS1PrivateKey(privateKey),
	})

	response, err := m.certIssuerClient.GetCertificate(ctx, &certissuerv1.GetCertificateRequest{
		CommonName: args.CommonName,
		Locality:   args.Locality,
		Validity: &certissuerv1.GetCertificateValidity{
			Value: int64(m.cfg.ValidityDays),
			Type:  certissuerv1.ValidityType_VALIDITY_TYPE_DAYS,
		},
		PrivateKey: &certissuerv1.PrivateKey{
			Data: pKeyPem,
		},
	})
	if err != nil {
		return nil, nil, errs.Wrap(ErrCertificateManager, err)
	}

	certificationChain := response.GetCertificateChain()

	_, clientCertChain, err := DecodeCertificateChain([]byte(certificationChain))
	if err != nil {
		return nil, nil, errs.Wrap(ErrCertificateManager, err)
	}

	cert, err := extractPublicKeyFromChain(clientCertChain, privateKey)
	if err != nil {
		return nil, nil, errs.Wrap(ErrCertificateManager, err)
	}

	certificate := buildCertificateModel(cert, pKeyPem, args.CertPurpose,
		clientCertChain, args.Supersedes)

	err = m.repo.Create(ctx, certificate)
	if err != nil {
		return nil, nil, errs.Wrap(ErrCertificateManager, err)
	}

	return certificate, privateKey, nil
}

func (m *CertificateManager) getDefaultHYOKClientCert(
	ctx context.Context,
) (*model.Certificate, error) {
	var (
		cert     *model.Certificate
		err      error
		tenantID string
		exists   bool
	)

	cert, exists, err = m.GetDefaultTenantCertificate(ctx)
	if err != nil {
		return nil, errs.Wrap(ErrGetDefaultTenantCertificate, err)
	}

	if !exists {
		tenantID, err = cmkcontext.ExtractTenantID(ctx)
		if err != nil {
			return nil, errs.Wrap(ErrGetTenantFromCtx, err)
		}

		cert, _, err = m.RequestNewCertificate(ctx, nil,
			RequestNewCertArgs{
				CertPurpose: model.CertificatePurposeTenantDefault,
				Supersedes:  nil,
				CommonName:  DefaultHYOKCertCommonName,
				Locality:    []string{tenantID},
			})
		if err != nil {
			return nil, errs.Wrap(ErrGetDefaultTenantCertificate, err)
		}
	}

	return cert, nil
}

func (m *CertificateManager) getDefaultKeystoreClientCert(
	ctx context.Context,
	localityID string,
	commonName string,
) (*model.Certificate, error) {
	var (
		cert   *model.Certificate
		err    error
		exists bool
	)

	cert, exists, err = m.GetDefaultKeystoreCertificate(ctx)
	if err != nil {
		return nil, errs.Wrap(ErrGetDefaultKeystoreCertificate, err)
	}

	if !exists {
		cert, _, err = m.RequestNewCertificate(ctx, nil,
			RequestNewCertArgs{
				CertPurpose: model.CertificatePurposeKeystoreDefault,
				Supersedes:  nil,
				CommonName:  commonName,
				Locality:    []string{localityID},
			})
		if err != nil {
			return nil, errs.Wrap(ErrGetDefaultKeystoreCertificate, err)
		}
	}

	return cert, nil
}
