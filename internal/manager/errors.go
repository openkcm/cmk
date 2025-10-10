package manager

import (
	"errors"
)

var (
	ErrLoadCryptoCerts         = errors.New("failed to load crypto certs")
	ErrUnmarshalCryptoCerts    = errors.New("failed to unmarshal crypto certs")
	ErrSetCryptoCerts          = errors.New("failed to set crypto certs")
	ErrPluginNotFound          = errors.New("plugin not found")
	ErrConfigNotFound          = errors.New("config not found")
	ErrKeyCreationFailed       = errors.New("failed to create key in provider")
	ErrKeyRegistration         = errors.New("failed to register key from provider")
	ErrUnsupportedKeyAlgorithm = errors.New("unsupported key algorithm")
	ErrInvalidKeyState         = errors.New("invalid key state")
	ErrHYOKKeyActionNotAllowed = errors.New("HYOK key action not allowed")
	ErrNameCannotBeEmpty       = errors.New("name field cannot be empty")
	ErrEventSendingFailed      = errors.New("failed to send event")
	ErrHYOKProviderKeyNotFound = errors.New("HYOK provider key not found")

	ErrCreateKeyConfiguration       = errors.New("failed to create key configuration")
	ErrConnectedSystemToKeyConfig   = errors.New("system is connected to keyconfig")
	ErrInvalidKeyAdminGroup         = errors.New("invalid keyconfig admin group")
	ErrDeleteKeyConfiguration       = errors.New("failed to delete key configuration")
	ErrQueryKeyConfigurationList    = errors.New("failed to query key configuration list")
	ErrGettingKeyConfigByID         = errors.New("failed to get key configuration by ID")
	ErrKeyConfigurationNotFound     = errors.New("KeyConfiguration not found")
	ErrKeyConfigurationIDNotFound   = errors.New("KeyConfigurationID not found")
	ErrFailedToInitProvider         = errors.New("failed to init provider")
	ErrFailedToEnableProviderKey    = errors.New("failed to enable provider key")
	ErrFailedToDisableProviderKey   = errors.New("failed to disable provider key")
	ErrFailedToDeleteProvider       = errors.New("failed to delete provider")
	ErrGetProviderKey               = errors.New("failed to get provider key")
	ErrGetImportParamsFromProvider  = errors.New("failed to get import parameters from provider")
	ErrImportKeyMaterialsToProvider = errors.New("failed to import key materials to provider")
	ErrKeyIsNotEnabled              = errors.New("key is not enabled")
	ErrPrimaryKeyUnmark             = errors.New("primary key cannot be unmarked primary")

	ErrGetKeyDB                         = errors.New("failed to get key from database")
	ErrGettingKeyByID                   = errors.New("failed to get key by ID")
	ErrListKeysDB                       = errors.New("failed to list keys from database")
	ErrUpdateKeyDB                      = errors.New("failed to update key in database")
	ErrCreateKeyDB                      = errors.New("failed to create key in database")
	ErrDeleteKeyDB                      = errors.New("failed to delete key from database")
	ErrSetImportParamsDB                = errors.New("failed to set import parameters in database")
	ErrDeleteImportParamsDB             = errors.New("failed to delete import parameters from database")
	ErrUpdateKeyConfiguration           = errors.New("failed to update key configuration")
	ErrUpdateKeyConfigurationDB         = errors.New("failed to update key configuration in database")
	ErrGetConfiguration                 = errors.New("failed to get configuration")
	ErrUpdatePrimary                    = errors.New("failed to update key primary state")
	ErrGetHYOKKeyInfoDB                 = errors.New("failed to get HYOK key info from database")
	ErrInvalidKeyTypeForHYOKSync        = errors.New("invalid key type for hyok sync")
	ErrListHYOKKeysDB                   = errors.New("failed to list hyok keys")
	ErrDeleteKey                        = errors.New("failed to delete key")
	ErrUpdatingTotalKeys                = errors.New("failed to update total keys")
	ErrUpdatingTotalSystems             = errors.New("failed to update total systems")
	ErrKeyNotAssignedToKeyConfiguration = errors.New("key not assigned to key configuration")
	ErrUpdateKeyVersionDisabled         = errors.New("cannot update key version when key is disabled")
	ErrUpdateSystemNoRegClient          = errors.New("system cannot be updated since no registry client")
	ErrLinkSystemProcessingOrFailed     = errors.New("System cannot be linked in PROCESSING/FAILED state")
	ErrUnlinkSystemProcessingOrFailed   = errors.New("System cannot be unlinked in PROCESSING/FAILED state")

	ErrRotateBYOKKey                       = errors.New("byok key must not be rotated")
	ErrUnsupportedBYOKProvider             = errors.New("unsupported BYOK provider")
	ErrBuildImportParams                   = errors.New("error building import parameters")
	ErrMarshalProviderParams               = errors.New("error marshaling provider parameters")
	ErrExtractCommonImportFields           = errors.New("error extracting common import fields")
	ErrInvalidKeyTypeForImportParams       = errors.New("invalid key type for import parameters")
	ErrInvalidKeyStateForImportParams      = errors.New("invalid key state for import parameters")
	ErrInvalidKeyTypeForImportKeyMaterial  = errors.New("invalid key type for import key materials")
	ErrInvalidKeyStateForImportKeyMaterial = errors.New("invalid key state for import key materials")
	ErrInvalidBYOKAction                   = errors.New("invalid BYOK action")
	ErrEmptyKeyMaterial                    = errors.New("key material cannot be empty")
	ErrInvalidBase64KeyMaterial            = errors.New("key material must be base64 encoded")
	ErrMissingOrExpiredImportParams        = errors.New("import parameters missing or expired")

	ErrGetKeyVersionDB         = errors.New("failed to get key version from database")
	ErrGetPrimaryKeyVersionDB  = errors.New("failed to get primary key version from database")
	ErrListKeyVersionsDB       = errors.New("failed to list key versions from database")
	ErrUpdateKeyVersionDB      = errors.New("failed to update key version in database")
	ErrCreateKeyVersionDB      = errors.New("failed to create key version in database")
	ErrInvalidKeyVersionNumber = errors.New("invalid key version number")

	ErrListTenants = errors.New("failed to list tenants from database")

	ErrListGroups         = errors.New("failed to list groups from database")
	ErrGetGroups          = errors.New("failed to get group from database")
	ErrCreateGroups       = errors.New("failed to create group from database")
	ErrUpdateGroups       = errors.New("failed to update group from database")
	ErrDeleteGroups       = errors.New("failed to delete group from database")
	ErrInvalidGroupRename = errors.New("group cannot be renamed")
	ErrInvalidGroupDelete = errors.New("group cannot be deleted")

	ErrNoBodyForCustomerHeldDB = errors.New(
		"body must be provided for customer held key rotation",
	)
	ErrBodyForNoCustomerHeldDB = errors.New(
		"body must be provided only for customer held key rotation",
	)

	ErrQueryLabelList    = errors.New("failed to query system list")
	ErrFetchLabel        = errors.New("failed to fetch label")
	ErrUpdateLabelDB     = errors.New("failed to update label")
	ErrInsertLabel       = errors.New("failed to insert label")
	ErrDeleteLabelDB     = errors.New("failed to delete label")
	ErrGetKeyIDDB        = errors.New("KeyID is required")
	ErrEmptyInputLabelDB = errors.New("invalid input empty label name")

	ErrQuerySystemList       = errors.New("failed to query system list")
	ErrGettingSystem         = errors.New("failed to get system")
	ErrCreatingSystem        = errors.New("failed to create system")
	ErrGettingSystemByID     = errors.New("failed to get system by ID")
	ErrGettingSystemLinkByID = errors.New("failed to get system link by ID")
	ErrAddSystemNoPrimaryKey = errors.New("system cannot be added without an enabled primary key")
	ErrUpdateSystem          = errors.New("failed to update system")
	ErrSettingKeyClaim       = errors.New("error setting key claim for system")
	ErrSystemNotLinked       = errors.New("system is not linked to a key configuration")

	ErrGetWorkflowDB        = errors.New("failed to get workflow")
	ErrOngoingWorkflowExist = errors.New("ongoing workflow for artifact already exists")
	ErrCreateWorkflowDB     = errors.New("failed to create workflow")
	ErrCheckOngoingWorkflow = errors.New("failed to check ongoing workflow for artifact")
	ErrWorkflowNotInitial   = errors.New("workflow is not in initial state")
	ErrValidateActor        = errors.New("failed to validate actor for workflow transition")
	ErrAddApproversDB       = errors.New("failed to add approvers to workflow")
	ErrApplyTransition      = errors.New("failed to apply transition to workflow")
	ErrInDBTransaction      = errors.New(
		"error when executing sequence of operations in a transaction",
	)
	ErrWorkflowCannotTransitionDB = errors.New("workflow cannot transition to specified state")
	ErrUpdateApproverDecision     = errors.New("failed to update approver decision")

	ErrLoadAuthzAllowList = errors.New("failed to load authz allow list for tenantID")
	ErrTenantNotExist     = errors.New("tenantID does not exist")
	ErrEmptyTenantID      = errors.New("tenantID cannot be empty")

	ErrPoolIsDrained               = errors.New("pool is drained")
	ErrCouldNotSaveConfiguration   = errors.New("could not save configuration")
	ErrCouldNotRemoveConfiguration = errors.New("could not remove configuration")
)

// HYOKAuthFailedError indicates a failure to authenticate with the keystore provider
// and holds a reason for the failure which can be extracted.
type HYOKAuthFailedError struct {
	Reason string
}

func (e HYOKAuthFailedError) Error() string {
	return "failed to authenticate with the keystore provider: " + e.Reason
}

func (e HYOKAuthFailedError) Is(target error) bool {
	if target == nil {
		return false
	}

	return errors.As(target, &HYOKAuthFailedError{})
}

func (e HYOKAuthFailedError) As(target any) bool {
	if target == nil {
		return false
	}

	_, ok := target.(*HYOKAuthFailedError)

	return ok
}

func NewHYOKAuthFailedError(reason string) error {
	return HYOKAuthFailedError{Reason: reason}
}
