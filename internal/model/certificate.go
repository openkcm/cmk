package model

import (
	"context"
	"crypto/x509/pkix"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/openkcm/cmk/internal/authz"
	"github.com/openkcm/cmk/internal/config"
)

type CertificateState string

const (
	CertificateStateActive  CertificateState = "ACTIVE"
	CertificateStateExpired CertificateState = "EXPIRED"

	CertificateSubjectKey string = "certificateSubject"
)

type CertificatePurpose string

// - Generic purpose is used as a fallback default purpose when no specific purpose is provided.
// - HYOKManagement purpose is used for managing tenant default HYOK certificates.
// The name is kept for backward compatibility.
// - RoleManagement purpose is used for managing other keystore roles.
// - KeyManagement purpose is used for managing BYOK/Managed key lifecycle.
// - Crypto purpose is used only for displaying purposes, not for creation.
const (
	CertificatePurposeGeneric        CertificatePurpose = "GENERIC"
	CertificatePurposeHYOKManagement CertificatePurpose = "TENANT_DEFAULT"
	CertificatePurposeRoleManagement CertificatePurpose = "ROLE_MANAGEMENT"
	CertificatePurposeKeyManagement  CertificatePurpose = "KEY_MANAGEMENT"
	CertificatePurposeCrypto         CertificatePurpose = "CRYPTO"
)

// SingletonCertificatePurposes defines the certificate purposes
// for which only one active certificate can exist at a time.
var SingletonCertificatePurposes = []CertificatePurpose{
	CertificatePurposeHYOKManagement,
	CertificatePurposeRoleManagement,
	CertificatePurposeKeyManagement,
}

type Certificate struct {
	ID             uuid.UUID          `gorm:"type:uuid;primaryKey"`
	Fingerprint    string             `gorm:"type:text;not null"`
	CommonName     string             `gorm:"type:varchar(64);not null"`
	State          CertificateState   `gorm:"type:varchar(255)"`
	Purpose        CertificatePurpose `gorm:"type:varchar(255)"`
	CreationDate   time.Time          `gorm:"not null"`
	ExpirationDate time.Time          `gorm:"not null"`
	CertPEM        string             `gorm:"type:text"` // Base64 encoded PEM certificate
	PrivateKeyPEM  string             `gorm:"type:text"` // Base64 encoded PEM private key
	AutoRotate     bool               `gorm:"not null;default:true"`
	SupersedesID   *uuid.UUID         `gorm:"foreignKey:CertificateID"`
}

// TableResourceType return the authz resource type
func (Certificate) TableResourceType() authz.RepoResourceTypeName {
	return authz.RepoResourceTypeCertificate
}

// TableName returns the table name for Certificate
func (m Certificate) TableName() string {
	return string(m.TableResourceType())
}

func (Certificate) IsSharedModel() bool {
	return false
}

func (m Certificate) CheckAuthz(ctx context.Context,
	authzHandler *authz.Handler[authz.RepoResourceTypeName, authz.RepoAction],
	action authz.RepoAction,
) (bool, error) {
	return authz.CheckAuthz(ctx, authzHandler, m.TableResourceType(), action)
}

type RequestCertArgs struct {
	CertPurpose CertificatePurpose
	Supersedes  *uuid.UUID
	CommonName  string
	Locality    []string
}

type ClientCertificates map[CertificatePurpose][]*ClientCertificate

// ClientCertificate represents a client certificate used for HYOK key management.
type ClientCertificate struct {
	Name    string             `yaml:"name"`
	RootCA  string             `yaml:"rootCA"` //nolint:tagliatelle
	Subject CertificateSubject `yaml:"subject"`
}

// CertificateSubject holds the subject fields of a client certificate.
type CertificateSubject struct {
	Locality           []string `yaml:"locality"`
	OrganizationalUnit []string `yaml:"organizationUnit"` //nolint:tagliatelle
	Organization       []string `yaml:"organization"`
	Country            []string `yaml:"country"`
	CommonName         string
}

func NewClientCertificate(value config.CryptoCert, tenant string) ClientCertificate {
	return ClientCertificate{
		Name:   value.Name,
		RootCA: value.RootCA,
		Subject: CertificateSubject{
			CommonName:         value.Subject.CommonNamePrefix + tenant,
			Country:            value.Subject.Country,
			Organization:       value.Subject.Organization,
			OrganizationalUnit: value.Subject.OrganizationalUnit,
			Locality:           value.Subject.Locality,
		},
	}
}

func ToCertificateSubjectFromPKIX(subject pkix.Name) CertificateSubject {
	return CertificateSubject{
		Locality:           subject.Locality,
		OrganizationalUnit: subject.OrganizationalUnit,
		Organization:       subject.Organization,
		Country:            subject.Country,
		CommonName:         subject.CommonName,
	}
}

func (subject CertificateSubject) String() string {
	s := pkix.Name{
		Locality:           subject.Locality,
		Country:            subject.Country,
		Organization:       subject.Organization,
		OrganizationalUnit: subject.OrganizationalUnit,
		CommonName:         subject.CommonName,
	}
	if len(s.OrganizationalUnit) <= 1 {
		return s.String()
	}

	standardSubject := s.String()
	combinedOU := "OU=" + strings.Join(s.OrganizationalUnit, "/")
	ouPattern := `OU=[^,+]+((\+OU=[^,+]+)+)`
	re := regexp.MustCompile(ouPattern)

	return re.ReplaceAllString(standardSubject, combinedOU)
}
