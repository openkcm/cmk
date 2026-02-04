package sanitise

import (
	"encoding/json"
	"errors"
	"reflect"
	"strings"

	"github.com/microcosm-cc/bluemonday"

	tagManager "github.com/openkcm/cmk/utils/tags"
)

var ErrNilPtr = errors.New("input must be a non-nil pointer")

var policy = bluemonday.StrictPolicy()

// Sanitize modifies a pointer value to prevent XSS attacks
// It changes the value instead of returning a new one to prevent memory duplication
func Sanitize(value any) error {
	v := reflect.ValueOf(value)
	if v.Kind() != reflect.Pointer || v.IsNil() {
		return ErrNilPtr
	}
	return sanitize(v)
}

//nolint:cyclop
func sanitize(v reflect.Value) error {
	for v.Kind() == reflect.Pointer || v.Kind() == reflect.Interface {
		if v.IsNil() {
			return nil
		}
		v = v.Elem()
	}

	if v.Type() == reflect.TypeFor[json.RawMessage]() {
		sanitised, err := sanitiseJSON(string(v.Bytes()))
		if err != nil {
			return err
		}
		v.SetBytes([]byte(sanitised))
	}

	switch v.Kind() {
	case reflect.String:
		v.SetString(policy.Sanitize(v.String()))
	case reflect.Struct:
		err := sanitiseStruct(v)
		if err != nil {
			return err
		}
	case reflect.Slice, reflect.Array:
		l := v.Len()
		for i := range l {
			err := sanitize(v.Index(i))
			if err != nil {
				return err
			}
		}
	case reflect.Map:
		err := sanitiseMap(v)
		if err != nil {
			return err
		}
	default:
		return nil
	}
	return nil
}

// Only sanitise exported fields as others wont be mapped
// Allow to enable/disable sanitise with tag on the field
func sanitiseStruct(v reflect.Value) error {
	l := v.NumField()
	for i := range l {
		f := v.Type().Field(i)
		if !f.IsExported() {
			return nil
		}
		ok, err := checkSanitiseTag(f)
		if err != nil {
			return err
		}
		if !ok {
			return nil
		}
		err = sanitize(v.Field(i))
		if err != nil {
			return err
		}
	}
	return nil
}

// Maps values cant be directly changed
// Need to create a new map and populate with its values sanitized
func sanitiseMap(v reflect.Value) error {
	if v.IsNil() {
		return nil
	}
	newMap := reflect.MakeMap(v.Type())
	for _, key := range v.MapKeys() {
		val := v.MapIndex(key)

		// Call sanitize on temporary pointers
		// as maps values cannot be directly changed
		tmpKey := reflect.New(key.Type())
		tmpKey.Elem().Set(key)
		err := sanitize(tmpKey)
		if err != nil {
			return err
		}

		tmpVal := reflect.New(val.Type())
		tmpVal.Elem().Set(val)
		err = sanitize(tmpVal)
		if err != nil {
			return err
		}

		newMap.SetMapIndex(tmpKey.Elem(), tmpVal.Elem())
	}
	v.Set(newMap)
	return nil
}

func sanitiseJSON(value string) (json.RawMessage, error) {
	replacer := strings.NewReplacer(
		`&`, "&amp;",
		`'`, "&#39;", // "&#39;" is shorter than "&apos;" and apos was not in HTML until HTML5.
		`<`, "&lt;",
		`>`, "&gt;",
	)
	value = replacer.Replace(value)

	_, err := json.Marshal(&value)
	if err != nil {
		return nil, err
	}

	return json.RawMessage(value), nil
}

func checkSanitiseTag(field reflect.StructField) (bool, error) {
	tags, err := tagManager.Get(field.Tag, "repo")
	if err != nil {
		return false, err
	}

	sanitise, err := tagManager.CheckBool(tags, "sanitise", true)
	if err != nil {
		return false, err
	}

	return sanitise, nil
}
