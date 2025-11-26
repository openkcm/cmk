package async_test

import (
	"context"
	"testing"
	"time"

	"github.com/bartventer/gorm-multitenancy/v8/pkg/driver"
	"github.com/stretchr/testify/assert"

	multitenancy "github.com/bartventer/gorm-multitenancy/v8"

	"github.com/openkcm/cmk/internal/async"
	"github.com/openkcm/cmk/internal/config"
	"github.com/openkcm/cmk/internal/grpc/catalog"
	"github.com/openkcm/cmk/internal/manager"
	"github.com/openkcm/cmk/internal/model"
	"github.com/openkcm/cmk/internal/repo"
	"github.com/openkcm/cmk/internal/repo/sql"
	"github.com/openkcm/cmk/internal/testutils"
	wfMechanism "github.com/openkcm/cmk/internal/workflow"
	integrationutils "github.com/openkcm/cmk/test/integration/integration_utils"
	"github.com/openkcm/cmk/utils/base62"
	ctxUtils "github.com/openkcm/cmk/utils/context"
)

// The identity service group has been created for this tenant and group name specifically
const (
	tenantID  = "tenant1"
	groupName = "KeyAdminTest1"
)

var schemaName, _ = base62.EncodeSchemaNameBase62(tenantID)

func TestWorkflowApproversAssignment(t *testing.T) {
	if integrationutils.CheckAllPluginsMissingFiles(t) {
		return
	}

	testConfig := getConfig(t, config.Scheduler{
		TaskQueue: integrationutils.MessageService,
	})
	SetupTestContainers(t, testConfig)
	db, _, _ := testutils.NewTestDB(t,
		testutils.TestDBConfig{
			Models: []driver.TenantTabler{
				&model.Workflow{},
				&model.WorkflowApprover{},
				&model.Group{},
				&model.Key{},
				&model.System{},
				&model.KeyConfiguration{},
			},
			CreateDatabase: true,
		},
		testutils.WithDatabase(testConfig.Database),
		testutils.WithInitTenants(model.Tenant{
			ID: tenantID,
			TenantModel: multitenancy.TenantModel{
				DomainURL:  schemaName + ".example.com",
				SchemaName: schemaName,
			},
		}),
	)

	ctx := t.Context()

	asyncApp, err := async.New(testConfig)
	assert.NoError(t, err)

	overrideDatabase(t, asyncApp, db, testConfig)

	// Start worker
	go func(ctx context.Context) {
		err := asyncApp.RunWorker(ctx)
		assert.NoError(t, err)
	}(ctx)

	ctx = ctxUtils.CreateTenantContext(ctx, tenantID)

	var (
		adminGroup *model.Group
		groups     []model.Group
	)

	repository := sql.NewRepository(db)

	ck := repo.NewCompositeKey().Where(repo.Name, groupName)
	count, err := repository.List(ctx, model.Group{}, &groups,
		*repo.NewQuery().Where(repo.NewCompositeKeyGroup(ck)))
	assert.NoError(t, err)

	if count == 0 {
		adminGroup = testutils.NewGroup(func(g *model.Group) {
			g.Name = groupName
			g.IAMIdentifier = model.NewIAMIdentifier(groupName, tenantID)
		})
		err = repository.Create(ctx, adminGroup)
		assert.NoError(t, err)
	} else {
		adminGroup = &groups[0]
	}

	keyConfig := testutils.NewKeyConfig(func(kc *model.KeyConfiguration) {
		kc.AdminGroup = *adminGroup
	})
	err = repository.Create(ctx, keyConfig)
	assert.NoError(t, err)

	workflow := testutils.NewWorkflow(func(w *model.Workflow) {
		w.ArtifactID = keyConfig.ID
		w.ArtifactType = wfMechanism.ArtifactTypeKeyConfiguration.String()
		w.ActionType = wfMechanism.ActionTypeDelete.String()
	})

	ctlg, err := catalog.New(ctx, *testConfig)
	tenantConfigManager := manager.NewTenantConfigManager(repository, ctlg)
	keyConfigManager := manager.NewKeyConfigManager(repository, nil, testConfig)
	keyManager := manager.NewKeyManager(repository, ctlg, nil, keyConfigManager, nil, nil, nil)
	systemManager := manager.NewSystemManager(ctx, repository, nil, nil, ctlg, nil, testConfig)
	groupManager := manager.NewGroupManager(repository, ctlg)
	workflowManager := manager.NewWorkflowManager(repository, keyManager, keyConfigManager, systemManager,
		groupManager, asyncApp.Client(), tenantConfigManager)

	assert.NoError(t, err)

	workflow, err = workflowManager.CreateWorkflow(ctx, workflow)
	assert.NoError(t, err)

	time.Sleep(5 * time.Second)

	var approvers []model.WorkflowApprover

	ck = repo.NewCompositeKey().Where("workflow_id", workflow.ID)
	count, err = repository.List(ctx, model.WorkflowApprover{}, &approvers,
		*repo.NewQuery().Where(repo.NewCompositeKeyGroup(ck)))

	assert.NoError(t, err)
	assert.GreaterOrEqual(t, count, 1)
}
