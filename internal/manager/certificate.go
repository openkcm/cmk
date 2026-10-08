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

	"github.com/google/uuid"
	"github.com/openkcm/common-sdk/pkg/commoncfg"
	"gopkg.in/yaml.v3"

	"github.com/openkcm/cmk/internal/config"
	"github.com/openkcm/cmk/internal/errs"
	"github.com/openkcm/cmk/internal/log"
	"github.com/openkcm/cmk/internal/model"
	serviceapi "github.com/openkcm/cmk/internal/pluginregistry/service/api"
	"github.com/openkcm/cmk/internal/pluginregistry/service/api/certificateissuer"
	"github.com/openkcm/cmk/internal/repo"
	cmkcontext "github.com/openkcm/cmk/utils/context"
	"github.com/openkcm/cmk/utils/crypto"
)

var (
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
	DefaultKeyBitSize = 3072
)

type CertificateManager struct {
	repo                repo.Repo
	certIssuer          certificateissuer.CertificateIssuer
	cfg                 *config.Config
	privateKeyGenerator func() (*rsa.PrivateKey, error)
}

func NewCertificateManager(
	ctx context.Context,
	repo repo.Repo,
	svcRegistry serviceapi.Registry,
	cfg *config.Config,
) *CertificateManager {
	certIssuer, err := svcRegistry.CertificateIssuer()
	if err != nil {
		log.Error(ctx, "failed creating certificate issuer client", err)
	}

	return &CertificateManager{
		repo:       repo,
		certIssuer: certIssuer,
		cfg:        cfg,
	}
}

func (m *CertificateManager) GetCertificate(
	ctx context.Context,
	certificateID *uuid.UUID,
) (*model.Certificate, error) {
	cert := &model.Certificate{ID: *certificateID}

	_, err := m.repo.First(
		ctx,
		cert,
		*repo.NewQuery(),
	)
	if err != nil {
		return nil, errs.Wrap(ErrCertificateManager, err)
	}

	return cert, nil
}

func (m *CertificateManager) RotateExpiredCertificates(ctx context.Context) error {
	rotateDate := time.Now().AddDate(0, 0, m.cfg.Certificates.RotationThresholdDays)
	compositeKey := repo.NewCompositeKey().
		Where(repo.AutoRotateField, true).
		Where(repo.PurposeField, model.SingletonCertificatePurposes).
		Where(repo.ExpirationDateField, rotateDate, repo.Lt)
	query := repo.NewQuery().Where(repo.NewCompositeKeyGroup(compositeKey))

	return repo.ProcessInBatch(ctx, m.repo, query, repo.DefaultLimit, func(certs []*model.Certificate) error {
		for _, cert := range certs {
			_, _, err := m.RotateCertificate(ctx, model.RequestCertArgs{
				CertPurpose: cert.Purpose,
				Supersedes:  &cert.ID,
				CommonName:  cert.CommonName,
				Locality:    m.resolveRotationLocality(cert.CertPEM),
			})
			if err != nil {
				return err
			}
		}
		return nil
	})
}

func (m *CertificateManager) UpdateCertificate(
	ctx context.Context,
	certificateID *uuid.UUID,
	autoRotate bool,
) (*model.Certificate, error) {
	cert, err := m.GetCertificate(ctx, certificateID)
	if err != nil {
		return nil, errs.Wrap(ErrCertificateManager, err)
	}

	// Get the latest default certificate of the same purpose,
	// and prevent turning on auto rotate for expired certificates
	if autoRotate {
		var defaultCert *model.Certificate

		defaultCert, _, err = m.getCertificateByPurpose(ctx, cert.Purpose)
		if err != nil {
			return nil, errs.Wrap(ErrCertificateManager, err)
		}

		// Check that we are only turning on autorotate for the latest default certificate
		if defaultCert != nil && defaultCert.ID != *certificateID {
			return nil, errs.Wrap(ErrCertificateManager, ErrCannotRotateOldCerts)
		}
	}

	cert.AutoRotate = autoRotate

	_, err = m.repo.Patch(ctx, cert, *repo.NewQuery().UpdateAll(true))
	if err != nil {
		return nil, errs.Wrap(ErrCertificateManager, err)
	}

	return cert, nil
}

func (m *CertificateManager) RequestNewCertificate(
	ctx context.Context,
	privateKey *rsa.PrivateKey,
	args model.RequestCertArgs,
) (*model.Certificate, *rsa.PrivateKey, error) {
	exist, err := m.isCertWithPurposeExist(ctx, args.CertPurpose)
	if err != nil {
		return nil, nil, errs.Wrap(ErrCertificateManager, err)
	}

	if exist {
		return nil, nil, ErrDefaultTenantCertificateAlreadyExists
	}

	return m.getNewCertificate(ctx, privateKey, args)
}

func (m *CertificateManager) RotateCertificate(ctx context.Context,
	args model.RequestCertArgs,
) (*model.Certificate, *rsa.PrivateKey, error) {
	cert, pk, err := m.getNewCertificate(ctx, nil, args)
	if err != nil {
		return nil, nil, errs.Wrap(ErrCertificateManager, err)
	}

	_, err = m.UpdateCertificate(ctx, args.Supersedes, false)
	if err != nil {
		return nil, nil, errs.Wrap(ErrCertificateManager, err)
	}

	return cert, pk, nil
}

func (m *CertificateManager) generatePrivateKey() (*rsa.PrivateKey, error) {
	if m.privateKeyGenerator != nil {
		return m.privateKeyGenerator()
	}

	privateKey, err := crypto.GeneratePrivateKey(DefaultKeyBitSize)
	if err != nil {
		return nil, errs.Wrap(ErrCertificateManager, err)
	}

	return privateKey, nil
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

func extractPublicKeyFromChain(
	certificateChainPEM []byte,
	privateKey *rsa.PrivateKey,
) (*x509.Certificate, error) {
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

func decodeCertificateChain(chainPEM []byte) ([]*x509.Certificate, []byte, error) {
	var certs []*x509.Certificate

	rest := chainPEM

	for {
		var block *pem.Block

		block, rest = pem.Decode(rest)
		if block == nil {
			break
		}

		if block.Type != "CERTIFICATE" {
			continue
		}

		cert, err := x509.ParseCertificate(block.Bytes)
		if err != nil {
			return nil, nil, errs.Wrap(ErrCertificateManager, err)
		}

		certs = append(certs, cert)
	}

	if len(certs) == 0 {
		return nil, nil, ErrInvalidCertEmptyChain
	}

	return certs, chainPEM, nil
}

func (m *CertificateManager) isCertWithPurposeExist(
	ctx context.Context,
	purpose model.CertificatePurpose,
) (bool, error) {
	compositeKey := repo.NewCompositeKey().Where(repo.PurposeField, purpose)

	count, err := m.repo.Count(
		ctx,
		&model.Certificate{}, *repo.NewQuery().Where(repo.NewCompositeKeyGroup(compositeKey)),
	)
	if err != nil {
		return false, errs.Wrap(ErrDefaultTenantError, err)
	}

	return count > 0, nil
}

func (m *CertificateManager) getCertificateByPurpose(
	ctx context.Context,
	purpose model.CertificatePurpose,
) (*model.Certificate, bool, error) {
	compositeKey := repo.NewCompositeKey().Where(repo.PurposeField, purpose)

	cert := &model.Certificate{}

	found, err := m.repo.First(ctx, cert, *repo.NewQuery().Where(repo.NewCompositeKeyGroup(
		compositeKey,
	)).Order(repo.OrderField{
		Field:     repo.CreationDateField,
		Direction: repo.Desc,
	}))
	if err != nil && !errors.Is(err, repo.ErrNotFound) {
		return nil, found, errs.Wrap(ErrCertificateManager, err)
	}

	return cert, found, nil
}

func (m *CertificateManager) getNewCertificate(
	ctx context.Context,
	privateKey *rsa.PrivateKey,
	args model.RequestCertArgs,
) (*model.Certificate, *rsa.PrivateKey, error) {
	if privateKey == nil {
		var pkErr error

		privateKey, pkErr = m.generatePrivateKey()
		if pkErr != nil {
			return nil, nil, errs.Wrap(ErrCertificateManager, pkErr)
		}
	}

	pKeyPem := pem.EncodeToMemory(&pem.Block{
		Type:  crypto.PEMArmorPKCS1RSAPrivateKey,
		Bytes: x509.MarshalPKCS1PrivateKey(privateKey),
	})

	response, err := m.certIssuer.IssueCertificate(ctx, &certificateissuer.IssueCertificateRequest{
		CommonName: args.CommonName,
		Localities: args.Locality,
		Validity: &certificateissuer.CertificateValidity{
			Value: int64(m.cfg.Certificates.ValidityDays),
			Type:  certificateissuer.Days,
		},
		PrivateKey: &certificateissuer.CertificatePrivateKey{
			Data: pKeyPem,
		},
	})
	if err != nil {
		return nil, nil, errs.Wrap(ErrCertificateManager, err)
	}

	certificationChain := response.ChainPem

	_, clientCertChain, err := decodeCertificateChain([]byte(certificationChain))
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
	cert, exists, err := m.getCertificateByPurpose(ctx, model.CertificatePurposeHYOKManagement)
	if err != nil {
		return nil, errs.Wrap(ErrGetDefaultTenantCertificate, err)
	}

	tenantID, err := cmkcontext.ExtractTenantID(ctx)
	if err != nil {
		return nil, errs.Wrap(ErrGetTenantFromCtx, err)
	}

	if !exists {
		commonName := m.cfg.Certificates.DefaultTenantCertPrefix + tenantID
		cert, _, err = m.RequestNewCertificate(ctx, nil,
			model.RequestCertArgs{
				CertPurpose: model.CertificatePurposeHYOKManagement,
				Supersedes:  nil,
				CommonName:  commonName,
				Locality:    []string{m.cfg.Landscape.Region},
			})
		if err != nil {
			return nil, errs.Wrap(ErrGetDefaultTenantCertificate, err)
		}
	} else if cert.ExpirationDate.Before(time.Now()) {
		// Determine locality for rotation: preserve backward compatibility.
		// Old certs were issued with tenantID as locality; new certs use region.
		// Parse the existing cert's PEM to detect which locality was used.
		locality := m.resolveRotationLocality(cert.CertPEM)
		cert, _, err = m.RotateCertificate(ctx, model.RequestCertArgs{
			CertPurpose: cert.Purpose,
			Supersedes:  &cert.ID,
			CommonName:  cert.CommonName,
			Locality:    locality,
		})
		if err != nil {
			return nil, err
		}
	}

	return cert, nil
}

// getDefaultKeystoreClientCert returns role management or key management certificate depending on the provided purpose
func (m *CertificateManager) getDefaultKeystoreClientCert(
	ctx context.Context,
	localityID string,
	commonName string,
	purpose model.CertificatePurpose,
) (*model.Certificate, error) {
	var (
		cert   *model.Certificate
		err    error
		exists bool
	)

	if purpose != model.CertificatePurposeRoleManagement && purpose != model.CertificatePurposeKeyManagement {
		return nil, errs.Wrapf(ErrGetDefaultKeystoreCertificate, "unsupported certificate purpose")
	}

	cert, exists, err = m.getCertificateByPurpose(ctx, purpose)
	if err != nil {
		return nil, errs.Wrap(ErrGetDefaultKeystoreCertificate, err)
	}

	if !exists {
		cert, _, err = m.RequestNewCertificate(ctx, nil,
			model.RequestCertArgs{
				CertPurpose: purpose,
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

// resolveRotationLocality returns the locality that should be used when rotating a certificate.
// It reads the locality directly from the existing cert's PEM so the rotated cert is always
// issued with the same locality as the one it supersedes — preserving backward compatibility
// for old certs that were issued with tenantID as locality, while new certs continue to use region.
// Falls back to the configured region if the PEM cannot be parsed or carries no locality.
func (m *CertificateManager) resolveRotationLocality(certPEM string) []string {
	block, _ := pem.Decode([]byte(certPEM))
	if block == nil {
		return []string{m.cfg.Landscape.Region}
	}

	parsed, err := x509.ParseCertificate(block.Bytes)
	if err != nil || len(parsed.Subject.Locality) == 0 {
		return []string{m.cfg.Landscape.Region}
	}

	return parsed.Subject.Locality
}

// getCryptoCertificates retrieves crypto certificates from config
func (m *CertificateManager) getCryptoCertificates(ctx context.Context) ([]*model.ClientCertificate, error) {
	bytes, err := commoncfg.LoadValueFromSourceRef(m.cfg.CryptoLayer.CertX509Trusts)
	if err != nil {
		return nil, errs.Wrap(ErrLoadCryptoCerts, err)
	}

	var (
		certConfigurations []*config.CryptoCert
		certs              []*model.ClientCertificate
	)

	err = yaml.Unmarshal(bytes, &certConfigurations)
	if err != nil {
		return nil, errs.Wrap(ErrUnmarshalCryptoCerts, err)
	}

	tenantID, err := cmkcontext.ExtractTenantID(ctx)
	if err != nil {
		return nil, err
	}

	for _, certCfg := range certConfigurations {
		cert := model.NewClientCertificate(*certCfg, tenantID)
		certs = append(certs, &cert)
	}

	return certs, nil
}
