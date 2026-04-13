package apiregistry_test

import (
	"testing"

	"github.com/openkcm/common-sdk/pkg/commoncfg"

	"github.com/openkcm/cmk/internal/apiregistry"
	tenantapi "github.com/openkcm/cmk/internal/apiregistry/api/tenant"
	"github.com/openkcm/cmk/internal/config"
)

func TestNew_AllServicesEnabled(t *testing.T) {
	services := &config.Services{
		Registry: &commoncfg.GRPCClient{
			Enabled: true,
			Address: ":8080",
		},
		SessionManager: &commoncfg.GRPCClient{
			Enabled: true,
			Address: ":8081",
		},
	}

	reg, err := apiregistry.New(services)
	if err != nil {
		t.Fatalf("New() error = %v, wantErr nil", err)
	}
	if reg == nil {
		t.Fatal("New() returned nil registry")
	}

	if _, err := reg.Tenant(); err != nil {
		t.Error("Tenant() should not return an error")
	}
	if _, err := reg.System(); err != nil {
		t.Error("System() should not return an error")
	}
	if _, err := reg.Mapping(); err != nil {
		t.Error("Mapping() should not return an error")
	}
	if _, err := reg.OIDCMapping(); err != nil {
		t.Error("OIDCMapping() should not return an error")
	}
}

func TestNew_OnlyRegistryEnabled(t *testing.T) {
	services := &config.Services{
		Registry: &commoncfg.GRPCClient{
			Enabled: true,
			Address: ":8080",
		},
		SessionManager: &commoncfg.GRPCClient{
			Enabled: false,
		},
	}

	reg, err := apiregistry.New(services)
	if err != nil {
		t.Fatalf("New() error = %v, wantErr nil", err)
	}
	if reg == nil {
		t.Fatal("New() returned nil registry")
	}

	if _, err := reg.Tenant(); err != nil {
		t.Error("Tenant() should not return an error")
	}
	if _, err := reg.System(); err != nil {
		t.Error("System() should not return an error")
	}
	if _, err := reg.Mapping(); err != nil {
		t.Error("Mapping() should not return an error")
	}
	if _, err := reg.OIDCMapping(); err == nil {
		t.Error("OIDCMapping() should return an error")
	}
}

func TestNew_OnlySessionManagerEnabled(t *testing.T) {
	services := &config.Services{
		Registry: &commoncfg.GRPCClient{
			Enabled: false,
		},
		SessionManager: &commoncfg.GRPCClient{
			Enabled: true,
			Address: ":8081",
		},
	}

	reg, err := apiregistry.New(services)
	if err != nil {
		t.Fatalf("New() error = %v, wantErr nil", err)
	}
	if reg == nil {
		t.Fatal("New() returned nil registry")
	}

	if _, err := reg.Tenant(); err == nil {
		t.Error("Tenant() should return an error")
	}
	if _, err := reg.System(); err == nil {
		t.Error("System() should return an error")
	}
	if _, err := reg.Mapping(); err == nil {
		t.Error("Mapping() should return an error")
	}
	if _, err := reg.OIDCMapping(); err != nil {
		t.Error("OIDCMapping() should not return an error")
	}
}

func TestNew_InvalidRegistryEndpoint(t *testing.T) {
	services := &config.Services{
		Registry: &commoncfg.GRPCClient{
			Enabled: true,
			Address: ":///",
		},
		SessionManager: &commoncfg.GRPCClient{
			Enabled: false,
		},
	}

	svc, err := apiregistry.New(services)
	if err != nil {
		t.Fatalf("New() error = %v, wantErr nil", err)
	}

	tenant, err := svc.Tenant()
	if err != nil {
		t.Fatalf("Tenant() should not return an error")
	}

	_, err = tenant.SetTenantLabels(t.Context(), &tenantapi.SetTenantLabelsRequest{
		ID:     "test-tenant",
		Labels: map[string]string{"env": "test"},
	})
	if err == nil {
		t.Error("New() should have returned an error for invalid URL")
	}
}

func TestRegistryGetters(t *testing.T) {
	// Create a registry with all services disabled
	services := &config.Services{
		Registry: &commoncfg.GRPCClient{
			Enabled: false,
		},
		SessionManager: &commoncfg.GRPCClient{
			Enabled: false,
		},
	}

	reg, err := apiregistry.New(services)
	if err != nil {
		t.Fatalf("New() error = %v, wantErr nil", err)
	}

	if _, err := reg.Tenant(); err == nil {
		t.Error("Tenant() should return an error when registry is not enabled")
	}
	if _, err := reg.System(); err == nil {
		t.Error("System() should return an error when registry is not enabled")
	}
	if _, err := reg.Mapping(); err == nil {
		t.Error("Mapping() should return an error when registry is not enabled")
	}
	if _, err := reg.OIDCMapping(); err == nil {
		t.Error("OIDCMapping() should return an error when session manager is not enabled")
	}
}
