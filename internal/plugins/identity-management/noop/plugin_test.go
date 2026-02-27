package noop

import (
	"context"
	"os"
	"reflect"
	"testing"

	"github.com/hashicorp/go-hclog"
	"github.com/openkcm/plugin-sdk/pkg/hclog2slog"

	idmangv1 "github.com/openkcm/plugin-sdk/proto/plugin/identity_management/v1"
	configv1 "github.com/openkcm/plugin-sdk/proto/service/common/config/v1"
)

const testStaticConfig = `
groups:
  - id: "group-1"
    name: "group1"
    users:
      - id: "user-1"
        name: "user1"
        email: "user1@example.com"
  - id: "group-2"
    name: "group2"
    users:
      - id: "user-2"
        name: "user2"
        email: "user2@example.com"
`

func setupPlugin(t *testing.T) *Plugin {
	t.Helper()
	p := NewPlugin()
	p.SetLogger(hclog.NewNullLogger())

	tmpFile, err := os.CreateTemp("", "static-config-*.yaml")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	t.Cleanup(func() { os.Remove(tmpFile.Name()) })

	if _, err := tmpFile.WriteString(testStaticConfig); err != nil {
		t.Fatalf("Failed to write to temp file: %v", err)
	}
	if err := tmpFile.Close(); err != nil {
		t.Fatalf("Failed to close temp file: %v", err)
	}

	configYAML := `
staticJsonContent:
  source: file
  file:
    path: "` + tmpFile.Name() + `"`

	req := &configv1.ConfigureRequest{
		YamlConfiguration: configYAML,
	}

	if _, err = p.Configure(context.Background(), req); err != nil {
		t.Fatalf("Failed to configure plugin: %v", err)
	}

	return p
}

func TestNewPlugin(t *testing.T) {
	p := NewPlugin()
	if p == nil {
		t.Fatal("NewPlugin() returned nil")
	}
	if p.buildInfo != "{}" {
		t.Errorf("Expected buildInfo to be '{}', got '%s'", p.buildInfo)
	}
}

func TestSetLogger(t *testing.T) {
	p := NewPlugin()
	logger := hclog.NewNullLogger()
	p.SetLogger(logger)
	expectedLogger := hclog2slog.New(logger)
	if !reflect.DeepEqual(p.logger, expectedLogger) {
		t.Errorf("Expected logger to be %v, got %v", expectedLogger, p.logger)
	}
}

func TestConfigure(t *testing.T) {
	p := NewPlugin()

	t.Run("Invalid YAML", func(t *testing.T) {
		req := &configv1.ConfigureRequest{
			YamlConfiguration: "invalid-yaml",
		}
		_, err := p.Configure(context.Background(), req)
		if err == nil {
			t.Error("Expected an error for invalid YAML, but got nil")
		}
	})

	t.Run("Invalid static config path", func(t *testing.T) {
		req := &configv1.ConfigureRequest{
			YamlConfiguration: "staticJsonContent: { filePath: \"/non-existent-file\" }",
		}
		_, err := p.Configure(context.Background(), req)
		if err == nil {
			t.Error("Expected an error for non-existent file, but got nil")
		}
	})
}

func TestGetGroup(t *testing.T) {
	p := setupPlugin(t)

	t.Run("Group found", func(t *testing.T) {
		req := &idmangv1.GetGroupRequest{GroupName: "group1"}
		resp, err := p.GetGroup(context.Background(), req)
		if err != nil {
			t.Fatalf("GetGroup failed: %v", err)
		}
		if resp.Group == nil {
			t.Fatal("Expected group, but got nil")
		}
		if resp.Group.Id != "group-1" {
			t.Errorf("Expected group ID 'group-1', got '%s'", resp.Group.Id)
		}
		if resp.Group.Name != "group1" {
			t.Errorf("Expected group name 'group1', got '%s'", resp.Group.Name)
		}
	})

	t.Run("Group not found", func(t *testing.T) {
		req := &idmangv1.GetGroupRequest{GroupName: "non-existent-group"}
		resp, err := p.GetGroup(context.Background(), req)
		if err != nil {
			t.Fatalf("GetGroup failed: %v", err)
		}
		if resp.Group != nil {
			t.Errorf("Expected nil group, but got %v", resp.Group)
		}
	})
}

func TestGetAllGroups(t *testing.T) {
	p := setupPlugin(t)

	req := &idmangv1.GetAllGroupsRequest{}
	resp, err := p.GetAllGroups(context.Background(), req)
	if err != nil {
		t.Fatalf("GetAllGroups failed: %v", err)
	}
	if len(resp.Groups) != 2 {
		t.Errorf("Expected 2 groups, got %d", len(resp.Groups))
	}
}

func TestGetUsersForGroup(t *testing.T) {
	p := setupPlugin(t)

	t.Run("Group found", func(t *testing.T) {
		req := &idmangv1.GetUsersForGroupRequest{GroupId: "group-1"}
		resp, err := p.GetUsersForGroup(context.Background(), req)
		if err != nil {
			t.Fatalf("GetUsersForGroup failed: %v", err)
		}
		if len(resp.Users) != 1 {
			t.Fatalf("Expected 1 user, got %d", len(resp.Users))
		}
		if resp.Users[0].Id != "user-1" {
			t.Errorf("Expected user ID 'user-1', got '%s'", resp.Users[0].Id)
		}
	})

	t.Run("Group not found", func(t *testing.T) {
		req := &idmangv1.GetUsersForGroupRequest{GroupId: "non-existent-group"}
		resp, err := p.GetUsersForGroup(context.Background(), req)
		if err != nil {
			t.Fatalf("GetUsersForGroup failed: %v", err)
		}
		if len(resp.Users) != 0 {
			t.Errorf("Expected 0 users, got %d", len(resp.Users))
		}
	})
}

func TestGetGroupsForUser(t *testing.T) {
	p := setupPlugin(t)

	t.Run("User found", func(t *testing.T) {
		req := &idmangv1.GetGroupsForUserRequest{UserId: "user-1"}
		resp, err := p.GetGroupsForUser(context.Background(), req)
		if err != nil {
			t.Fatalf("GetGroupsForUser failed: %v", err)
		}
		if len(resp.Groups) != 1 {
			t.Fatalf("Expected 1 group, got %d", len(resp.Groups))
		}
		if resp.Groups[0].Id != "group-1" {
			t.Errorf("Expected group ID 'group-1', got '%s'", resp.Groups[0].Id)
		}
	})

	t.Run("User not found", func(t *testing.T) {
		req := &idmangv1.GetGroupsForUserRequest{UserId: "non-existent-user"}
		resp, err := p.GetGroupsForUser(context.Background(), req)
		if err != nil {
			t.Fatalf("GetGroupsForUser failed: %v", err)
		}
		if len(resp.Groups) != 0 {
			t.Errorf("Expected 0 groups, got %d", len(resp.Groups))
		}
	})
}
