package cmkpluginregistry_test

import (
	"encoding/json"
	"testing"

	"github.com/openkcm/common-sdk/pkg/commoncfg"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	plugincatalog "github.com/openkcm/plugin-sdk/pkg/catalog"

	"github.com/openkcm/cmk/internal/config"
	cmkpluginregistry "github.com/openkcm/cmk/internal/pluginregistry"
	servicewrapper "github.com/openkcm/cmk/internal/pluginregistry/service/wrapper"
)

// --- mergePluginConfigsWithCMKConfigs ---

func TestMergePluginConfigs_EmptyOverrides(t *testing.T) {
	plugins := []plugincatalog.PluginConfig{
		{Name: "p", Type: "SomeType", YamlConfiguration: "key: val\n"},
	}
	result, err := cmkpluginregistry.MergePluginConfigsWithCMKConfigs(plugins, nil)
	require.NoError(t, err)
	assert.Nil(t, result[0].DataSource)
}

func TestMergePluginConfigs_InjectsKeyIntoEmptyYAML(t *testing.T) {
	plugins := []plugincatalog.PluginConfig{
		{Name: "p", Type: "KeystoreProvider"},
	}
	overrides := map[string]map[string]any{
		"KeystoreProvider": {"supportedRegions": []any{"eu-central-1"}},
	}

	result, err := cmkpluginregistry.MergePluginConfigsWithCMKConfigs(plugins, overrides)
	require.NoError(t, err)
	require.NotNil(t, result[0].DataSource)

	data, err := result[0].DataSource.Load()
	require.NoError(t, err)

	var out map[string]any
	require.NoError(t, yaml.Unmarshal([]byte(data), &out))
	assert.Equal(t, []any{"eu-central-1"}, out["supportedRegions"])
}

func TestMergePluginConfigs_ExistingYAMLKeysPreserved(t *testing.T) {
	plugins := []plugincatalog.PluginConfig{
		{Name: "p", Type: "KeystoreProvider", YamlConfiguration: "existing: value\n"},
	}
	overrides := map[string]map[string]any{
		"KeystoreProvider": {"injected": "newval"},
	}

	result, err := cmkpluginregistry.MergePluginConfigsWithCMKConfigs(plugins, overrides)
	require.NoError(t, err)

	data, err := result[0].DataSource.Load()
	require.NoError(t, err)

	var out map[string]any
	require.NoError(t, yaml.Unmarshal([]byte(data), &out))
	assert.Equal(t, "value", out["existing"])
	assert.Equal(t, "newval", out["injected"])
}

func TestMergePluginConfigs_UnmatchedTypeUnchanged(t *testing.T) {
	plugins := []plugincatalog.PluginConfig{
		{Name: "cert", Type: "CertificateIssuer", YamlConfiguration: "cert: val\n"},
	}
	overrides := map[string]map[string]any{
		"KeystoreProvider": {"key": "val"},
	}

	result, err := cmkpluginregistry.MergePluginConfigsWithCMKConfigs(plugins, overrides)
	require.NoError(t, err)
	assert.Nil(t, result[0].DataSource)
	assert.Equal(t, "cert: val\n", result[0].YamlConfiguration)
}

func TestMergePluginConfigs_OriginalSliceUnmodified(t *testing.T) {
	original := []plugincatalog.PluginConfig{
		{Name: "p", Type: "KeystoreProvider"},
	}
	overrides := map[string]map[string]any{
		"KeystoreProvider": {"key": "val"},
	}

	_, err := cmkpluginregistry.MergePluginConfigsWithCMKConfigs(original, overrides)
	require.NoError(t, err)
	assert.Nil(t, original[0].DataSource)
}

func TestMergePluginConfigs_MultiplePluginTypes(t *testing.T) {
	plugins := []plugincatalog.PluginConfig{
		{Name: "ks", Type: "KeystoreProvider"},
		{Name: "cert", Type: "CertificateIssuer"},
		{Name: "notif", Type: "Notification"},
	}
	overrides := map[string]map[string]any{
		"KeystoreProvider":  {"regionKey": "r1"},
		"CertificateIssuer": {"certKey": "c1"},
	}

	result, err := cmkpluginregistry.MergePluginConfigsWithCMKConfigs(plugins, overrides)
	require.NoError(t, err)
	assert.NotNil(t, result[0].DataSource)
	assert.NotNil(t, result[1].DataSource)
	assert.Nil(t, result[2].DataSource)
}

// --- buildCMKConfigOverrides ---

func TestBuildCMKConfigOverrides_WithSupportedRegions(t *testing.T) {
	cfg := &config.Config{}
	cfg.KeystorePool.SupportedRegions = []config.Region{
		{Name: "EU", TechnicalName: "eu-central-1"},
	}

	overrides, err := cmkpluginregistry.BuildCMKConfigOverrides(cfg)
	require.NoError(t, err)

	keystoreOverrides, ok := overrides[servicewrapper.KeystoreManagementType]
	require.True(t, ok)

	sr, ok := keystoreOverrides["supportedregions"].(commoncfg.SourceRef)
	require.True(t, ok)
	assert.Equal(t, commoncfg.EmbeddedSourceValue, sr.Source)

	raw, err := commoncfg.LoadValueFromSourceRef(sr)
	require.NoError(t, err)

	var wrapped map[string]any
	require.NoError(t, json.Unmarshal(raw, &wrapped))
	regions, ok := wrapped["regions"].([]any)
	require.True(t, ok)
	require.Len(t, regions, 1)
	region, ok := regions[0].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "EU", region["name"])
	assert.Equal(t, "eu-central-1", region["technicalName"])
}

func TestBuildCMKConfigOverrides_NoRegionsConfigured(t *testing.T) {
	cfg := &config.Config{}

	overrides, err := cmkpluginregistry.BuildCMKConfigOverrides(cfg)
	require.NoError(t, err)

	keystoreOverrides, ok := overrides[servicewrapper.KeystoreManagementType]
	require.True(t, ok, "override must always be present even with no regions")

	sr, ok := keystoreOverrides["supportedregions"].(commoncfg.SourceRef)
	require.True(t, ok)

	raw, err := commoncfg.LoadValueFromSourceRef(sr)
	require.NoError(t, err)

	var wrapped map[string]any
	require.NoError(t, json.Unmarshal(raw, &wrapped))
	regions, ok := wrapped["regions"].([]any)
	require.True(t, ok)
	assert.Empty(t, regions)
}

func TestBuildCMKConfigOverrides_YAMLRoundTrip(t *testing.T) {
	cfg := &config.Config{}
	cfg.KeystorePool.SupportedRegions = []config.Region{
		{Name: "EU", TechnicalName: "eu-central-1"},
	}

	plugins := []plugincatalog.PluginConfig{
		{Name: "ks", Type: servicewrapper.KeystoreManagementType},
	}

	overrides, err := cmkpluginregistry.BuildCMKConfigOverrides(cfg)
	require.NoError(t, err)

	result, err := cmkpluginregistry.MergePluginConfigsWithCMKConfigs(plugins, overrides)
	require.NoError(t, err)
	require.NotNil(t, result[0].DataSource)

	data, err := result[0].DataSource.Load()
	require.NoError(t, err)

	// The merged YAML should contain supportedregions as a SourceRef shape.
	var out map[string]any
	require.NoError(t, yaml.Unmarshal([]byte(data), &out))

	sr, ok := out["supportedregions"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, string(commoncfg.EmbeddedSourceValue), sr["source"])

	srValue, ok := sr["value"].(string)
	require.True(t, ok)
	raw, err := commoncfg.LoadValueFromSourceRef(commoncfg.SourceRef{
		Source: commoncfg.EmbeddedSourceValue,
		Value:  srValue,
	})
	require.NoError(t, err)

	var wrapped map[string]any
	require.NoError(t, json.Unmarshal(raw, &wrapped))
	regions, ok := wrapped["regions"].([]any)
	require.True(t, ok)
	require.Len(t, regions, 1)
	region, ok2 := regions[0].(map[string]any)
	require.True(t, ok2)
	assert.Equal(t, "EU", region["name"])
	assert.Equal(t, "eu-central-1", region["technicalName"])
}
