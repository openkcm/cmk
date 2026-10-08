package tenantdatamigrations

import (
	"context"
	"database/sql"
	"sync"

	"github.com/openkcm/common-sdk/pkg/commoncfg"
	"gopkg.in/yaml.v3"

	"github.com/openkcm/cmk/internal/config"
	"github.com/openkcm/cmk/internal/model"
)

var (
	cfgOnce   sync.Once
	cachedCfg *config.Config
	errCached error
)

func getConfig() (*config.Config, error) {
	cfgOnce.Do(func() {
		cachedCfg, errCached = config.LoadConfig()
	})
	return cachedCfg, errCached
}

func upCryptoSubjectFromConfig(ctx context.Context, tx *sql.Tx) error {
	exists, err := cryptoAccessDataColumnExists(ctx, tx)
	if err != nil {
		return err
	}
	if !exists {
		return nil
	}

	cfg, err := getConfig()
	if err != nil {
		return err
	}

	// The migrator sets the schema via search_path but does not put the tenant
	// in the context
	tenantID, err := getTenantID(ctx, tx)
	if err != nil {
		return err
	}

	certsByRegion, err := getCryptoCerts(cfg, tenantID)
	if err != nil {
		return err
	}

	for region, cert := range certsByRegion {
		// For every key where a region certificateSubject is equal to the region recalculate the certificateSubject
		_, err := tx.ExecContext(ctx, `
			UPDATE keys
			SET crypto_access_data = jsonb_set(
				crypto_access_data,
				ARRAY[$1, 'certificateSubject'],
				to_jsonb($2::text),
				false
			)
			WHERE crypto_access_data IS NOT NULL
			  AND crypto_access_data -> $1 ->> 'certificateSubject' = $1
		`, region, cert.Subject.String())
		if err != nil {
			return err
		}
	}

	return nil
}

func downCryptoSubjectFromConfig(_ context.Context, _ *sql.Tx) error {
	return nil
}

// getCryptoCerts loads the configured X.509 trust certificates and returns
// them keyed by region (the certificate Name). The subject CommonName embeds
// the given tenant ID, matching how the runtime builds it.
func getCryptoCerts(cfg *config.Config, tenantID string) (map[string]*model.ClientCertificate, error) {
	bytes, err := commoncfg.LoadValueFromSourceRef(cfg.CryptoLayer.CertX509Trusts)
	if err != nil {
		return nil, err
	}

	var certConfigs []*config.CryptoCert
	if err := yaml.Unmarshal(bytes, &certConfigs); err != nil {
		return nil, err
	}

	certsByRegion := make(map[string]*model.ClientCertificate, len(certConfigs))
	for _, certCfg := range certConfigs {
		if certCfg == nil {
			continue
		}
		cert := model.NewClientCertificate(*certCfg, tenantID)
		certsByRegion[cert.Name] = &cert
	}

	return certsByRegion, nil
}

func getTenantID(ctx context.Context, tx *sql.Tx) (string, error) {
	var tenantID string
	err := tx.QueryRowContext(ctx,
		`SELECT id FROM public.tenants WHERE schema_name = current_schema()`,
	).Scan(&tenantID)
	if err != nil {
		return "", err
	}
	return tenantID, nil
}

func cryptoAccessDataColumnExists(ctx context.Context, tx *sql.Tx) (bool, error) {
	var exists bool
	err := tx.QueryRowContext(ctx, `
		SELECT EXISTS (
			SELECT 1
			FROM information_schema.columns
			WHERE table_name = 'keys'
			  AND column_name = 'crypto_access_data'
		)
	`).Scan(&exists)
	if err != nil {
		return false, err
	}
	return exists, nil
}
