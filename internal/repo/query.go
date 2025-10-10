package repo

import (
	"errors"
	"fmt"
)

var ErrMultipleOperationsProvided = errors.New("multiple operations provided")

type (
	ComparisonOp   string
	OrderDirection string
)

const (
	Equal       ComparisonOp = "eq"
	GreaterThan ComparisonOp = "gt"
	LessThan    ComparisonOp = "lt"

	Desc OrderDirection = "desc"
	Asc  OrderDirection = "asc"

	IDField             QueryField = "id"
	TypeField           QueryField = "type"
	RegionField         QueryField = "region"
	IdentifierField     QueryField = "identifier"
	KeyField            QueryField = "key"
	KeyTypeField        QueryField = "key_type"
	KeyConfigIDField    QueryField = "key_configuration_id"
	AdminGroupIDField   QueryField = "admin_group_id"
	ResourceIDField     QueryField = "resource_id"
	IsPrimaryField      QueryField = "is_primary"
	VersionField        QueryField = "version"
	StateField          QueryField = "state"
	StatusField         QueryField = "status"
	ArtifactField       QueryField = "artifact"
	UserField           QueryField = "user"
	WorkflowField       QueryField = "workflow"
	ApprovedField       QueryField = "approved"
	ArtifactTypeField   QueryField = "artifact_type"
	ArtifactIDField     QueryField = "artifact_id"
	ActionTypeField     QueryField = "action_type"
	InitiatorIDField    QueryField = "initiator_id"
	PrimaryKeyIDField   QueryField = "primary_key_id"
	PurposeField        QueryField = "purpose"
	NameField           QueryField = "name"
	AutoRotateField     QueryField = "auto_rotate"
	ExpirationDateField QueryField = "expiration_date"
	CreationDateField   QueryField = "creation_date"
	CreatedField        QueryField = "created_at"
	Name                QueryField = "name"

	// KeyconfigTotalSystems and KeyconfigTotalKeys are used as aliases in JOIN operations,
	// typically in combination with the tableName to reference aggregated fields.
	KeyconfigTotalSystems QueryField = "total_systems"
	KeyconfigTotalKeys    QueryField = "total_keys"
	SystemKeyconfigName   QueryField = "key_configuration_name"

	NotEmpty  QueryFieldValue = "not_empty"
	Empty     QueryFieldValue = "empty"
	FalseNull QueryFieldValue = "false_null"

	InnerJoin JoinType = "INNER"
	LeftJoin  JoinType = "LEFT"
	RightJoin JoinType = "RIGHT"
	FullJoin  JoinType = "FULL"

	// AllFunc is used to select all fields in a table, such as table.*.
	// For a simple * selection, do not provide any query function.
	AllFunc AggregateFunction = "*"

	MinFunc   AggregateFunction = "MIN"
	MaxFunc   AggregateFunction = "MAX"
	CountFunc AggregateFunction = "COUNT"
	SumFunc   AggregateFunction = "SUM"
	AvgFunc   AggregateFunction = "AVG"
)

type Key struct {
	Value     any
	Operation ComparisonOp
}

// CompositeKeyEntry represents an entry in a CompositeKey,
// containing a Key and an optional error for validation or processing.
type CompositeKeyEntry struct {
	Key Key
	Err error
}

// CompositeKey is a collection of QueryField and matching value that are collectively used to find a record.
// IsStrict: False Conds: Key = 1, Key2 = 1  where Key = 1 OR Key2 = 1
type CompositeKey struct {
	IsStrict bool // IsStrict indicates if the composite key will use AND logic / OR logic for conditions.
	Conds    map[QueryField]CompositeKeyEntry
}

// NewCompositeKey creates and returns a new CompositeKey.
func NewCompositeKey() CompositeKey {
	return CompositeKey{
		IsStrict: true,
		Conds:    make(map[QueryField]CompositeKeyEntry),
	}
}

// Where adds a condition to the CompositeKey.
func (c CompositeKey) Where(q QueryField, v any,
	options ...func(v any) Key,
) CompositeKey {
	switch {
	case len(options) == 0:
		c.Conds[q] = CompositeKeyEntry{Key: Key{Value: v, Operation: Equal}}
	case len(options) > 1:
		c.Conds[q] = CompositeKeyEntry{Err: ErrMultipleOperationsProvided}
	default:
		c.Conds[q] = CompositeKeyEntry{Key: options[0](v)}
	}

	return c
}

func Gt(v any) Key {
	return Key{Value: v, Operation: GreaterThan}
}

func Lt(v any) Key {
	return Key{Value: v, Operation: LessThan}
}

func (c CompositeKey) Validate() error {
	if len(c.Conds) == 0 {
		return ErrInvalidFieldName
	}

	reservedKeys := map[QueryField]struct{}{
		IdentifierField: {},
		IDField:         {},
	}
	for column := range c.Conds {
		if _, ok := reservedKeys[column]; ok {
			return ErrInvalidFieldName
		}
	}

	return nil
}

type Query struct {
	// Limit is a max size of returned elements.
	Limit int

	Offset int

	// CompositeKeys form the where part of the Query
	CompositeKeyGroup []CompositeKeyGroup

	// PreloadModel specifies which associations to preload.
	PreloadModel Preload

	// Used when updating a model with zero-values
	// If All is true all fields will be updated. Otherwise only the provided will be updated
	// If this is not provided, only non-zero values are updated
	UpdateFields Update

	// Used whenever a custom select is desired
	// By default, if this is not provided select all fields
	SelectFields []*SelectField

	// This could be used to save associations and their references
	// When creating or updating records, updating foreign keys
	Association Association

	// Joins stores the JOIN clauses for the query.
	Joins []JoinClause

	// Used to aggregate columns. Use on GroupBy
	Group []QueryField

	OrderFields []OrderField
}

type JoinType string

type JoinCondition struct {
	Table     table
	Field     string
	JoinTable table
	JoinField string
}
type JoinClause struct {
	OnCondition JoinCondition
	Type        JoinType
}

func (r *JoinClause) JoinStatement() string {
	statement := fmt.Sprintf("%s JOIN %s ON %s.%s = %s.%s",
		r.Type,
		r.OnCondition.JoinTable.TableName(),
		r.OnCondition.Table.TableName(),
		r.OnCondition.Field,
		r.OnCondition.JoinTable.TableName(),
		r.OnCondition.JoinField)

	return statement
}

type Preload []string

type Association struct {
	Field string
	Value any
}

func (a *Association) IsValid() bool {
	return a.Field != "" && a.Value != nil
}

type SelectField struct {
	Field QueryField
	Func  QueryFunction
	Alias string
}

func NewSelectField(field QueryField, f QueryFunction) *SelectField {
	return &SelectField{
		Field: field,
		Func:  f,
	}
}

func (f *SelectField) SelectStatement() string {
	field := f.Field
	switch f.Func.Function {
	case AllFunc:
		field += ".*"
	case MaxFunc, MinFunc, AvgFunc, CountFunc, SumFunc:
		if f.Func.Distinct {
			field = fmt.Sprintf("%s(DISTINCT %s)", f.Func.Function, field)
		} else {
			field = fmt.Sprintf("%s(%s)", f.Func.Function, field)
		}
	}

	if f.Alias != "" {
		field = fmt.Sprintf("%s as %s", field, f.Alias)
	}

	return field
}

type Update struct {
	Fields []QueryField
	All    bool
}

type QueryField = string

type AggregateFunction string

type QueryFunction struct {
	Function AggregateFunction
	Distinct bool
}

type QueryFieldValue = string

type OrderField struct {
	Field     QueryField
	Direction OrderDirection
}

// NewQuery creates and returns a new empty query.
func NewQuery() *Query {
	return &Query{
		CompositeKeyGroup: make([]CompositeKeyGroup, 0),
		Joins:             make([]JoinClause, 0),
		UpdateFields: Update{
			Fields: make([]QueryField, 0),
			All:    false,
		},
		SelectFields: make([]*SelectField, 0),
	}
}

type LoadingFields struct {
	Table       table
	JoinField   QueryField
	SelectField SelectField
}

func NewQueryWithFieldLoading(table table, fields ...LoadingFields) *Query {
	query := NewQuery()

	selectFields := []*SelectField{
		NewSelectField(table.TableName(), QueryFunction{Function: AllFunc}),
	}

	for _, f := range fields {
		selectField := NewSelectField(fmt.Sprintf("%s.%s", f.Table.TableName(), f.SelectField.Field), f.SelectField.Func)
		if f.SelectField.Alias != "" {
			selectField.SetAlias(f.SelectField.Alias)
		}

		selectFields = append(selectFields, selectField)
	}

	query = query.Select(selectFields...)

	for _, f := range fields {
		// It's an aggregate on the same table
		if f.Table == table {
			continue
		}

		joinCond := JoinCondition{
			Table:     table,
			Field:     IDField,
			JoinTable: f.Table,
			JoinField: f.JoinField,
		}

		query = query.Join(FullJoin, joinCond)
	}

	// There are fields outside of the selected table
	if len(query.Joins) > 0 {
		query = query.GroupBy(fmt.Sprintf("%s.%s", table.TableName(), IDField))
	}

	return query
}

func (f *SelectField) SetAlias(alias string) *SelectField {
	f.Alias = alias
	return f
}

type CompositeKeyGroup struct {
	CompositeKey CompositeKey
	IsStrict     bool
}

func NewCompositeKeyGroup(key CompositeKey) CompositeKeyGroup {
	return CompositeKeyGroup{
		CompositeKey: key,
		IsStrict:     true,
	}
}

func (q *Query) Where(conds ...CompositeKeyGroup) *Query {
	q.CompositeKeyGroup = append(q.CompositeKeyGroup, conds...)
	return q
}

func (q *Query) Preload(model Preload) *Query {
	q.PreloadModel = append(q.PreloadModel, model...)
	return q
}

func (q *Query) GroupBy(field ...QueryField) *Query {
	q.Group = append(q.Group, field...)
	return q
}

func (q *Query) Associate(association Association) *Query {
	q.Association = association
	return q
}

func (q *Query) UpdateAll(b bool) *Query {
	q.UpdateFields.All = b
	return q
}

func (q *Query) Update(fields ...QueryField) *Query {
	q.UpdateFields.Fields = append(q.UpdateFields.Fields, fields...)
	return q
}

func (q *Query) Select(fields ...*SelectField) *Query {
	q.SelectFields = append(q.SelectFields, fields...)
	return q
}

// SetLimit sets the limit value for the query.
func (q *Query) SetLimit(limit int) *Query {
	q.Limit = limit
	return q
}

// SetOffset sets the offset value for the query.
func (q *Query) SetOffset(offset int) *Query {
	q.Offset = offset
	return q
}

type table interface {
	TableName() string
}

func (q *Query) Join(joinType JoinType, onCondition JoinCondition) *Query {
	q.Joins = append(q.Joins, JoinClause{
		Type:        joinType,
		OnCondition: onCondition,
	})

	return q
}

func (q *Query) Order(orderFields ...OrderField) *Query {
	q.OrderFields = append(q.OrderFields, orderFields...)
	return q
}
