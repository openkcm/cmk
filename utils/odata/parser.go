package odata

import (
	"strings"
	"unicode"
)

// We currently only support "eq" and "and" odata operations.
// As we extend support we will have to expand the parsers below.
var nonSupportedOperations = []string{" or ", " not ", " ne ",
	" gt ", " ge ", " lt ", " le ", "(", ")"}

const numComparisonOperands = 2

// parseFilter parses an odata filter string and returns ordered fields and values
func parseFilter(odataFilter string) ([]string, []string, error) {
	parsedFilterFields := []string{}
	parsedFilterValues := []string{}

	if len(odataFilter) == 0 {
		// Special case when no filter provided
		return parsedFilterFields, parsedFilterValues, nil
	}

	comparisonOperations, err := splitBooleanOperations(odataFilter)
	if err != nil {
		return parsedFilterFields, parsedFilterValues, err
	}

	for _, comparisonOperation := range comparisonOperations {
		field, value, err := splitComparisonOperation(comparisonOperation)
		if err != nil {
			return parsedFilterFields, parsedFilterValues, err
		}

		parsedFilterFields = append(parsedFilterFields, field)
		parsedFilterValues = append(parsedFilterValues, value)
	}

	return parsedFilterFields, parsedFilterValues, nil
}

// parseBooleanOperations parses an odata filter string and returns comparison operators
// as separated by boolean operations (currently only "and"s supported).
func splitBooleanOperations(odataFilter string) ([]string, error) {
	// We traverse the string counting quotes and checking for the operations.
	// If we find an operation and there have been an odd number of quotes it
	// means we are inside a string value and we can ignore it as it's part of the string.
	// This is because a string literal must always be enclosed in quotes and any quotes
	// within the string escaped via double quotes. Therefore when we leave a string
	// there will always have been an even number of quotes.
	quoteCount := 0
	boolOpIndices := []int{}
	properDyadFound := false

	for i, c := range odataFilter {
		if c == '\'' {
			quoteCount++
		} else if quoteCount%2 == 0 {
			found, err := checkFilterForOperation(odataFilter[i:], &properDyadFound)
			if err != nil {
				return []string{}, err
			}

			if found {
				boolOpIndices = append(boolOpIndices, i)
			}
		}
	}

	if quoteCount%2 == 1 {
		// Can't end with an odd number of quotes (always two enclosing quotes and
		// one escape quote with each literal quote)
		return []string{}, ErrFilterNotToSpec
	}

	return splitBooleanOperandsFromIndices(odataFilter, boolOpIndices), nil
}

func splitComparisonOperation(odataFilterOp string) (string, string, error) {
	splitOp := strings.SplitN(odataFilterOp, " eq ", numComparisonOperands)

	if len(splitOp) != numComparisonOperands {
		return "", "", ErrFilterNotToSpec
	}

	// Check it's not inside a string literal
	// Since this is the first operation it's invalid state
	quoteIndex := strings.Index(odataFilterOp, "'")
	if quoteIndex != -1 && quoteIndex < len(splitOp[0]) {
		return "", "", ErrFilterNotToSpec
	}

	return strings.TrimSpace(splitOp[0]), strings.TrimSpace(splitOp[1]), nil
}

func checkFilterForOperation(splitOdataFilter string, properDyadFound *bool) (bool, error) {
	isFinalChar := len(splitOdataFilter) == 1
	switch {
	case !isFinalChar && (splitOdataFilter[1] == '\'' ||
		unicode.IsSpace(rune(splitOdataFilter[1]))):
		// Don't bother looking ahead when next char legal
		// If this is start of an op we'll get next iteration
		return false, nil
	case startsWithOp(splitOdataFilter, " and "):
		*properDyadFound = false // reset the flag
		return true, nil
	case startsWithOp(splitOdataFilter, " eq "):
		if *properDyadFound {
			// Dyads must be separated by boolean operators
			// eg "num eq gt 6" is wrong
			return false, ErrFilterNotToSpec
		}

		*properDyadFound = true
	default:
		for _, nonSupportedOperation := range nonSupportedOperations {
			if startsWithOp(splitOdataFilter, nonSupportedOperation) {
				return false, ErrFilterOperationNotSupported
			}
		}
	}

	return false, nil
}

// startsWithOp is a parse helper function used to look ahead for any supported odata operations.
func startsWithOp(slicedFilterParam, op string) bool {
	if len(slicedFilterParam) >= len(op) {
		if slicedFilterParam[0:len(op)] == op {
			return true
		}
	}

	return false
}

// splitBooleanOperandsFromIndices is a parse helper function which splits
// indexed anded ops into fields and values.
func splitBooleanOperandsFromIndices(param string, boolOpIndices []int) []string {
	booleanOperands := make([]string, 0, len(boolOpIndices))

	lastSplitIndex := 0
	for _, andOpIndex := range boolOpIndices {
		splitAnd := param[lastSplitIndex:andOpIndex]
		booleanOperands = append(booleanOperands, splitAnd)
		lastSplitIndex = andOpIndex + len(" and ")
	}

	return append(booleanOperands, param[lastSplitIndex:])
}
