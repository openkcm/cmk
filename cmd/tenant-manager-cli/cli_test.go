package main_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/bartventer/gorm-multitenancy/v8/pkg/driver"
	"github.com/google/uuid"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/suite"

	multitenancy "github.com/bartventer/gorm-multitenancy/v8"
	tenantgrpc "github.com/openkcm/api-sdk/proto/kms/api/cmk/registry/tenant/v1"

	"github.tools.sap/kms/cmk/cmd/tenant-manager-cli/commands"
	"github.tools.sap/kms/cmk/internal/auditor"
	"github.tools.sap/kms/cmk/internal/clients"
	"github.tools.sap/kms/cmk/internal/config"
	eventprocessor "github.tools.sap/kms/cmk/internal/event-processor"
	"github.tools.sap/kms/cmk/internal/grpc/catalog"
	"github.tools.sap/kms/cmk/internal/manager"
	"github.tools.sap/kms/cmk/internal/model"
	"github.tools.sap/kms/cmk/internal/repo/sql"
	"github.tools.sap/kms/cmk/internal/testutils"
	integrationutils "github.tools.sap/kms/cmk/test/integration/integration_utils"
	"github.tools.sap/kms/cmk/utils/base62"
)

type CLISuite struct {
	suite.Suite

	cancel context.CancelFunc

	db              *multitenancy.DB
	rootCmd         *cobra.Command
	createGroupsCmd *cobra.Command
	createCmd       *cobra.Command
	deleteTenantCmd *cobra.Command
	getTenantCmd    *cobra.Command
	updateTenantCmd *cobra.Command
	listTenantsCmd  *cobra.Command

	gm *manager.GroupManager
	tm *manager.TenantManager
}

func (s *CLISuite) SetupSuite() {
	db, _, dbCfg := testutils.NewTestDB(
		s.T(), testutils.TestDBConfig{
			CreateDatabase: true,
			Models:         []driver.TenantTabler{&model.Tenant{}, &model.Group{}},
		},
	)
	s.db = db

	ctx := s.T().Context()
	cfg := &config.Config{
		Plugins:  testutils.SetupMockPlugins(testutils.IdentityPlugin),
		Database: dbCfg,
	}
	r := sql.NewRepository(s.db)
	ctlg, err := catalog.New(ctx, cfg)
	s.NoError(err)

	cmkAuditor := auditor.New(ctx, cfg)

	clientsFactory, err := clients.NewFactory(cfg.Services)
	s.NoError(err)

	reconciler, err := eventprocessor.NewCryptoReconciler(
		ctx, cfg, r,
		ctlg, clientsFactory,
	)
	s.NoError(err)

	cm := manager.NewCertificateManager(ctx, r, ctlg, &cfg.Certificates)
	kcm := manager.NewKeyConfigManager(r, cm, cmkAuditor, cfg)

	sys := manager.NewSystemManager(
		ctx,
		r,
		clientsFactory,
		reconciler,
		ctlg,
		cfg,
		kcm,
	)

	km := manager.NewKeyManager(
		r,
		ctlg,
		manager.NewTenantConfigManager(r, ctlg),
		kcm,
		cm,
		reconciler,
		cmkAuditor,
	)

	s.gm = manager.NewGroupManager(r, ctlg)
	s.tm = manager.NewTenantManager(r, sys, km, cmkAuditor)

	factory, err := commands.NewCommandFactory(ctx, cfg, s.db, ctlg)
	s.NoError(err)
	s.rootCmd = factory.NewRootCmd(s.T().Context())

	s.createGroupsCmd = factory.NewCreateGroupsCmd(s.T().Context())
	s.rootCmd.AddCommand(s.createGroupsCmd)

	s.createCmd = factory.NewCreateTenantCmd(s.T().Context())
	s.rootCmd.AddCommand(s.createCmd)

	s.deleteTenantCmd = factory.NewDeleteTenantCmd(s.T().Context())
	s.rootCmd.AddCommand(s.deleteTenantCmd)

	s.getTenantCmd = factory.NewGetTenantCmd(s.T().Context())
	s.rootCmd.AddCommand(s.getTenantCmd)

	s.listTenantsCmd = factory.NewListTenantsCmd(s.T().Context())
	s.rootCmd.AddCommand(s.listTenantsCmd)

	s.updateTenantCmd = factory.NewUpdateTenantCmd(s.T().Context())
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

func (s *CLISuite) TestFormatTenant() {
	tenant, err := s.createTenant()
	s.Require().NoError(err)

	command := &cobra.Command{}

	var buf bytes.Buffer
	command.SetOut(&buf)
	command.SetErr(&buf)

	err = commands.FormatTenant(tenant, command)
	s.Require().NoError(err)

	output := buf.String()
	s.Require().NotEmpty(output)

	var parsed model.Tenant

	err = json.Unmarshal([]byte(output), &parsed)
	s.Require().NoError(err)
	s.Require().Equal(tenant, &parsed)
}

func (s *CLISuite) TestListTenantsCmd() {
	tenant, err := s.createTenant()
	s.Require().NoError(err)

	err = s.tm.CreateTenant(s.T().Context(), tenant)
	s.Require().NoError(err)

	s.rootCmd.SetArgs([]string{"list"})

	out := new(bytes.Buffer)
	s.rootCmd.SetOut(out)

	err = s.rootCmd.Execute()
	s.Require().NoError(err, "unexpected error: %v", err)

	result := strings.TrimSpace(out.String())

	expected := fmt.Sprintf(`"domainURL": "%s"`, tenant.DomainURL)
	s.Contains(result, expected, "output should contain expected domainURL")
	expected = fmt.Sprintf(`"schemaName": "%s"`, tenant.SchemaName)
	s.Contains(result, expected, "output should contain expected schemaName")
	expected = fmt.Sprintf(`"ID": "%s"`, tenant.ID)
	s.Contains(result, expected, "output should contain expected ID")
	expected = fmt.Sprintf(`"Status": "%s"`, "STATUS_ACTIVE")
	s.Contains(result, expected, "output should contain expected status")
}

func (s *CLISuite) TestCreateTenantCmd() {
	cliTenantID := uuid.NewString()

	err := s.createCmd.Flags().Set("id", cliTenantID)
	s.NoError(err)
	err = s.createCmd.Flags().Set("region", "us-west")
	s.NoError(err)
	err = s.createCmd.Flags().Set("status", "STATUS_ACTIVE")
	s.NoError(err)
	err = s.createCmd.Flags().Set("role", "ROLE_TEST")
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

func (s *CLISuite) TestUpdateTenantCmd() {
	ctx := s.T().Context()
	tenant, err := s.createTestTenant(ctx)
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

func (s *CLISuite) TestGetTenantNonExisting() {
	id := uuid.NewString()

	err := s.getTenantCmd.Flags().Set("id", id)
	s.Require().NoError(err)

	s.rootCmd.SetArgs([]string{"get"})

	out := new(bytes.Buffer)
	s.rootCmd.SetOut(out)

	err = s.rootCmd.Execute()
	s.Require().NoError(err)

	result := strings.TrimSpace(out.String())

	s.Require().NoError(err)
	s.Require().Empty(result)
}

func (s *CLISuite) TestGetTenant() {
	ctx := s.T().Context()
	tenant, err := s.createTestTenant(ctx)
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
	tenant, err := s.createTestTenant(s.T().Context())
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
	tenant, err := s.createTestTenant(ctx)
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

func (s *CLISuite) createTenant() (*model.Tenant, error) {
	id := uuid.NewString()

	encodedSchemaName, err := base62.EncodeSchemaNameBase62(id)
	if err != nil {
		return nil, err
	}

	tenant := testutils.NewTenant(
		func(l *model.Tenant) {
			l.ID = id
			l.SchemaName = encodedSchemaName
			l.DomainURL = encodedSchemaName
			l.Region = "us-west-2"
			l.Status = model.TenantStatus(tenantgrpc.Status_STATUS_ACTIVE.String())
		},
	)

	return tenant, nil
}

func (s *CLISuite) createTestTenant(ctx context.Context) (*model.Tenant, error) {
	tenant, err := s.createTenant()
	if err != nil {
		return nil, err
	}

	err = s.tm.CreateTenant(ctx, tenant)
	if err != nil {
		return nil, err
	}

	return tenant, nil
}
