# KMS20-6953: Make Managed Key Versions Count Configurable Per Landscape
## Updated Analysis - September 2026

**Last Analysis:** August 18, 2026  
**Re-analysis Date:** September 4, 2026  
**Reason:** Significant changes merged to main branch since original analysis

---

## Executive Summary

### ⚠️ Major Changes Since Last Analysis

Between August 18 and September 4, 2026, **significant architectural changes** were made to key version management that fundamentally impact this ticket's implementation:

1. **✅ NEW: GetKeyVersions Plugin API** (PR #377, commit deb86e51)
   - Providers can now return **multiple versions** in one call
   - New `UpdateVersions()` batch method in `KeyVersionManager`
   - **Status field** added to `KeyVersion` model
   - Migration 00017: `ALTER TABLE key_versions ADD COLUMN status VARCHAR(50) NOT NULL DEFAULT 'UNKNOWN'`

2. **✅ NEW: Pending Registration State** (PR #393, commit 89b4e0e9)
   - New `PENDING_REGISTRATION` key state for HYOK keys
   - New `Keys` config section with timeout settings
   - Changes to key lifecycle management

3. **Feature Flags for BYOK/HYOK** (PR #394, commit 62a371b8)
   - Feature flag system introduced

### Impact on Implementation

**GOOD NEWS:** The new architecture makes version limit enforcement **cleaner and more efficient**:

- ✅ **Batch operations** - `UpdateVersions()` processes all versions at once
- ✅ **Status tracking** - New `Status` field provides version state info
- ✅ **Provider versioning** - Plugins can report complete version history

**RECOMMENDATION:** Implement eviction logic in the **new `UpdateVersions()` path** rather than the old `CreateVersion()` approach.

---

## Ticket Context (Unchanged)

**Issue:** KMS20-6953  
**Type:** Improvement  
**Priority:** Medium  
**Story Points:** 5  
**Sprint:** Team A - Aug 11  
**Status:** Open  
**Assignee:** David Bolet  

### Requirements Summary

1. **Configuration:** Per landscape + per provider (AWS, GCP, FORTANIX)
2. **Default:** 5 versions per key
3. **Unlimited:** `-1` means no eviction
4. **Ordering:** Primary first, then n-1 non-primary by recency
5. **Eviction:** Remove oldest non-primary when limit exceeded
6. **Re-addition:** Restore previously evicted version if it becomes primary

---

## Updated Codebase Analysis

### 1. New KeyVersion Model (CHANGED)

**File:** `internal/model/keyversion.go`

```go
type KeyVersion struct {
    AutoTimeModel
    ID        uuid.UUID `gorm:"type:uuid;primaryKey"`
    NativeID  string    `gorm:"type:varchar(255);not null"`
    KeyID     uuid.UUID `gorm:"type:uuid;not null;index"`
    RotatedAt time.Time `gorm:"type:timestamptz;not null"`
    Status    string    `gorm:"type:varchar(50);not null;default:'UNKNOWN'"` // NEW!
}
```

**Migration:** `migrations/tenant/schema/00017_add_status_to_keyversions.sql`
```sql
-- +goose Up
ALTER TABLE key_versions ADD COLUMN status VARCHAR(50) NOT NULL DEFAULT 'UNKNOWN';
```

**Impact:** Status field can be used for filtering/validation during eviction

### 2. New Plugin API (MAJOR CHANGE)

**File:** `internal/pluginregistry/service/api/keymanagement/key_management.go`

```go
type KeyManagement interface {
    // NEW METHOD:
    GetKeyVersions(ctx context.Context, req *GetKeyVersionsRequest) (*GetKeyVersionsResponse, error)
    // ... existing methods ...
}

type KeyVersion struct {
    ID           string
    CreationTime *time.Time
    Status       string      // Version status from provider
}

type GetKeyVersionsRequest struct {
    Parameters RequestParameters
}

type GetKeyVersionsResponse struct {
    Versions []KeyVersion   // Multiple versions returned at once
}
```

**Key Change:** Providers now return **all versions** in one call, not just latest!

### 3. New KeyVersionManager.UpdateVersions() (MAJOR CHANGE)

**File:** `internal/manager/keyversion.go` (lines 162-200)

```go
func (kvm *KeyVersionManager) UpdateVersions(
    ctx context.Context,
    keyID uuid.UUID,
    versions []keymanagement.KeyVersion,
) error {
    for _, k := range versions {
        if k.CreationTime != nil {
            // Upsert: INSERT ON CONFLICT DO UPDATE
            err := kvm.repo.Set(ctx, &model.KeyVersion{
                ID:        uuid.New(),
                NativeID:  k.ID,
                KeyID:     keyID,
                RotatedAt: *k.CreationTime,
                Status:    k.Status,           // NEW: Status from provider
            }, *repo.NewQuery().
                OnConflict(repo.KeyIDField, repo.NativeIDField).
                Update(repo.RotatedField, repo.UpdatedField, repo.StatusField),
            )
            if err != nil {
                return err
            }
        } else {
            // Fallback for missing timestamp
            k.CreationTime = new(time.Now().UTC())
            err := kvm.repo.Create(ctx, &model.KeyVersion{
                ID:        uuid.New(),
                NativeID:  k.ID,
                KeyID:     keyID,
                RotatedAt: *k.CreationTime,
                Status:    k.Status,
            })
            if err != nil {
                return err
            }
        }
    }
    return nil
}
```

**Critical Insight:** This method uses **upsert logic** (`Set` with `OnConflict`) to handle:
- New versions → INSERT
- Existing versions → UPDATE (rotatedAt, status)
- No explicit version limit enforcement yet!

### 4. Updated Key Sync Flow (CHANGED)

**File:** `internal/manager/key.go`

**Old Flow (August):**
```
syncKeyVersion() → check if version changed → CreateVersion() for single new version
```

**New Flow (September):**
```
syncKeyVersions() → getKeyVersionsFromProvider() → handleNewKeyVersion() → UpdateVersions(all versions)
```

**Implementation:**

```go
// NEW: Batch sync all versions from provider
func (km *KeyManager) syncKeyVersions(
    ctx context.Context,
    provider *ProviderConfig,
    key *model.Key,
) error {
    keyResp, err := km.getKeyVersionsFromProvider(ctx, provider, key)
    if err != nil {
        return err
    }

    if len(keyResp.Versions) < 1 {
        return ErrNoKeyVersionsFound
    }

    return km.handleNewKeyVersion(ctx, key, keyResp)
}

// NEW: Call plugin's GetKeyVersions
func (km *KeyManager) getKeyVersionsFromProvider(
    ctx context.Context,
    provider *ProviderConfig,
    key *model.Key,
) (*keymanagement.GetKeyVersionsResponse, error) {
    configValues, err := mergeProviderConfigValuesWithKeyAccessData(provider, key)
    if err != nil {
        return nil, err
    }

    keyResp, err := provider.Client.GetKeyVersions(ctx, &keymanagement.GetKeyVersionsRequest{
        Parameters: keymanagement.RequestParameters{
            Config: common.KeystoreConfig{Values: configValues},
            KeyID:  *key.NativeID,
        },
    })
    if err != nil {
        return nil, errs.Wrap(ErrGetProviderKeyVersions, err)
    }

    return keyResp, nil
}

// NEW: Batch update versions
func (km *KeyManager) handleNewKeyVersion(
    ctx context.Context,
    key *model.Key,
    keyResp *keymanagement.GetKeyVersionsResponse,
) error {
    // Batch upsert all versions from provider
    err := km.keyVersionManager.UpdateVersions(
        ctx,
        key.ID,
        keyResp.Versions,
    )
    if err != nil {
        return err
    }

    // Audit logging and system notification...
    return nil
}
```

**Key Insight:** The system now processes **all provider versions** in one batch, not incrementally!

### 5. Configuration Structure (UNCHANGED)

**File:** `internal/config/config.go`

Landscape config is still the same:
```go
type Landscape struct {
    Name      string `yaml:"name"`
    UIBaseUrl string `yaml:"uiBaseUrl"`
    Region    string `yaml:"region"`
    // Need to add: MaxKeyVersions map[string]int
}
```

New Keys config was added (for different purpose):
```go
type Keys struct {
    PendingRegistrationTimeout time.Duration `yaml:"pendingRegistrationTimeout" default:"15m"`
    PendingCreationTimeout     time.Duration `yaml:"pendingCreationTimeout"     default:"15m"`
}
```

---

## Updated Implementation Strategy

### ⚠️ Critical Change: Where to Enforce Limits

**OLD PLAN (August):**
- Enforce in `CreateVersion()` after creating single version
- Simple, but doesn't align with new batch architecture

**NEW PLAN (September):**
- Enforce in `UpdateVersions()` after upserting all provider versions
- OR: Add new method `enforceVersionLimits()` called after `UpdateVersions()`

### Option A: Enforce Inside UpdateVersions() ⭐ RECOMMENDED

**Pros:**
- Single transaction - all versions updated + eviction happens atomically
- Aligns with batch processing model
- No extra DB round-trips

**Cons:**
- `UpdateVersions()` becomes more complex
- Config dependency (needs access to landscape config)

**Implementation:**

```go
func (kvm *KeyVersionManager) UpdateVersions(
    ctx context.Context,
    keyID uuid.UUID,
    versions []keymanagement.KeyVersion,
) error {
    // 1. Upsert all versions from provider (existing logic)
    for _, k := range versions {
        // ... existing upsert logic ...
    }

    // 2. Enforce version limits per provider
    if err := kvm.enforceVersionLimitForKey(ctx, keyID); err != nil {
        // Log warning but don't fail - versions were already saved
        log.Warn(ctx, "failed to enforce version limit", err)
    }

    return nil
}

func (kvm *KeyVersionManager) enforceVersionLimitForKey(
    ctx context.Context,
    keyID uuid.UUID,
) error {
    // 1. Get key to determine provider
    key := &model.Key{ID: keyID}
    found, err := kvm.repo.First(ctx, key, *repo.NewQuery())
    if err != nil || !found {
        return errs.Wrap(ErrGetKeyDB, err)
    }

    // 2. Get max versions for this provider from landscape config
    maxVersions := kvm.getMaxVersionsForProvider(key.Provider)
    if maxVersions == -1 {
        return nil // Unlimited
    }

    // 3. Get all current versions ordered by RotatedAt DESC
    allVersions, _, err := kvm.GetKeyVersions(
        ctx,
        keyID,
        repo.Pagination{Limit: 1000, Offset: 0}, // High limit to get all
    )
    if err != nil {
        return err
    }

    // 4. If count <= limit, nothing to do
    if len(allVersions) <= maxVersions {
        return nil
    }

    // 5. Keep first `maxVersions` (most recent), delete the rest
    //    Primary = most recent (first in DESC order)
    versionsToDelete := allVersions[maxVersions:]
    
    for _, v := range versionsToDelete {
        if err := kvm.repo.Delete(ctx, v, *repo.NewQuery()); err != nil {
            log.Error(ctx, "failed to delete old version", err,
                slog.String("versionId", v.ID.String()),
                slog.String("nativeId", v.NativeID))
            // Continue deleting others
        }
    }

    return nil
}

func (kvm *KeyVersionManager) getMaxVersionsForProvider(provider string) int {
    // TODO: Load from landscape config
    // For now, return default
    defaults := map[string]int{
        "AWS":      5,
        "GCP":      5,
        "FORTANIX": 5,
    }
    
    if limit, ok := defaults[provider]; ok {
        return limit
    }
    return 5 // Fallback default
}
```

### Option B: Separate Enforcement Method

**Alternative:** Keep `UpdateVersions()` clean, call eviction separately:

```go
// In key.go handleNewKeyVersion()
func (km *KeyManager) handleNewKeyVersion(...) error {
    // Upsert versions
    err := km.keyVersionManager.UpdateVersions(ctx, key.ID, keyResp.Versions)
    if err != nil {
        return err
    }

    // Enforce limits (separate call)
    if err := km.keyVersionManager.EnforceVersionLimit(ctx, key.ID, key.Provider); err != nil {
        log.Warn(ctx, "failed to enforce version limit", err)
    }

    // ... rest of logic ...
}
```

**Verdict:** Option A is cleaner - keeps enforcement logic in KeyVersionManager

---

## Updated Task Breakdown

### Task #2: Design Configuration Schema ✅ UNCHANGED

Still need:
```go
type Landscape struct {
    Name           string         `yaml:"name"`
    UIBaseUrl      string         `yaml:"uiBaseUrl"`
    Region         string         `yaml:"region"`
    MaxKeyVersions map[string]int `yaml:"maxKeyVersions"` // NEW
}
```

YAML:
```yaml
landscape:
  name: dev
  maxKeyVersions:
    AWS: 10
    GCP: 5
    FORTANIX: -1
```

### Task #3: Implement Configuration Loading ✅ READY

**Changes:**
1. Add `MaxKeyVersions` to `Landscape` struct
2. Add helper method:
   ```go
   func (l *Landscape) GetMaxVersionsForProvider(provider string) int {
       if l.MaxKeyVersions == nil {
           return 5 // Default
       }
       if limit, ok := l.MaxKeyVersions[provider]; ok {
           return limit
       }
       return 5 // Provider not in map → default
   }
   ```
3. Wire config to `KeyVersionManager` (pass via constructor or global access)

### Task #4: Implement Version Eviction Logic ⚠️ UPDATED

**Updated Location:** `internal/manager/keyversion.go`

**New Implementation:**

```go
// Add to KeyVersionManager
type KeyVersionManager struct {
    ProviderConfigManager
    cmkAuditor *auditor.Auditor
    landscape  *config.Landscape // NEW: Need landscape config access
}

// Update UpdateVersions to enforce limits
func (kvm *KeyVersionManager) UpdateVersions(
    ctx context.Context,
    keyID uuid.UUID,
    versions []keymanagement.KeyVersion,
) error {
    // Existing upsert logic...
    for _, k := range versions {
        // ... unchanged ...
    }

    // NEW: Enforce version limits after upserting
    if err := kvm.enforceVersionLimitForKey(ctx, keyID); err != nil {
        log.Warn(ctx, "failed to enforce version limit", err)
        // Don't fail - versions were already saved
    }

    return nil
}

// NEW METHOD
func (kvm *KeyVersionManager) enforceVersionLimitForKey(
    ctx context.Context,
    keyID uuid.UUID,
) error {
    // 1. Get key to find provider
    key := &model.Key{ID: keyID}
    found, err := kvm.repo.First(ctx, key, *repo.NewQuery())
    if err != nil || !found {
        return fmt.Errorf("failed to get key: %w", err)
    }

    // 2. Get limit for provider
    maxVersions := kvm.landscape.GetMaxVersionsForProvider(key.Provider)
    if maxVersions == -1 {
        return nil // Unlimited
    }

    // 3. Get current version count
    allVersions, _, err := kvm.GetKeyVersions(
        ctx,
        keyID,
        repo.Pagination{Limit: 10000, Offset: 0},
    )
    if err != nil {
        return fmt.Errorf("failed to get versions: %w", err)
    }

    // 4. Check if eviction needed
    if len(allVersions) <= maxVersions {
        return nil // Within limit
    }

    // 5. Delete oldest versions (beyond limit)
    // Versions are already ordered by RotatedAt DESC, CreatedAt DESC
    // Keep first N (most recent), delete the rest
    versionsToDelete := allVersions[maxVersions:]

    log.Info(ctx, "Evicting old key versions",
        slog.String("keyId", keyID.String()),
        slog.Int("currentCount", len(allVersions)),
        slog.Int("limit", maxVersions),
        slog.Int("toDelete", len(versionsToDelete)))

    for _, v := range versionsToDelete {
        if err := kvm.repo.Delete(ctx, v, *repo.NewQuery()); err != nil {
            log.Error(ctx, "failed to delete version", err,
                slog.String("versionId", v.ID.String()),
                slog.String("nativeId", v.NativeID))
            // Continue with others
        }
    }

    return nil
}
```

**Key Design Decisions:**

1. **Non-failing eviction:** Log warning if eviction fails, don't rollback upserts
2. **Order preservation:** Rely on existing `GetKeyVersions()` ordering (RotatedAt DESC)
3. **Primary is safe:** Most recent version (first in list) is never deleted
4. **Batch processing:** Aligns with new multi-version sync approach

### Task #5: Handle Re-addition ✅ STILL WORKS

**Good News:** The new `UpdateVersions()` already handles this!

**Scenario:**
1. Version X is evicted (deleted from DB)
2. Version X becomes primary in provider again
3. `GetKeyVersions()` from provider includes X
4. `UpdateVersions()` uses **upsert** logic (`Set` with `OnConflict`)
5. Version X is **re-inserted** automatically

**No additional code needed!** ✅

### Task #6: Unit Tests ⚠️ UPDATED

**New Test Cases:**

1. **UpdateVersions with eviction:**
   ```go
   func TestKeyVersionManager_UpdateVersions_EvictsOldVersions(t *testing.T) {
       // Setup: landscape config with limit=3, provider=AWS
       // Create key with provider=AWS
       // Call UpdateVersions with 5 versions
       // Assert: Only 3 most recent versions remain
   }
   ```

2. **Per-provider limits:**
   ```go
   func TestKeyVersionManager_EnforceVersionLimit_PerProvider(t *testing.T) {
       // AWS key with limit=5 → keep 5
       // GCP key with limit=3 → keep 3
       // FORTANIX key with limit=-1 → keep all
   }
   ```

3. **Re-addition via upsert:**
   ```go
   func TestKeyVersionManager_UpdateVersions_RestoresEvictedVersion(t *testing.T) {
       // Create 5 versions, limit=3
       // Version 1,2 get evicted
       // Call UpdateVersions with versions [5,4,3,2] (2 is back!)
       // Assert: Version 2 is re-inserted
   }
   ```

4. **Unlimited (-1):**
   ```go
   func TestKeyVersionManager_EnforceVersionLimit_Unlimited(t *testing.T) {
       // Config: provider limit = -1
       // Create 20 versions
       // Assert: All 20 remain
   }
   ```

5. **Status field:**
   ```go
   func TestKeyVersionManager_UpdateVersions_UpdatesStatus(t *testing.T) {
       // Create version with status=ENABLED
       // Update with same nativeID, status=DISABLED
       // Assert: Status updated via upsert
   }
   ```

### Task #7: Update API Responses ✅ UNCHANGED

No changes needed - `GetKeyVersions()` API already queries DB, evicted versions won't appear.

### Task #8: Integration Tests ⚠️ UPDATED

**New Scenarios:**

1. **Multi-provider batch sync:**
   ```
   - Create AWS, GCP, FORTANIX keys
   - Configure limits: AWS=5, GCP=3, FORTANIX=-1
   - Mock providers to return 10 versions each
   - Call syncKeyVersions() for each key
   - Verify: AWS has 5, GCP has 3, FORTANIX has 10
   ```

2. **Status tracking:**
   ```
   - Provider returns versions with different statuses
   - Verify statuses persisted correctly
   - Update status, verify upsert works
   ```

3. **Batch upsert correctness:**
   ```
   - Initial sync: 5 versions
   - Second sync: 7 versions (2 new, 5 updated timestamps)
   - Verify: 7 versions total, timestamps updated, limit enforced
   ```

### Task #9: Documentation ✅ READY

**Additional Documentation Needed:**

1. **New Batch Sync Behavior:**
   - Document that `GetKeyVersions` retrieves all versions from provider
   - Explain upsert behavior (updates timestamps/status)
   - Clarify version limit applies after sync

2. **Status Field:**
   - Document status values (provider-dependent)
   - Explain status is informational, doesn't affect eviction

3. **Migration Note:**
   - Document DB migration 00017 (status column)
   - Note: existing installations need migration run

---

## Prerequisites Re-assessment

### ✅ Still Met

1. **Database schema:** Updated with status field (migration 00017) ✅
2. **Version management:** Enhanced with batch operations ✅
3. **Provider identification:** Still works via Key.Provider ✅
4. **Configuration infrastructure:** Unchanged ✅

### ⚠️ New Dependencies

1. **Plugin Support:**
   - **Requirement:** Plugins must implement `GetKeyVersions()` method
   - **Status:** 
     - ✅ AWS plugin: Likely implemented (check)
     - ⚠️ GCP plugin: KMS20-6484 still in progress
     - ✅ FORTANIX plugin: Check implementation status
   - **Action:** Verify plugin compatibility before deployment

2. **Config Access:**
   - `KeyVersionManager` needs access to `Landscape` config
   - Need to pass via constructor or use config singleton
   - **Action:** Design config injection pattern

3. **Migration Deployment:**
   - Migration 00017 must run before new code deploys
   - **Action:** Coordinate with deployment process

---

## Risk Re-assessment

### 🔴 New Risks

1. **Plugin Compatibility (HIGH):**
   - If plugins don't implement `GetKeyVersions()`, sync fails
   - **Mitigation:** 
     - Check plugin implementations before merging
     - Add fallback to old `GetKey()` single-version path?
     - Document plugin requirements

2. **Batch Performance (MEDIUM):**
   - Providers might return 100+ versions for old keys
   - Upserting all versions could be slow
   - **Mitigation:**
     - Add pagination to `GetKeyVersions` API?
     - Document expected version counts
     - Monitor sync performance

3. **Status Field Usage (LOW):**
   - New status field not yet fully utilized
   - Could conflict with eviction logic if status-based filtering added later
   - **Mitigation:** Document status is informational only for now

### 🟡 Existing Risks (Still Valid)

1. **GCP Dependency (MEDIUM):**
   - KMS20-6484 still in progress
   - **Mitigation:** Test with AWS first, add GCP config when ready

2. **Kernel Service Impact (MEDIUM):**
   - Still need to verify Kernel Service doesn't break with evicted versions
   - **Mitigation:** Test with Kernel Service team

### 🟢 Reduced Risks

1. **Re-addition Complexity (RESOLVED):**
   - Upsert logic handles automatically ✅
   - No additional code needed ✅

---

## Updated Open Questions

### New Questions

1. **Plugin Implementation Status:**
   - Do AWS, GCP, FORTANIX plugins all implement `GetKeyVersions()`?
   - What version counts do they typically return?
   - Any performance concerns with large version lists?

2. **Config Injection:**
   - How should `KeyVersionManager` access `Landscape` config?
   - Constructor parameter? Config singleton? Context value?

3. **Status Field Semantics:**
   - What status values are returned by each provider?
   - Should eviction filter by status (e.g., keep ENABLED versions longer)?
   - Or is status purely informational?

4. **Backward Compatibility:**
   - Can we deploy without migration 00017 running first?
   - What happens if plugin doesn't implement `GetKeyVersions()`?

### Original Questions (Still Relevant)

1. **Kernel Service:** Does it cache versions or query CMK? (STILL UNRESOLVED)
2. **Existing Keys:** Evict immediately or on next rotation? (Recommend: next rotation)
3. **Minimum Limit:** Validate `limit >= 1`? (Recommend: yes)
4. **Config Hot Reload:** Requires restart? (Likely yes, confirm)

---

## Revised Implementation Plan

### Phase 1: Core Implementation (3-4 days)

1. ✅ **Config Schema** (0.5 days)
   - Add `MaxKeyVersions map[string]int` to `Landscape`
   - Add `GetMaxVersionsForProvider()` helper
   - Update config examples

2. ✅ **Eviction Logic** (2 days)
   - Implement `enforceVersionLimitForKey()` in KeyVersionManager
   - Update `UpdateVersions()` to call enforcement
   - Wire landscape config to KeyVersionManager
   - Handle edge cases (no versions, exactly at limit, etc.)

3. ✅ **Unit Tests** (1 day)
   - Test eviction with various limits
   - Test per-provider independence
   - Test re-addition via upsert
   - Test unlimited (-1) behavior

### Phase 2: Integration & Testing (2-3 days)

4. ✅ **Plugin Verification** (0.5 days)
   - Confirm AWS plugin implements `GetKeyVersions()`
   - Check version counts returned
   - Verify performance acceptable

5. ✅ **Integration Tests** (1 day)
   - Multi-provider scenario
   - Batch sync correctness
   - Status field handling

6. ✅ **Kernel Service Testing** (1 day)
   - Coordinate with Kernel Service team
   - Test with evicted versions
   - Verify decryption still works

### Phase 3: Documentation & Deployment (1 day)

7. ✅ **Documentation** (0.5 days)
   - Config reference
   - Batch sync behavior
   - Migration notes

8. ✅ **Deployment Prep** (0.5 days)
   - Verify migration 00017 in deployment pipeline
   - Coordinate rollout with GCP HYOK readiness
   - Prepare monitoring/alerts

**Total Estimate:** 6-8 days (vs. original 5-day estimate)

**Confidence:** High - architecture changes actually simplify implementation

---

## Acceptance Criteria Validation (Updated)

| Criteria | Status | Notes |
|----------|--------|-------|
| Max versions configurable per landscape & provider | ✅ Ready | Config map in Landscape struct |
| Independent provider limits | ✅ Ready | Map key = provider name |
| Default = 5 if not configured | ✅ Ready | Helper method with default |
| -1 = unlimited | ✅ Ready | Check in enforcement logic |
| Primary first, then n-1 non-primary by recency | ✅ Implemented | GetKeyVersions ordering unchanged |
| Evict oldest non-primary when exceeded | ⚠️ Changed | Now in UpdateVersions, not CreateVersion |
| Re-add previously removed primary | ✅ Works | Upsert logic handles automatically |

**New Consideration:** Status field could enable smarter eviction (e.g., keep ENABLED versions preferentially), but out of scope for now.

---

## Summary of Changes Since August

| Aspect | August Analysis | September Reality | Impact |
|--------|----------------|-------------------|---------|
| **Version Sync** | Single version, incremental | Batch all versions from provider | 🟢 Cleaner enforcement |
| **Eviction Point** | After `CreateVersion()` | After `UpdateVersions()` | 🟢 Better fit for batch |
| **Re-addition** | Complex, needed logic | Automatic via upsert | 🟢 Simpler! |
| **Status Tracking** | Not considered | New field added | 🟡 Future extensibility |
| **Plugin API** | GetKey only | GetKeyVersions added | 🟢 Enables batch sync |
| **Risk Profile** | Low-medium | Medium (plugin dependency) | 🟡 Need verification |

**Overall:** Architecture changes are **favorable** - implementation is actually **cleaner** than originally planned!

---

## Next Actions

### Immediate (This Week)

1. ✅ **Confirm with team:**
   - Plugin implementation status for AWS, GCP, FORTANIX
   - Kernel Service version caching strategy
   - Config injection pattern preference

2. ✅ **Start implementation:**
   - Branch: `feature/KMS20-6953-version-limits`
   - Begin with Task #3 (config loading)
   - Mock landscape config for unit tests

### Short-term (Next Sprint)

3. ✅ **Core development:**
   - Implement `enforceVersionLimitForKey()`
   - Update `UpdateVersions()` 
   - Unit test coverage

4. ✅ **Integration testing:**
   - Test with real plugin (AWS)
   - Coordinate Kernel Service test

### Before Deployment

5. ✅ **Verification checklist:**
   - [ ] Migration 00017 deployed to all environments
   - [ ] Plugin compatibility confirmed
   - [ ] Kernel Service tested
   - [ ] Performance benchmarked (100+ version keys)
   - [ ] Documentation complete
   - [ ] Monitoring/alerts configured

---

## Conclusion

**The good news:** Recent architecture changes make this implementation **cleaner and more maintainable** than the August plan.

**The challenge:** New dependency on `GetKeyVersions()` plugin API requires verification across all providers.

**Confidence level:** **High** (85%) - implementation path is clear, main risk is plugin compatibility.

**Recommendation:** **Proceed with implementation**, starting with config loading and AWS plugin verification in parallel.

---

**Analysis by:** David Bolet  
**Date:** September 4, 2026  
**Supersedes:** KMS20-6953-ANALYSIS.md (August 18, 2026)
