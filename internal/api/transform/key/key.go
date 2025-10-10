package key

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"

	"github.com/openkcm/cmk/internal/api/cmkapi"
	"github.com/openkcm/cmk/internal/api/transform"
	"github.com/openkcm/cmk/internal/api/transform/key/hyokkey"
	"github.com/openkcm/cmk/internal/api/transform/key/keyshared"
	"github.com/openkcm/cmk/internal/api/transform/key/sysmr"
	"github.com/openkcm/cmk/internal/api/transform/key/transformer"
	"github.com/openkcm/cmk/internal/apierrors"
	"github.com/openkcm/cmk/internal/constants"
	"github.com/openkcm/cmk/internal/errs"
	"github.com/openkcm/cmk/internal/model"
	"github.com/openkcm/cmk/utils/ptr"
)

var (
	ErrInvalidKeyType           = errors.New("invalid key type")
	ErrDeserializeKeyAccessData = errors.New("error deserializing key access data from model to API")
)

// FromAPI converts a cmkapi.Key to a model.Key.
func FromAPI(ctx context.Context, apiKey cmkapi.Key, tf transformer.ProviderTransformer) (*model.Key, error) {
	if apiKey.Name == "" {
		return nil, apierrors.ErrNameFieldMissingProperty
	}

	if apiKey.Type == "" {
		return nil, apierrors.ErrTypeFieldMissingProperty
	}

	if apiKey.KeyConfigurationID == uuid.Nil {
		return nil, apierrors.ErrKeyConfigurationFieldMissingProperty
	}

	dbKey, err := getKeyModel(ctx, tf, apiKey)
	if err != nil {
		return nil, errs.Wrap(keyshared.ErrFromAPI, err)
	}

	dbKey.Name = apiKey.Name
	dbKey.KeyType = string(apiKey.Type)
	dbKey.KeyConfigurationID = apiKey.KeyConfigurationID

	if apiKey.Description != nil {
		dbKey.Description = *apiKey.Description
	}

	now := time.Now()
	dbKey.ID = uuid.New()
	dbKey.CreatedAt = now
	dbKey.UpdatedAt = now

	if apiKey.Enabled == nil || *apiKey.Enabled {
		dbKey.State = string(cmkapi.KeyStateENABLED)
	} else {
		dbKey.State = string(cmkapi.KeyStateDISABLED)
	}

	return dbKey, nil
}

// ToAPI converts a model.Key to a cmkapi.Key
func ToAPI(k model.Key) (*cmkapi.Key, error) {
	var apiKey cmkapi.Key

	apiKey.Id = &k.ID

	if k.Algorithm != "" {
		algorithm := cmkapi.KeyAlgorithm(k.Algorithm)
		apiKey.Algorithm = &algorithm
	}

	if k.Provider != "" {
		apiKey.Provider = &k.Provider
	}

	if k.Region != "" {
		apiKey.Region = &k.Region
	}

	apiKey.NativeID = k.NativeID

	apiKey.Name = k.Name
	if k.Description != "" {
		apiKey.Description = &k.Description
	}

	state := cmkapi.KeyState(k.State)
	apiKey.State = &state

	createdAt := k.CreatedAt.Format(transform.DefTimeFormat)
	updatedAt := k.UpdatedAt.Format(transform.DefTimeFormat)

	apiKey.Metadata = &cmkapi.KeyMetadata{
		CreatedAt: &createdAt,
		UpdatedAt: &updatedAt,
	}

	if k.KeyVersions != nil && k.KeyType == constants.KeyTypeSystemManaged {
		apiKey.Metadata.TotalVersions = ptr.PointTo(len(k.KeyVersions))
		apiKey.Metadata.PrimaryVersion = ptr.PointTo(findPrimaryVersion(k))
	}

	apiKey.KeyConfigurationID = k.KeyConfigurationID
	apiKey.Type = cmkapi.KeyType(k.KeyType)

	if k.KeyType == constants.KeyTypeHYOK {
		accessDetails, err := getAccessDetailsFromModel(k)
		if err != nil {
			return nil, err
		}

		apiKey.AccessDetails = accessDetails
	}

	apiKey.IsPrimary = &k.IsPrimary

	return &apiKey, nil
}

func findPrimaryVersion(k model.Key) int {
	for _, kv := range k.KeyVersions {
		if kv.IsPrimary {
			return kv.Version
		}
	}

	return -1
}

func getKeyModel(ctx context.Context, tf transformer.ProviderTransformer, apiKey cmkapi.Key) (*model.Key, error) {
	var selectedProvider func(
		ctx context.Context, apikey cmkapi.Key, transformer transformer.ProviderTransformer,
	) (*model.Key, error)

	switch apiKey.Type {
	case cmkapi.KeyTypeSYSTEMMANAGED, cmkapi.KeyTypeBYOK:
		selectedProvider = sysmr.FromCmkAPIKey
	case cmkapi.KeyTypeHYOK:
		selectedProvider = hyokkey.FromCmkAPIKey
	default:
		return nil, ErrInvalidKeyType
	}

	return selectedProvider(ctx, apiKey, tf)
}

func getAccessDetailsFromModel(k model.Key) (*cmkapi.KeyAccessDetails, error) {
	var (
		management, crypto map[string]any
		err                error
	)

	err = json.Unmarshal(k.ManagementAccessData, &management)
	if err != nil {
		return nil, errs.Wrap(ErrDeserializeKeyAccessData, err)
	}

	err = json.Unmarshal(k.CryptoAccessData, &crypto)
	if err != nil {
		return nil, errs.Wrap(ErrDeserializeKeyAccessData, err)
	}

	return &cmkapi.KeyAccessDetails{
		Management: ptr.PointTo(management),
		Crypto:     ptr.PointTo(crypto),
	}, nil
}
