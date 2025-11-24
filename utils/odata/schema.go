package odata

import (
	"strings"

	"github.com/openkcm/cmk/internal/repo"
)

// Type specifies the odata type.
type Type string

const (
	String Type = "STRING"
	Bool   Type = "BOOL"
	Int    Type = "INT"
	UUID   Type = "UUID"
)

// DBQuery specifies the mapping between odata filters (and potentially other things) to a CMK repo operation.
type DBQuery string

const (
	WhereQuery DBQuery = ""     // Default, Therefore leave as empty string
	NoQuery    DBQuery = "None" // Client must handle
)

// FilterSchemaEntry provides an endpoint specific specification for supported filters.
// FieldName and Type specify the odata filter names and types.
// DBName specifies the DB column name the filter maps to.
// DBQuery specifies what type of DB operation the odata filter should map to.
// ValueModifier and ValueValidator act on the filter field values provided on an endpoint call.
type FilterSchemaEntry struct {
	FilterName     string
	FilterType     Type
	DBName         repo.QueryField
	DBQuery        DBQuery
	ValueModifier  func(string) (string, bool)
	ValueValidator func(string) bool
}

// ToUpper is helper ValueValidator. More can be added if generic enough.
func ToUpper(s string) (string, bool) {
	return strings.ToUpper(s), true
}

// preProcessValue provides any standard preprocessing before any of the user
// definied activity on the odataValue.
func (fs FilterSchemaEntry) preProcessValue(odataValue string) (string, error) {
	if fs.FilterType == String || fs.FilterType == UUID {
		// Check string is quoted
		if len(odataValue) < 2 ||
			odataValue[0] != '\'' ||
			odataValue[len(odataValue)-1] != '\'' {
			return "", ErrFilterNotToSpec
		}
		// Remove quotes from string value
		return odataValue[1 : len(odataValue)-1], nil
	}

	return odataValue, nil
}

// modify applies the FilterSchemaEntry.ValueModifier on the entry value and returns the
// modified value.
func (fs FilterSchemaEntry) modify(preProcessedOdataValue string) (string, error) {
	if fs.ValueModifier == nil {
		return preProcessedOdataValue, nil
	}

	var ok bool

	modifiedOdataValue, ok := fs.ValueModifier(preProcessedOdataValue)
	if !ok {
		return "", ErrFilterValueModificationFailed
	}

	return modifiedOdataValue, nil
}

// convert converts the modified entry value to any type as required by the repo.
func (fs FilterSchemaEntry) convert(modifiedOdataValue string) (any, error) {
	var retVal any

	var retErr error

	switch fs.FilterType {
	case String:
		retVal, retErr = convertString(modifiedOdataValue)
	case UUID:
		retVal, retErr = convertUUID(modifiedOdataValue)
	case Bool:
		retVal, retErr = convertBool(modifiedOdataValue)
	case Int:
		retVal, retErr = convertInt(modifiedOdataValue)
	default:
		retErr = ErrFilterTypeNotSupported
	}

	return retVal, retErr
}

// apply applies the schema to an odata value
func (fs FilterSchemaEntry) apply(odataValue string) (any, error) {
	processedOdataValue, err := fs.preProcessValue(odataValue)
	if err != nil {
		return nil, err
	}

	processedOdataValue, err = fs.modify(processedOdataValue)
	if err != nil {
		return nil, err
	}

	convertedValue, err := fs.convert(processedOdataValue)
	if err != nil {
		return nil, err
	}

	if fs.ValueValidator != nil {
		if !fs.ValueValidator(processedOdataValue) {
			return nil, ErrFilterInvalidValue
		}
	}

	return convertedValue, nil
}

// FilterSchema contains all the FilterSchemaEntry associated with an endpoint.
type FilterSchema struct {
	// We avoid a map here as it's unsorted and we probably want to keep our
	// sql deterministic.
	Entries []FilterSchemaEntry
}

// validate validates the FilterSchema.
func (fs FilterSchema) validate() error {
	// Currently none necessary
	return nil
}

// getEntryFromDBName maps the DBName to FilterSchemaEntry. This is used post parse, since the
// parsed data is referenced from the DBName.
func (fs FilterSchema) getEntryFromDBName(dbName repo.QueryField) (*FilterSchemaEntry, error) {
	for _, entry := range fs.Entries {
		if entry.DBName == dbName {
			return &entry, nil
		}
	}

	return nil, ErrFilterNonSchema
}

// assertTypeFromDBName asserts the type from the DBName post parse. It's really just a type
// check when an incorrect field name is used to try and reference an explicit type
// (eg GetUUID function), which is an edge case which shouldn't occur at run time. Look at
// the usage for more details.
func (fs FilterSchema) assertTypeFromDBName(dbName repo.QueryField, filterType Type) error {
	entry, err := fs.getEntryFromDBName(dbName)
	if err != nil {
		return err
	}

	if entry.FilterType != filterType {
		return ErrFilterTypeIncompatible
	}

	return nil
}

// convert converts all the schema entries to the any type as required by repo.
func (fs FilterSchema) apply(odataField, odataValue string) (string, any, error) {
	for _, entry := range fs.Entries {
		if odataField == entry.FilterName {
			convertedValue, err := entry.apply(odataValue)
			if err != nil {
				return "", nil, err
			}

			return entry.DBName, convertedValue, nil
		}
	}

	return "", nil, ErrFilterNonSchema
}
