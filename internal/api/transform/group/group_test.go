package group_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/openkcm/cmk/internal/api/cmkapi"
	"github.com/openkcm/cmk/internal/api/transform/group"
	"github.com/openkcm/cmk/internal/model"
	"github.com/openkcm/cmk/utils/ptr"
)

func TestToAPI(t *testing.T) {
	t.Run("Should convert to API type", func(t *testing.T) {
		expected := &cmkapi.Group{
			Name:        "test",
			Role:        "test",
			Description: ptr.PointTo("test"),
		}
		res := group.ToAPI(model.Group{
			Name:        "test",
			Role:        "test",
			Description: "test",
		})
		assert.Equal(t, expected.Name, res.Name)
		assert.Equal(t, expected.Role, res.Role)
		assert.Equal(t, expected.Description, res.Description)
	})
}

func TestFromAPI(t *testing.T) {
	t.Run("Should convert from API type", func(t *testing.T) {
		expected := &model.Group{
			Name:          "test",
			Role:          "test",
			IAMIdentifier: model.NewIAMIdentifier("test", "test"),
		}
		res := group.FromAPI(cmkapi.Group{
			Name: "test",
			Role: "a",
		}, "test")
		assert.Equal(t, expected.Name, res.Name)
	})
}
