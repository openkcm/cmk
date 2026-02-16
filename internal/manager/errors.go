package manager

import (
	"errors"

	"github.com/openkcm/cmk/internal/errs"
)

var (
	ErrIncompatibleQueryField = errors.New("incompatible query field")

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
	ErrConvertAccessData       = errors.New("failed to convert access data")

	ErrGetTags      = errors.New("failed getting tags")
	ErrDeletingTags = errors.New("failed to delete tags")

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
	ErrManagementDetailsUpdate          = errors.New("management credentials cannot be updated")
	ErrCryptoDetailsUpdate              = errors.New("crypto credentials cannot be updated")
	ErrCryptoRegionNotExists            = errors.New("crypto region does not exist")
	ErrNonEditableCryptoRegionUpdate    = errors.New("crypto region cant be updated as it's not editable")
	ErrBadCryptoRegionData              = errors.New("crypto region data invalid")
	ErrEditableCryptoRegionField        = errors.New("editable crypto region field has to be boolean")
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
	ErrUnsupportedSystemAction          = errors.New("system action not supported")
	ErrKeyNotAssignedToKeyConfiguration = errors.New("key not assigned to key configuration")
	ErrUpdateKeyVersionDisabled         = errors.New("cannot update key version when key is disabled")
	ErrUpdateSystemNoRegClient          = errors.New("system cannot be updated since no registry client")
	ErrLinkSystemProcessingOrFailed     = errors.New("system cannot be linked in PROCESSING/FAILED state")
	ErrUnlinkSystemProcessing           = errors.New("system cannot be unlinked in PROCESSING state")
	ErrRetryNonFailedSystem             = errors.New("system can action only be retried on failed state")

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

	ErrListTenants      = errors.New("failed to list tenants from database")
	ErrGetTenantInfo    = errors.New("failed to get tenant info")
	ErrTenantNotAllowed = errors.New("user has no permission to access tenant")

	ErrListGroups            = errors.New("failed to list groups from database")
	ErrGetGroups             = errors.New("failed to get group from database")
	ErrCreateGroups          = errors.New("failed to create group from database")
	ErrUpdateGroups          = errors.New("failed to update group from database")
	ErrDeleteGroups          = errors.New("failed to delete group from database")
	ErrInvalidGroupUpdate    = errors.New("group cannot be updated")
	ErrInvalidGroupDelete    = errors.New("group cannot be deleted")
	ErrMultipleRolesInGroups = errors.New("users with multiple roles are not allowed")
	ErrZeroRolesInGroups     = errors.New("users without any roles are not allowed")

	ErrCheckIAMExistenceOfGroups = errors.New("failed to check IAM existence of groups")
	ErrCheckTenantHasIAMGroups   = errors.New("failed to check tenant has IAM groups")

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

	ErrQuerySystemList           = errors.New("failed to query system list")
	ErrGettingSystem             = errors.New("failed to get system")
	ErrCreatingSystem            = errors.New("failed to create system")
	ErrGettingSystemByID         = errors.New("failed to get system by ID")
	ErrGettingSystemLinkByID     = errors.New("failed to get system link by ID")
	ErrConnectSystemNoPrimaryKey = errors.New("system cannot be connect without an enabled primary key")
	ErrUpdateSystem              = errors.New("failed to update system")
	ErrSystemNotLinked           = errors.New("system is not linked to a key configuration")
	ErrFailedToReencryptSystem   = errors.New("system reencrypt failed on new key")
	ErrNotAllSystemsConnected    = errors.New("keyconfig contains systems not connected")

	ErrGetWorkflowDB        = errors.New("failed to get workflow")
	ErrOngoingWorkflowExist = errors.New("ongoing workflow for artifact already exists")
	ErrCreateWorkflowDB     = errors.New("failed to create workflow")
	ErrCheckWorkflow        = errors.New("failed to check workflow")
	ErrCheckOngoingWorkflow = errors.New("failed to check ongoing workflow for artifact")
	ErrValidateActor        = errors.New("failed to validate actor for workflow transition")
	ErrAddApproversDB       = errors.New("failed to add approvers to workflow")
	ErrAddApproverGroupsDB  = errors.New("failed to add approver groups to workflow")
	ErrApplyTransition      = errors.New("failed to apply transition to workflow")
	ErrInDBTransaction      = errors.New(
		"error when executing sequence of operations in a transaction",
	)
	ErrWorkflowCannotTransitionDB = errors.New("workflow cannot transition to specified state")
	ErrUpdateApproverDecision     = errors.New("failed to update approver decision")
	ErrGetKeyConfigFromArtifact   = errors.New("failed to get key configuration from artifact")
	ErrAutoAssignApprover         = errors.New("failed to auto assign approver")
	ErrCreateApproverAssignTask   = errors.New("failed to create auto approver assignment task")

	ErrLoadIdentityManagementPlugin = errors.New("failed to load identity management plugin")

	ErrTenantNotExist = errors.New("tenantID does not exist")
	ErrEmptyTenantID  = errors.New("tenantID cannot be empty")

	ErrPoolIsDrained               = errors.New("pool is drained")
	ErrCouldNotSaveConfiguration   = errors.New("could not save configuration")
	ErrCouldNotRemoveConfiguration = errors.New("could not remove configuration")
	ErrOnboardingInProgress        = errors.New("another onboarding is already in progress")
	ErrCreatingGroups              = errors.New("creating user groups for existing tenant")
	ErrInvalidGroupType            = errors.New("invalid group type")

	ErrSchemaNameLength = errors.New("schema name length must be between 3 and 63 characters")
	ErrCreatingTenant   = errors.New("creating tenant failed")
	ErrValidatingTenant = errors.New("tenant validation failed")
	ErrInvalidSchema    = errors.New("invalid schema name pattern")

	ErrGroupRole = errors.New("unsupported role for group creation")
)

const (
	GRPCErrorCodeHYOKAuthFailed errs.GRPCErrorCode = "HYOK_AUTH_FAILED"
)

// Predefined GRPC errors

var ErrGRPCHYOKAuthFailed = errs.GRPCError{
	Code:        GRPCErrorCodeHYOKAuthFailed,
	BaseMessage: "failed to authenticate with the keystore provider",
}
