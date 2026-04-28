package testplugins

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/openkcm/plugin-sdk/api"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/types/known/structpb"

	"github.com/openkcm/cmk/internal/errs"
	"github.com/openkcm/cmk/internal/pluginregistry/service/api/keymanagement"
	servicewrapper "github.com/openkcm/cmk/internal/pluginregistry/service/wrapper"
)

var (
	EnabledKeyStatus         = "ENABLED"
	DisabledKeyStatus        = "DISABLED"
	PendingImportKeyStatus   = "PENDING_IMPORT"
	PendingDeletionKeyStatus = "PENDING_DELETION"

	ErrKeyIDIsNil          = errors.New("keyId is nil")
	ErrTransformAccessData = errors.New("failed to transform access data")
)

const importParamsValidityHours = 24

type KeyRecord struct {
	KeyID        string `gorm:"primaryKey;column:key_id"`
	Status       string
	VersionID    string
	RotationTime string // RFC3339 format
}

var InitialKeys = map[string]KeyRecord{
	"mock-key/11111111": {Status: EnabledKeyStatus},
	"mock-key/22222222": {Status: EnabledKeyStatus},
	"mock-key/33333333": {Status: EnabledKeyStatus},
}

type TestKeyManagement struct {
	KeyStore  map[string]*KeyRecord
	IsHYOK    bool
	IsDefault bool
}

var _ keymanagement.KeyManagement = (*TestKeyManagement)(nil)

func NewTestKeyManagement(isHYOK, isDefault bool) *TestKeyManagement {
	km := &TestKeyManagement{
		KeyStore:  make(map[string]*KeyRecord),
		IsHYOK:    isHYOK,
		IsDefault: isDefault,
	}
	for keyID, record := range InitialKeys {
		km.HandleKeyRecord(keyID, record.Status)
	}
	return km
}

func (s *TestKeyManagement) ServiceInfo() api.Info {
	var tags []string
	if s.IsHYOK {
		tags = append(tags, "hyok")
	}
	if s.IsDefault {
		tags = append(tags, "default_keystore")
	}

	return testInfo{
		configuredType: servicewrapper.KeyManagementType,
		configuredTags: tags,
	}
}

func (s *TestKeyManagement) HandleKeyRecord(keyID, status string) {
	record, exists := s.KeyStore[keyID]
	if !exists {
		record = &KeyRecord{KeyID: keyID, Status: status}
		s.KeyStore[keyID] = record
	}
	record.Status = status
}

// SetKeyVersionInfo sets version and rotation metadata for a key, mirroring the
// KeystoreOperator helper used in tests.
func (s *TestKeyManagement) SetKeyVersionInfo(keyID, versionID, rotationTime string) {
	record, exists := s.KeyStore[keyID]
	if !exists {
		record = &KeyRecord{KeyID: keyID, Status: EnabledKeyStatus}
		s.KeyStore[keyID] = record
	}
	record.VersionID = versionID
	record.RotationTime = rotationTime
}

func (s *TestKeyManagement) GetKey(
	_ context.Context,
	req *keymanagement.GetKeyRequest,
) (*keymanagement.GetKeyResponse, error) {
	cfg := req.Parameters.Config.Values
	if cfg["authType"] == "AUTH_TYPE_CERTIFICATE" &&
		(cfg["AccountID"] != ValidKeystoreAccountInfo["AccountID"] ||
			cfg["UserID"] != ValidKeystoreAccountInfo["UserID"]) {
		return nil, keymanagement.ErrProviderAuthenticationFailed
	}

	record, exists := s.KeyStore[req.Parameters.KeyID]
	if !exists {
		return nil, keymanagement.ErrHYOKKeyNotFound
	}

	resp := &keymanagement.GetKeyResponse{
		KeyAlgorithm: keymanagement.AES256,
		Status:       record.Status,
	}

	if record.VersionID != "" {
		resp.LatestKeyVersionId = record.VersionID
	}

	if record.RotationTime != "" {
		t, err := time.Parse(time.RFC3339Nano, record.RotationTime)
		if err != nil {
			t, err = time.Parse(time.RFC3339, record.RotationTime)
		}
		if err == nil {
			resp.RotationTime = &t
		}
	}

	return resp, nil
}

func (s *TestKeyManagement) CreateKey(
	_ context.Context,
	req *keymanagement.CreateKeyRequest,
) (*keymanagement.CreateKeyResponse, error) {
	st := EnabledKeyStatus
	if req.KeyType == keymanagement.BYOK {
		st = PendingImportKeyStatus
	}

	keyID := "mock-key/" + uuid.NewString()
	s.HandleKeyRecord(keyID, st)

	return &keymanagement.CreateKeyResponse{
		KeyID:  keyID,
		Status: st,
	}, nil
}

func (s *TestKeyManagement) DeleteKey(
	_ context.Context,
	req *keymanagement.DeleteKeyRequest,
) (*keymanagement.DeleteKeyResponse, error) {
	if req != nil && req.Parameters.KeyID != "" {
		s.HandleKeyRecord(req.Parameters.KeyID, PendingDeletionKeyStatus)
	}
	return &keymanagement.DeleteKeyResponse{}, nil
}

func (s *TestKeyManagement) EnableKey(
	_ context.Context,
	req *keymanagement.EnableKeyRequest,
) (*keymanagement.EnableKeyResponse, error) {
	if req.Parameters.KeyID == "" {
		return nil, ErrKeyIDIsNil
	}
	s.HandleKeyRecord(req.Parameters.KeyID, EnabledKeyStatus)
	return &keymanagement.EnableKeyResponse{}, nil
}

func (s *TestKeyManagement) DisableKey(
	_ context.Context,
	req *keymanagement.DisableKeyRequest,
) (*keymanagement.DisableKeyResponse, error) {
	if req.Parameters.KeyID == "" {
		return nil, ErrKeyIDIsNil
	}
	s.HandleKeyRecord(req.Parameters.KeyID, DisabledKeyStatus)
	return &keymanagement.DisableKeyResponse{}, nil
}

func (s *TestKeyManagement) GetImportParameters(
	_ context.Context,
	req *keymanagement.GetImportParametersRequest,
) (*keymanagement.GetImportParametersResponse, error) {
	validTime := time.Now().Add(importParamsValidityHours * time.Hour)
	return &keymanagement.GetImportParametersResponse{
		KeyID: req.Parameters.KeyID,
		ImportParameters: map[string]any{
			"publicKey":         "mock-public-key-from-provider",
			"wrappingAlgorithm": "CKM_RSA_AES_KEY_WRAP",
			"hashFunction":      "SHA256",
			"providerParams":    "mock-provider-params-from-provider",
			"validTo":           validTime.Format(time.RFC3339),
		},
	}, nil
}

func (s *TestKeyManagement) ImportKeyMaterial(
	_ context.Context,
	req *keymanagement.ImportKeyMaterialRequest,
) (*keymanagement.ImportKeyMaterialResponse, error) {
	if req.Parameters.KeyID != "" {
		s.HandleKeyRecord(req.Parameters.KeyID, EnabledKeyStatus)
	}
	return &keymanagement.ImportKeyMaterialResponse{}, nil
}

func (s *TestKeyManagement) ValidateKey(
	_ context.Context,
	_ *keymanagement.ValidateKeyRequest,
) (*keymanagement.ValidateKeyResponse, error) {
	return &keymanagement.ValidateKeyResponse{IsValid: true}, nil
}

func (s *TestKeyManagement) ValidateKeyAccessData(
	_ context.Context,
	req *keymanagement.ValidateKeyAccessDataRequest,
) (*keymanagement.ValidateKeyAccessDataResponse, error) {
	if len(req.Management) == 0 || len(req.Crypto) == 0 {
		return nil, keymanagement.ErrHYOKKeyNotFound
	}
	return &keymanagement.ValidateKeyAccessDataResponse{IsValid: true}, nil
}

func (s *TestKeyManagement) TransformCryptoAccessData(
	_ context.Context,
	req *keymanagement.TransformCryptoAccessDataRequest,
) (*keymanagement.TransformCryptoAccessDataResponse, error) {
	cryptoAccessDataMap := make(map[string]json.RawMessage)
	if err := json.Unmarshal(req.AccessData, &cryptoAccessDataMap); err != nil {
		return nil, errs.Wrap(ErrTransformAccessData, err)
	}

	transformed := make(map[string][]byte, len(cryptoAccessDataMap))
	for instanceName, instanceData := range cryptoAccessDataMap {
		data := &structpb.Struct{}
		if err := protojson.Unmarshal(instanceData, data); err != nil {
			return nil, errs.Wrap(ErrTransformAccessData, err)
		}
		data.Fields["keyID"] = structpb.NewStringValue(req.NativeKeyID)
		b, err := protojson.Marshal(data)
		if err != nil {
			return nil, errs.Wrap(ErrTransformAccessData, err)
		}
		transformed[instanceName] = b
	}

	return &keymanagement.TransformCryptoAccessDataResponse{
		TransformedAccessData: transformed,
	}, nil
}

func (s *TestKeyManagement) ExtractKeyRegion(
	_ context.Context,
	_ *keymanagement.ExtractKeyRegionRequest,
) (*keymanagement.ExtractKeyRegionResponse, error) {
	return &keymanagement.ExtractKeyRegionResponse{Region: "test-region"}, nil
}
