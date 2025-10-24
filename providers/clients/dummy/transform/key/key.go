package key

import (
	"github.com/openkcm/cmk-core/internal/model"
	"github.com/openkcm/cmk-core/providers"
	"github.com/openkcm/cmk-core/utils/ptr"
)

// ToProvider converts a model.Key and array of providers.KeyVersion to a providers.Key.
func ToProvider(k model.Key, kv []providers.KeyVersion) (*providers.Key, error) {
	key := &providers.Key{
		ID:          ptr.PointTo(k.ID.String()),
		Provider:    k.Provider,
		Region:      k.Region,
		KeyVersions: kv,
	}

	return key, nil
}
