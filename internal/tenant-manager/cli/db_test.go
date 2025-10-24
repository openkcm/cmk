package cli_test

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"

	"github.com/bartventer/gorm-multitenancy/v8/pkg/driver"
	"github.com/google/uuid"
	"github.com/stretchr/testify/suite"
	"gorm.io/gorm"

	_ "github.com/bartventer/gorm-multitenancy/postgres/v8"

	multitenancy "github.com/bartventer/gorm-multitenancy/v8"

	"github.com/openkcm/cmk-core/internal/model"
	sqlRepo "github.com/openkcm/cmk-core/internal/repo/sql"
	tmdb "github.com/openkcm/cmk-core/internal/tenant-manager/db"
	"github.com/openkcm/cmk-core/internal/testutils"
	integrationutils "github.com/openkcm/cmk-core/test/integration_utils"
	"github.com/openkcm/cmk-core/utils/base62"
)

type DBSuite struct {
	suite.Suite

	cancel context.CancelFunc
	db     *multitenancy.DB
}

func (s *DBSuite) SetupSuite() {
	s.db, _ = testutils.NewTestDB(s.T(), testutils.TestDBConfig{
		RequiresMultitenancyOrShared: false,
		Models:                       []driver.TenantTabler{&model.Tenant{}, &model.Group{}, &model.KeyConfiguration{}},
	})
}

func (s *DBSuite) TearDownSuite() {
	if s.cancel != nil {
		s.cancel()
	}
}

func TestDBSuite(t *testing.T) {
	suite.Run(t, new(DBSuite))
}

func (s *DBSuite) TestStartDB() {
	err := s.db.Session(&gorm.Session{AllowGlobalUpdate: true}).Delete(&model.Tenant{}).Error
	s.Require().NoError(err)
}

//nolint:funlen
func (s *DBSuite) TestCreateTenantSchema() {
	ctx := s.T().Context()
	tests := []struct {
		name            string
		tenantName      string
		tenantID        string
		domainURL       string
		expectError     bool
		preCreateTenant bool
	}{
		{
			name:            "Successful onboarding lowercase",
			tenantName:      "newtenant",
			tenantID:        uuid.NewString(),
			domainURL:       "newtenant.example.com",
			expectError:     false,
			preCreateTenant: false,
		},
		{
			name:            "Successful onboarding uppercase and lowercase",
			tenantName:      "NewTenant",
			tenantID:        uuid.NewString(),
			domainURL:       "NewTenant.example.com",
			expectError:     false,
			preCreateTenant: false,
		},
		{
			name:            "Successful onboarding uppercase",
			tenantName:      "NEWTENANT",
			tenantID:        uuid.NewString(),
			domainURL:       "newTenant.example.com",
			expectError:     false,
			preCreateTenant: false,
		},
		{
			name:            "Successful onboarding with digits",
			tenantName:      "newtenant123",
			tenantID:        uuid.NewString(),
			domainURL:       "newtenant123.example.com",
			expectError:     false,
			preCreateTenant: false,
		},
		{
			name:            "Successful onboarding with underscore",
			tenantName:      "new_tenant",
			tenantID:        uuid.NewString(),
			domainURL:       "new_tenant.example.com",
			expectError:     false,
			preCreateTenant: false,
		},
		{
			name:            "Failed onboarding with dot",
			tenantName:      "new.tenant",
			tenantID:        uuid.NewString(),
			domainURL:       "new.tenant.example.com",
			expectError:     true,
			preCreateTenant: false,
		},
		{
			name:            "Failed onboarding with tilde",
			tenantName:      "new~tenant",
			tenantID:        uuid.NewString(),
			domainURL:       "new~tenant.example.com",
			expectError:     true,
			preCreateTenant: false,
		},
		{
			name:            "Failed onboarding with dash",
			tenantName:      "new-tenant",
			tenantID:        uuid.NewString(),
			domainURL:       "new-tenant.example.com",
			expectError:     true,
			preCreateTenant: false,
		},
		{
			name:            "Tenant already exists",
			tenantName:      "existing_tenant",
			tenantID:        uuid.NewString(),
			domainURL:       "existing.example.com",
			expectError:     true,
			preCreateTenant: true,
		},
		{
			name:            "Tenant with invalid schema name",
			tenantName:      "%%%",
			tenantID:        uuid.NewString(),
			domainURL:       "%%%.example.com",
			expectError:     true,
			preCreateTenant: false,
		},
		{
			name:            "Tenant with invalid schema name colon",
			tenantName:      "tenant:invalid",
			tenantID:        uuid.NewString(),
			domainURL:       "tenant:invalid.example.com",
			expectError:     true,
			preCreateTenant: false,
		},
	}

	for _, tt := range tests {
		s.Run(tt.name, func() {
			if tt.preCreateTenant {
				tenantModel := testutils.NewTenant(func(l *model.Tenant) {
					l.SchemaName = tt.tenantName
					l.DomainURL = tt.domainURL + ".example.com"
					l.ID = tt.tenantID
				})
				err := tmdb.CreateSchema(ctx, s.db, tenantModel)
				s.Require().NoError(err)
			}

			tenantModel := testutils.NewTenant(func(l *model.Tenant) {
				l.SchemaName = tt.tenantName
				l.DomainURL = tt.domainURL + ".example.com"
				l.ID = tt.tenantID
			})

			err := tmdb.CreateSchema(ctx, s.db, tenantModel)

			if tt.expectError {
				s.Require().Error(err)
				return
			}

			s.Require().NoError(err)

			exists, err := integrationutils.TenantExistsInPublicSchema(s.db, tenantModel.SchemaName)
			s.Require().NoError(err)
			s.True(exists, "tenant %s should exist", tenantModel.SchemaName)

			if !tt.expectError {
				exists, err := integrationutils.TenantSchemaExists(s.db, tenantModel.SchemaName)
				s.True(exists, "Schema %s should exist", tenantModel.SchemaName)
				s.Require().NoError(err)

				exists, err = integrationutils.TenantExistsInPublicSchema(s.db, tenantModel.SchemaName)
				s.True(exists, "Tenant %s should exist in public schema", tenantModel.SchemaName)
				s.Require().NoError(err)

				exists, err = integrationutils.NamespaceExists(s.db, tenantModel.SchemaName)
				s.True(exists, "Tenant %s namespace schould exists", tenantModel.SchemaName)
				s.Require().NoError(err)

				exists, err = integrationutils.TableInTenantSchemaExist(
					s.db,
					tenantModel.SchemaName,
					model.KeyConfiguration{}.TableName(),
				)
				s.True(
					exists,
					"Table schould exist in tenant %s schema: %s",
					tenantModel.SchemaName,
					model.KeyConfiguration{}.TableName(),
				)
				s.Require().NoError(err)
			}

			err = s.db.OffboardTenant(ctx, tenantModel.SchemaName)
			s.Require().NoError(err, "failed to drop schema %s: %v", tenantModel.SchemaName, err)

			err = s.db.Where("schema_name = ?", tenantModel.SchemaName).Delete(&model.Tenant{}).Error
			s.Require().NoError(err, "failed to delete tenant meta %s: %v", tenantModel.SchemaName, err)
		})
	}
}

func (s *DBSuite) TestConcurrentOnboardTenant() {
	ctx := s.T().Context()

	var (
		wg          sync.WaitGroup
		numRoutines = 20
		namesMu     sync.Mutex
		tenants     []model.Tenant
	)

	errs := make(chan error, numRoutines)

	for range numRoutines {
		wg.Add(1)

		tenantUUID := "t_" + strings.ReplaceAll(uuid.NewString(), "-", "")

		go func(name string) {
			defer wg.Done()

			tenant := testutils.NewTenant(func(l *model.Tenant) {
				l.SchemaName = name
				l.DomainURL = name + ".example.com"
				l.ID = name
			})

			err := tmdb.CreateSchema(ctx, s.db, tenant)
			if err != nil {
				errs <- fmt.Errorf("Onboarding failed for %s: %w", tenant.SchemaName, err)
			} else {
				errs <- nil
			}

			namesMu.Lock()

			tenants = append(tenants, *tenant)

			namesMu.Unlock()
		}(tenantUUID)
	}

	wg.Wait()
	close(errs)

	for err := range errs {
		s.Require().NoError(err)
	}
}

func (s *DBSuite) TestConcurrentOnboardSameTenant() {
	ctx := s.T().Context()

	var (
		wg          sync.WaitGroup
		numRoutines = 3
		tenantName  = "concurrent_same_tenant"
		schemaName  = tenantName
		ID          = uuid.NewString()
	)

	errs := make(chan error, numRoutines)

	for range numRoutines {
		wg.Add(1)

		go func() {
			defer wg.Done()

			tenant := testutils.NewTenant(func(l *model.Tenant) {
				l.SchemaName = tenantName
				l.DomainURL = tenantName + ".example.com"
				l.ID = ID
			})

			err := tmdb.CreateSchema(ctx, s.db, tenant)
			if err != nil {
				errs <- fmt.Errorf("Onboarding failed: %w", err)
			} else {
				errs <- nil
			}
		}()
	}

	wg.Wait()
	close(errs)

	var errorCount int

	for err := range errs {
		if err != nil {
			errorCount++
		}
	}

	s.Equal(2, errorCount, "Expected ErrOnboardingInProgress errors.")

	integrationutils.TenantExists(s.T(), s.db, schemaName, model.KeyConfiguration{}.TableName())
}

//nolint:funlen
func (s *DBSuite) TestCreateGroups() {
	ctx := s.T().Context()

	tests := []struct {
		name           string
		tenant         *model.Tenant
		setup          func(ctx context.Context, tenant *model.Tenant)
		expectError    bool
		expectedGroups bool
	}{
		{
			name: "creates groups in new schema",
			tenant: testutils.NewTenant(func(l *model.Tenant) {
				l.ID = uuid.NewString()
				l.SchemaName, _ = base62.EncodeSchemaNameBase62(l.ID)
				l.DomainURL, _ = base62.EncodeSchemaNameBase62(l.ID)
			}),
			setup: func(_ context.Context, tenant *model.Tenant) {
				err := tmdb.CreateSchema(ctx, s.db, tenant)
				s.NoError(err, "failed to create tenant schema")
			},
			expectError:    false,
			expectedGroups: true,
		},
		{
			name: "groups already exist",
			tenant: testutils.NewTenant(func(l *model.Tenant) {
				l.ID = uuid.NewString()
				l.SchemaName, _ = base62.EncodeSchemaNameBase62(l.ID)
				l.DomainURL, _ = base62.EncodeSchemaNameBase62(l.ID)
			}),
			setup: func(ctx context.Context, tenant *model.Tenant) {
				err := tmdb.CreateSchema(ctx, s.db, tenant)
				s.Require().NoError(err, "failed to create tenant schema")
				err = tmdb.CreateDefaultGroups(ctx, tenant, sqlRepo.NewRepository(s.db))
				s.Require().NoError(err, "failed to create tenant groups")
			},
			expectError:    true,
			expectedGroups: true,
		},
		{
			name: "schema does not exist",
			tenant: testutils.NewTenant(func(l *model.Tenant) {
				l.ID = uuid.NewString()
				l.SchemaName, _ = base62.EncodeSchemaNameBase62(l.ID)
				l.DomainURL, _ = base62.EncodeSchemaNameBase62(l.ID)
			}),
			setup: func(_ context.Context, _ *model.Tenant) {
				// Do not create schema
			},
			expectError:    true,
			expectedGroups: false,
		},
	}

	for _, tt := range tests {
		s.Run(tt.name, func() {
			tt.setup(ctx, tt.tenant)

			err := tmdb.CreateDefaultGroups(ctx, tt.tenant, sqlRepo.NewRepository(s.db))
			if tt.expectError {
				s.Require().Error(err, "Expected error but got none")
			} else {
				s.Require().NoError(err, "Did not expect error but got one")
			}

			if tt.expectedGroups {
				integrationutils.GroupsExists(ctx, s.T(), tt.tenant.ID, s.db)
			}
		})
	}
}

func (s *DBSuite) TestConcurrentCreateGroups() {
	ctx := s.T().Context()

	var (
		wg          sync.WaitGroup
		numRoutines = 3
		schemaName  = "t_example_schema_name"
		domainURL   = "t_example_schema_name.example.com"
		ID          = uuid.NewString()
		repo        = sqlRepo.NewRepository(s.db)
		tenant      = testutils.NewTenant(func(l *model.Tenant) {
			l.SchemaName = schemaName
			l.DomainURL = domainURL
			l.ID = ID
		})
	)

	err := tmdb.CreateSchema(ctx, s.db, tenant)
	s.NoError(err, "failed to create tenant schema")

	errs := make(chan error, numRoutines)

	for range numRoutines {
		wg.Add(1)

		go func() {
			defer wg.Done()

			tenant := testutils.NewTenant(func(l *model.Tenant) {
				l.SchemaName = schemaName
				l.DomainURL = schemaName + ".example.com"
				l.ID = ID
			})

			err = tmdb.CreateDefaultGroups(ctx, tenant, repo)
			if err != nil {
				errs <- fmt.Errorf("onboarding failed: %w", err)
			} else {
				errs <- nil
			}
		}()
	}

	wg.Wait()
	close(errs)

	var errorCount int

	for err = range errs {
		if err != nil {
			errorCount++
		}
	}

	s.Equal(2, errorCount, "Expected errors. ErrOnboardingInProgress errors ")

	integrationutils.GroupsExists(ctx, s.T(), tenant.ID, s.db)
}
