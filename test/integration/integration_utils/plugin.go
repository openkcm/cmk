package integrationutils

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/openkcm/common-sdk/pkg/commoncfg"
	"github.com/openkcm/plugin-sdk/pkg/catalog"
	"github.com/stretchr/testify/assert"
	"gopkg.in/yaml.v3"

	certificate_issuerv1 "github.com/openkcm/plugin-sdk/proto/plugin/certificate_issuer/v1"
	idmangv1 "github.com/openkcm/plugin-sdk/proto/plugin/identity_management/v1"
	keystoremanv1 "github.com/openkcm/plugin-sdk/proto/plugin/keystore/management/v1"
	keystoreopv1 "github.com/openkcm/plugin-sdk/proto/plugin/keystore/operations/v1"
	notificationv1 "github.com/openkcm/plugin-sdk/proto/plugin/notification/v1"
	systeminformationv1 "github.com/openkcm/plugin-sdk/proto/plugin/systeminformation/v1"
)

const (
	PKIUAAConfigPath             = "../../env/secret/cert-issuer-plugins/uaa.json"
	PKIServiceConfigPath         = "../../env/secret/cert-issuer-plugins/service.json"
	NotificationUAAConfigPath    = "../../env/secret/notification-plugins/uaa.json"
	NotificationEndpointsPath    = "../../env/secret/notification-plugins/endpoints.json"
	IdentityManagementConfigPath = "../../env/secret/identity-management/scim.json"
)

var KeystorePlugin = func(t *testing.T) catalog.PluginConfig {
	t.Helper()

	return catalog.PluginConfig{
		Name:     "AWS",
		Type:     keystoreopv1.Type,
		LogLevel: "debug",
		Path:     keystorePath(),
		Tags: []string{
			"default_keystore",
		},
	}
}

var keystorePath = func() string {
	_, filename, _, _ := runtime.Caller(0)
	baseDir := filepath.Dir(filename)
	return filepath.Join(baseDir, "../../keystore-plugins/bin/keystoreop/aws")
}

var PKIPlugin = func(t *testing.T) catalog.PluginConfig {
	t.Helper()

	return catalog.PluginConfig{
		Name:              "CERT_ISSUER",
		Type:              certificate_issuerv1.Type,
		LogLevel:          "debug",
		YamlConfiguration: pkiYaml(t),
		Path:              pkiPath(),
	}
}

var pkiYaml = func(t *testing.T) string {
	t.Helper()
	cfg := struct {
		UAA                commoncfg.SourceRef `yaml:"uaa"`
		CertificateService commoncfg.SourceRef `yaml:"certificateservice"` //nolint:tagliatelle
	}{
		UAA: commoncfg.SourceRef{
			Source: commoncfg.FileSourceValue,
			File: commoncfg.CredentialFile{
				Path:   PKIUAAConfigPath,
				Format: commoncfg.BinaryFileFormat,
			},
		},
		CertificateService: commoncfg.SourceRef{
			Source: commoncfg.FileSourceValue,
			File: commoncfg.CredentialFile{
				Path:   PKIServiceConfigPath,
				Format: commoncfg.BinaryFileFormat,
			},
		},
	}

	bytes, _ := yaml.Marshal(cfg)
	return string(bytes)
}

var pkiPath = func() string {
	_, filename, _, _ := runtime.Caller(0)
	baseDir := filepath.Dir(filename)
	return filepath.Join(baseDir, "../../cert-issuer-plugins/bin/cert-issuer")
}

var notificationPath = func() string {
	_, filename, _, _ := runtime.Caller(0)
	baseDir := filepath.Dir(filename)

	return filepath.Join(baseDir, "../../notification-plugins/bin/notification")
}

var notificationYaml = func(t *testing.T) string {
	t.Helper()

	cfg := struct {
		UAA       commoncfg.SourceRef `yaml:"uaa"`
		Endpoints commoncfg.SourceRef `yaml:"endpoints"`
	}{
		UAA: commoncfg.SourceRef{
			Source: commoncfg.FileSourceValue,
			File: commoncfg.CredentialFile{
				Path:   NotificationUAAConfigPath,
				Format: commoncfg.BinaryFileFormat,
			},
		},
		Endpoints: commoncfg.SourceRef{
			Source: commoncfg.FileSourceValue,
			File: commoncfg.CredentialFile{
				Path:   NotificationEndpointsPath,
				Format: commoncfg.BinaryFileFormat,
			},
		},
	}

	bytes, _ := yaml.Marshal(cfg)
	return string(bytes)
}

var SISPlugin = func(t *testing.T) catalog.PluginConfig {
	t.Helper()

	return catalog.PluginConfig{
		Name:              "SYSINFO",
		Type:              systeminformationv1.Type,
		Path:              sisPath(),
		LogLevel:          "debug",
		YamlConfiguration: sisYaml(t),
	}
}

var NotificationPlugin = func(t *testing.T) catalog.PluginConfig {
	t.Helper()

	return catalog.PluginConfig{
		Name:              "NOTIFICATION",
		Type:              notificationv1.Type,
		LogLevel:          "debug",
		YamlConfiguration: notificationYaml(t),
		Path:              notificationPath(),
	}
}

var sisYaml = func(t *testing.T) string {
	t.Helper()

	_, filename, _, _ := runtime.Caller(0)
	baseDir := filepath.Dir(filename)

	ststemInfoCertPath := filepath.Join(baseDir, "../../sis-plugins/mocks/cld/local")

	certPath := filepath.Join(ststemInfoCertPath, "mtls_client_cert.pem")
	keyPath := filepath.Join(ststemInfoCertPath, "mtls_client_key.pem")

	key, err := os.ReadFile(keyPath)
	assert.NoError(t, err)
	certificate, err := os.ReadFile(certPath)
	assert.NoError(t, err)

	uaa := struct {
		ID             string `json:"clientid"`    //nolint:tagliatelle
		Certificate    string `json:"certificate"` //nolint:tagliatelle
		Key            string `json:"key"`
		CertURL        string `json:"certurl"`         //nolint:tagliatelle
		CredentialType string `json:"credential-type"` //nolint:tagliatelle
	}{
		ID:             "validClientId",
		Certificate:    string(certificate),
		Key:            string(key),
		CertURL:        "https://localhost:8001",
		CredentialType: "x509",
	}

	uaaBytes, err := json.Marshal(uaa)
	assert.NoError(t, err)

	cfg := struct {
		CLDISEndpoint commoncfg.SourceRef `yaml:"cldisEndpoint"`
		UAA           commoncfg.SourceRef `yaml:"uaa"`
	}{
		UAA: commoncfg.SourceRef{
			Source: commoncfg.EmbeddedSourceValue,
			Value:  string(uaaBytes),
		},
		CLDISEndpoint: commoncfg.SourceRef{
			Source: commoncfg.EmbeddedSourceValue,
			Value:  "https://localhost:8001/cldPublic/v1",
		},
	}

	bytes, _ := yaml.Marshal(cfg)
	return string(bytes)
}

var sisPath = func() string {
	_, filename, _, _ := runtime.Caller(0)
	baseDir := filepath.Dir(filename)

	return filepath.Join(baseDir, "../../sis-plugins/bin/uli")
}

var KeystoreProviderPlugin = func(t *testing.T) catalog.PluginConfig {
	t.Helper()

	return catalog.PluginConfig{
		Name:     "AWS",
		Type:     keystoremanv1.Type,
		Path:     keystoreProviderPath(),
		LogLevel: "debug",
	}
}

var keystoreProviderPath = func() string {
	_, filename, _, _ := runtime.Caller(0)
	baseDir := filepath.Dir(filename)

	return filepath.Join(baseDir,
		"../../internal/testutils/testplugins/keystoreman/testpluginbinary")
}

var IDMangementPlugin = func(t *testing.T) catalog.PluginConfig {
	t.Helper()

	return catalog.PluginConfig{
		Name:              "IDENTITY_MANAGEMENT",
		Type:              idmangv1.Type,
		Path:              IDManagementPath(),
		LogLevel:          "debug",
		YamlConfiguration: IDManagementYaml(t),
	}
}

var IDManagementYaml = func(t *testing.T) string {
	t.Helper()

	type Params struct {
		GroupAttribute          commoncfg.SourceRef `yaml:"groupAttribute"`
		UserAttribute           commoncfg.SourceRef `yaml:"userAttribute"`
		GroupMembersAttribute   commoncfg.SourceRef `yaml:"groupMembersAttribute"`
		ListMethod              commoncfg.SourceRef `yaml:"listMethod"`
		AllowSearchUsersByGroup commoncfg.SourceRef `yaml:"allowSearchUsersByGroup"`
	}

	cfg := struct {
		Host   commoncfg.SourceRef `yaml:"host"`
		Auth   commoncfg.SecretRef `yaml:"auth"`
		Params Params              `yaml:"params"`
	}{
		Host: commoncfg.SourceRef{
			Source: commoncfg.FileSourceValue,
			File: commoncfg.CredentialFile{
				Path:     IdentityManagementConfigPath,
				Format:   commoncfg.JSONFileFormat,
				JSONPath: "$['host']",
			},
		},
		Auth: commoncfg.SecretRef{
			Type: commoncfg.MTLSSecretType,
			MTLS: commoncfg.MTLS{
				Cert: commoncfg.SourceRef{
					Source: commoncfg.FileSourceValue,
					File: commoncfg.CredentialFile{
						Path:     IdentityManagementConfigPath,
						Format:   commoncfg.JSONFileFormat,
						JSONPath: "$['auth']['cert']",
					},
				},
				CertKey: commoncfg.SourceRef{
					Source: commoncfg.FileSourceValue,
					File: commoncfg.CredentialFile{
						Path:     IdentityManagementConfigPath,
						Format:   commoncfg.JSONFileFormat,
						JSONPath: "$['auth']['certKey']",
					},
				},
			},
		},
		Params: Params{
			GroupAttribute: commoncfg.SourceRef{
				Source: commoncfg.FileSourceValue,
				File: commoncfg.CredentialFile{
					Path:     IdentityManagementConfigPath,
					Format:   commoncfg.JSONFileFormat,
					JSONPath: "$['params']['groupAttribute']",
				},
			},
			UserAttribute: commoncfg.SourceRef{
				Source: commoncfg.FileSourceValue,
				File: commoncfg.CredentialFile{
					Path:     IdentityManagementConfigPath,
					Format:   commoncfg.JSONFileFormat,
					JSONPath: "$['params']['userAttribute']",
				},
			},
			GroupMembersAttribute: commoncfg.SourceRef{
				Source: commoncfg.FileSourceValue,
				File: commoncfg.CredentialFile{
					Path:     IdentityManagementConfigPath,
					Format:   commoncfg.JSONFileFormat,
					JSONPath: "$['params']['groupMembersAttribute']",
				},
			},
			ListMethod: commoncfg.SourceRef{
				Source: commoncfg.FileSourceValue,
				File: commoncfg.CredentialFile{
					Path:     IdentityManagementConfigPath,
					Format:   commoncfg.JSONFileFormat,
					JSONPath: "$['params']['listMethod']",
				},
			},
			AllowSearchUsersByGroup: commoncfg.SourceRef{
				Source: commoncfg.FileSourceValue,
				File: commoncfg.CredentialFile{
					Path:     IdentityManagementConfigPath,
					Format:   commoncfg.JSONFileFormat,
					JSONPath: "$['params']['allowSearchUsersByGroup']",
				},
			},
		},
	}

	bytes, _ := yaml.Marshal(cfg)

	return string(bytes)
}

var IDManagementPath = func() string {
	_, filename, _, _ := runtime.Caller(0)
	baseDir := filepath.Dir(filename)
	return filepath.Join(baseDir, "../../identity-management-plugins/bin/scim")
}
