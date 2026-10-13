package structreader

import (
	"errors"
	"fmt"

	"google.golang.org/protobuf/types/known/structpb"
)

var (
	ErrFieldMissing = errors.New("field is missing")
	ErrFieldEmpty   = errors.New("field is empty")
	ErrNilStruct    = errors.New("struct is nil")
)

// StructReader provides methods to safely extract values from structpb.Struct
type StructReader struct {
	fields map[string]*structpb.Value
}

// New creates a new StructReader from a structpb.Struct
func New(s *structpb.Struct) (*StructReader, error) {
	if s == nil {
		return nil, ErrNilStruct
	}

	return &StructReader{fields: s.GetFields()}, nil
}

// GetOptionalString returns the string value and true if the key exists and is non-empty, otherwise ("", false).
func (r *StructReader) GetOptionalString(key string) (string, bool) {
	value, ok := r.fields[key]
	if !ok {
		return "", false
	}
	strValue := value.GetStringValue()
	if strValue == "" {
		return "", false
	}
	return strValue, true
}

// GetString safely extracts string values from struct
func (r *StructReader) GetString(key string) (string, error) {
	value, ok := r.fields[key]
	if !ok {
		return "", fmt.Errorf("%w: %s", ErrFieldMissing, key)
	}

	strValue := value.GetStringValue()
	if strValue == "" {
		return "", fmt.Errorf("%w: %s", ErrFieldEmpty, key)
	}

	return strValue, nil
}
