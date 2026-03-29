package authapi

type AuthStatus string

const (
	AuthStatusUnspecified     AuthStatus = "UNSPECIFIED"
	AuthStatusApplying        AuthStatus = "APPLYING"
	AuthStatusApplyingError   AuthStatus = "APPLYING_ERROR"
	AuthStatusApplied         AuthStatus = "APPLIED"
	AuthStatusRemoving        AuthStatus = "REMOVING"
	AuthStatusRemovingError   AuthStatus = "REMOVING_ERROR"
	AuthStatusRemoved         AuthStatus = "REMOVED"
	AuthStatusBlocking        AuthStatus = "BLOCKING"
	AuthStatusBlockingError   AuthStatus = "BLOCKING_ERROR"
	AuthStatusBlocked         AuthStatus = "BLOCKED"
	AuthStatusUnblocking      AuthStatus = "UNBLOCKING"
	AuthStatusUnblockingError AuthStatus = "UNBLOCKING_ERROR"
)

type AuthInfo struct {
	ExternalID   string
	TenantID     string
	Type         string
	Properties   map[string]string
	Status       AuthStatus
	ErrorMessage string
	UpdatedAt    string
	CreatedAt    string
}

type ApplyAuthRequest struct {
	ExternalID string
	TenantID   string
	Type       string
	Properties map[string]string
}

type ApplyAuthResponse struct {
	Success bool
}

type GetAuthRequest struct {
	ExternalID string
}

type GetAuthResponse struct {
	Auth *AuthInfo
}

type ListAuthsRequest struct {
	TenantID      string
	Limit         int32
	NextPageToken string
}

type ListAuthsResponse struct {
	Auths         []*AuthInfo
	NextPageToken string
}

type RemoveAuthRequest struct {
	ExternalID string
}

type RemoveAuthResponse struct {
	Success bool
}
