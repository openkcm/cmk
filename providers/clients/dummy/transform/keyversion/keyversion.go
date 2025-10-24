package keyversion

import (
	"time"

	"github.com/openkcm/cmk-core/internal/model"
	"github.com/openkcm/cmk-core/providers"
	"github.com/openkcm/cmk-core/utils/ptr"
)

// FromProvider converts a providers.KeyVersion and model.Key to a model.KeyVersion.
func FromProvider(kv providers.KeyVersion, k model.Key) (model.KeyVersion, error) {
	var updatedAt time.Time
	if kv.UpdatedAt == nil {
		updatedAt = *kv.CreatedAt
	} else {
		updatedAt = *kv.UpdatedAt
	}

	keyVersion := model.KeyVersion{
		ExternalID: *kv.ExternalID,
		AutoTimeModel: model.AutoTimeModel{
			CreatedAt: *kv.CreatedAt,
			UpdatedAt: updatedAt,
		},
		Version: kv.Version,
		KeyID:   k.ID,
		Key:     k,
	}

	return keyVersion, nil
}

// ToProvider converts a model.KeyVersion to a providers.KeyVersion.
func ToProvider(k model.KeyVersion) (providers.KeyVersion, error) {
	keyVersion := providers.KeyVersion{
		ExternalID: ptr.PointTo(k.ExternalID),
		CreatedAt:  ptr.PointTo(k.CreatedAt),
		UpdatedAt:  ptr.PointTo(k.UpdatedAt),
		Version:    k.Version,
	}

	return keyVersion, nil
}
