package business

import (
	"context"

	"github.com/openkcm/common-sdk/pkg/commoncfg"
	"github.com/openkcm/orbital/client/amqp"
	"github.com/openkcm/orbital/codec"
	"github.com/samber/oops"

	"github.com/openkcm/cmk-core/internal/clients"
	"github.com/openkcm/cmk-core/internal/config"
	"github.com/openkcm/cmk-core/internal/db"
	"github.com/openkcm/cmk-core/internal/log"
	"github.com/openkcm/cmk-core/tenant-manager/internal/operator"
)

const logDomain = "business"

func Main(ctx context.Context, cfg *config.Config) error {
	// Initialize the database connection
	dbConn, err := db.StartDB(ctx, cfg.Database, cfg.Provisioning, nil)
	if err != nil {
		return oops.In(logDomain).Wrapf(err, "Failed to start the database connection")
	}

	// Initialize AMQP client
	opts := amqp.WithNoAuth()
	if cfg.TenantManager.SecretRef.Type == commoncfg.MTLSSecretType {
		opts = operator.WithMTLS(cfg.TenantManager.SecretRef.MTLS)
	}

	amqpClient, err := amqp.NewClient(ctx, codec.Proto{}, amqp.ConnectionInfo{
		URL:    cfg.TenantManager.AMQP.URL,
		Target: cfg.TenantManager.AMQP.Target,
		Source: cfg.TenantManager.AMQP.Source,
	}, opts)
	if err != nil {
		return oops.In(logDomain).
			Wrapf(err, "Failed to create AMQP client: %v", err)
	}

	// Initialize gRPC client connection to Registry service
	clientsFactory, err := clients.NewFactory(cfg.Services)
	if err != nil {
		return oops.In(logDomain).
			Wrapf(err, "Failed to create clients factory: %v", err)
	}

	if clientsFactory.RegistryService() == nil {
		return oops.In(logDomain).
			Errorf("Registry client is nil, please check gRPC configuration")
	}

	tenantClient := clientsFactory.RegistryService().Tenant()

	// Create the TenantOperator
	tenantOperator, err := operator.NewTenantOperator(dbConn, amqpClient, tenantClient)
	if err != nil {
		return oops.In(logDomain).
			Wrapf(err, "Failed to create TenantOperator: %v", err)
	}

	// Run the TenantOperator
	err = tenantOperator.RunOperator(ctx)
	if err != nil {
		return err
	}

	<-ctx.Done()
	log.Info(ctx, "Shutting down Tenant Operator")

	return nil
}
