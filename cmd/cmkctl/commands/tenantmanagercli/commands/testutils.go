package commands

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/spf13/cobra"

	multitenancy "github.com/bartventer/gorm-multitenancy/v8"
	tenantgrpc "github.com/openkcm/api-sdk/proto/kms/api/cmk/registry/tenant/v1"

	"github.com/openkcm/cmk/internal/config"
	"github.com/openkcm/cmk/internal/constants"
	"github.com/openkcm/cmk/internal/model"
	"github.com/openkcm/cmk/internal/testutils"
	"github.com/openkcm/cmk/utils/base62"
	cmkcontext "github.com/openkcm/cmk/utils/context"
)

type MockCommandFactory struct {
	DB     *multitenancy.DB
	Config *config.Config
}

func SetupCommandTest(t *testing.T, cmd *cobra.Command) (*cobra.Command, string) {
	t.Helper()

	dbCon, tenants, dbCfg := testutils.NewTestDB(
		t, testutils.TestDBConfig{
			CreateDatabase: true,
		},
	)

	ctx := t.Context()

	cfg := &config.Config{
		Database: dbCfg,
	}

	svcRegistry := testutils.NewTestPlugins()

	factory, err := NewCommandFactory(ctx, cfg, dbCon, svcRegistry)
	if err != nil {
		t.Fatalf("failed to create command factory: %v", err)
	}

	cmdCtx, err := cmkcontext.InjectInternalUserData(ctx, constants.InternalTenantCLIRole)
	if err != nil {
		t.Fatalf("failed to inject internal user data: %v", err)
	}

	cmdCtx = context.WithValue(cmdCtx, TenantManagerFactoryKey, factory)
	cmd.SetContext(cmdCtx)

	return cmd, tenants[0]
}

func CreateMockTenant(t *testing.T) *model.Tenant {
	t.Helper()

	id := uuid.NewString()

	encodedSchemaName, err := base62.EncodeSchemaNameBase62(id)
	if err != nil {
		t.Fatalf("failed to encode schema name: %v", err)
	}

	tenant := testutils.NewTenant(
		func(l *model.Tenant) {
			l.ID = id
			l.SchemaName = encodedSchemaName
			l.DomainURL = encodedSchemaName
			l.Status = model.TenantStatus(tenantgrpc.Status_STATUS_ACTIVE.String())
		},
	)

	return tenant
}
