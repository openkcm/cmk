package commands_test

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"

	"github.com/openkcm/cmk/cmd/cmkctl/commands/tenantmanagercli/commands"
	"github.com/openkcm/cmk/internal/model"
	"github.com/openkcm/cmk/utils/base62"
)

func TestCreateTenantCmd(t *testing.T) {
	cmd, _ := commands.SetupCommandTest(t, commands.NewCreateTenantCmd())

	cliTenantID := uuid.NewString()

	err := cmd.Flags().Set("id", cliTenantID)
	assert.NoError(t, err)
	err = cmd.Flags().Set("status", "STATUS_ACTIVE")
	assert.NoError(t, err)
	err = cmd.Flags().Set("role", "ROLE_TEST")
	assert.NoError(t, err)

	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)

	err = cmd.Execute()
	assert.NoError(t, err, "unexpected error: %v", err)

	schemaName, err := base62.EncodeSchemaNameBase62(cliTenantID)
	assert.NoError(t, err, "Encoding schema name failed")

	result := out.String()
	assert.Contains(t, result, cliTenantID)
	assert.Contains(t, result, schemaName)
}

func TestFormatTenant(t *testing.T) {
	tenant := commands.CreateMockTenant(t)

	var buf bytes.Buffer
	cmd := commands.NewCreateTenantCmd()
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)

	err := commands.FormatTenant(tenant, cmd)
	assert.NoError(t, err)

	output := buf.String()
	assert.NotEmpty(t, output)

	var parsed model.Tenant
	err = json.Unmarshal([]byte(output), &parsed)
	assert.NoError(t, err)
	assert.Equal(t, tenant.ID, parsed.ID)
	assert.Equal(t, tenant.SchemaName, parsed.SchemaName)
}
