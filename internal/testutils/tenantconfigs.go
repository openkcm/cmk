package testutils

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/openkcm/cmk/internal/model"
	"github.com/openkcm/cmk/internal/repo"
)

func WriteTenantConfig(ctx context.Context, tb testing.TB, r repo.Repo, config model.TenantConfiguration) {
	tb.Helper()

	flattened, err := model.Flatten(config)
	require.NoError(tb, err)

	query := repo.NewQuery().OnConflict(repo.KeyField, repo.TypeField)
	for k, v := range flattened {
		tenantConfig := &model.TenantConfig{Key: k, Value: fmt.Sprint(v), Type: config.Type()}
		err := r.Set(ctx, tenantConfig, *query)
		require.NoError(tb, err)
	}
}
