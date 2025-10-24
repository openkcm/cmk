package hyokkey

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

var (
	ErrNativeIDIsRequired      = errors.New("nativeID is required")
	ErrAccessDetailsIsRequired = errors.New("accessDetails is required")
	ErrTransformAccessData     = errors.New("error transforming access data from API to model")
)

// FromCmkAPIKey propose to retrieve HYOKey from CMK Api Key
//

func FromCmkAPIKey(
	ctx context.Context,
	apiKey cmkapi.Key,
	transformer transformer.ProviderTransformer,
) (*model.Key, error) {
	if apiKey.NativeID == nil {
		return nil, ErrNativeIDIsRequired
	}

	if apiKey.Provider == nil {
		return nil, keyshared.ErrProviderIsRequired
	}

	if apiKey.Region != nil {
		return nil, errs.Wrapf(transform.ErrAPIUnexpectedProperty, "region")
	}

	if apiKey.Algorithm != nil {
		return nil, errs.Wrapf(transform.ErrAPIUnexpectedProperty, "algorithm")
	}

	if apiKey.AccessDetails == nil {
		return nil, ErrAccessDetailsIsRequired
	}

	accessData, err := transformer.SerializeKeyAccessData(ctx, apiKey)
	if err != nil {
		return nil, errs.Wrap(ErrTransformAccessData, err)
	}

	region, err := transformer.GetRegion(ctx, apiKey)
	if err != nil {
		return nil, err
	}

	description := ""
	if apiKey.Description != nil {
		description = *apiKey.Description
	}

	return &model.Key{
		KeyType:              string(apiKey.Type),
		Description:          description,
		NativeID:             apiKey.NativeID,
		Provider:             *apiKey.Provider,
		Region:               region,
		ManagementAccessData: accessData.Management,
		CryptoAccessData:     accessData.Crypto,
	}, nil
}
