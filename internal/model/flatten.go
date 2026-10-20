package model

import (
	"errors"
	"fmt"
	"reflect"
	"strings"

	"github.com/go-viper/mapstructure/v2"

	"github.com/openkcm/cmk/internal/errs"
)

const tag = "flatten"

// ErrMissingTag is returned when an exported struct field has no `flatten` tag.
var (
	ErrMissingTag       = errors.New("exported struct field missing flatten tag")
	ErrUnsuportedField  = errors.New("field type slice/array is not supported")
	ErrUnflattenDecoder = errors.New("failed to create unflatten decoder")
	ErrDecodeFlatten    = errors.New("failed to decode flatten")
)

type Flattenable interface {
	FlattenKeyRoot() string
}

// Flatten takes an item and returns all of its fields as a map[string]any
// It puts all first level items under <root>/<field>
// Root is set on the Flatabble interface implementation and field from the flatten tag on the struct fields
// All the fields and nested fields must contain a flatten tag, otherwise this method will throw an error
// This method does also not support slices, throwing UnsupportedField
//
// Only fields that are exported and do not have tag set as "-" are flatten
func Flatten[T Flattenable](item T) (map[string]any, error) {
	res := map[string]any{}
	err := flatten(item.FlattenKeyRoot(), reflect.ValueOf(item), res)
	return res, err
}

// Unflatten takes a map[string]any, containing each flattenKey <root>/<field>... and transforms it into T.
// The flattenKeys must map exactly into the struct that it's trying to be built. So this should only be used for
// structures that were flattened using the Flatten() method.
func Unflatten[T Flattenable](flatValues map[string]any) (T, error) {
	var out T
	res := map[string]any{}
	for k, v := range flatValues {
		key := strings.Split(k, "/")

		if out.FlattenKeyRoot() != "" {
			if isOnRoot(key, out.FlattenKeyRoot()) {
				key = key[1:]
			}
		}

		if len(key) > 0 {
			unflatten(res, key, v)
		}
	}

	dec, err := mapstructure.NewDecoder(&mapstructure.DecoderConfig{TagName: tag, Result: &out, WeaklyTypedInput: true})
	if err != nil {
		return out, errs.Wrap(ErrUnflattenDecoder, err)
	}

	err = dec.Decode(res)
	if err != nil {
		return out, errs.Wrap(ErrDecodeFlatten, err)
	}
	return out, nil
}

func isOnRoot(flatKeys []string, root string) bool {
	if len(flatKeys) < 2 || flatKeys[0] != root {
		return false
	}
	return true
}

//nolint:cyclop
func flatten(prefix string, v reflect.Value, res map[string]any) error {
	switch v.Kind() {
	case reflect.Interface, reflect.Pointer:
		if v.IsNil() {
			return nil
		}
		return flatten(prefix, v.Elem(), res)
	case reflect.Struct:
		structField := v.Type()
		for i := range structField.NumField() {
			f := structField.Field(i)
			if !f.IsExported() {
				continue
			}
			name, err := getTag(f)
			if err != nil {
				return err
			}
			if name == "-" {
				continue
			}
			fieldPrefix := join(prefix, name)
			err = flatten(fieldPrefix, v.Field(i), res)
			if err != nil {
				return err
			}
		}
	case reflect.Map:
		for _, k := range v.MapKeys() {
			fieldPrefix := join(prefix, fmt.Sprint(k.Interface()))
			err := flatten(fieldPrefix, v.MapIndex(k), res)
			if err != nil {
				return err
			}
		}
	case reflect.Slice, reflect.Array:
		return ErrUnsuportedField
	default:
		res[prefix] = v.Interface()
	}
	return nil
}

func getTag(f reflect.StructField) (string, error) {
	// Cut "," as mapstruct allows operations, but this is not supported here
	name, _, _ := strings.Cut(f.Tag.Get(tag), ",")
	if name == "" {
		return "", ErrMissingTag
	}
	return name, nil
}

func join(prefix, seg string) string {
	if prefix == "" {
		return seg
	}
	return prefix + "/" + seg
}

func unflatten(res map[string]any, flattenKey []string, val any) {
	// Node is a map[string]any, except the edge one, as that is simply added into the parent
	for _, v := range flattenKey[:len(flattenKey)-1] {
		next, ok := res[v].(map[string]any)
		if !ok {
			next = map[string]any{}
			res[v] = next
		}
		res = next
	}
	res[flattenKey[len(flattenKey)-1]] = val
}
