package protobuf

import (
	"encoding/json"

	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/types/known/structpb"
)

func StructToProtobuf[T any](t T) (*structpb.Struct, error) {
	b, err := json.Marshal(t)
	if err != nil {
		return nil, err
	}

	s := &structpb.Struct{}

	err = protojson.Unmarshal(b, s)
	if err != nil {
		return nil, err
	}

	return s, nil
}
