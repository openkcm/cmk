package integration_test

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/bartventer/gorm-multitenancy/v8/pkg/driver"
	"github.com/google/uuid"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/suite"

	multitenancy "github.com/bartventer/gorm-multitenancy/v8"
	tenantgrpc "github.com/openkcm/api-sdk/proto/kms/api/cmk/registry/tenant/v1"

	"github.com/openkcm/cmk/internal/model"
	"github.com/openkcm/cmk/internal/testutils"
	tmdb "github.com/openkcm/cmk/tenant-manager/internal/db"
	"github.com/openkcm/cmk/tenant-manager/tenant-cli/cmd"
	integrationutils "github.com/openkcm/cmk/test/integration_utils"
	"github.com/openkcm/cmk/utils/base62"
)

type CLISuite struct {
	suite.Suite

	cancel context.CancelFunc

	db              *multitenancy.DB
	sleep           bool
	id              string
	status          string
	region          string
	rootCmd         *cobra.Command
	createGroupsCmd *cobra.Command
	createCmd       *cobra.Command
	deleteTenantCmd *cobra.Command
	getTenantCmd    *cobra.Command
	updateTenantCmd *cobra.Command
	listTenantsCmd  *cobra.Command
}

func (s *CLISuite) SetupSuite() {
	s.db, _ = testutils.NewTestDB(s.T(), testutils.TestDBConfig{
		RequiresMultitenancyOrShared: false,
		Models:                       []driver.TenantTabler{&model.Tenant{}, &model.Group{}},
	})

	factory := cmd.NewCommandFactory(s.db)
	s.rootCmd = factory.NewRootCmd(s.T().Context())
	s.rootCmd.PersistentFlags().BoolVar(&s.sleep, "sleep", false, "Enable sleep mode")

	s.createGroupsCmd = factory.NewCreateGroupsCmd(s.T().Context())
	s.createGroupsCmd.Flags().StringVarP(&s.id, "id", "i", "", "Tenant id")
	s.rootCmd.AddCommand(s.createGroupsCmd)

	s.createCmd = factory.NewCreateTenantCmd(s.T().Context())
	s.createCmd.Flags().StringVarP(&s.id, "id", "i", "", "Tenant id")
	s.createCmd.Flags().StringVarP(&s.region, "region", "r", "", "Tenant region")
	s.createCmd.Flags().StringVarP(&s.status, "status", "s", "", "Tenant status")
	s.rootCmd.AddCommand(s.createCmd)

	s.deleteTenantCmd = factory.NewDeleteTenantCmd(s.T().Context())
	s.deleteTenantCmd.Flags().StringVarP(&s.id, "id", "i", "", "Tenant id")
	s.rootCmd.AddCommand(s.deleteTenantCmd)

	s.getTenantCmd = factory.NewGetTenantCmd(s.T().Context())
	s.getTenantCmd.Flags().StringVarP(&s.id, "id", "i", "", "Tenant id")
	s.rootCmd.AddCommand(s.getTenantCmd)

	s.listTenantsCmd = factory.NewListTenantsCmd(s.T().Context())
	s.rootCmd.AddCommand(s.listTenantsCmd)

	s.updateTenantCmd = factory.NewUpdateTenantCmd(s.T().Context())
	s.updateTenantCmd.Flags().StringVarP(&s.id, "id", "i", "", "Tenant id")
	s.updateTenantCmd.Flags().StringVarP(&s.region, "region", "r", "", "Tenant region")
	s.updateTenantCmd.Flags().StringVarP(&s.status, "status", "s", "", "Tenant status")
	s.rootCmd.AddCommand(s.updateTenantCmd)
}

func (s *CLISuite) TearDownSuite() {
	if s.cancel != nil {
		s.cancel()
	}
}

func TestCLISuite(t *testing.T) {
	suite.Run(t, new(CLISuite))
}

func (s *CLISuite) TestCreateTenantCmd() {
	cliTenantID := uuid.NewString()

	err := s.createCmd.Flags().Set("id", cliTenantID)
	s.NoError(err)
	err = s.createCmd.Flags().Set("region", "us-west")
	s.NoError(err)
	err = s.createCmd.Flags().Set("status", "active")
	s.NoError(err)

	s.rootCmd.SetArgs([]string{"create"})

	out := new(bytes.Buffer)
	s.rootCmd.SetOut(out)

	err = s.rootCmd.Execute()
	s.Require().NoError(err, "unexpected error: %v", err)

	schemaName, err := base62.EncodeSchemaNameBase62(cliTenantID)

	result := strings.TrimSpace(out.String())
	s.Equal("Tenant schema created: "+schemaName, result)
	s.NoError(err, "Encoding schema name failed")
	integrationutils.TenantExists(s.T(), s.db, schemaName, model.Group{}.TableName())
}

func createTestTenant(ctx context.Context, db *multitenancy.DB) (*model.Tenant, error) {
	id := uuid.NewString()

	encodedSchemaName, err := base62.EncodeSchemaNameBase62(id)
	if err != nil {
		return nil, err
	}

	tenant := testutils.NewTenant(func(l *model.Tenant) {
		l.ID = id
		l.SchemaName = encodedSchemaName
		l.DomainURL = encodedSchemaName
		l.Region = "us-west-2"
		l.Status = model.TenantStatus(tenantgrpc.Status_STATUS_ACTIVE.String())
	})

	err = tmdb.CreateSchema(ctx, db, tenant)
	if err != nil {
		return nil, err
	}

	return tenant, nil
}

func (s *CLISuite) TestUpdateTenantCmd() {
	ctx := s.T().Context()
	tenant, err := createTestTenant(ctx, s.db)
	s.Require().NoError(err)

	id := tenant.ID
	encodedSchemaName := tenant.SchemaName

	err = s.updateTenantCmd.Flags().Set("id", id)
	s.Require().NoError(err)
	err = s.updateTenantCmd.Flags().Set("region", "us-east-1")
	s.Require().NoError(err)
	err = s.updateTenantCmd.Flags().Set("status", "STATUS_BLOCKED")
	s.Require().NoError(err)

	s.rootCmd.SetArgs([]string{"update"})

	out := new(bytes.Buffer)
	s.rootCmd.SetOut(out)

	err = s.rootCmd.Execute()
	s.Require().NoError(err)

	result := strings.TrimSpace(out.String())
	s.Equal("Tenant updated", result)

	integrationutils.TenantExists(s.T(), s.db, encodedSchemaName, model.Group{}.TableName())
}

func (s *CLISuite) TestUpdateTenantCmdNoID() {
	err := s.updateTenantCmd.Flags().Set("id", "")
	s.Require().NoError(err)

	s.rootCmd.SetArgs([]string{"update"})

	out := new(bytes.Buffer)
	s.rootCmd.SetOut(out)

	err = s.rootCmd.Execute()
	s.Require().Error(err)

	result := strings.TrimSpace(out.String())
	s.Require().Contains(result, "Tenant id is required")
}

func (s *CLISuite) TestGetTenant() {
	ctx := s.T().Context()
	tenant, err := createTestTenant(ctx, s.db)
	s.Require().NoError(err)

	id := tenant.ID

	err = s.getTenantCmd.Flags().Set("id", id)
	s.Require().NoError(err)

	s.rootCmd.SetArgs([]string{"get"})

	out := new(bytes.Buffer)
	s.rootCmd.SetOut(out)

	err = s.rootCmd.Execute()
	s.Require().NoError(err)

	result := strings.TrimSpace(out.String())

	expected, err := json.MarshalIndent(tenant, "", "  ")
	s.Require().NoError(err)
	s.Require().Equal(string(expected), result)
}

func (s *CLISuite) TestCreateDefaultGroups() {
	ctx := s.T().Context()
	tenant, err := createTestTenant(s.T().Context(), s.db)
	s.Require().NoError(err)

	id := tenant.ID

	s.rootCmd.SetArgs([]string{"add-default-groups"})

	err = s.createGroupsCmd.Flags().Set("id", id)
	s.Require().NoError(err)

	out := new(bytes.Buffer)
	s.rootCmd.SetOut(out)

	err = s.rootCmd.Execute()
	s.Require().NoError(err)

	integrationutils.GroupsExists(ctx, s.T(), id, s.db)
}

func (s *CLISuite) TestDeleteTenant() {
	ctx := s.T().Context()
	tenant, err := createTestTenant(ctx, s.db)
	s.Require().NoError(err)

	id := tenant.ID

	s.rootCmd.SetArgs([]string{"delete"})

	err = s.deleteTenantCmd.Flags().Set("id", id)
	s.Require().NoError(err)

	out := new(bytes.Buffer)
	s.rootCmd.SetOut(out)

	err = s.rootCmd.Execute()
	s.Require().NoError(err)

	exists, err := integrationutils.TenantSchemaExists(s.db, tenant.SchemaName)
	s.Require().False(exists, "Schema %s should exist", tenant.SchemaName)
	s.Require().NoError(err)

	exists, err = integrationutils.TenantExistsInPublicSchema(s.db, tenant.SchemaName)
	s.Require().False(exists, "Tenant %s schould not exists in public schema", tenant.SchemaName)
	s.Require().NoError(err)
}
