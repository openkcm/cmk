package testplugins

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"time"

	"github.com/google/uuid"
	"github.com/openkcm/plugin-sdk/api"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/types/known/structpb"

	keystoreErrs "github.com/openkcm/plugin-sdk/pkg/plugin/keystore/errors"

	"github.com/openkcm/cmk/internal/errs"
	"github.com/openkcm/cmk/internal/pluginregistry/service/api/keymanagement"
	servicewrapper "github.com/openkcm/cmk/internal/pluginregistry/service/wrapper"
)

var (
	EnabledKeyStatus         = "ENABLED"
	DisabledKeyStatus        = "DISABLED"
	PendingImportKeyStatus   = "PENDING_IMPORT"
	PendingDeletionKeyStatus = "PENDING_DELETION"

	ErrKeyIDIsNil                = errors.New("keyId is nil")
	ErrTransformAccessData       = errors.New("failed to transform access data")
	ErrNativeKeyIDInvalidPattern = errors.New("native key ID does not match valid pattern")

	// ValidManagementAccessData is the management access data the test plugin accepts
	// in ValidateKeyAccessData. It mirrors the fields returned by CreateKeystore and
	// used throughout HYOK key tests.
	ValidManagementAccessData = map[string]bool{
		"AccountID": true,
		"UserID":    true,
	}
)

const importParamsValidityHours = 24

type KeyVersionRecord struct {
	VersionID    string
	CreationTime *time.Time
	Status       string
}

type KeyRecord struct {
	KeyID        string `gorm:"primaryKey;column:key_id"`
	Status       string
	PKeyVersion  string
	Versions     []KeyVersionRecord
	RotationTime *time.Time // RFC3339 format
}

type TestKeyManagement struct {
	KeyStore             map[string]*KeyRecord
	IsHYOK               bool
	IsDefault            bool
	validRegions         map[string]bool // if non-nil, ValidateKey rejects regions not in this set
	validNativeIDPattern *regexp.Regexp  // if non-nil, ExtractKeyRegion rejects non-matching native IDs
}

var _ keymanagement.KeyManagement = (*TestKeyManagement)(nil)

func NewTestKeyManagement(isHYOK, isDefault bool) *TestKeyManagement {
	km := &TestKeyManagement{
		KeyStore:  make(map[string]*KeyRecord),
		IsHYOK:    isHYOK,
		IsDefault: isDefault,
	}
	for range 3 {
		km.CreateKey(context.Background(), &keymanagement.CreateKeyRequest{
			KeyType: keymanagement.HYOK,
		})
	}
	return km
}

// WithValidRegions restricts ValidateKey to the given regions
func (s *TestKeyManagement) WithValidRegions(regions ...string) *TestKeyManagement {
	s.validRegions = make(map[string]bool, len(regions))
	for _, r := range regions {
		s.validRegions[r] = true
	}
	return s
}

// WithValidNativeIDPattern restricts ExtractKeyRegion to native IDs matching the given pattern,
// returning an error for non-matching IDs.
func (s *TestKeyManagement) WithValidNativeIDPattern(pattern string) *TestKeyManagement {
	s.validNativeIDPattern = regexp.MustCompile(pattern)
	return s
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

// RotateKey sets version and rotation metadata for a key, mirroring the KeystoreOperator helper used in tests.
func (s *TestKeyManagement) RotateKey(keyID string, versionID string, rotationTime *time.Time) error {
	record, exists := s.KeyStore[keyID]
	if !exists {
		return fmt.Errorf("key does not exist")
	}

	record.PKeyVersion = versionID
	record.RotationTime = rotationTime
	// Newer versions should appear first in the slice
	record.Versions = append([]KeyVersionRecord{
		{
			VersionID:    versionID,
			CreationTime: rotationTime,
		},
	}, record.Versions...)
	s.KeyStore[keyID] = record
	return nil
}

func (s *TestKeyManagement) GetKeyVersions(
	_ context.Context,
	req *keymanagement.GetKeyVersionsRequest,
) (*keymanagement.GetKeyVersionsResponse, error) {
	record, exists := s.KeyStore[req.Parameters.KeyID]
	if !exists {
		return nil, keymanagement.ErrHYOKKeyNotFound
	}

	res := make([]keymanagement.KeyVersion, 0, len(record.Versions))
	for _, v := range record.Versions {
		res = append(res, keymanagement.KeyVersion{
			ID:           v.VersionID,
			CreationTime: v.CreationTime,
			Status:       v.Status,
		})
	}

	return &keymanagement.GetKeyVersionsResponse{
		Versions: res,
	}, nil
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
		RotationTime: record.RotationTime,
	}

	if record.PKeyVersion != "" {
		resp.LatestKeyVersionId = record.PKeyVersion
	}

	return resp, nil
}

func (s *TestKeyManagement) CreateKey(
	_ context.Context,
	req *keymanagement.CreateKeyRequest,
) (*keymanagement.CreateKeyResponse, error) {
	keyID := uuid.NewString()

	return &keymanagement.CreateKeyResponse{
		KeyID:  keyID,
		Status: s.createKey(keyID, req.KeyType),
	}, nil
}

func (s *TestKeyManagement) createKey(
	keyID string,
	keyType keymanagement.KeyType,
) string {
	st := EnabledKeyStatus
	if keyType == keymanagement.BYOK {
		st = PendingImportKeyStatus
	}
	initialVersion := "version0"
	time := time.Now().UTC()
	s.KeyStore[keyID] = &KeyRecord{
		KeyID:       keyID,
		Status:      st,
		PKeyVersion: initialVersion,
		Versions: []KeyVersionRecord{
			{
				VersionID:    initialVersion,
				CreationTime: &time,
			},
		},
		RotationTime: &time,
	}
	return st
}

func (s *TestKeyManagement) updateKeyStatus(key string, status string) error {
	record, exists := s.KeyStore[key]
	if !exists {
		return fmt.Errorf("key does not exist")
	}
	record.Status = status
	s.KeyStore[key] = record
	return nil
}

func (s *TestKeyManagement) DeleteKey(
	_ context.Context,
	req *keymanagement.DeleteKeyRequest,
) (*keymanagement.DeleteKeyResponse, error) {
	if req != nil && req.Parameters.KeyID != "" {
		err := s.updateKeyStatus(req.Parameters.KeyID, PendingDeletionKeyStatus)
		if err != nil {
			return nil, err
		}
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

	err := s.updateKeyStatus(req.Parameters.KeyID, EnabledKeyStatus)
	if err != nil {
		return nil, err
	}
	return &keymanagement.EnableKeyResponse{}, nil
}

func (s *TestKeyManagement) DisableKey(
	_ context.Context,
	req *keymanagement.DisableKeyRequest,
) (*keymanagement.DisableKeyResponse, error) {
	if req.Parameters.KeyID == "" {
		return nil, ErrKeyIDIsNil
	}
	err := s.updateKeyStatus(req.Parameters.KeyID, DisabledKeyStatus)
	if err != nil {
		return nil, err
	}
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
	ctx context.Context,
	req *keymanagement.ImportKeyMaterialRequest,
) (*keymanagement.ImportKeyMaterialResponse, error) {
	if req.Parameters.KeyID != "" {
		err := s.updateKeyStatus(req.Parameters.KeyID, EnabledKeyStatus)
		if err != nil {
			return nil, err
		}
	}
	return &keymanagement.ImportKeyMaterialResponse{}, nil
}

func (s *TestKeyManagement) ValidateKey(
	_ context.Context,
	req *keymanagement.ValidateKeyRequest,
) (*keymanagement.ValidateKeyResponse, error) {
	if s.validRegions != nil && !s.validRegions[req.Region] {
		return &keymanagement.ValidateKeyResponse{
			IsValid: false,
			Message: fmt.Sprintf("region %s is not supported", req.Region),
		}, nil
	}
	return &keymanagement.ValidateKeyResponse{IsValid: true}, nil
}

func (s *TestKeyManagement) ValidateKeyAccessData(
	_ context.Context,
	req *keymanagement.ValidateKeyAccessDataRequest,
) (*keymanagement.ValidateKeyAccessDataResponse, error) {
	if len(req.Management) == 0 || len(req.Crypto) == 0 {
		return nil, keymanagement.ErrHYOKKeyNotFound
	}
	for k := range ValidManagementAccessData {
		if _, ok := req.Management[k]; !ok {
			return nil, keystoreErrs.StatusInvalidKeyAccessData.Err()
		}
	}

	// Mirror the real wrapper: verify each crypto region can be converted to a proto struct.
	for regionName, region := range req.Crypto {
		b, err := json.Marshal(region)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal crypto region %q: %w", regionName, err)
		}

		var regionMap map[string]any
		if err := json.Unmarshal(b, &regionMap); err != nil {
			return nil, fmt.Errorf("failed to unmarshal crypto region %q: %w", regionName, err)
		}
		if _, err := structpb.NewStruct(regionMap); err != nil {
			return nil, fmt.Errorf("failed to convert crypto region %q to proto struct: %w", regionName, err)
		}
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
	req *keymanagement.ExtractKeyRegionRequest,
) (*keymanagement.ExtractKeyRegionResponse, error) {
	if s.validNativeIDPattern != nil && !s.validNativeIDPattern.MatchString(req.NativeKeyID) {
		return nil, fmt.Errorf("%w: %q", ErrNativeKeyIDInvalidPattern, req.NativeKeyID)
	}
	return &keymanagement.ExtractKeyRegionResponse{Region: "test-region"}, nil
}
