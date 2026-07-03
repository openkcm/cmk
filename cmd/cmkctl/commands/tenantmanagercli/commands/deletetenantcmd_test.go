package commands_test

import (
	"bytes"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"

	"github.com/openkcm/cmk/cmd/cmkctl/commands/tenantmanagercli/commands"
)

func TestDeleteTenantCmd(t *testing.T) {
	cmd, _ := commands.SetupCommandTest(t, commands.NewDeleteTenantCmd())

	tenant := commands.CreateMockTenant(t)

	err := cmd.Flags().Set("id", tenant.ID)
	assert.NoError(t, err)

	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)

	err = cmd.Execute()
	// Command returns error when tenant not found
	assert.Error(t, err)
}

func TestDeleteTenantCmd_NonExisting(t *testing.T) {
	cmd, _ := commands.SetupCommandTest(t, commands.NewDeleteTenantCmd())

	nonExistingID := uuid.NewString()
	err := cmd.Flags().Set("id", nonExistingID)
	assert.NoError(t, err)

	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)

	err = cmd.Execute()
	// Should handle non-existing tenant gracefully
	assert.Error(t, err)
}
