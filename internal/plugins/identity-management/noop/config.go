package noop

import "github.com/openkcm/common-sdk/pkg/commoncfg"

type Config struct {
	StaticJsonContent commoncfg.SourceRef `yaml:"staticJsonContent"`
}

type StaticIdentityManagement struct {
	Groups []Group `json:"groups"`
}

type Group struct {
	ID    string `json:"id,omitempty"`
	Name  string `json:"name,omitempty"`
	Users []User `json:"users,omitempty"`
}

type User struct {
	ID    string `json:"id,omitempty"`
	Name  string `json:"name,omitempty"`
	Email string `json:"email,omitempty"`
}
