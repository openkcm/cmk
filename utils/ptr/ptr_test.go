package ptr_test

import (
	"reflect"
	"runtime"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"

	"github.com/openkcm/cmk-core/utils/ptr"
)

func TestPanicIfDifferent(t *testing.T) {
	func1 := func() {}
	func2 := func() {}
	func3 := func() {}

	t.Run("should panic when functions are different", func(t *testing.T) {
		defer func() {
			if r := recover(); r != nil {
				expected := "must be " + runtime.FuncForPC(reflect.ValueOf(func2).Pointer()).Name()
				assert.Equal(
					t,
					expected,
					r,
					"PanicWhenDifferent panic with message %v, but got %v",
					expected,
					r,
				)
			} else {
				assert.Fail(t, "PanicWhenDifferent function to panic, but it did not")
			}
		}()

		ptr.PanicIfDifferent(func1, func2)
	})

	t.Run("should not panic when functions are the same", func(t *testing.T) {
		defer func() {
			if r := recover(); r != nil {
				assert.Fail(
					t,
					"PanicWhenDifferent function not to panic, but it did with message",
					r,
				)
			}
		}()

		ptr.PanicIfDifferent(func3, func3)
	})
}

func TestIsValidStrPtr(t *testing.T) {
	validStr := "valid"
	emptyStr := ""
	whitespaceStr := "   "

	tests := []struct {
		name     string
		input    *string
		expected bool
	}{
		{"Valid string pointer", &validStr, true},
		{"Empty string pointer", &emptyStr, false},
		{"Whitespace string pointer", &whitespaceStr, false},
		{"Nil pointer", nil, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ptr.IsValidStrPtr(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGetSafeDeref(t *testing.T) {
	t.Run("Should string on string pointer", func(t *testing.T) {
		exepected := "test"
		res := ptr.GetSafeDeref(ptr.PointTo(exepected))
		assert.Equal(t, exepected, res)
	})

	t.Run("Should empty string on nil string pointer", func(t *testing.T) {
		var expected string

		res := ptr.GetSafeDeref[string](nil)
		assert.Equal(t, expected, res)
	})

	t.Run("Should number on number pointer", func(t *testing.T) {
		expected := 5
		res := ptr.GetSafeDeref(ptr.PointTo(expected))
		assert.Equal(t, expected, res)
	})

	t.Run("Should zero on nil int pointer", func(t *testing.T) {
		var expected int

		res := ptr.GetSafeDeref[int](nil)
		assert.Equal(t, expected, res)
	})
}

func TestIsNotNilUUID(t *testing.T) {
	validUUID := ptr.PointTo(uuid.MustParse("123e4567-e89b-12d3-a456-426614174000"))
	invalidUUID := ptr.PointTo(uuid.Nil)
	nilUUID := (*uuid.UUID)(nil)

	tests := []struct {
		name     string
		input    *uuid.UUID
		expected bool
	}{
		{"Valid UUID", validUUID, true},
		{"Invalid UUID", invalidUUID, false},
		{"Nil UUID", nilUUID, false},
		{"Nil pointer", nil, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ptr.IsNotNilUUID(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestPointerArrayToValueArray(t *testing.T) {
	t.Run("Should convert pointer array to value array", func(t *testing.T) {
		intPtrArray := []*int{ptr.PointTo(1), ptr.PointTo(1)}
		intValArray := ptr.PointerArrayToValueArray(intPtrArray)
		assert.Equal(t, intPtrArray[0], &intValArray[0])
		assert.Equal(t, intPtrArray[1], &intValArray[1])
	})
}
