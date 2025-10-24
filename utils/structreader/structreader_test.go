package structreader_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"google.golang.org/protobuf/types/known/structpb"

	"github.com/openkcm/cmk-core/utils/structreader"
)

func TestNewConfigReader(t *testing.T) {
	t.Run("nil config", func(t *testing.T) {
		reader, err := structreader.New(nil)
		assert.Nil(t, reader)
		assert.ErrorIs(t, err, structreader.ErrNilStruct)
	})

	t.Run("valid config", func(t *testing.T) {
		config, err := structpb.NewStruct(map[string]interface{}{
			"key": "value",
		})
		assert.NoError(t, err)

		reader, err := structreader.New(config)
		assert.NotNil(t, reader)
		assert.NoError(t, err)
	})
}

func TestConfigReader_GetString(t *testing.T) {
	config, err := structpb.NewStruct(map[string]interface{}{
		"key": "value",
	})
	assert.NoError(t, err)

	reader, err := structreader.New(config)
	assert.NoError(t, err)

	t.Run("existing key", func(t *testing.T) {
		value, err := reader.GetString("key")
		assert.NoError(t, err)
		assert.Equal(t, "value", value)
	})

	t.Run("missing key", func(t *testing.T) {
		value, err := reader.GetString("missing")
		assert.Empty(t, value)
		assert.ErrorIs(t, err, structreader.ErrFieldMissing)
	})

	t.Run("empty value", func(t *testing.T) {
		config, err := structpb.NewStruct(map[string]interface{}{
			"empty": "",
		})
		assert.NoError(t, err)

		reader, err := structreader.New(config)
		assert.NoError(t, err)

		value, err := reader.GetString("empty")
		assert.Empty(t, value)
		assert.ErrorIs(t, err, structreader.ErrFieldEmpty)
	})
}
