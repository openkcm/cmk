package benchmark_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/google/uuid"

	cmkapi "github.com/openkcm/cmk/internal/api/cmk/generated"
	"github.com/openkcm/cmk/internal/auditor"
	"github.com/openkcm/cmk/internal/config"
	"github.com/openkcm/cmk/internal/constants"
	"github.com/openkcm/cmk/internal/manager"
	"github.com/openkcm/cmk/internal/model"
	"github.com/openkcm/cmk/internal/repo"
	"github.com/openkcm/cmk/internal/repo/sql"
	"github.com/openkcm/cmk/internal/testutils"
	cmkcontext "github.com/openkcm/cmk/utils/context"
)

func setupKeyConfigManager(b *testing.B) (*manager.KeyConfigManager, *sql.ResourceRepository, context.Context) {
	b.Helper()

	db, tenants, _ := testutils.NewTestDB(b, testutils.TestDBConfig{WithOrbital: true})
	r := sql.NewRepository(db)

	cmkAuditor := auditor.New(b.Context(), &config.Config{})
	userManager := manager.NewUserManager(r, cmkAuditor)
	mgr := manager.NewKeyConfigManager(r, nil, userManager, nil, nil, nil, &config.Config{})

	ctx := cmkcontext.CreateTenantContext(b.Context(), tenants[0])
	ctx = cmkcontext.InjectRequestID(ctx, uuid.NewString())

	return mgr, r, ctx
}

// seedKeyConfigData creates n key configurations, each with the given number of
// systems and workflows. Systems cycle across four states:
//   - CONNECTED  (KeyConfigurationID = kc.ID)
//   - FAILED     (KeyConfigurationID = kc.ID)
//   - PROCESSING (KeyConfigurationID = kc.ID)
//   - CONNECTING: PROCESSING with TargetKeyConfigurationID = kc.ID (no KeyConfigurationID)
//
// Workflows cycle across WAIT_APPROVAL/INITIAL/SUCCESSFUL
func seedKeyConfigData(
	b *testing.B,
	n, systemsPerKC, workflowsPerKC int,
) (*manager.KeyConfigManager, context.Context, []uuid.UUID) {
	b.Helper()

	mgr, r, ctx := setupKeyConfigManager(b)

	group := testutils.NewGroup(func(g *model.Group) { g.Role = constants.KeyAdminRole })
	ctx = testutils.InjectBusinessUserDataIntoContext(ctx, uuid.NewString(), []string{group.IAMIdentifier})
	testutils.CreateTestEntities(ctx, b, r, group)

	type systemMut func(kcID uuid.UUID) *model.System
	statusMuts := []systemMut{
		func(kcID uuid.UUID) *model.System {
			return testutils.NewSystem(func(s *model.System) {
				s.KeyConfigurationID = &kcID
				s.Status = cmkapi.SystemStatusCONNECTED
			})
		},
		func(kcID uuid.UUID) *model.System {
			return testutils.NewSystem(func(s *model.System) {
				s.KeyConfigurationID = &kcID
				s.Status = cmkapi.SystemStatusFAILED
			})
		},
		func(kcID uuid.UUID) *model.System {
			return testutils.NewSystem(func(s *model.System) {
				s.KeyConfigurationID = &kcID
				s.Status = cmkapi.SystemStatusPROCESSING
			})
		},
		func(kcID uuid.UUID) *model.System {
			// connecting: PROCESSING toward this KC from outside
			return testutils.NewSystem(func(s *model.System) {
				s.TargetKeyConfigurationID = &kcID
				s.Status = cmkapi.SystemStatusPROCESSING
			})
		},
	}

	wfStates := []model.WorkflowState{
		model.WorkflowStateWaitApproval,
		model.WorkflowStateInitial,
		model.WorkflowStateSuccessful, // terminal — should not count
	}

	ids := make([]uuid.UUID, 0, n)

	for i := range n {
		primaryKey := testutils.NewKey(func(k *model.Key) { k.State = cmkapi.KeyStateENABLED })
		kc := testutils.NewKeyConfig(func(kc *model.KeyConfiguration) {
			kc.Name = fmt.Sprintf("bench-kc-%d-%s", i, uuid.NewString())
			kc.AdminGroupID = group.ID
			kc.AdminGroup = *group
			kc.PrimaryKeyID = &primaryKey.ID
		})
		primaryKey.KeyConfigurationID = kc.ID
		testutils.CreateTestEntities(ctx, b, r, primaryKey, kc)

		for j := range systemsPerKC {
			sys := statusMuts[j%len(statusMuts)](kc.ID)
			testutils.CreateTestEntities(ctx, b, r, sys)
		}

		for j := range workflowsPerKC {
			wf := testutils.NewWorkflow(func(w *model.Workflow) {
				w.State = wfStates[j%len(wfStates)]
			})
			testutils.CreateTestEntities(ctx, b, r, wf)
			testutils.CreateTestEntities(ctx, b, r,
				testutils.NewWorkflowKeyConfiguration(func(wkc *model.WorkflowKeyConfiguration) {
					wkc.WorkflowID = wf.ID
					wkc.KeyConfigurationID = kc.ID
				}))
		}

		ids = append(ids, kc.ID)
	}

	return mgr, ctx, ids
}

func BenchmarkGetKeyConfigurations(b *testing.B) {
	sizes := []struct {
		keyConfigs int
		systems    int
		workflows  int
	}{
		{10, 10, 10},
		{50, 50, 50},
		{100, 100, 100},
		{100, 100, 250},
	}

	for _, s := range sizes {
		name := fmt.Sprintf("kcs=%d/systems=%d/workflows=%d", s.keyConfigs, s.systems, s.workflows)

		b.Run(name+"/basic", func(b *testing.B) {
			mgr, ctx, _ := seedKeyConfigData(b, s.keyConfigs, s.systems, s.workflows)
			filter := manager.KeyConfigFilter{Pagination: repo.Pagination{Top: s.keyConfigs}}
			b.ResetTimer()
			for range b.N {
				_, _, _ = mgr.GetKeyConfigurations(ctx, filter)
			}
		})

		b.Run(name+"/extendedMetadata", func(b *testing.B) {
			mgr, ctx, _ := seedKeyConfigData(b, s.keyConfigs, s.systems, s.workflows)
			filter := manager.KeyConfigFilter{
				ExtendedMetadata: true,
				Pagination:       repo.Pagination{Top: s.keyConfigs},
			}
			b.ResetTimer()
			for range b.N {
				_, _, _ = mgr.GetKeyConfigurations(ctx, filter)
			}
		})
	}
}

func BenchmarkGetKeyConfigurationByID(b *testing.B) {
	sizes := []struct {
		systems   int
		workflows int
	}{
		{10, 5},
		{100, 50},
		{500, 250},
	}

	for _, s := range sizes {
		name := fmt.Sprintf("systems=%d/workflows=%d", s.systems, s.workflows)

		b.Run(name+"/basic", func(b *testing.B) {
			mgr, ctx, ids := seedKeyConfigData(b, 1, s.systems, s.workflows)
			id := ids[0]
			b.ResetTimer()
			for range b.N {
				_, _ = mgr.GetKeyConfigurationByID(ctx, id, false)
			}
		})

		b.Run(name+"/extendedMetadata", func(b *testing.B) {
			mgr, ctx, ids := seedKeyConfigData(b, 1, s.systems, s.workflows)
			id := ids[0]
			b.ResetTimer()
			for range b.N {
				_, _ = mgr.GetKeyConfigurationByID(ctx, id, true)
			}
		})
	}
}
