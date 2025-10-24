package benchmark_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/bartventer/gorm-multitenancy/v8/pkg/driver"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"gorm.io/gorm"

	multitenancy "github.com/bartventer/gorm-multitenancy/v8"

	"github.com/openkcm/cmk-core/internal/model"
	"github.com/openkcm/cmk-core/internal/repo"
	"github.com/openkcm/cmk-core/internal/repo/sql"
	"github.com/openkcm/cmk-core/internal/testutils"
	cmkcontext "github.com/openkcm/cmk-core/utils/context"
)

type SystemProperty struct {
	ID    uuid.UUID `gorm:"type:uuid;primaryKey"`
	Key   string    `gorm:"type:varchar(255);primaryKey"`
	Value string    `gorm:"type:varchar(255)"`
}

func (SystemProperty) TableName() string {
	return "systems_properties"
}

func (SystemProperty) IsSharedModel() bool {
	return false
}

type System struct {
	ID         uuid.UUID         `gorm:"type:uuid;primaryKey"`
	Region     string            `gorm:"type:varchar(255)"`
	Properties map[string]string `gorm:"-:all"`
}

func (s System) BeforeCreate(tx *gorm.DB) error {
	props := make([]SystemProperty, len(s.Properties))

	i := 0
	for k, v := range s.Properties {
		props[i] = SystemProperty{
			ID:    s.ID,
			Key:   k,
			Value: v,
		}
		i++
	}

	tx.Create(props)

	return nil
}

func (System) TableName() string {
	return "systems"
}

func (System) IsSharedModel() bool {
	return false
}

type SystemWithProperties struct {
	ID         uuid.UUID         `gorm:"type:uuid;primaryKey"`
	Region     string            `gorm:"type:varchar(255)"`
	Properties map[string]string `gorm:"serializer:json"`
}

func (SystemWithProperties) TableName() string {
	return "systems_json"
}

func (SystemWithProperties) IsSharedModel() bool {
	return false
}

type JoinSystem struct {
	System

	Key   string `gorm:"type:varchar(255);primaryKey"`
	Value string `gorm:"type:varchar(255)"`
}

func createTestObjects(ctx context.Context, b *testing.B, r repo.Repo, n int) {
	b.Helper()

	for i := range n {
		err := r.Create(ctx, &System{
			ID: uuid.New(),
			Properties: map[string]string{
				fmt.Sprintf("test-%d", i):  fmt.Sprintf("test-%d", i),
				fmt.Sprintf("test2-%d", i): fmt.Sprintf("test2-%d", i),
			},
			Region: "test",
		})
		assert.NoError(b, err)

		err = r.Create(ctx, &SystemWithProperties{
			ID: uuid.New(),
			Properties: map[string]string{
				fmt.Sprintf("test-%d", i):  fmt.Sprintf("test-%d", i),
				fmt.Sprintf("test2-%d", i): fmt.Sprintf("test2-%d", i),
			},
			Region: "test",
		})
		assert.NoError(b, err)
	}
}

func BenchmarkCreate(b *testing.B) {
	db, tenants := testutils.NewTestDB(b, testutils.TestDBConfig{
		RequiresMultitenancyOrShared: false,
		Models:                       []driver.TenantTabler{&System{}, &SystemProperty{}, &SystemWithProperties{}},
	})
	ctx := cmkcontext.CreateTenantContext(b.Context(), tenants[0])
	r := sql.NewRepository(db)

	b.Run("PropertiesTable", func(b *testing.B) {
		for b.Loop() {
			err := r.Create(ctx, &System{
				ID: uuid.New(),
				Properties: map[string]string{
					"test-1": "test-1",
					"test-2": "test-2",
				},
				Region: "test",
			})
			assert.NoError(b, err)
		}
	})

	b.Run("PropertiesJSON", func(b *testing.B) {
		for b.Loop() {
			err := r.Create(ctx, &SystemWithProperties{
				ID: uuid.New(),
				Properties: map[string]string{
					"test-1": "test-1",
					"test-2": "test-2",
				},
				Region: "test",
			})
			assert.NoError(b, err)
		}
	})
}

func BenchmarkList(b *testing.B) {
	db, tenants := testutils.NewTestDB(b, testutils.TestDBConfig{
		RequiresMultitenancyOrShared: false,
		Models:                       []driver.TenantTabler{&System{}, &SystemProperty{}, &SystemWithProperties{}},
	})
	ctx := cmkcontext.CreateTenantContext(b.Context(), tenants[0])
	r := sql.NewRepository(db)

	var tenantID model.Tenant

	err := db.Where(repo.IDField+" = ?", tenants[0]).First(&tenantID).Error
	assert.NoError(b, err)

	n := 100
	createTestObjects(ctx, b, r, n)

	b.Run("PropertiesTable", func(b *testing.B) {
		for b.Loop() {
			var rows []JoinSystem

			err := db.WithTenant(ctx, tenantID.SchemaName, func(tx *multitenancy.DB) error {
				return tx.Table(System{}.TableName()).
					Select("*").
					Joins("inner join systems_properties on systems_properties.id = systems.id").
					Scan(&rows).Error
			})
			assert.NoError(b, err)

			systemsMap := make(map[uuid.UUID]System)
			for _, row := range rows {
				system, exist := systemsMap[row.ID]
				if !exist {
					system = System{
						ID:         row.ID,
						Region:     row.Region,
						Properties: make(map[string]string),
					}
					systemsMap[row.ID] = system
				}

				system.ID = row.ID
				system.Region = row.Region
				system.Properties[row.Key] = row.Value
			}

			systems := make([]System, len(systemsMap))

			i := 0
			for _, v := range systemsMap {
				systems[i] = v
				i++
			}

			assert.Len(b, systems, n)
		}
	})

	b.Run("PropertiesJSON", func(b *testing.B) {
		for b.Loop() {
			var sys []SystemWithProperties

			err := db.WithTenant(ctx, tenantID.SchemaName, func(tx *multitenancy.DB) error {
				return tx.Table("systems_json").
					Select("*").
					Scan(&sys).
					Error
			})
			assert.NoError(b, err)
			assert.Len(b, sys, n)
		}
	})
}

func BenchmarkFiltered(b *testing.B) {
	db, tenants := testutils.NewTestDB(b, testutils.TestDBConfig{
		RequiresMultitenancyOrShared: false,
		Models:                       []driver.TenantTabler{&System{}, &SystemProperty{}, &SystemWithProperties{}},
	})
	ctx := cmkcontext.CreateTenantContext(b.Context(), tenants[0])
	r := sql.NewRepository(db)

	n := 100
	createTestObjects(ctx, b, r, n)

	var tenantID model.Tenant

	err := db.Where(repo.IDField+" = ?", tenants[0]).First(&tenantID).Error
	assert.NoError(b, err)

	b.Run("PropertiesTable", func(b *testing.B) {
		for b.Loop() {
			var rows []JoinSystem

			err := db.WithTenant(ctx, tenantID.SchemaName, func(tx *multitenancy.DB) error {
				return tx.Table("systems").
					Select("*").
					Joins("join systems_properties on systems_properties.id = systems.id").
					Where("key = ?", "test-50").
					Scan(&rows).Error
			})
			assert.NoError(b, err)

			systemsMap := make(map[uuid.UUID]System)
			for _, row := range rows {
				system, exist := systemsMap[row.ID]
				if !exist {
					system = System{
						ID:         row.ID,
						Region:     row.Region,
						Properties: make(map[string]string),
					}
					systemsMap[row.ID] = system
				}

				system.ID = row.ID
				system.Region = row.Region
				system.Properties[row.Key] = row.Value
			}

			systems := make([]System, len(systemsMap))

			i := 0
			for _, v := range systemsMap {
				systems[i] = v
				i++
			}

			assert.Len(b, systems, 1)
		}
	})

	b.Run("PropertiesJSON", func(b *testing.B) {
		for b.Loop() {
			var sys []SystemWithProperties

			err := db.WithTenant(ctx, tenantID.SchemaName, func(tx *multitenancy.DB) error {
				return tx.Table("systems_json").
					Select("*").
					Scan(&sys).
					Error
			})
			assert.NoError(b, err)

			var system SystemWithProperties

			for _, s := range sys {
				if s.Properties["test-50"] != "test-50" {
					continue
				}

				system = s
			}

			assert.Equal(b, "test-50", system.Properties["test-50"])
		}
	})
}
