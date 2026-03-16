package model

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/openkcm/cmk/internal/authz"
)

type CertificateState string

const (
	CertificateStateActive  CertificateState = "ACTIVE"
	CertificateStateExpired CertificateState = "EXPIRED"
)

type CertificatePurpose string

const (
	CertificatePurposeGeneric         CertificatePurpose = "GENERIC"
	CertificatePurposeTenantDefault   CertificatePurpose = "TENANT_DEFAULT"
	CertificatePurposeKeystoreDefault CertificatePurpose = "KEYSTORE_DEFAULT"
	CertificatePurposeCrypto          CertificatePurpose = "CRYPTO"
)

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
	action authz.RepoAction) (bool, error) {
	return authz.CheckAuthz(ctx, authzHandler, m.TableResourceType(), action)
}

type RequestCertArgs struct {
	CertPurpose CertificatePurpose
	Supersedes  *uuid.UUID
	CommonName  string
	Locality    []string
}
