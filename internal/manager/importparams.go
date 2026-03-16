package manager

import (
	"encoding/json"
	"time"

	"google.golang.org/protobuf/types/known/structpb"

	"github.com/openkcm/cmk/internal/errs"
	"github.com/openkcm/cmk/internal/model"
	"github.com/openkcm/cmk/internal/pluginregistry/service/api/keymanagement"
	"github.com/openkcm/cmk/utils/structreader"
)

// CommonImportFields contains fields that are common across all providers
type CommonImportFields struct {
	PublicKeyPEM      string
	WrappingAlgorithm string
	HashFunction      string
}

// ProviderImportFields contains provider-specific parameters and optional expiration
type ProviderImportFields struct {
	ProviderParams map[string]any
	Expires        *time.Time
}

// BuildImportParamsFromAPI creates import parameters from API response
func BuildImportParamsFromAPI(
	key *model.Key,
	importParamsResp *keymanagement.GetImportParametersResponse,
) (*model.ImportParams, error) {
	// Convert map to protobuf struct for the reader
	protoStruct, err := structpb.NewStruct(importParamsResp.ImportParameters)
	if err != nil {
		return nil, errs.Wrap(ErrBuildImportParams, err)
	}

	reader, err := structreader.New(protoStruct)
	if err != nil {
		return nil, errs.Wrap(ErrBuildImportParams, err)
	}

	// Extract common fields
	commonFields, err := extractCommonFields(reader)
	if err != nil {
		return nil, errs.Wrap(ErrBuildImportParams, err)
	}

	// Build provider-specific parameters
	providerParams, err := buildProviderParams(reader)
	if err != nil {
		return nil, errs.Wrap(ErrBuildImportParams, err)
	}

	paramsJSON, err := json.Marshal(providerParams.ProviderParams)
	if err != nil {
		return nil, errs.Wrap(ErrMarshalProviderParams, err)
	}

	return &model.ImportParams{
		KeyID:              key.ID,
		PublicKeyPEM:       commonFields.PublicKeyPEM,
		HashFunction:       commonFields.HashFunction,
		WrappingAlg:        commonFields.WrappingAlgorithm,
		Expires:            providerParams.Expires,
		ProviderParameters: paramsJSON,
	}, nil
}

// extractCommonFields extracts fields that are common across all providers
func extractCommonFields(reader *structreader.StructReader) (*CommonImportFields, error) {
	publicKey, err := reader.GetString("publicKey")
	if err != nil {
		return nil, errs.Wrap(ErrExtractCommonImportFields, err)
	}

	wrappingAlgorithm, err := reader.GetString("wrappingAlgorithm")
	if err != nil {
		return nil, errs.Wrap(ErrExtractCommonImportFields, err)
	}

	hashFunction, err := reader.GetString("hashFunction")
	if err != nil {
		return nil, errs.Wrap(ErrExtractCommonImportFields, err)
	}

	return &CommonImportFields{
		PublicKeyPEM:      publicKey,
		WrappingAlgorithm: wrappingAlgorithm,
		HashFunction:      hashFunction,
	}, nil
}

// buildProviderParams extracts provider-specific parameters, optionally including expiration
func buildProviderParams(reader *structreader.StructReader) (*ProviderImportFields, error) {
	providerParams, err := reader.GetString("providerParams")
	if err != nil {
		return nil, err
	}

	validTo, err := reader.GetString("validTo")
	if err != nil {
		return nil, err
	}

	expires, err := time.Parse(time.RFC3339, validTo)
	if err != nil {
		return nil, err
	}

	return &ProviderImportFields{
		ProviderParams: map[string]any{
			"providerParams": providerParams,
		},
		Expires: &expires,
	}, nil
}
