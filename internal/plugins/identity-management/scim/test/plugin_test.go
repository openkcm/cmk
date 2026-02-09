package testplugin_test

import (
	"context"
	"log/slog"
	"testing"

	"github.com/magodo/slog2hclog"
	"github.com/stretchr/testify/assert"

	idmangv1 "github.com/openkcm/plugin-sdk/proto/plugin/identity_management/v1"

	tp "github.com/openkcm/cmk/internal/plugins/identity-management/scim/test"
)

func setupTest() *tp.TestPlugin {
	p := tp.NewTestPlugin()
	logLevelPlugin := new(slog.LevelVar)
	logLevelPlugin.Set(slog.LevelError)

	p.SetLogger(slog2hclog.New(slog.Default(), logLevelPlugin))

	return p
}

func TestGetGroup(t *testing.T) {
	p := setupTest()

	responseMsg, err := p.GetGroup(context.Background(),
		&idmangv1.GetGroupRequest{})
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	assert.Equal(
		t,
		&idmangv1.GetGroupResponse{},
		responseMsg,
	)
}

func TestGetAllGroups(t *testing.T) {
	p := setupTest()

	responseMsg, err := p.GetAllGroups(context.Background(),
		&idmangv1.GetAllGroupsRequest{})
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	assert.Equal(
		t,
		&idmangv1.GetAllGroupsResponse{},
		responseMsg,
	)
}

func TestGetUsersForGroup(t *testing.T) {
	p := setupTest()

	responseMsg, err := p.GetUsersForGroup(context.Background(),
		&idmangv1.GetUsersForGroupRequest{})
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	assert.Equal(
		t,
		&idmangv1.GetUsersForGroupResponse{},
		responseMsg,
	)
}

func TestGetGroupsForUser(t *testing.T) {
	p := setupTest()

	responseMsg, err := p.GetGroupsForUser(context.Background(),
		&idmangv1.GetGroupsForUserRequest{})
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	assert.Equal(
		t,
		&idmangv1.GetGroupsForUserResponse{},
		responseMsg,
	)
}

func TestNewTestPlugin(t *testing.T) {
	p := setupTest()
	assert.NotNil(t, p)
}
