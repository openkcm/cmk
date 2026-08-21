package featureflags

import (
	"context"

	"github.com/open-feature/go-sdk/openfeature"
	"github.com/openkcm/common-sdk/pkg/commoncfg"
	sdkfeatureflags "github.com/openkcm/common-sdk/pkg/featureflags"
	"github.com/samber/oops"
)

const clientName = "cmk"

// Client abstracts OpenFeature boolean flag evaluation.
type Client interface {
	BooleanValue(
		ctx context.Context, flag string, defaultValue bool, evalCtx openfeature.EvaluationContext,
	) (bool, error)
}

// clientAdapter bridges *openfeature.Client to Client.
// The SDK's BooleanValue has variadic options, so it doesn't directly satisfy the interface.
type clientAdapter struct{ c *openfeature.Client }

func (a *clientAdapter) BooleanValue(
	ctx context.Context,
	flag string,
	defaultValue bool,
	evalCtx openfeature.EvaluationContext,
) (bool, error) {
	return a.c.BooleanValue(ctx, flag, defaultValue, evalCtx)
}

// NewClient returns a Client backed by the global OpenFeature provider.
func NewClient() Client {
	return &clientAdapter{c: openfeature.NewClient(clientName)}
}

// Init registers the EmbeddedProvider as the global OpenFeature provider when
// cfg.Enabled is true. It is a no-op when feature flags are disabled.
func Init(cfg commoncfg.FeatureFlags) error {
	if !cfg.Enabled {
		return nil
	}

	provider, err := sdkfeatureflags.NewEmbeddedProvider(cfg)
	if err != nil {
		return oops.Wrapf(err, "failed to create feature flag provider")
	}

	err = openfeature.SetProviderAndWait(provider)
	if err != nil {
		return oops.Wrapf(err, "failed to initialise feature flag provider")
	}
	return nil
}
