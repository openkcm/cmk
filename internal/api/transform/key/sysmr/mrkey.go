package sysmr

import (
	"context"
	"errors"

	"github.com/openkcm/cmk-core/internal/api/cmkapi"
	"github.com/openkcm/cmk-core/internal/api/transform"
	"github.com/openkcm/cmk-core/internal/api/transform/key/keyshared"
	"github.com/openkcm/cmk-core/internal/api/transform/key/transformer"
	"github.com/openkcm/cmk-core/internal/errs"
	"github.com/openkcm/cmk-core/internal/model"
)

// FromCmkAPIKey propose to retrieve Managed key Request from CMK Api Key
func FromCmkAPIKey(
	ctx context.Context,
	apiKey cmkapi.Key,
	transformer transformer.ProviderTransformer,
) (*model.Key, error) {
	err := isAPIKeyValid(apiKey)
	if err != nil {
		return nil, err
	}

	err = transformer.ValidateAPI(ctx, apiKey)
	if err != nil {
		return nil, err
	}

	description := ""
	if apiKey.Description != nil {
		description = *apiKey.Description
	}

	return &model.Key{
		KeyType:     string(apiKey.Type),
		Description: description,
		Algorithm:   string(*apiKey.Algorithm),
		Provider:    *apiKey.Provider,
		Region:      *apiKey.Region,
	}, nil
}

// isAPIKeyValid checks if apiKey ora apiKey fields are nil
func isAPIKeyValid(apikey cmkapi.Key) error {
	var err error

	if apikey.Provider == nil {
		err = errors.Join(keyshared.ErrProviderIsRequired)
	}

	if apikey.Region == nil {
		err = errors.Join(err, keyshared.ErrRegionIsRequired)
	}

	if apikey.Algorithm == nil {
		err = errors.Join(err, keyshared.ErrAlgorithmIsRequired)
	}

	if apikey.NativeID != nil {
		err = errors.Join(err, errs.Wrapf(transform.ErrAPIUnexpectedProperty, "nativeID"))
	}

	return err
}
