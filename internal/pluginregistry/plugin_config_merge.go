package cmkpluginregistry

import (
	"encoding/json"
	"fmt"
	"maps"

	"github.com/openkcm/common-sdk/pkg/commoncfg"
	"gopkg.in/yaml.v3"

	plugincatalog "github.com/openkcm/plugin-sdk/pkg/catalog"

	"github.com/openkcm/cmk/internal/config"
	servicewrapper "github.com/openkcm/cmk/internal/pluginregistry/service/wrapper"
)

// buildCMKConfigOverrides assembles the overrides map consumed by
// mergePluginConfigsWithCMKConfigs from the CMK config.
func buildCMKConfigOverrides(cfg *config.Config) (map[string]map[string]any, error) {
	overrides := make(map[string]map[string]any)

	if len(cfg.KeystorePool.SupportedRegions) > 0 {
		wrapped, err := json.Marshal(map[string]any{"regions": cfg.KeystorePool.SupportedRegions})
		if err != nil {
			return nil, fmt.Errorf("failed to marshal supported regions: %w", err)
		}
		overrides[servicewrapper.KeystoreManagementType] = map[string]any{
			"supportedregions": commoncfg.SourceRef{
				Source: commoncfg.EmbeddedSourceValue,
				Value:  string(wrapped),
			},
		}
	}

	return overrides, nil
}

// mergePluginConfigsWithCMKConfigs returns a copy of pluginConfigs where, for
// each plugin type present in overrides, the corresponding key/value pairs are
// merged into the plugin's YAML configuration.
func mergePluginConfigsWithCMKConfigs(
	pluginConfigs []plugincatalog.PluginConfig,
	overrides map[string]map[string]any,
) ([]plugincatalog.PluginConfig, error) {
	if len(overrides) == 0 {
		return pluginConfigs, nil
	}

	result := make([]plugincatalog.PluginConfig, len(pluginConfigs))
	copy(result, pluginConfigs)

	for i, pluginCfg := range result {
		keys, ok := overrides[pluginCfg.Type]
		if !ok {
			continue
		}

		merged, err := mergeKeysIntoYAML(pluginCfg.YamlConfiguration, keys)
		if err != nil {
			return nil, fmt.Errorf("failed to merge CMK config into plugin %q (type %q): %w",
				pluginCfg.Name, pluginCfg.Type, err)
		}

		result[i].DataSource = plugincatalog.FixedData(merged)
	}

	return result, nil
}

// mergeKeysIntoYAML unmarshals yamlConfig into a map, applies each key/value
// from keys, and returns the re-marshaled YAML string.
func mergeKeysIntoYAML(yamlConfig string, keys map[string]any) (string, error) {
	cfgMap := make(map[string]any)
	if yamlConfig != "" {
		if err := yaml.Unmarshal([]byte(yamlConfig), &cfgMap); err != nil {
			return "", fmt.Errorf("failed to parse plugin YamlConfiguration: %w", err)
		}
	}

	maps.Copy(cfgMap, keys)

	out, err := yaml.Marshal(cfgMap)
	if err != nil {
		return "", fmt.Errorf("failed to marshal merged plugin configuration: %w", err)
	}

	return string(out), nil
}
