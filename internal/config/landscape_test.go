package config_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/openkcm/cmk/internal/config"
)

func TestLandscape_GetMaxVersionsForProvider(t *testing.T) {
	t.Run("Returns default when MaxKeyVersions is nil", func(t *testing.T) {
		landscape := config.Landscape{
			Name:           "test",
			MaxKeyVersions: nil,
		}

		limit := landscape.GetMaxVersionsForProvider("AWS")
		assert.Equal(t, config.DefaultMaxKeyVersions, limit)
	})

	t.Run("Returns configured value for provider", func(t *testing.T) {
		landscape := config.Landscape{
			Name: "test",
			MaxKeyVersions: map[string]int{
				"AWS":      10,
				"GCP":      3,
				"FORTANIX": -1,
			},
		}

		assert.Equal(t, 10, landscape.GetMaxVersionsForProvider("AWS"))
		assert.Equal(t, 3, landscape.GetMaxVersionsForProvider("GCP"))
		assert.Equal(t, -1, landscape.GetMaxVersionsForProvider("FORTANIX"))
	})

	t.Run("Returns default for unconfigured provider", func(t *testing.T) {
		landscape := config.Landscape{
			Name: "test",
			MaxKeyVersions: map[string]int{
				"AWS": 10,
			},
		}

		limit := landscape.GetMaxVersionsForProvider("GCP")
		assert.Equal(t, config.DefaultMaxKeyVersions, limit)
	})

	t.Run("Returns unlimited (-1) when configured", func(t *testing.T) {
		landscape := config.Landscape{
			Name: "test",
			MaxKeyVersions: map[string]int{
				"FORTANIX": config.UnlimitedKeyVersions,
			},
		}

		limit := landscape.GetMaxVersionsForProvider("FORTANIX")
		assert.Equal(t, config.UnlimitedKeyVersions, limit)
	})
}

func TestLandscape_Validate(t *testing.T) {
	t.Run("Valid configuration with nil MaxKeyVersions", func(t *testing.T) {
		landscape := config.Landscape{
			Name:           "test",
			MaxKeyVersions: nil,
		}

		err := landscape.Validate()
		assert.NoError(t, err)
	})

	t.Run("Valid configuration with positive limits", func(t *testing.T) {
		landscape := config.Landscape{
			Name: "test",
			MaxKeyVersions: map[string]int{
				"AWS":      5,
				"GCP":      10,
				"FORTANIX": 1,
			},
		}

		err := landscape.Validate()
		assert.NoError(t, err)
	})

	t.Run("Valid configuration with unlimited (-1)", func(t *testing.T) {
		landscape := config.Landscape{
			Name: "test",
			MaxKeyVersions: map[string]int{
				"AWS":      5,
				"FORTANIX": -1,
			},
		}

		err := landscape.Validate()
		assert.NoError(t, err)
	})

	t.Run("Invalid configuration with zero limit", func(t *testing.T) {
		landscape := config.Landscape{
			Name: "test",
			MaxKeyVersions: map[string]int{
				"AWS": 0,
			},
		}

		err := landscape.Validate()
		assert.Error(t, err)
		assert.ErrorIs(t, err, config.ErrInvalidMaxKeyVersions)
		assert.Contains(t, err.Error(), "AWS")
	})

	t.Run("Invalid configuration with negative limit (not -1)", func(t *testing.T) {
		landscape := config.Landscape{
			Name: "test",
			MaxKeyVersions: map[string]int{
				"GCP": -5,
			},
		}

		err := landscape.Validate()
		assert.Error(t, err)
		assert.ErrorIs(t, err, config.ErrInvalidMaxKeyVersions)
		assert.Contains(t, err.Error(), "GCP")
	})

	t.Run("Invalid configuration with multiple providers, one invalid", func(t *testing.T) {
		landscape := config.Landscape{
			Name: "test",
			MaxKeyVersions: map[string]int{
				"AWS": 5,
				"GCP": 0, // Invalid
			},
		}

		err := landscape.Validate()
		assert.Error(t, err)
		assert.ErrorIs(t, err, config.ErrInvalidMaxKeyVersions)
	})
}
