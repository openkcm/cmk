package commands_test

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/openkcm/cmk/cmd/cmkctl/commands/tenantmanagercli/commands"
)

func TestListTenantsCmd(t *testing.T) {
	cmd, _ := commands.SetupCommandTest(t, commands.NewListTenantsCmd())

	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)

	err := cmd.Execute()
	assert.NoError(t, err)

	// Should return empty list or list of tenants
	result := out.String()
	assert.NotEmpty(t, result)
}
