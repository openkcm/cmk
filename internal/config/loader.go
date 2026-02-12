package config

import (
	"log/slog"

	"github.com/openkcm/common-sdk/pkg/commoncfg"
	"github.com/samber/oops"
	"gopkg.in/yaml.v3"

	"github.com/openkcm/cmk/internal/constants"
)

//nolint:mnd
var defaultConfig = map[string]any{"Certificates": map[string]int{"ValidityDays": 30}}

func LoadConfig(opts ...commoncfg.Option) (*Config, error) {
	cfg := &Config{}

	// If loadconfig is called with one of the default ones but different values
	// these are overridden as only the last one takes efect
	options := make([]commoncfg.Option, 0, 2)
	options = append(options,
		commoncfg.WithDefaults(defaultConfig),
		commoncfg.WithPaths(
			constants.DefaultConfigPath1,
			constants.DefaultConfigPath2,
			".",
		),
	)

	options = append(options, opts...)

	loader := commoncfg.NewLoader(
		cfg,
		options...,
	)

	err := loader.LoadConfig()
	if err != nil {
		return nil, oops.Wrapf(err, "failed to load config")
	}

	bytes, err := commoncfg.LoadValueFromSourceRef(cfg.ConfigurableContext)
	if err != nil {
		slog.Warn("No configurable context set")
	} else {
		err = yaml.Unmarshal(bytes, &cfg.ContextModels)
		if err != nil {
			return nil, oops.Wrapf(err, "failed to load configurable-context models")
		}
	}

	err = cfg.Validate()
	if err != nil {
		return nil, oops.Wrapf(err, "failed to validate config")
	}

	return cfg, nil
}
