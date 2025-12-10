package db_test

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/bartventer/gorm-multitenancy/v8/pkg/driver"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm/logger"

	"github.tools.sap/kms/cmk/internal/config"
	"github.tools.sap/kms/cmk/internal/db"
	"github.tools.sap/kms/cmk/internal/model"
	"github.tools.sap/kms/cmk/internal/testutils"
)

func TestStartDB(t *testing.T) {
	t.Run("should start db connection and run migration", func(t *testing.T) {
		models := []driver.TenantTabler{
			&model.KeyConfiguration{},
			&model.Key{},
			&model.KeyVersion{},
			&model.KeyLabel{},
			&model.System{},
			&model.SystemProperty{},
			&model.Workflow{},
			&model.WorkflowApprover{},
			&model.Tenant{},
			&model.TenantConfig{},
			&model.Certificate{},
			&model.Group{},
			&model.ImportParams{},
			&model.KeystoreConfiguration{},
			&model.Event{},
		}
		// Disable tenant creation
		con, _, dbCfg := testutils.NewTestDB(t, testutils.TestDBConfig{
			Models: models,
			Logger: logger.Default.LogMode(logger.Error),
		}, testutils.WithGenerateTenants(0))

		cfg := &config.Config{}

		cfg.Database = dbCfg

		err := con.RegisterModels(t.Context(), models...)
		require.NoError(t, err)
		err = con.MigrateSharedModels(t.Context())
		require.NoError(t, err)

		for range 2 {
			err := con.Create(testutils.NewTenant(func(_ *model.Tenant) {})).Error
			require.NoError(t, err)
		}

		conn, err := db.StartDB(
			t.Context(),
			cfg,
		)

		assert.NoError(t, err)
		assert.NotNil(t, conn)
		err = conn.Exec("DELETE FROM tenants").Error
		assert.NoError(t, err)
	})
}

func TestAddKeystoreFromConfig(t *testing.T) {
	t.Run("Successfully add keystore from config", func(t *testing.T) {
		// Arrange
		testDB, _, _ := testutils.NewTestDB(t, testutils.TestDBConfig{
			Models: []driver.TenantTabler{&model.TenantConfig{}, &model.KeystoreConfiguration{}},
		})

		initKeystoreConfig := config.InitKeystoreConfig{
			Enabled:  true,
			Provider: "AWS",
			Value: config.KeystoreConfigValue{
				LocalityID: testutils.TestLocalityID,
				CommonName: testutils.TestDefaultKeystoreCommonName,
				ManagementAccessData: fmt.Sprintf(
					"roleArn: %s\ntrustAnchorArn: %s\nprofileArn: %s",
					testutils.TestRoleArn,
					testutils.TestTrustAnchorArn,
					testutils.TestProfileArn,
				),
				SupportedRegions: testutils.SupportedRegions,
			},
		}

		// Act
		err := db.AddKeystoreFromConfig(t.Context(), testDB, initKeystoreConfig)

		// Assert
		assert.NoError(t, err)

		// Verify the keystore config was stored
		var storedConfig model.KeystoreConfiguration

		err = testDB.WithContext(t.Context()).Where("provider = ?", "AWS").First(&storedConfig).Error
		assert.NoError(t, err)

		var parsedValue map[string]any

		err = json.Unmarshal(storedConfig.Value, &parsedValue)
		assert.NoError(t, err)
		assert.Equal(t, testutils.TestLocalityID, parsedValue["localityId"])
		assert.Equal(t, testutils.TestDefaultKeystoreCommonName, parsedValue["commonName"])

		var managementData map[string]any

		if parsedValue["managementAccessData"] != nil {
			var ok bool

			managementData, ok = parsedValue["managementAccessData"].(map[string]any)
			if !ok {
				t.Fatal("managementAccessData is not of type map[string]any")
			}

			assert.Equal(t, testutils.TestRoleArn, managementData["roleArn"])
			assert.Equal(t, testutils.TestTrustAnchorArn, managementData["trustAnchorArn"])
			assert.Equal(t, testutils.TestProfileArn, managementData["profileArn"])
		} else {
			t.Fatal("managementAccessData is nil")
		}

		if parsedValue["supportedRegions"] != nil {
			supportedRegions, ok := parsedValue["supportedRegions"].([]any)
			if !ok {
				t.Fatal("supportedRegions is not of type []any")
			}

			assert.Len(t, supportedRegions, 2)

			region1, ok := supportedRegions[0].(map[string]any)
			if !ok {
				t.Fatal("first region is not of type map[string]any")
			}

			assert.Equal(t, testutils.SupportedRegions[0].Name, region1["name"])
			assert.Equal(t, testutils.SupportedRegions[0].TechnicalName, region1["technicalName"])

			region2, ok := supportedRegions[1].(map[string]any)
			if !ok {
				t.Fatal("second region is not of type map[string]any")
			}

			assert.Equal(t, testutils.SupportedRegions[1].Name, region2["name"])
			assert.Equal(t, testutils.SupportedRegions[1].TechnicalName, region2["technicalName"])
		} else {
			t.Fatal("supportedRegions is nil")
		}
	})

	t.Run("Skip adding keystore when disabled", func(t *testing.T) {
		// Arrange
		testDB, _, _ := testutils.NewTestDB(t, testutils.TestDBConfig{
			Models: []driver.TenantTabler{&model.TenantConfig{}, &model.KeystoreConfiguration{}},
		})

		initKeystoreConfig := config.InitKeystoreConfig{
			Enabled:  false,
			Provider: "AWS",
			Value:    config.KeystoreConfigValue{},
		}
		// Act
		err := db.AddKeystoreFromConfig(t.Context(), testDB, initKeystoreConfig)
		// Assert
		assert.NoError(t, err)
		// Verify no keystore config was stored
		var count int64

		err = testDB.WithContext(t.Context()).Model(&model.KeystoreConfiguration{}).Count(&count).Error
		assert.NoError(t, err)
		assert.Equal(t, int64(0), count)
	})

	t.Run("Handle missing locality ID", func(t *testing.T) {
		// Arrange
		testDB, _, _ := testutils.NewTestDB(t, testutils.TestDBConfig{
			Models: []driver.TenantTabler{&model.TenantConfig{}, &model.KeystoreConfiguration{}},
		})
		initKeystoreConfig := config.InitKeystoreConfig{
			Enabled:  true,
			Provider: "AWS",
			Value: config.KeystoreConfigValue{
				LocalityID: "",
				CommonName: testutils.TestDefaultKeystoreCommonName,
				ManagementAccessData: fmt.Sprintf(
					"roleArn: %s\ntrustAnchorArn: %s\nprofileArn: %s",
					testutils.TestRoleArn,
					testutils.TestTrustAnchorArn,
					testutils.TestProfileArn,
				),
			},
		}
		// Act
		err := db.AddKeystoreFromConfig(t.Context(), testDB, initKeystoreConfig)
		// Assert
		assert.Error(t, err)
		assert.Equal(t, db.ErrEmptyLocalityID, err)
	})

	t.Run("Handle missing common name", func(t *testing.T) {
		// Arrange
		testDB, _, _ := testutils.NewTestDB(t, testutils.TestDBConfig{
			Models: []driver.TenantTabler{&model.TenantConfig{}, &model.KeystoreConfiguration{}},
		})
		initKeystoreConfig := config.InitKeystoreConfig{
			Enabled:  true,
			Provider: "AWS",
			Value: config.KeystoreConfigValue{
				LocalityID: testutils.TestLocalityID,
				CommonName: "",
				ManagementAccessData: fmt.Sprintf(
					"roleArn: %s\ntrustAnchorArn: %s\nprofileArn: %s",
					testutils.TestRoleArn,
					testutils.TestTrustAnchorArn,
					testutils.TestProfileArn,
				),
			},
		}
		// Act
		err := db.AddKeystoreFromConfig(t.Context(), testDB, initKeystoreConfig)
		// Assert
		assert.Error(t, err)
		assert.Equal(t, db.ErrEmptyCommonName, err)
	})

	t.Run("Handle nil managementDataAccess", func(t *testing.T) {
		// Arrange
		testDB, _, _ := testutils.NewTestDB(t, testutils.TestDBConfig{
			Models: []driver.TenantTabler{&model.TenantConfig{}, &model.KeystoreConfiguration{}},
		})
		initKeystoreConfig := config.InitKeystoreConfig{
			Enabled:  true,
			Provider: "AWS",
			Value: config.KeystoreConfigValue{
				LocalityID:           testutils.TestLocalityID,
				CommonName:           testutils.TestDefaultKeystoreCommonName,
				ManagementAccessData: nil,
				SupportedRegions:     testutils.SupportedRegions,
			},
		}
		// Act
		err := db.AddKeystoreFromConfig(t.Context(), testDB, initKeystoreConfig)
		// Assert
		assert.Error(t, err)
		assert.Equal(t, db.ErrNilManagementAccessData, err)
	})

	t.Run("Handle invalid YAML in management access data", func(t *testing.T) {
		// Arrange
		testDB, _, _ := testutils.NewTestDB(t, testutils.TestDBConfig{
			Models: []driver.TenantTabler{&model.TenantConfig{}, &model.KeystoreConfiguration{}},
		})

		initKeystoreConfig := config.InitKeystoreConfig{
			Enabled:  true,
			Provider: "AWS",
			Value: config.KeystoreConfigValue{
				LocalityID:           testutils.TestLocalityID,
				CommonName:           testutils.TestDefaultKeystoreCommonName,
				ManagementAccessData: `invalid: yaml: content: [unclosed`,
				SupportedRegions:     testutils.SupportedRegions,
			},
		}
		// Act
		err := db.AddKeystoreFromConfig(t.Context(), testDB, initKeystoreConfig)
		// Assert
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to unmarshal YAML config value")
	})

	t.Run("Handle duplicate keystore config", func(t *testing.T) {
		// Arrange
		testDB, _, _ := testutils.NewTestDB(t, testutils.TestDBConfig{
			Models: []driver.TenantTabler{&model.TenantConfig{}, &model.KeystoreConfiguration{}},
		})

		initKeystoreConfig := config.InitKeystoreConfig{
			Enabled:  true,
			Provider: "AWS",
			Value: config.KeystoreConfigValue{
				LocalityID: testutils.TestLocalityID,
				CommonName: testutils.TestDefaultKeystoreCommonName,
				ManagementAccessData: fmt.Sprintf(
					"roleArn: %s\ntrustAnchorArn: %s\nprofileArn: %s",
					testutils.TestRoleArn,
					testutils.TestTrustAnchorArn,
					testutils.TestProfileArn,
				),
				SupportedRegions: testutils.SupportedRegions,
			},
		}
		// Act
		err := db.AddKeystoreFromConfig(t.Context(), testDB, initKeystoreConfig)
		// Assert
		assert.NoError(t, err)
		// Try adding the same config again to test duplicate handling
		err = db.AddKeystoreFromConfig(t.Context(), testDB, initKeystoreConfig)
		assert.NoError(t, err)
		// Verify only one keystore config was stored
		var count int64

		err = testDB.WithContext(t.Context()).Model(&model.KeystoreConfiguration{}).Count(&count).Error
		assert.NoError(t, err)
		assert.Equal(t, int64(1), count)
	})

	t.Run("Handle empty management access data", func(t *testing.T) {
		// Arrange
		testDB, _, _ := testutils.NewTestDB(t, testutils.TestDBConfig{
			Models: []driver.TenantTabler{&model.TenantConfig{}, &model.KeystoreConfiguration{}},
		})

		initKeystoreConfig := config.InitKeystoreConfig{
			Enabled:  true,
			Provider: "AWS",
			Value: config.KeystoreConfigValue{
				LocalityID:           testutils.TestLocalityID,
				CommonName:           testutils.TestDefaultKeystoreCommonName,
				ManagementAccessData: "",
				SupportedRegions:     testutils.SupportedRegions,
			},
		}
		// Act
		err := db.AddKeystoreFromConfig(t.Context(), testDB, initKeystoreConfig)
		// Assert
		assert.Error(t, err)
		assert.Equal(t, db.ErrEmptyManagementAccessData, err)
	})

	t.Run("Handle empty supported regions", func(t *testing.T) {
		// Arrange
		testDB, _, _ := testutils.NewTestDB(t, testutils.TestDBConfig{
			Models: []driver.TenantTabler{&model.TenantConfig{}, &model.KeystoreConfiguration{}},
		})

		initKeystoreConfig := config.InitKeystoreConfig{
			Enabled:  true,
			Provider: "AWS",
			Value: config.KeystoreConfigValue{
				LocalityID: testutils.TestLocalityID,
				CommonName: testutils.TestDefaultKeystoreCommonName,
				ManagementAccessData: fmt.Sprintf(
					"roleArn: %s\ntrustAnchorArn: %s\nprofileArn: %s",
					testutils.TestRoleArn,
					testutils.TestTrustAnchorArn,
					testutils.TestProfileArn,
				),
				SupportedRegions: []config.Region{},
			},
		}
		// Act
		err := db.AddKeystoreFromConfig(t.Context(), testDB, initKeystoreConfig)
		// Assert
		assert.Error(t, err)
		assert.Equal(t, db.ErrEmptySupportedRegions, err)
	})

	t.Run("Handle region with empty name", func(t *testing.T) {
		// Arrange
		testDB, _, _ := testutils.NewTestDB(t, testutils.TestDBConfig{
			Models: []driver.TenantTabler{&model.TenantConfig{}, &model.KeystoreConfiguration{}},
		})

		initKeystoreConfig := config.InitKeystoreConfig{
			Enabled:  true,
			Provider: "AWS",
			Value: config.KeystoreConfigValue{
				LocalityID: testutils.TestLocalityID,
				CommonName: testutils.TestDefaultKeystoreCommonName,
				ManagementAccessData: fmt.Sprintf(
					"roleArn: %s\ntrustAnchorArn: %s\nprofileArn: %s",
					testutils.TestRoleArn,
					testutils.TestTrustAnchorArn,
					testutils.TestProfileArn,
				),
				SupportedRegions: []config.Region{
					{Name: "", TechnicalName: "region-1"},
				},
			},
		}
		// Act
		err := db.AddKeystoreFromConfig(t.Context(), testDB, initKeystoreConfig)
		// Assert
		assert.Error(t, err)
		assert.Equal(t, db.ErrEmptyRegionName, err)
	})

	t.Run("Handle region with empty technical name", func(t *testing.T) {
		// Arrange
		testDB, _, _ := testutils.NewTestDB(t, testutils.TestDBConfig{
			Models: []driver.TenantTabler{&model.TenantConfig{}, &model.KeystoreConfiguration{}},
		})

		initKeystoreConfig := config.InitKeystoreConfig{
			Enabled:  true,
			Provider: "AWS",
			Value: config.KeystoreConfigValue{
				LocalityID: testutils.TestLocalityID,
				CommonName: testutils.TestDefaultKeystoreCommonName,
				ManagementAccessData: fmt.Sprintf(
					"roleArn: %s\ntrustAnchorArn: %s\nprofileArn: %s",
					testutils.TestRoleArn,
					testutils.TestTrustAnchorArn,
					testutils.TestProfileArn,
				),
				SupportedRegions: []config.Region{
					{Name: "Region 1", TechnicalName: ""},
				},
			},
		}
		// Act
		err := db.AddKeystoreFromConfig(t.Context(), testDB, initKeystoreConfig)
		// Assert
		assert.Error(t, err)
		assert.Equal(t, db.ErrEmptyRegionTechName, err)
	})
}
