package model_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/openkcm/cmk/internal/constants"
	"github.com/openkcm/cmk/internal/model"
	"github.com/openkcm/cmk/internal/repo/sql"
	"github.com/openkcm/cmk/internal/testutils"
)

func TestGroupTable(t *testing.T) {
	t.Run("Should have table name group", func(t *testing.T) {
		expectedTableName := "groups"

		tableName := model.Group{}.TableName()

		assert.Equal(t, expectedTableName, tableName)
	})

	t.Run("Should be a tenant table", func(t *testing.T) {
		assert.False(t, model.Group{}.IsSharedModel())
	})
}

func TestGroupValidation(t *testing.T) {
	db, tenants, _ := testutils.NewTestDB(t, testutils.TestDBConfig{})
	r := sql.NewRepository(db)
	ctx := testutils.CreateCtxWithTenant(tenants[0])

	t.Run("Name", func(t *testing.T) {
		tests := []struct {
			name  string
			input string
			err   assert.ErrorAssertionFunc
		}{
			{name: "Should have valid characters", input: "Name_123-testb", err: assert.NoError},
			{name: "Should have valid length", input: strings.Repeat("t", 64), err: assert.NoError},
			{name: "Should have invalid characters", input: "$", err: assert.Error},
			{name: "Should have invalid length", input: strings.Repeat("t", 65), err: assert.Error},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				err := r.Create(ctx, testutils.NewGroup(func(g *model.Group) {
					g.Name = tt.input
				}))
				tt.err(t, err)
			})
		}
	})

	t.Run("IAMIdentifier", func(t *testing.T) {
		tests := []struct {
			name  string
			input string
			err   assert.ErrorAssertionFunc
		}{
			{name: "Should have valid characters", input: "IAMIdentifier_123-test b", err: assert.NoError},
			{name: "Should have valid length", input: strings.Repeat("t", 128), err: assert.NoError},
			{name: "Should have invalid characters", input: "$", err: assert.Error},
			{name: "Should have invalid length", input: strings.Repeat("t", 129), err: assert.Error},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				err := r.Create(ctx, testutils.NewGroup(func(g *model.Group) {
					g.IAMIdentifier = tt.input
				}))
				tt.err(t, err)
			})
		}
	})
}

func TestBuildIAMIdentifier(t *testing.T) {
	type args struct {
		groupType string
		tenantID  string
	}

	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "Valid Admin Group",
			args: args{
				groupType: constants.TenantAdminGroup,
				tenantID:  "A123",
			},
			want: "KMS_TenantAdministrator_A123",
		},

		{
			name: "Success_Admin",
			args: args{
				groupType: constants.TenantAdminGroup,
				tenantID:  "tenant123",
			},
			want: "KMS_" + constants.TenantAdminGroup + "_tenant123",
		},
		{
			name: "Success_Auditor",
			args: args{
				groupType: constants.TenantAuditorGroup,
				tenantID:  "tenant456",
			},
			want: "KMS_" + constants.TenantAuditorGroup + "_tenant456",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := model.NewIAMIdentifier(tt.args.groupType, tt.args.tenantID)
			assert.Equal(t, tt.want, got)
		})
	}
}
