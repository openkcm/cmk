package protobuf_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"google.golang.org/protobuf/types/known/structpb"

	"github.com/openkcm/cmk-core/utils/protobuf"
)

func TestStructToProtobuf(t *testing.T) {
	tests := []struct {
		name     string
		input    interface{}
		expected *structpb.Struct
		errMsg   string
	}{
		{
			name: "Valid Input",
			input: map[string]interface{}{
				"key1": "value1",
				"key2": 123,
				"key3": true,
			},
			expected: &structpb.Struct{
				Fields: map[string]*structpb.Value{
					"key1": structpb.NewStringValue("value1"),
					"key2": structpb.NewNumberValue(123),
					"key3": structpb.NewBoolValue(true),
				},
			},
			errMsg: "",
		},
		{
			name:     "Nil Input",
			input:    nil,
			expected: nil,
			errMsg:   "unexpected token null",
		},
		{
			name:     "Invalid Input Type",
			input:    make(chan int),
			expected: nil,
			errMsg:   "unsupported type",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := protobuf.StructToProtobuf(tt.input)
			if tt.errMsg != "" {
				assert.Nil(t, result)
				assert.Contains(t, err.Error(), tt.errMsg)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expected.AsMap(), result.AsMap())
			}
		})
	}
}
