package apiregistry

import (
	"errors"
	"testing"

	"github.com/openkcm/common-sdk/pkg/commoncfg"

	tenantapi "github.com/openkcm/cmk/internal/apiregistry/api/tenant"
	"github.com/openkcm/cmk/internal/config"
)

func TestNew(t *testing.T) {
	t.Run("Successful creation with all services enabled", func(t *testing.T) {
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

		reg, err := New(services)
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
	})

	t.Run("Successful creation with only registry enabled", func(t *testing.T) {
		services := &config.Services{
			Registry: &commoncfg.GRPCClient{
				Enabled: true,
				Address: ":8080",
			},
			SessionManager: &commoncfg.GRPCClient{
				Enabled: false,
			},
		}

		reg, err := New(services)
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
	})

	t.Run("Successful creation with only session manager enabled", func(t *testing.T) {
		services := &config.Services{
			Registry: &commoncfg.GRPCClient{
				Enabled: false,
			},
			SessionManager: &commoncfg.GRPCClient{
				Enabled: true,
				Address: ":8081",
			},
		}

		reg, err := New(services)
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
	})

	t.Run("Error on invalid registry endpoint", func(t *testing.T) {
		services := &config.Services{
			Registry: &commoncfg.GRPCClient{
				Enabled: true,
				Address: ":///",
			},
			SessionManager: &commoncfg.GRPCClient{
				Enabled: false,
			},
		}

		svc, err := New(services)
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
	})
}

func TestRegistryGetters(t *testing.T) {
	t.Run("Getters return error when services are not registered", func(t *testing.T) {
		reg := &registryStruct{} // Empty registry

		if _, err := reg.Tenant(); !errors.Is(err, ErrNotRegistered) {
			t.Errorf("Tenant() error = %v, wantErr %v", err, ErrNotRegistered)
		}
		if _, err := reg.System(); !errors.Is(err, ErrNotRegistered) {
			t.Errorf("System() error = %v, wantErr %v", err, ErrNotRegistered)
		}
		if _, err := reg.Mapping(); !errors.Is(err, ErrNotRegistered) {
			t.Errorf("Mapping() error = %v, wantErr %v", err, ErrNotRegistered)
		}
		if _, err := reg.OIDCMapping(); !errors.Is(err, ErrNotRegistered) {
			t.Errorf("OIDCMapping() error = %v, wantErr %v", err, ErrNotRegistered)
		}
	})
}
