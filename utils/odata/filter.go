package odata

import (
	"slices"
	"strconv"
	"strings"

	"github.com/google/uuid"
	"github.com/openkcm/cmk/internal/errs"
	"github.com/openkcm/cmk/internal/repo"
)

type Type string

const (
	Eq = "eq"
	Ne = "ne"
	Gt = "gt"
	Ge = "ge"
	Lt = "lt"
	Le = "le"

	String Type = "STRING"
	Bool   Type = "BOOL"
	Int    Type = "INT"
	UUID   Type = "UUID"
)

type FilterItem struct {
	Conditional string // "" for first, then "and" or "or" (connector before it)
	Field       string
	Operation   string
	Value       string
}

type Filter struct {
	Items           []FilterItem
	FilterToRepoMap FilterToRepoMap
}

type (
	RepoValueValidator func(string) bool
	RepoValueModifier  func(string) string
)

func ToUpper(s string) string {
	return strings.ToUpper(s)
}

func MaxLengthValidator(maxLength int) func(string) bool {
	return func(s string) bool {
		return len(s) <= maxLength
	}
}

type FilterToRepoMap map[string]FilterToRepoItem

type FilterToRepoItem struct {
	Type           Type
	DBName         repo.QueryField                                // used whenever there is a direct mapping to the DB
	DBQuery        func(query *repo.Query, value any) *repo.Query // used whenever odata maps to a query
	ValueModifier  RepoValueModifier
	ValueValidator RepoValueValidator
}

// This currently does not support nested operations ()
// If needed in the future the data structure needs to be swap from a slice to a tree
// As this is not a simple implementation it was skipped for now
func NewFilter(filter *string, filterToRepoMap FilterToRepoMap) (*Filter, error) {
	f := &Filter{FilterToRepoMap: filterToRepoMap}
	if filter == nil || len(*filter) == 0 {
		return f, nil
	}

	// If there is an odd amount of quotes there is a bad formatted odata string
	if strings.Count(*filter, "'")%2 != 0 {
		return nil, ErrFilterNotToSpec
	}

	tokens := tokenise(*filter)

	items, err := buildFilterItems(tokens)
	if err != nil {
		return nil, err
	}
	f.Items = items

	return f, nil
}

func (f *Filter) GetFieldValues(field repo.QueryField) ([]any, error) {
	values := make([]any, 0)
	for _, e := range f.Items {
		repoEntry, ok := f.FilterToRepoMap[e.Field]
		if !ok {
			return nil, ErrFilterNonSchema
		}
		if repoEntry.DBName != field {
			continue
		}
		v, err := convertToRepoValue(e.Value, repoEntry.Type)
		if err != nil {
			return nil, err
		}
		values = append(values, v)
	}
	return values, nil
}

func (f *Filter) GetQuery() (*repo.Query, error) {
	query := repo.NewQuery()
	for _, e := range f.Items {
		repoEntry, ok := f.FilterToRepoMap[e.Field]
		if !ok {
			return nil, ErrFilterNonSchema
		}
		if repoEntry.ValueModifier != nil {
			e.Value = repoEntry.ValueModifier(e.Value)
		}
		if repoEntry.ValueValidator != nil {
			if !repoEntry.ValueValidator(e.Value) {
				return nil, ErrFilterInvalidValue
			}
		}

		value, err := convertToRepoValue(e.Value, repoEntry.Type)
		if err != nil {
			return nil, err
		}

		op, err := covertToRepoOperation(e.Operation)
		if err != nil {
			return nil, err
		}

		if repoEntry.DBQuery != nil {
			query = repoEntry.DBQuery(query, value)
		} else {
			ck := repo.NewCompositeKey()
			entry := repo.CompositeKeyEntry{
				Key: repo.Key{
					Value:     value,
					Operation: op,
				},
			}
			cond := repo.Condition{
				Field: repoEntry.DBName,
				Value: entry,
			}
			ck.Conds = append(ck.Conds, cond)
			ckg := repo.NewCompositeKeyGroup(ck)
			if e.Conditional == "or" {
				ckg.IsStrict = false
			}
			query.Where(ckg)
		}

	}
	return query, nil
}

func covertToRepoOperation(op string) (repo.ComparisonOp, error) {
	switch op {
	case "eq":
		return repo.Equal, nil
	case "ne":
		return repo.NotEqual, nil
	case "gt":
		return repo.GreaterThan, nil
	case "lt":
		return repo.LessThan, nil
	}
	return "", ErrFilterOperationNotSupported
}

func buildFilterItems(tokens []string) ([]FilterItem, error) {
	items := make([]FilterItem, 0)
	logic := ""
	i := 0

	for i < len(tokens) {
		// Check for outbounds, it needs two more indexes, one for Operation and Value
		if i+2 >= len(tokens) {
			return nil, ErrFilterNotToSpec
		}

		field := tokens[i]
		if isReserved(field) {
			return nil, ErrFilterNotToSpec
		}

		op := tokens[i+1]
		if !isSupportedOperation(op) {
			return nil, ErrFilterOperationNotSupported
		}

		value, err := unquote(tokens[i+2])
		if err != nil {
			return nil, err
		}
		items = append(items, FilterItem{
			Conditional: logic,
			Field:       field,
			Operation:   op,
			Value:       value,
		})

		i += 3
		if i == len(tokens) {
			return items, nil
		}

		if tokens[i] != And && tokens[i] != Or {
			return nil, ErrFilterNotToSpec
		}

		logic = tokens[i]
		i++

		if i == len(tokens) {
			return nil, ErrFilterNotToSpec
		}
	}
	return items, nil
}

func isSupportedOperation(op string) bool {
	reserved := []string{
		Eq,
		Ne,
		Gt,
		Ge,
		Lt,
		Le,
	}
	return slices.Contains(reserved, op)
}

func convertToRepoValue(value string, typ Type) (any, error) {
	var (
		converted any
		err       error
	)
	switch typ {
	case String:
		converted = value
	case UUID:
		converted, err = convertUUID(value)
	case Bool:
		converted, err = convertBool(value)
	case Int:
		converted, err = convertInt(value)
	default:
		return nil, ErrFilterTypeNotSupported
	}

	return converted, err
}

func convertUUID(value string) (uuid.UUID, error) {
	u, err := uuid.Parse(value)
	if err != nil {
		return uuid.Nil, errs.Wrap(ErrFilterValueConversionFailed, err)
	}
	return u, nil
}

func convertBool(value string) (bool, error) {
	switch value {
	case "true":
		return true, nil
	case "false":
		return false, nil
	default:
		return false, ErrFilterNotToSpec // OData allows only true/false
	}
}

func convertInt(value string) (int64, error) {
	v, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return 0, errs.Wrap(ErrFilterValueConversionFailed, err)
	}
	return v, nil
}
