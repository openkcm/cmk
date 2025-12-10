package odata

import (
	"strconv"
	"strings"

	"github.com/google/uuid"

	"github.tools.sap/kms/cmk/internal/errs"
	"github.tools.sap/kms/cmk/utils/slice"
)

// convertString converts the odataValue in the http odata parameter to a string.
// It accounts for enclosing and escaped quotes.
func convertString(odataValue string) (string, error) {
	cnt := 0

	for _, c := range odataValue {
		switch {
		case c == '\'':
			cnt++
		case cnt%2 != 0:
			// Only even number quotes valid per clump, since two to escape
			return "", ErrFilterNotToSpec
		default:
			// Reset the counter for next clump
			cnt = 0
		}
	}

	str := strings.ReplaceAll(odataValue, "''", "'")

	return str, nil
}

// convertUUID converts the odataValue in the http odata parameter to a UUID.
func convertUUID(value string) (uuid.UUID, error) {
	str, err := convertString(value)
	if err != nil {
		return uuid.Nil, err
	}

	uuidVal, err := uuid.Parse(str)
	if err != nil {
		return uuid.Nil, errs.Wrap(ErrFilterValueConversionFailed, err)
	}

	return uuidVal, nil
}

// convertBool converts the odataValue in the http odata parameter to a boolean.
func convertBool(value string) (bool, error) {
	validOdataBools := []string{"true", "false"}
	if !slice.Contains(validOdataBools, value) {
		return false, ErrFilterNotToSpec
	}

	val, err := strconv.ParseBool(value)
	if err != nil {
		return false, ErrFilterValueConversionFailed
	}

	return val, nil
}

// convertInt converts the odataValue in the http odata parameter to an int.
func convertInt(value string) (int64, error) {
	val, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		// Can probably safely assume this isn't to spec
		return 0, ErrFilterNotToSpec
	}

	return val, nil
}
