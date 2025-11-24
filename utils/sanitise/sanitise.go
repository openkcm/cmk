package sanitise

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"reflect"
	"slices"
	"strings"

	"github.com/microcosm-cc/bluemonday"

	"github.com/openkcm/cmk/internal/errs"
	tagManager "github.com/openkcm/cmk/utils/tags"
)

var (
	ErrSanitisation                = errors.New("failed sanitisation")
	ErrUnsupportedSanitisationType = errors.New("sanitisation type not supported")
	ErrUnstableSanitisation        = errors.New("sanitisation unstable")
	ErrNonSettableString           = errors.New("non settable string")
	ErrTypeConversion              = errors.New("err converting type")
)

// Currently the sanitisation (mostly) sanitises the strings for the passed object in situ.
// The map type is the exception, since it is non-addressable, so a copy is made for this
// case. Also, maps are only supported when they have both keys and values as strings and
// not as the root object (otherwise they are non-addressable).
// This code could be improved by using copies throughout.

func Stringlikes[T any](obj T) (T, error) {
	fieldValue := getDerefFieldValueFromObj(obj)
	if fieldValue.Kind() == reflect.Map {
		return obj, errs.Wrap(ErrSanitisation, ErrUnsupportedSanitisationType)
	}

	err := sanitiseSwitch(fieldValue)
	if err != nil {
		return obj, errs.Wrap(ErrSanitisation, err)
	}

	return obj, nil
}

func sanitiseSwitch(fieldValue reflect.Value) error {
	if fieldValue.Type() == reflect.TypeFor[json.RawMessage]() {
		rawJSON, ok := fieldValue.Interface().(json.RawMessage)
		if !ok {
			return ErrTypeConversion
		}

		sanitisedJSON, err := sanitiseJSON(string(rawJSON))
		if err != nil {
			return err
		}

		fieldValue.Set(reflect.ValueOf(sanitisedJSON))

		return nil
	}

	switch fieldValue.Kind() {
	case reflect.Slice, reflect.Array:
		return sanitiseFieldSlice(fieldValue)
	case reflect.Struct:
		return sanitiseFieldStruct(fieldValue)
	case reflect.Map:
		return sanitiseFieldMap(fieldValue)
	default:
		return sanitiseFieldString(fieldValue)
	}
}

func sanitiseFieldStruct(fieldValue reflect.Value) error {
	fieldType := fieldValue.Type()
	for i := range fieldValue.NumField() {
		field := fieldType.Field(i)
		if !field.IsExported() {
			continue
		}

		sanitise, err := checkSanitiseTag(field)
		if err != nil {
			return err
		}

		if !sanitise {
			continue
		}

		fieldValue := getDerefFieldValue(fieldValue.Field(i))
		if !fieldValue.IsValid() {
			// nil pointer
			continue
		}

		err = sanitiseSwitch(fieldValue)
		if err != nil {
			return err
		}
	}

	return nil
}

func sanitiseFieldSlice(fieldValue reflect.Value) error {
	for i := range fieldValue.Len() {
		var err error

		fieldValue := getDerefFieldValue(fieldValue.Index(i))
		if !fieldValue.IsValid() {
			// nil pointer
			continue
		}

		err = sanitiseSwitch(fieldValue)
		if err != nil {
			return err
		}
	}

	return nil
}

func sanitiseFieldMap(fieldValue reflect.Value) error {
	newMap := reflect.MakeMap(fieldValue.Type())
	for _, fieldMapKey := range fieldValue.MapKeys() {
		var err error

		var sanitisedKeyString, sanitisedValueString string

		if fieldMapKey.Kind() == reflect.String {
			sanitisedKeyString, err = sanitiseString(fieldMapKey.String())
			if err != nil {
				return err
			}
		} else {
			return ErrUnsupportedSanitisationType
		}

		fieldMapValue := fieldValue.MapIndex(fieldMapKey)
		if fieldMapValue.Kind() == reflect.String {
			sanitisedValueString, err = sanitiseString(fieldMapValue.String())
			if err != nil {
				return err
			}
		} else {
			return ErrUnsupportedSanitisationType
		}

		sanitisedKeyField := reflect.ValueOf(sanitisedKeyString)
		sanitisedKeyValue := reflect.ValueOf(sanitisedValueString)

		newMap.SetMapIndex(sanitisedKeyField, sanitisedKeyValue)
	}

	fieldValue.Set(newMap)

	return nil
}

func sanitiseFieldString(fieldValue reflect.Value) error {
	fieldValue = getDerefFieldValue(fieldValue)
	if !fieldValue.IsValid() {
		// nil pointer
		return nil
	}

	var value string
	if fieldValue.Kind() == reflect.String { //nolint: gocritic
		value = fieldValue.String()
	} else if isIgnoredType(fieldValue.Kind()) {
		return nil
	} else {
		slog.Error(fmt.Sprintf("Ignored sanitisation type: %v. Add handling or explicit ignore.",
			fieldValue.Kind()))

		return ErrUnsupportedSanitisationType
	}

	sanitisedString, err := sanitiseString(value)
	if err != nil {
		return err
	}

	if !fieldValue.CanSet() {
		return ErrNonSettableString
	}

	fieldValue.SetString(sanitisedString)

	return nil
}

func sanitiseString(value string) (string, error) {
	p := bluemonday.StrictPolicy()
	maxCntForStabilisation := 10
	cnt := 0

	var sanitisedValue string

	// We loop here for sanity, incase attacker tries to embed his XSS.
	// Likely bluemonday already accounts for this.
	for {
		sanitisedValue = p.Sanitize(value)
		if sanitisedValue == value {
			break
		}

		value = sanitisedValue

		cnt++
		if cnt == maxCntForStabilisation {
			return "", ErrUnstableSanitisation
		}
	}

	return sanitisedValue, nil
}

func sanitiseJSON(value string) (json.RawMessage, error) {
	value = strings.ReplaceAll(value, "<", "&lt;")
	value = strings.ReplaceAll(value, ">", "&gt;")

	value = strings.ReplaceAll(value, "'", "&apos;")

	_, err := json.Marshal(&value)
	if err != nil {
		return nil, err
	}

	return json.RawMessage(value), nil
}

func isIgnoredType(kind reflect.Kind) bool {
	// We have an allow list for the ignored supported primitive types, which we deem don't
	// require sanitisation. We only currently support slices and structs as non-primitives.
	ignored := []reflect.Kind{reflect.Bool, reflect.Int, reflect.Int8, reflect.Int16,
		reflect.Int32, reflect.Int64, reflect.Uint, reflect.Uint8, reflect.Uint16,
		reflect.Uint32, reflect.Uint64, reflect.Uintptr, reflect.Float32, reflect.Float64,
		reflect.Complex64, reflect.Complex128}

	return slices.Contains(ignored, kind)
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

func getDerefFieldValue(fieldValue reflect.Value) reflect.Value {
	for fieldValue.Kind() == reflect.Pointer {
		fieldValue = fieldValue.Elem()
	}

	return fieldValue
}

func getDerefFieldValueFromObj(obj any) reflect.Value {
	return getDerefFieldValue(reflect.ValueOf(obj))
}
