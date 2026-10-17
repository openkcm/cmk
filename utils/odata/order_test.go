package odata_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/openkcm/cmk/internal/model"
	"github.com/openkcm/cmk/internal/repo"
	"github.com/openkcm/cmk/utils/odata"
)

func TestNewOrder(t *testing.T) {
	schema := odata.OrderToRepoMap{
		"name":      {DBName: "name"},
		"createdAt": {DBName: "created_at"},
		"type":      {DBName: "key_type"},
	}

	ptrOf := func(s string) *string { return &s }

	tests := []struct {
		name    string
		orderby *string
		want    []repo.OrderField
		wantErr error
	}{
		{
			name:    "nil yields empty",
			orderby: nil,
			want:    nil,
		},
		{
			name:    "empty string yields empty",
			orderby: ptrOf("   "),
			want:    nil,
		},
		{
			name:    "single field defaults to asc",
			orderby: ptrOf("name"),
			want:    []repo.OrderField{{Field: "name", Direction: repo.Asc}},
		},
		{
			name:    "explicit desc",
			orderby: ptrOf("createdAt desc"),
			want:    []repo.OrderField{{Field: "created_at", Direction: repo.Desc}},
		},
		{
			name:    "direction is case insensitive",
			orderby: ptrOf("name DESC"),
			want:    []repo.OrderField{{Field: "name", Direction: repo.Desc}},
		},
		{
			name:    "multi field",
			orderby: ptrOf("type asc, name desc"),
			want: []repo.OrderField{
				{Field: "key_type", Direction: repo.Asc},
				{Field: "name", Direction: repo.Desc},
			},
		},
		{
			name:    "surrounding whitespace tolerated",
			orderby: ptrOf("  name   desc  "),
			want:    []repo.OrderField{{Field: "name", Direction: repo.Desc}},
		},
		{
			name:    "unknown field rejected",
			orderby: ptrOf("unknown"),
			wantErr: odata.ErrOrderNonSchema,
		},
		{
			name:    "invalid direction rejected",
			orderby: ptrOf("name sideways"),
			wantErr: odata.ErrOrderInvalidDirection,
		},
		{
			name:    "too many tokens rejected",
			orderby: ptrOf("name asc extra"),
			wantErr: odata.ErrOrderNotToSpec,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			order, err := odata.NewOrder(tt.orderby, schema, nil)
			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
				return
			}

			require.NoError(t, err)

			_, got, err := order.GetOrder()
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestNewOrderInjectionAttempts(t *testing.T) {
	schema := odata.OrderToRepoMap{"name": {DBName: "name"}}
	ptrOf := func(s string) *string { return &s }

	injections := []string{
		"name; drop table systems",
		"name asc) --",
		"name/**/desc",
		"(select 1)",
		"name asc,2",
		"1=1",
	}

	for _, inj := range injections {
		t.Run(inj, func(t *testing.T) {
			order, err := odata.NewOrder(ptrOf(inj), schema, nil)
			if err != nil {
				return
			}

			_, fields, err := order.GetOrder()
			require.NoError(t, err)
			for _, f := range fields {
				assert.Equal(t, "name", f.Field)
			}
		})
	}
}

func TestNewOrderExceedsMaxClauses(t *testing.T) {
	schema := odata.OrderToRepoMap{"name": {DBName: "name"}}

	parts := make([]string, odata.MaxOrderClauses+1)
	for i := range parts {
		parts[i] = "name"
	}
	clauses := strings.Join(parts, ",")

	_, err := odata.NewOrder(&clauses, schema, nil)
	require.ErrorIs(t, err, odata.ErrOrderNotToSpec)
}

func TestNewOrderJoinField(t *testing.T) {
	ptrOf := func(s string) *string { return &s }
	joinFields := []odata.OrderJoinField{{
		UserField: "properties.role",
		Column:    "sp_ord_role.value",
		Join: repo.JoinCondition{
			Table:     &model.System{},
			Field:     repo.IDField,
			JoinTable: &model.SystemProperty{},
			JoinField: repo.IDField,
			Alias:     "sp_ord_role",
		},
	}}

	order, err := odata.NewOrder(ptrOf("properties.role desc"), odata.OrderToRepoMap{}, joinFields)
	require.NoError(t, err)

	joins, fields, err := order.GetOrder()
	require.NoError(t, err)
	assert.Equal(t, []repo.OrderField{{Field: "sp_ord_role.value", Direction: repo.Desc}}, fields)
	assert.Len(t, joins.Joins, 1)
}
