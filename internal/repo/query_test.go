package repo_test

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/openkcm/cmk/internal/model"
	"github.com/openkcm/cmk/internal/repo"
)

func TestQuery_Join(t *testing.T) {
	q := repo.NewQuery()
	joinCond := repo.JoinCondition{
		Table:     &model.Workflow{},
		Field:     fmt.Sprintf("%s_%s", repo.WorkflowField, repo.IDField),
		JoinTable: &model.WorkflowApprover{},
		JoinField: repo.IDField,
	}

	q.Join(repo.LeftJoin, joinCond)
	assert.Len(t, q.Joins, 1)
	statement := q.Joins[0].JoinStatement()
	assert.Equal(t, `LEFT JOIN "workflow_approvers" ON "workflows".workflow_id = "workflow_approvers".id`, statement)
}

func TestQuery_AggregateFunctions(t *testing.T) {
	t.Run("Should aggregate function", func(t *testing.T) {
		q := repo.NewQuery().Select(
			repo.NewSelectField(repo.IDField, repo.QueryFunction{
				Function: repo.CountFunc,
			}),
		)

		expected := "COUNT(id)"
		assert.Equal(t, expected, q.SelectFields[0].SelectStatement())
	})

	t.Run("Should have distinct on aggregate function", func(t *testing.T) {
		q := repo.NewQuery().Select(
			repo.NewSelectField(repo.IDField, repo.QueryFunction{
				Function: repo.CountFunc,
				Distinct: true,
			}),
		)
		expected := "COUNT(DISTINCT id)"
		assert.Equal(t, expected, q.SelectFields[0].SelectStatement())
	})

	t.Run("Should have only field on empty function", func(t *testing.T) {
		q := repo.NewQuery().Select(
			repo.NewSelectField(repo.IDField, repo.QueryFunction{}),
		)
		expected := "id"
		assert.Equal(t, expected, q.SelectFields[0].SelectStatement())
	})

	t.Run("Should select all of a table", func(t *testing.T) {
		q := repo.NewQuery().Select(
			repo.NewSelectField(model.Key{}.TableName(), repo.QueryFunction{Function: repo.AllFunc}),
		)
		expected := "keys.*"
		assert.Equal(t, expected, q.SelectFields[0].SelectStatement())
	})

	t.Run("Should apply alias", func(t *testing.T) {
		q := repo.NewQuery().Select(
			repo.NewSelectField(repo.IDField, repo.QueryFunction{}).SetAlias("i"),
		)
		expected := "id as i"
		assert.Equal(t, expected, q.SelectFields[0].SelectStatement())
	})
}

func TestConditionalSelect(t *testing.T) {
	t.Run("Should write conditional select field", func(t *testing.T) {
		field := repo.NewConditionalSelectField(
			"test",
			repo.NewCompositeKeyGroup(
				repo.NewCompositeKey().
					Where(repo.IsPrimaryField, true).
					Where(repo.KeyTypeField, "test"),
			),
		)

		expected := "((is_primary = 'true' AND key_type = 'test')) as test"
		assert.Equal(t, expected, field.SelectStatement())
	})
}
