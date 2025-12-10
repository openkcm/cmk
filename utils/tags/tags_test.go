package tags_test

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.tools.sap/kms/cmk/utils/tags"
)

func TestGet(t *testing.T) {
	type s1 struct {
		I int `test:"a:b;c:d"`
	}

	s := s1{}
	value := reflect.ValueOf(s)
	tags, err := tags.Get(value.Type().Field(0).Tag, "test")
	assert.NoError(t, err)
	assert.Equal(t, map[string]string{"a": "b", "c": "d"}, tags)
}

func TestCheckBool(t *testing.T) {
	tagsTrue := map[string]string{"a": "b", "c": "true"}
	b, err := tags.CheckBool(tagsTrue, "c", false)
	assert.NoError(t, err)
	assert.True(t, b)

	tagsFalse := map[string]string{"a": "b", "c": "false"}
	b, err = tags.CheckBool(tagsFalse, "c", true)
	assert.NoError(t, err)
	assert.False(t, b)

	// Test error for non bool
	_, err = tags.CheckBool(tagsFalse, "a", true)
	assert.Error(t, err)
}
