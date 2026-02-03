package main

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/hashicorp/go-hclog"
	"github.com/openkcm/plugin-sdk/pkg/plugin"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/types/known/structpb"

	commonErrs "github.com/openkcm/plugin-sdk/pkg/plugin/keystore/errors"
	keyopv1 "github.com/openkcm/plugin-sdk/proto/plugin/keystore/operations/v1"
	configv1 "github.com/openkcm/plugin-sdk/proto/service/common/config/v1"

	"github.com/openkcm/cmk/internal/testutils"
)

const importParamsValidityHours = 24

var (
	ErrRequestIsNil       = errors.New("request is nil")
	ErrParameterIsNil     = errors.New("parameter is nil")
	ErrKeyIDIsNil         = errors.New("keyId is nil")
	ErrUnmarshalJSON      = errors.New("failed to unmarshal JSON access data")
	ErrUnmarshalProtoJSON = errors.New("failed to unmarshal protoJSON access data")
	ErrMarshalProto       = errors.New("failed to marshal proto access data")
)

var initialKeys = map[string]KeyRecord{
	"mock-key/11111111": {Status: EnabledKeyStatus},
	"mock-key/22222222": {Status: EnabledKeyStatus},
	"mock-key/33333333": {Status: EnabledKeyStatus},
}

// TestPlugin is a simple test implementation of KeystoreProviderClient
type TestPlugin struct {
	keyopv1.UnsafeKeystoreInstanceKeyOperationServer
	configv1.UnsafeConfigServer

	logger   hclog.Logger
	keyStore map[string]*KeyRecord
}

func (p *TestPlugin) CreateKey(
	_ context.Context,
	request *keyopv1.CreateKeyRequest,
) (*keyopv1.CreateKeyResponse, error) {
	p.logger.Info("CreateKey method has been called;")

	status := EnabledKeyStatus
	if request.GetKeyType() == keyopv1.KeyType_KEY_TYPE_BYOK {
		status = PendingImportKeyStatus
	}

	keyID := "mock-key/" + uuid.NewString()

	p.handleKeyRecord(keyID, status)

	return &keyopv1.CreateKeyResponse{
		KeyId:  keyID,
		Status: status,
	}, nil
}

func (p *TestPlugin) DeleteKey(
	_ context.Context,
	request *keyopv1.DeleteKeyRequest,
) (*keyopv1.DeleteKeyResponse, error) {
	p.logger.Info("DeleteKey method has been called;")

	if request != nil && request.GetParameters() != nil {
		keyID := request.GetParameters().GetKeyId()
		if keyID != "" {
			if p.keyStore != nil {
				p.handleKeyRecord(keyID, PendingDeletionKeyStatus)
			}
		}
	}

	return &keyopv1.DeleteKeyResponse{}, nil
}

func (p *TestPlugin) EnableKey(
	_ context.Context,
	request *keyopv1.EnableKeyRequest,
) (*keyopv1.EnableKeyResponse, error) {
	if request == nil {
		return nil, ErrRequestIsNil
	}

	if request.GetParameters() == nil {
		return nil, ErrParameterIsNil
	}

	keyID := request.GetParameters().GetKeyId()
	if keyID == "" {
		return nil, ErrKeyIDIsNil
	}

	p.logger.Info("EnableKey method has been called;")

	p.handleKeyRecord(keyID, EnabledKeyStatus)

	return &keyopv1.EnableKeyResponse{}, nil
}

func (p *TestPlugin) DisableKey(
	_ context.Context,
	request *keyopv1.DisableKeyRequest,
) (*keyopv1.DisableKeyResponse, error) {
	if request == nil {
		return nil, ErrRequestIsNil
	}

	if request.GetParameters() == nil {
		return nil, ErrParameterIsNil
	}

	keyID := request.GetParameters().GetKeyId()
	if keyID == "" {
		return nil, ErrKeyIDIsNil
	}

	p.logger.Info("DisableKey method has been called;")

	p.handleKeyRecord(keyID, DisabledKeyStatus)

	return &keyopv1.DisableKeyResponse{}, nil
}

func (p *TestPlugin) GetKey(
	_ context.Context,
	request *keyopv1.GetKeyRequest,
) (*keyopv1.GetKeyResponse, error) {
	p.logger.Info("Get method has been called;")

	config := request.GetParameters().GetConfig().GetValues().AsMap()
	if config["authType"] == "AUTH_TYPE_CERTIFICATE" &&
		(config["AccountID"] != testutils.ValidKeystoreAccountInfo["AccountID"] ||
			config["UserID"] != testutils.ValidKeystoreAccountInfo["UserID"]) {
		return nil, commonErrs.NewGrpcErrorWithDetails(
			commonErrs.StatusProviderAuthenticationError,
			"Invalid account information", nil,
		)
	}

	keyID := request.GetParameters().GetKeyId()

	if p.keyStore == nil {
		p.keyStore = make(map[string]*KeyRecord)
	}

	record, exists := p.keyStore[keyID]

	var status string

	if !exists {
		return nil, commonErrs.StatusKeyNotFound.Err()
	}

	status = record.Status

	return &keyopv1.GetKeyResponse{
		Algorithm: keyopv1.KeyAlgorithm_KEY_ALGORITHM_AES256,
		Status:    status,
	}, nil
}

func (p *TestPlugin) GetImportParameters(
	_ context.Context,
	request *keyopv1.GetImportParametersRequest,
) (*keyopv1.GetImportParametersResponse, error) {
	p.logger.Info("GetImportParameters method has been called;")

	validTime := time.Now().Add(importParamsValidityHours * time.Hour)
	validTimeStr := validTime.Format(time.RFC3339)

	importParametersStruct, _ := structpb.NewStruct(map[string]any{
		"publicKey":         "mock-public-key-from-provider",
		"wrappingAlgorithm": "CKM_RSA_AES_KEY_WRAP",
		"hashFunction":      "SHA256",
		"providerParams":    "mock-provider-params-from-provider",
		"validTo":           validTimeStr,
	})

	return &keyopv1.GetImportParametersResponse{
		KeyId:            request.GetParameters().GetKeyId(),
		ImportParameters: importParametersStruct,
	}, nil
}

func (p *TestPlugin) ImportKeyMaterial(
	_ context.Context,
	request *keyopv1.ImportKeyMaterialRequest,
) (*keyopv1.ImportKeyMaterialResponse, error) {
	p.logger.Info("ImportKeyMaterial method has been called;")

	keyID := request.GetParameters().GetKeyId()
	if keyID != "" {
		p.handleKeyRecord(keyID, EnabledKeyStatus)
	}

	return &keyopv1.ImportKeyMaterialResponse{}, nil
}

func (p *TestPlugin) ValidateKey(
	_ context.Context,
	_ *keyopv1.ValidateKeyRequest,
) (*keyopv1.ValidateKeyResponse, error) {
	p.logger.Info("ValidateKey method has been called;")
	return &keyopv1.ValidateKeyResponse{IsValid: true}, nil
}

func (p *TestPlugin) ValidateKeyAccessData(
	_ context.Context,
	req *keyopv1.ValidateKeyAccessDataRequest,
) (*keyopv1.ValidateKeyAccessDataResponse, error) {
	p.logger.Info("ValidateKeyAccessData method has been called;")

	if len(req.GetManagement().GetFields()) == 0 || len(req.GetCrypto().GetFields()) == 0 {
		return nil, commonErrs.StatusInvalidKeyAccessData.Err()
	}

	return &keyopv1.ValidateKeyAccessDataResponse{IsValid: true}, nil
}

func (p *TestPlugin) TransformCryptoAccessData(
	_ context.Context,
	request *keyopv1.TransformCryptoAccessDataRequest,
) (*keyopv1.TransformCryptoAccessDataResponse, error) {
	p.logger.Info("TransformCryptoAccessData method has been called;")

	cryptoAccessDataMap := make(map[string]json.RawMessage)
	transformedCryptoAccessDataMap := make(map[string][]byte)

	err := json.Unmarshal(request.GetAccessData(), &cryptoAccessDataMap)
	if err != nil {
		return nil, ErrUnmarshalJSON
	}

	for instanceName, instanceData := range cryptoAccessDataMap {
		data := &structpb.Struct{}

		err = protojson.Unmarshal(instanceData, data)
		if err != nil {
			return nil, ErrUnmarshalProtoJSON
		}

		data.Fields["keyID"] = structpb.NewStringValue(request.GetNativeKeyId())

		instanceBytes, err := protojson.Marshal(data)
		if err != nil {
			return nil, ErrMarshalProto
		}

		transformedCryptoAccessDataMap[instanceName] = instanceBytes
	}

	return &keyopv1.TransformCryptoAccessDataResponse{
		TransformedAccessData: transformedCryptoAccessDataMap,
	}, nil
}

func (p *TestPlugin) ExtractKeyRegion(
	_ context.Context,
	_ *keyopv1.ExtractKeyRegionRequest,
) (*keyopv1.ExtractKeyRegionResponse, error) {
	p.logger.Info("ExtractKeyRegion method has been called;")
	return &keyopv1.ExtractKeyRegionResponse{Region: "test-region"}, nil
}

func New() *TestPlugin {
	p := &TestPlugin{
		keyStore: make(map[string]*KeyRecord),
	}

	for keyID, record := range initialKeys {
		p.handleKeyRecord(keyID, record.Status)
	}

	return p
}

func (p *TestPlugin) SetLogger(logger hclog.Logger) {
	p.logger = logger
	p.logger.Info("SetLogger method has been called;")
}

// Configure configures the plugin.

func (p *TestPlugin) Configure(
	_ context.Context,
	_ *configv1.ConfigureRequest,
) (*configv1.ConfigureResponse, error) {
	p.logger.Info("Configure method has been called;")

	var buildInfo = "{}"

	return &configv1.ConfigureResponse{
		BuildInfo: &buildInfo,
	}, nil
}

func (p *TestPlugin) handleKeyRecord(keyID, status string) {
	if p.keyStore == nil {
		p.keyStore = make(map[string]*KeyRecord)
	}

	record, exists := p.keyStore[keyID]
	if !exists {
		record = &KeyRecord{
			KeyID:  keyID,
			Status: status,
		}
		p.keyStore[keyID] = record
	}

	record.Status = status
}

func main() {
	p := New()

	plugin.Serve(
		keyopv1.KeystoreInstanceKeyOperationPluginServer(p),
		configv1.ConfigServiceServer(p),
	)
}
