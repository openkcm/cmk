# Internal Role Policy — Test Design and Coverage

> **Maintenance note:** This document must be updated whenever a role is added,
> removed, or its policy changes. Keeping it current is part of the definition of
> done for any internal role work.

This document explains the design and current status of authz policy tests for all
roles defined in `repo_internal_policies.go`, organised by the application entry
point (`cmd/`) that injects each role at runtime.

## Coverage Summary

| Entry point | Coverage |
|---|---|
| [cmd/task-worker](#cmdtask-worker) | Partial ¹ |
| [cmd/task-worker and cmd/task-scheduler](#cmdtask-worker-and-cmdtask-scheduler) | Complete |
| [cmd/tenant-manager and cmd/operator](#cmdtenant-manager-and-cmdoperator) | Complete |
| [cmd/tenant-manager-cli](#cmdtenant-manager-cli) | Complete |
| [cmd/event-reconciler](#cmdevent-reconciler) | Partial ³ ⁴ |
| [cmd/api-server](#cmdapi-server) | Partial ² |

¹ Some permissions within each role are not exercised — tests use empty batches or early exits to avoid plugin dependencies. See the Tested column in each role table for details.
² Count on KeyConfiguration (`UserManager.CheckKeyConfigManagedByIAMGroups`) is not exercised — the test only drives the `NeedsGroupFiltering`/`GetRoleFromIAM` path. See the Tested column.
³ Only the `KeyTaskInfoResolver.ResolveTasks` path is tested (Key:First, System:Count+List, Tenant:First). Key:List, Key:Update, System:First, and System:Update are exercised by system-action and key-detach handlers which require live plugin targets and are not covered by the current unit test. Event:Update and Event:Delete (used by `updateEventError` and `cleanUpEvent`) are covered by a dedicated sub-test.

## Design

Each test wires real managers through a real `AuthzRepo` backed by a test DB
(via `testutils.NewTestDB`). No managers are mocked. The role is injected via
`cmkcontext.InjectInternalUserData` inside `ProcessTask` (or the equivalent entry
point) exactly as it is in production. The test asserts:

- No error is returned (or, where `ProcessTask` is specified to propagate errors for
  retry, that any error is not an authz error).
- No error-level log lines are produced.

Where a task processes data in batches, the test seeds only enough data to exercise
the authz-checked repo operations and then exit cleanly (empty batch, early return,
etc.) without needing to invoke external plugins.

---

## cmd/task-worker

The task worker processes async tasks enqueued by the scheduler. Each task handler
injects its own role via `cmkcontext.InjectInternalUserData` at the start of
`ProcessTask`.

### `InternalTaskCertRotationRole`

| Permission | Resource | Required by | Tested |
|---|---|---|---|
| Count, List | Certificate | `CertificateManager.RotateExpiredCertificates` | ✓ |
| First, Create, Update | Certificate | `CertificateManager.RotateExpiredCertificates` | – |

**Test:** `internal/authz/policy_tests/cert_rotation_test.go`
`TestCertRotation_AuthzPolicy/InternalTaskCertRotationRole_allows_Count_and_List_on_Certificate`

No certificates matching the rotation predicate (`AutoRotate=true AND ExpirationDate <
threshold`) are seeded. `RotateExpiredCertificates` calls `ProcessInBatch` →
Count+List on Certificate → empty batch → clean exit. Count, List, First, Create,
and Update are all declared in the policy; the test confirms Count and List are
permitted at the entry point without requiring a real cert-issuer call.

---

### `InternalTaskHYOKSyncRole`

| Permission | Resource | Required by | Tested |
|---|---|---|---|
| Count, List | Key | `KeyManager.SyncHYOKKeys` | ✓ |
| Update | Key | `KeyManager.SyncHYOKKeys` | – |
| First, Count, Create, Update | Certificate | `KeyManager.SyncHYOKKeys` → `GetOrInitProvider` → `getDefaultHYOKClientCert` | ✓ |

**Test:** `internal/authz/policy_tests/hyok_sync_test.go`
`TestHYOKSync_AuthzPolicy/InternalTaskHYOKSyncRole_allows_full_HYOK_sync_path_including_Certificate_access`

A HYOK key and a tenant-default certificate are seeded. `SyncHYOKKeys` calls `ProcessInBatch` → Count+List on Key → finds the HYOK key → calls `GetOrInitProvider` → `getDefaultHYOKClientCert` which exercises First, Count, and Create on Certificate. `Update` on Key is not reached because the mock plugin does not return a new native ID that would trigger a key state change.

---

### `InternalTaskKeystorePoolRole`

| Permission | Resource | Required by | Tested |
|---|---|---|---|
| Count | Keystore | `ProviderConfigManager.FillKeystorePool` | ✓ |
| Create | Keystore | `ProviderConfigManager.FillKeystorePool` | – |

**Test:** `internal/authz/policy_tests/keystore_pool_test.go`
`TestKeystorePool_AuthzPolicy/InternalTaskKeystorePoolRole_allows_Count_on_Keystore`

`KeystorePool.Size` is set to `0`. `FillKeystorePool` calls `Pool.Count` on
Keystore then immediately returns — no Create is attempted. This keeps the test free
of keystore plugin dependencies while confirming Count is permitted.

---

### `InternalTaskSystemRefreshRole`

| Permission | Resource | Required by | Tested |
|---|---|---|---|
| Count, List | System | `SystemInformationManager.UpdateSystems` | ✓ |
| Count, List | KeyConfiguration | `SystemInformationManager.UpdateSystems` | – |
| Count, List | Event | `SystemInformationManager.UpdateSystems` | – |
| Count, List | SystemProperty | `SystemInformationManager.UpdateSystems` | – |
| Update | SystemProperty | `SystemInformationManager.UpdateSystems` | – |

**Test:** `internal/authz/policy_tests/system_refresh_test.go`
`TestSystemRefresh_AuthzPolicy/InternalTaskSystemRefreshRole_allows_Count_and_List_on_System`

No systems are seeded. `UpdateSystems` calls `ProcessInBatch` → Count+List on
System → empty batch → clean exit.

---

### `InternalTaskTenantRefreshRole`

| Permission | Resource | Required by | Tested |
|---|---|---|---|
| Update | Tenant | `TenantNameRefresher.ProcessTask` | ✓ |

**Test:** `internal/authz/policy_tests/tenant_name_refresh_test.go`
`TestTenantNameRefresh_AuthzPolicy/InternalTaskTenantRefreshRole_allows_Update_on_Tenant`

A tenant with an empty `Name` is seeded (matching the task's `WHERE name = ''`
filter). A fake registry client returns a tenant name so the authz-guarded `Patch`
call is reached and exercised.

---

### `InternalTaskWorkflowApproversRole`

| Permission | Resource | Required by | Tested |
|---|---|---|---|
| First | Tenant | `WorkflowProcessor.ProcessTask` | ✓ |
| First | TenantConfig | `WorkflowProcessor.ProcessTask` | ✓ |
| First | Workflow | `WorkflowManager.AutoAssignApprovers` | ✓ |
| Update | Workflow | `WorkflowManager.AutoAssignApprovers` | – |
| Create, Delete, Count, List | WorkflowApprover | `WorkflowManager.AutoAssignApprovers` | – |
| First | Key | `getKeyConfigFromKey` | ✓ |
| Update | Key | `getKeyConfigFromKey` | – |
| First | KeyVersion | `getKeyConfigFromKey` | ✓ |
| First | KeyConfiguration | `getKeyConfigurationsFromArtifact` | ✓ |
| List, Count | KeyConfiguration | `getKeyConfigurationsFromArtifact` | – |
| First | System | `getKeyConfigurationsFromArtifact` | ✓ |
| Count, List | System | `getKeyConfigurationsFromArtifact` | – |
| Count, List | SystemProperty | `getKeyConfigurationsFromArtifact` | – |
| First | Event | `getKeyConfigurationsFromArtifact` | ✓ |
| List, Count | Event | `getKeyConfigurationsFromArtifact` | – |
| Count, List | Group | `getApproversAndGroupsFromKeyConfigs` | – |

**Test:** `internal/authz/policy_tests/workflow_autoassign_test.go`
`TestWorkflowAutoAssign_AuthzPolicy/InternalTaskWorkflowApproversRole_allows_First_on_Workflow,_Key,_KeyConfiguration`

A group, key configuration, key, and workflow (with the key as artifact) are seeded.
`ProcessTask` finds the workflow and calls `AutoAssignApprovers`, which traverses the
full dependency chain: Workflow → Key → KeyVersion → KeyConfiguration → System →
group resolution. Execution fails at group resolution because that path calls into
business user context (`ExtractBusinessUserDataAuthContext`) which is not available
in a pure internal context. The test asserts that an error is returned, that no
`"allowed":false` authz denial appears in the logs, and that the error message does
not contain "unauthorized" — confirming all repo access up to that point was
permitted by the policy.

---

### `InternalTaskWorkflowCleanupRole`

| Permission | Resource | Required by | Tested |
|---|---|---|---|
| First | TenantConfig | `WorkflowManager.CleanupTerminalWorkflows` → `GetWorkflowConfig` | ✓ |
| Delete, Create | TenantConfig | `SetWorkflowConfig` → `repo.Set` (upsert when no config exists) | ✓ |
| First | Tenant | `SetWorkflowConfig` → `repo.GetTenant` (to pick default config) | ✓ |
| Count, List | Workflow | `WorkflowManager.CleanupTerminalWorkflows` | ✓ |
| Delete | Workflow | `WorkflowManager.CleanupTerminalWorkflows` | – |

**Test:** `internal/authz/policy_tests/workflow_cleanup_test.go`
`TestWorkflowCleanup_AuthzPolicy/InternalTaskWorkflowCleanupRole_allows_Count,_List,_Delete_on_Workflow`

---

### `InternalTaskWorkflowExpirationRole`

| Permission | Resource | Required by | Tested |
|---|---|---|---|
| Count, List | Workflow | `WorkflowManager.GetWorkflows` | ✓ |
| First, Update | Workflow | `WorkflowManager.GetWorkflows` | – |
| Update | System | `WorkflowManager.handleTerminalWorkflow` | – |

**Test:** `internal/authz/policy_tests/workflow_expiry_test.go`
`TestWorkflowExpiry_AuthzPolicy/InternalTaskWorkflowExpirationRole_allows_Count_and_List_on_Workflow`

No workflows are seeded. `GetWorkflows` calls Count+List on Workflow → empty result
→ clean exit without entering the per-workflow expiry loop. `NeedsGroupFiltering`
returns `false, nil` immediately for this role, so the internal context does not need
business user data.

---

## cmd/task-worker and cmd/task-scheduler

`InternalTaskProcessingRole` is injected by both the batch processor (used by the
task worker to iterate tenants) and the fanout (used by the scheduler to fan work
out across tenants).

### `InternalTaskProcessingRole`

| Permission | Resource | Required by | Tested |
|---|---|---|---|
| Count, List | Tenant | `BatchProcessor.ProcessTenantsInBatch` | ✓ |

**Test:** `internal/authz/policy_tests/batch_processor_test.go`
`TestBatchProcessor_AuthzPolicy/InternalTaskProcessingRole_allows_Count_and_List_on_Tenant`

No tenants are seeded beyond the default test tenant. `ProcessTenantsInBatch`
injects `InternalTaskProcessingRole` then calls `ProcessInBatch` → Count+List on
Tenant → empty batch → clean exit.

---

## cmd/tenant-manager and cmd/operator

The tenant manager and operator handle tenant lifecycle events (create, apply auth,
block, unblock, terminate) received via AMQP.

### `InternalTenantProvisioningRole`

| Permission | Resource | Required by | Tested |
|---|---|---|---|
| First | Group | `TenantProbe.Check` (probe.groupExists) | ✓ |
| Create | Group | `TenantOperator.createTenantGroups` | ✓ |
| Create | Tenant | `TenantManager.CreateTenant` (handleCreateTenant) | ✓ |
| Update | Tenant | `TenantOperator.applyOIDC` (handleApplyTenantAuth) | ✓ |
| Delete | Tenant | `TenantManager.DeleteTenant` (handleTerminateTenant) | ✓ |
| Count, List | System | `TenantManager.sendUnlinkForConnectedSystems`, `checkAllSystemsUnlinked`, `unmapAllSystemsFromRegistry` | ✓ |
| First | System | `TenantManager.sendUnlinkForConnectedSystems` → `UnlinkSystemAction` (when systems are still linked) | ✓ |
| Count, List | Key | `TenantManager.detachPrimaryKeys`, `checkAllPrimaryKeysProcessed`, `checkAllPrimaryKeysDetached` | ✓ |
| Update | Key | `TenantManager.detachPrimaryKeys` → `KeyManager.Detach` → `repo.Patch` (when primary keys are not yet detached) | ✓ |
| First | KeyConfiguration | `TenantManager.sendUnlinkForConnectedSystems` → `UnlinkSystemAction` → `repo.First(keyConfig)` | ✓ |

**Test:** `internal/operator/authz_test.go`
`TestTenantProvisioning_AuthzPolicy`

Two sub-tests drive the two database-touching handlers directly via
`orbital.ExecuteHandler` (no AMQP):

- `handleCreateTenant`:
  Calls `HandleCreateTenant` with a new tenant ID. `probe.Check` performs First on
  Group (none exist → not found, clean). `CreateTenant` creates the schema
  (Create on Tenant). `CreateDefaultGroups` creates the admin/auditor groups
  (Create on Group). Handler returns PROCESSING (waiting for registry confirmation).

- `handleApplyTenantAuth`:
  Calls `HandleApplyTenantAuth` with a seeded tenant ID and a fake session manager
  that returns success. `applyOIDC` performs Patch (Update) on Tenant and calls
  the session manager. Handler returns DONE.

The `handleTerminateTenant` permissions are covered by the manager-level tests
in `internal/manager/tenant_test.go`.

---

## cmd/tenant-manager-cli

### `InternalTenantCLIRole`

| Permission | Resource | Required by | Tested |
|---|---|---|---|
| List, First, Create, Delete, Update | Tenant | `TenantManager` (all CRUD ops) | ✓ |
| Create | Group | `GroupManager.CreateGroup` | ✓ |

**Test:** `cmd/tenant-manager-cli/cli_test.go` (`TestCLISuite`)

Wires real `authzRepo` and injects `InternalTenantCLIRole` in
`SetupSuite` and in each helper that calls `TenantManager` or `GroupManager`
directly. The suite exercises `CreateTenant` (Create on Tenant), `ListTenants`
(List on Tenant), `GetTenant` (First on Tenant), `UpdateTenant` (Update on Tenant),
`DeleteTenant` (Delete on Tenant), and `CreateDefaultGroups` (Create on Group) —
covering every permission in the policy.

---

## cmd/event-reconciler

### `InternalEventReconcilerRole`

| Permission | Resource | Required by | Tested |
|---|---|---|---|
| First | Tenant | `KeyTaskInfoResolver.getTaskInfo`, `SystemTaskInfoResolver.loadTenantAndSystem` | ✓ |
| First, List | Key | `KeyTaskInfoResolver.getRegionsByKeyID`, `KeyDetachJobHandler`, system handlers | ✓ / – |
| Update | Key | `KeyDetachJobHandler.terminate` → `updateKey` | – |
| First, Count, List | System | `KeyTaskInfoResolver.getRegionsByKeyID`, `SystemTaskInfoResolver.loadTenantAndSystem` | ✓ / – |
| Update | System | system event handlers → `updateSystem` | – |
| First | Certificate | `CryptoAccessDataSyncer.getRoleManagementCert` | ✓ |
| First | TenantConfig | `CryptoAccessDataSyncer.getDefaultKeystoreConfig` | ✓ |
| Delete, Create | TenantConfig | `CryptoAccessDataSyncer.setDefaultKeystoreConfig` (via `repo.Set`) | ✓ |
| Update | Event | `updateEventError` → `r.Patch` on Event | ✓ |
| Delete | Event | `cleanUpEvent` → `r.Delete` on Event | ✓ |

**Test:** `internal/authz/policy_tests/event_reconciler_test.go`
`TestEventReconciler_AuthzPolicy/InternalEventReconcilerRole_allows_deriving_connected_regions_for_key`
`TestEventReconciler_AuthzPolicy/InternalEventReconcilerRole_allows_CryptoAccessDataSyncer_to_read_TenantConfig_and_Certificate`
`TestEventReconciler_AuthzPolicy/InternalEventReconcilerRole_allows_CryptoAccessDataSyncer_Certificate:First_and_TenantConfig:Set`
`TestEventReconciler_AuthzPolicy/InternalEventReconcilerRole_allows_Event:Update_and_Event:Delete`

A key configuration, a HYOK key, and a CONNECTED system sharing the same `KeyConfigurationID` are seeded. `ResolveTasks` for `JobTypeKeyEnable` calls `getTenantByID` (Tenant:First), then `getRegionsByKeyID` which performs Key:First then `ProcessInBatch` → Count+List on System. Because no target region is configured for the seeded system's region, the test exits with `ErrNoConnectedRegionsForKey` — confirming authz passes through the entire resolver path. Key:List (used by system-action resolvers), Key:Update, System:First, and System:Update (used by system and key-detach job handlers) require live plugin targets and are not covered.

Two additional sub-tests cover `CryptoAccessDataSyncer`:

- **TenantConfig:First (read path):** A DEFAULT_KEYSTORE `TenantConfig` is seeded with a crypto cert entry whose subject already matches what the syncer computes for the test tenant. `SyncAndGetCryptoAccessData` under `InternalEventReconcilerRole` reads the config (TenantConfig:First), finds the cert is up-to-date, and returns without invoking the plugin or writing back. Confirms First on TenantConfig is permitted.

- **Certificate:First and TenantConfig:Set (grant-trust path):** A DEFAULT_KEYSTORE `TenantConfig` with no existing crypto entries and a role-management `Certificate` are seeded. `SyncAndGetCryptoAccessData` reads the config (TenantConfig:First), fetches the role-management cert (Certificate:First), calls `GrantTrust` on the `TestKeystoreManagement` plugin, then writes the updated config back (TenantConfig:Set = Delete+Create). Confirms all three operations are permitted by the policy.

---

## cmd/api-server

`InternalBusinessAuthzRole` is not a top-level entry point role. It is injected
mid-execution by `UserManager.BusinessToInternalContext` during business user
authorisation checks, allowing `UserManager` to look up group and key-config data
without carrying the original business user's credentials.

### `InternalBusinessAuthzRole`

| Permission | Resource | Required by | Tested |
|---|---|---|---|
| Count, List | Group | `UserManager.NeedsGroupFiltering`, `UserManager.GetRoleFromIAM` | ✓ |
| Count | KeyConfiguration | `UserManager.CheckKeyConfigManagedByIAMGroups` | – |

**Test:** `internal/authz/policy_tests/business_authz_test.go`
`TestBusinessAuthz_AuthzPolicy/InternalBusinessAuthzRole_allows_Count_and_List_on_Group`

A business user context carrying an IAM group identifier is constructed via
`InjectBusinessUserData`. No groups matching that identifier are seeded.
`NeedsGroupFiltering` calls `BusinessToInternalContext` (switching to
`InternalBusinessAuthzRole`) then `repo.Count` on Group and `repo.List` on Group
via `GetRoleFromIAM`. Both return zero results cleanly. The test asserts no
`"allowed":false` denial appears in the logs.
