package manager

import (
	"encoding/json"
	"time"

	keystoreopv1 "github.com/openkcm/plugin-sdk/proto/plugin/keystore/operations/v1"

	"github.com/openkcm/cmk/internal/errs"
	"github.com/openkcm/cmk/internal/model"
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

// BuildImportParams creates import parameters for the specified provider
func BuildImportParams(
	key *model.Key,
	importParamsResp *keystoreopv1.GetImportParametersResponse,
) (*model.ImportParams, error) {
	reader, err := structreader.New(importParamsResp.GetImportParameters())
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
