package commands_test

import (
	"bytes"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"

	"github.com/openkcm/cmk/cmd/cmkctl/commands/tenantmanagercli/commands"
)

func TestUpdateTenantCmd(t *testing.T) {
	cmd, _ := commands.SetupCommandTest(t, commands.NewUpdateTenantCmd())

	tenant := commands.CreateMockTenant(t)

	err := cmd.Flags().Set("id", tenant.ID)
	assert.NoError(t, err)
	err = cmd.Flags().Set("status", "STATUS_BLOCKED")
	assert.NoError(t, err)

	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)

	err = cmd.Execute()
	// Command returns nil even when tenant not found
	assert.NoError(t, err)

	result := out.String()
	assert.Contains(t, result, "Failed to get tenant")
}

func TestUpdateTenantCmd_NonExisting(t *testing.T) {
	cmd, _ := commands.SetupCommandTest(t, commands.NewUpdateTenantCmd())

	nonExistingID := uuid.NewString()
	err := cmd.Flags().Set("id", nonExistingID)
	assert.NoError(t, err)
	err = cmd.Flags().Set("status", "STATUS_BLOCKED")
	assert.NoError(t, err)

	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)

	err = cmd.Execute()
	assert.NoError(t, err)

	result := out.String()
	assert.Contains(t, result, "Failed to get tenant")
}

func TestUpdateTenantCmd_EmptyID(t *testing.T) {
	cmd, _ := commands.SetupCommandTest(t, commands.NewUpdateTenantCmd())

	err := cmd.Flags().Set("id", "")
	assert.NoError(t, err)
	err = cmd.Flags().Set("status", "STATUS_BLOCKED")
	assert.NoError(t, err)

	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)

	err = cmd.Execute()
	assert.NoError(t, err)

	result := out.String()
	assert.Contains(t, result, "Failed to get tenant")
}
