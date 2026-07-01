package commands

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"

	"github.com/openkcm/common-sdk/pkg/commoncfg"
	"github.com/openkcm/common-sdk/pkg/logger"
	"github.com/samber/oops"
	"github.com/spf13/cobra"

	multitenancy "github.com/bartventer/gorm-multitenancy/v8"

	"github.com/openkcm/cmk/internal/auditor"
	authz_loader "github.com/openkcm/cmk/internal/authz/loader"
	authz_repo "github.com/openkcm/cmk/internal/authz/repo"
	"github.com/openkcm/cmk/internal/clients"
	"github.com/openkcm/cmk/internal/config"
	"github.com/openkcm/cmk/internal/constants"
	"github.com/openkcm/cmk/internal/db"
	"github.com/openkcm/cmk/internal/errs"
	eventprocessor "github.com/openkcm/cmk/internal/event-processor"
	"github.com/openkcm/cmk/internal/log"
	"github.com/openkcm/cmk/internal/manager"
	"github.com/openkcm/cmk/internal/model"
	cmkpluginregistry "github.com/openkcm/cmk/internal/pluginregistry"
	serviceapi "github.com/openkcm/cmk/internal/pluginregistry/service/api"
	"github.com/openkcm/cmk/internal/repo"
	"github.com/openkcm/cmk/internal/repo/sql"
	"github.com/openkcm/cmk/utils/base62"
	cmkcontext "github.com/openkcm/cmk/utils/context"
)

var (
	ErrAuthzLoader      = errors.New("error creating authz repo loader")
	ErrTenantIDRequired = errors.New("tenant id is required")
	ErrDeleteTenant     = errors.New("failed to delete tenant")
	ErrTenantNotFound   = errors.New("tenant not found")
	ErrCreateTenant     = errors.New("failed to create tenant schema")
	ErrCreateGroups     = errors.New("failed to create gropus")
)

type CommandFactory struct {
	dbCon *multitenancy.DB
	r     repo.Repo
	gm    *manager.GroupManager
	tm    *manager.TenantManager
}

func NewTenantManagerCLI() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "tenant",
		Short: "Tenant management CLI",
		Long:  `Manage tenants: create, delete, list, and update tenant configurations.`,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			if ctx == nil {
				ctx = context.Background()
			}

			cfg, dbCon, svcRegistry, err := initializeTenantManager(ctx)
			if err != nil {
				return err
			}

			// Inject internal user data
			ctx, err = cmkcontext.InjectInternalUserData(ctx, constants.InternalTenantCLIRole)
			if err != nil {
				return fmt.Errorf("failed to inject internal user data: %w", err)
			}

			// Create command factory
			factory, err := NewCommandFactory(ctx, cfg, dbCon, svcRegistry)
			if err != nil {
				return fmt.Errorf("failed to create command factory: %w", err)
			}

			// Store factory in context using the shared context key
			ctx = context.WithValue(ctx, TenantManagerFactoryKey, *factory)
			cmd.SetContext(ctx)

			return nil
		},
	}

	// Add subcommands - they retrieve factory from context
	cmd.AddCommand(NewCreateTenantCmd())
	cmd.AddCommand(NewDeleteTenantCmd())
	cmd.AddCommand(NewGetTenantCmd())
	cmd.AddCommand(NewListTenantsCmd())
	cmd.AddCommand(NewUpdateTenantCmd())

	return cmd
}

func initializeTenantManager(ctx context.Context) (
	*config.Config,
	*multitenancy.DB,
	serviceapi.Registry,
	error,
) {
	cfg, err := config.LoadConfig(
		commoncfg.WithPaths(
			constants.DefaultConfigPath1,
			constants.DefaultConfigPath2,
			".",
		),
		commoncfg.WithEnvOverride("TENANT_MANAGER_CLI"),
	)
	if err != nil {
		log.Error(ctx, "Failed to load config:", err)
		return nil, nil, nil, err
	}

	err = logger.InitAsDefault(cfg.Logger, cfg.Application)
	if err != nil {
		return nil, nil, nil, oops.In("tenant-manager-cli").Wrapf(err, "Failed to initialise the logger")
	}

	log.Debug(ctx, "Starting tenant-manager-cli", slog.Any("config", cfg))

	dbCon, err := db.StartDBConnection(ctx, cfg.Database, cfg.DatabaseReplicas, &cfg.Telemetry)
	if err != nil {
		return nil, nil, nil, oops.In("tenant-manager-cli").Wrapf(err, "Failed to initialise db connection")
	}

	svcRegistry, err := cmkpluginregistry.New(ctx, cfg)
	if err != nil {
		return nil, nil, nil, oops.In("tenant-manager-cli").Wrapf(err, "Failed to initialise plugin catalog")
	}

	return cfg, dbCon, svcRegistry, nil
}

//nolint:funlen
func NewCommandFactory(
	ctx context.Context,
	cfg *config.Config,
	dbCon *multitenancy.DB,
	svcRegistry serviceapi.Registry,
) (*CommandFactory, error) {
	r := sql.NewRepository(dbCon)

	authzRepoLoader := authz_loader.NewRepoAuthzLoader(ctx, r, cfg)
	if authzRepoLoader.AuthzHandler == nil {
		return nil, ErrAuthzLoader
	}

	authzRepo := authz_repo.NewAuthzRepo(r, authzRepoLoader)

	cmkAuditor := auditor.New(ctx, cfg)

	clientsFactory, err := clients.NewFactory(cfg.Services)
	if err != nil {
		return nil, err
	}

	eventFactory, err := eventprocessor.NewEventFactory(ctx, cfg, authzRepo)
	if err != nil {
		return nil, err
	}

	cm := manager.NewCertificateManager(ctx, authzRepo, svcRegistry, cfg)
	um := manager.NewUserManager(authzRepo, cmkAuditor)
	tagm := manager.NewTagManager(authzRepo)
	kcm := manager.NewKeyConfigManager(authzRepo, cm, um, tagm, cmkAuditor, eventFactory, cfg)

	sys := manager.NewSystemManager(
		ctx,
		authzRepo,
		authzRepoLoader,
		clientsFactory,
		eventFactory,
		svcRegistry,
		cfg,
		kcm,
		um,
	)

	km := manager.NewKeyManager(
		authzRepo,
		svcRegistry,
		manager.NewTenantConfigManager(authzRepo, svcRegistry, cfg),
		kcm,
		um,
		cm,
		eventFactory,
		cmkAuditor,
	)

	migrator, err := db.NewMigrator(r, cfg)
	if err != nil {
		return nil, err
	}

	return &CommandFactory{
		dbCon: dbCon,
		r:     authzRepo,
		gm:    manager.NewGroupManager(authzRepo, svcRegistry, um),
		tm:    manager.NewTenantManager(authzRepo, sys, km, um, cmkAuditor, migrator),
	}, nil
}

//nolint:funlen
func NewCreateTenantCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a new tenant. Usage: tm create -i [tenant id] -s [tenant status] -R [tenant role]",
		Long: "Create a new tenant. Usage: tm create -id [tenant id]" +
			" -status [tenant status] -role [tenant role]",
		Args: cobra.ExactArgs(0),

		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := cmd.Context()
			f := cmkcontext.GetFromContext[*CommandFactory](ctx, TenantManagerFactoryKey)

			id, _ := cmd.Flags().GetString("id")
			status, _ := cmd.Flags().GetString("status")
			role, _ := cmd.Flags().GetString("role")

			encodedSchemaName, err := base62.EncodeSchemaNameBase62(id)
			if err != nil {
				cmd.Printf("Failed to encode schema name: %v\n", err)
				return err
			}

			tenant := &model.Tenant{
				ID:     id,
				Status: model.TenantStatus(status),
				Role:   model.TenantRole(role),
				TenantModel: multitenancy.TenantModel{
					DomainURL:  encodedSchemaName,
					SchemaName: encodedSchemaName,
				},
			}

			err = f.tm.CreateTenant(ctx, tenant)
			if errors.Is(err, manager.ErrOnboardingInProgress) {
				cmd.Printf("Tenant with ID: %s already exists\n", tenant.ID)
			} else if err != nil {
				cmd.Printf("Failed to create Tenant: %v\n", err)
				return errs.Wrap(ErrCreateTenant, err)
			}

			ctx = cmkcontext.CreateTenantContext(ctx, tenant.ID)

			err = f.gm.CreateDefaultGroups(ctx)
			if err != nil {
				if errors.Is(err, manager.ErrOnboardingInProgress) {
					cmd.Printf("Default groups for tenant already exists\n")
				} else if err != nil {
					cmd.Printf("Failed to create Default Gruops: %v\n", err)
					return errs.Wrap(ErrCreateGroups, err)
				}
			}

			cmd.Printf("Tenant: %s, created with schema: %s\n", id, encodedSchemaName)

			return nil
		},
	}

	var id, status, role string

	cmd.Flags().StringVarP(&id, "id", "i", "", "Tenant id")
	cmd.Flags().StringVarP(&status, "status", "s", "", "Tenant status")
	cmd.Flags().StringVarP(&role, "role", "R", "", "Tenant role")

	err := cmd.MarkFlagRequired("id")
	if err != nil {
		cmd.PrintErrf("failed to mark flag 'id' as required: %v\n", err)
	}

	err = cmd.MarkFlagRequired("status")
	if err != nil {
		cmd.PrintErrf("failed to mark flag 'status' as required: %v\n", err)
	}

	err = cmd.MarkFlagRequired("role")
	if err != nil {
		cmd.PrintErrf("failed to mark flag 'role' as required: %v\n", err)
	}

	return cmd
}

func NewDeleteTenantCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "delete",
		Short: "Delete a new tenant. Usage: tm create -i [tenant id] -r [tenant region] -s [tenant status]",
		Long:  "Delete a new tenant. Usage: tm create -id [tenant id] -region [tenant region] -status [tenant status]",
		Args:  cobra.ExactArgs(0),

		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := cmd.Context()
			f := cmkcontext.GetFromContext[*CommandFactory](ctx, TenantManagerFactoryKey)

			id, _ := cmd.Flags().GetString("id")
			if id == "" {
				cmd.Println("Tenant id is required")
				return ErrTenantIDRequired
			}

			tenant, err := f.tm.GetTenantByID(ctx, id)
			if err != nil {
				cmd.PrintErrf("Failed to get tenant by ID %s: %v", id, err)

				return err
			}

			if tenant == nil {
				cmd.Printf("Tenant with id %s not found\n", id)

				return ErrTenantNotFound
			}

			cmd.Printf("Deleting tenant. Id: %s, SchemaName: %s\n", tenant.ID, tenant.SchemaName)

			err = dropSchema(f.dbCon, tenant.SchemaName)
			if err != nil {
				cmd.PrintErrf("%v %v\n", ErrDeleteTenant, err)
				return err
			}

			_, err = f.r.Delete(ctx, &model.Tenant{ID: id}, *repo.NewQuery())
			if err != nil {
				cmd.PrintErrf("%v %v\n", ErrDeleteTenant, err)
				return err
			}

			cmd.Printf("Tenant deleted")

			return nil
		},
	}

	var id string
	cmd.Flags().StringVarP(&id, "id", "i", "", "Tenant id")

	err := cmd.MarkFlagRequired("id")
	if err != nil {
		cmd.PrintErrf("failed to mark flag 'id' as required: %v\n", err)
	}

	return cmd
}

func NewGetTenantCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "get",
		Short: "Get tenant by id. Usage: tm get -i [tenant id]",
		Long:  "Get tenant by id. Usage: tm get --id [tenant id]",
		Args:  cobra.ExactArgs(0),

		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := cmd.Context()
			f := cmkcontext.GetFromContext[*CommandFactory](ctx, TenantManagerFactoryKey)

			id, _ := cmd.Flags().GetString("id")

			if id == "" {
				cmd.Println("Tenant id is required")
				return ErrTenantIDRequired
			}

			tenant, err := f.tm.GetTenantByID(ctx, id)
			if err != nil {
				cmd.PrintErrf("Failed to get tenant by ID %s: %v", id, err)

				return nil
			}

			if tenant == nil {
				cmd.Printf("Tenant with id %s not found\n", id)

				return ErrTenantNotFound
			}

			out, err := json.MarshalIndent(tenant, "", "  ")
			if err != nil {
				return err
			}

			cmd.Println(string(out))

			return nil
		},
	}

	var id string
	cmd.Flags().StringVarP(&id, "id", "i", "", "Tenant id")

	err := cmd.MarkFlagRequired("id")
	if err != nil {
		cmd.PrintErrf("failed to mark flag 'id' as required: %v\n", err)
	}

	return cmd
}

func NewListTenantsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List all tenants. Usage: tm list",
		Long:  "List all tenants. Usage: tm list",

		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := cmd.Context()
			f := cmkcontext.GetFromContext[*CommandFactory](ctx, TenantManagerFactoryKey)

			var tenants []model.Tenant

			err := f.r.List(
				ctx, &model.Tenant{}, &tenants, *repo.NewQuery(),
			)
			if err != nil {
				cmd.PrintErrf("failed to get tenants")
				return err
			}

			for _, tenant := range tenants {
				err = FormatTenant(&tenant, cmd)
				if err != nil {
					return err
				}
			}

			return nil
		},
	}

	return cmd
}

func NewUpdateTenantCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "update",
		Short: "Update existing tenant. Usage: tm update -i [tenant id] (-s [tenant status])",
		Long: "Update existing tenant. Usage: tm update --id [tenant id] " +
			"(--status [tenant status])",
		Args: cobra.ExactArgs(0),

		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := cmd.Context()
			f := cmkcontext.GetFromContext[*CommandFactory](ctx, TenantManagerFactoryKey)

			id, _ := cmd.Flags().GetString("id")
			status, _ := cmd.Flags().GetString("status")

			tenant, err := f.tm.GetTenantByID(ctx, id)
			if err != nil {
				cmd.PrintErrf("Failed to get tenant by ID %s: %v", id, err)

				return nil
			}

			if tenant == nil {
				cmd.Printf("Tenant with id %s not found\n", id)

				return nil
			}

			query := repo.NewQuery()

			if status != "" {
				tenant.Status = model.TenantStatus(status)
			}

			_, err = f.r.Patch(ctx, tenant, *query)
			if err != nil {
				cmd.PrintErrf("Failed to update tenant: %v\n", err)
				return err
			}

			cmd.Print("Tenant updated")

			return nil
		},
	}

	var id, status string
	cmd.Flags().StringVarP(&id, "id", "i", "", "Tenant id")
	cmd.Flags().StringVarP(&status, "status", "s", "", "Tenant status")

	err := cmd.MarkFlagRequired("id")
	if err != nil {
		cmd.PrintErrf("failed to mark flag 'id' as required: %v\n", err)
	}

	return cmd
}

func FormatTenant(tenant *model.Tenant, cmd *cobra.Command) error {
	out, err := json.MarshalIndent(tenant, "", "  ")
	if err != nil {
		return err
	}

	cmd.Println(string(out))

	return nil
}

func dropSchema(db *multitenancy.DB, schemaName string) error {
	sql := fmt.Sprintf(`DROP SCHEMA IF EXISTS "%s" CASCADE`, schemaName)
	return db.Exec(sql).Error
}
