package commands_test

import (
	"bytes"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"

	"github.com/openkcm/cmk/cmd/cmkctl/commands/tenantmanagercli/commands"
)

func TestGetTenantCmd(t *testing.T) {
	cmd, tenantID := commands.SetupCommandTest(t, commands.NewGetTenantCmd())

	err := cmd.Flags().Set("id", tenantID)
	assert.NoError(t, err)

	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)

	err = cmd.Execute()
	assert.NoError(t, err)
}

func TestGetTenantCmd_NonExisting(t *testing.T) {
	cmd, _ := commands.SetupCommandTest(t, commands.NewGetTenantCmd())

	nonExistingID := uuid.NewString()
	err := cmd.Flags().Set("id", nonExistingID)
	assert.NoError(t, err)

	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)

	err = cmd.Execute()
	assert.Error(t, err)

	result := out.String()
	assert.Contains(t, result, "Failed to get tenant")
}

func TestGetTenantCmd_EmptyID(t *testing.T) {
	cmd, _ := commands.SetupCommandTest(t, commands.NewGetTenantCmd())

	err := cmd.Flags().Set("id", "")
	assert.NoError(t, err)

	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)

	err = cmd.Execute()
	assert.Error(t, err)
}
