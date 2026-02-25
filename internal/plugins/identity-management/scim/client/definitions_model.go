package client

//nolint:tagliatelle
type BaseResource struct {
	ID         string   `json:"id"`
	ExternalID string   `json:"externalId,omitempty"`
	Meta       struct{} `json:"meta"`
	Schemas    []string `json:"schemas,omitempty"`
}

type MultiValuedAttribute struct {
	Primary bool   `json:"primary,omitempty"`
	Display string `json:"display,omitempty"`
	Value   string `json:"value"`
}

type User struct {
	BaseResource

	UserName    string                 `json:"userName"`
	Name        struct{}               `json:"name"`
	DisplayName string                 `json:"displayName,omitempty"`
	Active      bool                   `json:"active"`
	Emails      []MultiValuedAttribute `json:"emails"`
	Groups      []MultiValuedAttribute `json:"groups"`
	UserType    string                 `json:"userType,omitempty"`
}

type Group struct {
	BaseResource

	DisplayName string                 `json:"displayName,omitempty"`
	Members     []MultiValuedAttribute `json:"members,omitempty"`
}

//nolint:tagliatelle
type UserList struct {
	Resources []User `json:"Resources"`
}

//nolint:tagliatelle
type GroupList struct {
	Resources []Group `json:"Resources"`
}

type SearchRequest struct {
	Schemas []string `json:"schemas"`
	Filter  *string  `json:"filter,omitempty"`
	Count   *int     `json:"count,omitempty"`
	Cursor  *string  `json:"cursor,omitempty"`
}
