package mock_test

import (
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"

	"github.com/openkcm/cmk-core/internal/model"
	"github.com/openkcm/cmk-core/internal/repo"
	"github.com/openkcm/cmk-core/internal/repo/mock"
)

func TestCreate(t *testing.T) {
	tests := []struct {
		name        string
		CreateModel func() (any, repo.Resource)
		AssertFunc  func(any, any)
		expectedErr bool
	}{
		{
			name: "Create nil Failed",
			CreateModel: func() (any, repo.Resource) {
				return nil, nil
			},
			expectedErr: true,
		},
		{
			name: "Create Certificate Success",
			CreateModel: func() (any, repo.Resource) {
				id := uuid.New()
				data := model.Certificate{ID: id}

				return id, data
			},
			AssertFunc: func(id any, retrieved any) {
				result, ok := retrieved.(model.Certificate)
				assert.True(t, ok)
				assert.Equal(t, id, result.ID)
			},
		},
		{
			name: "Create Group Success",
			CreateModel: func() (any, repo.Resource) {
				id := uuid.New()
				data := model.Group{ID: id}

				return id, data
			},
			AssertFunc: func(id any, retrieved any) {
				result, ok := retrieved.(model.Group)
				assert.True(t, ok)
				assert.Equal(t, id, result.ID)
			},
		},
		{
			name: "Create Key Success",
			CreateModel: func() (any, repo.Resource) {
				id := uuid.New()
				data := model.Key{ID: id}

				return id, data
			},
			AssertFunc: func(id any, retrieved any) {
				result, ok := retrieved.(model.Key)
				assert.True(t, ok)
				assert.Equal(t, id, result.ID)
			},
		},
		{
			name: "Create KeyConfiguration Success",
			CreateModel: func() (any, repo.Resource) {
				id := uuid.New()
				data := model.KeyConfiguration{ID: id}

				return id, data
			},
			AssertFunc: func(id any, retrieved any) {
				result, ok := retrieved.(model.KeyConfiguration)
				assert.True(t, ok)
				assert.Equal(t, id, result.ID)
			},
		},
		{
			name: "Create KeystoreConfiguration Success",
			CreateModel: func() (any, repo.Resource) {
				id := uuid.New()
				data := model.KeystoreConfiguration{ID: id}

				return id, data
			},
			AssertFunc: func(id any, retrieved any) {
				result, ok := retrieved.(model.KeystoreConfiguration)
				assert.True(t, ok)
				assert.Equal(t, id, result.ID)
			},
		},
		{
			name: "Create Key Version Success",
			CreateModel: func() (any, repo.Resource) {
				id := "test version 1"
				data := model.KeyVersion{ExternalID: id}

				return id, data
			},
			AssertFunc: func(id any, retrieved any) {
				result, ok := retrieved.(model.KeyVersion)
				assert.True(t, ok)
				assert.Equal(t, id, result.ExternalID)
			},
		},
		{
			name: "Create Label Success",
			CreateModel: func() (any, repo.Resource) {
				id := uuid.New()
				data := model.KeyLabel{BaseLabel: model.BaseLabel{ID: id}}

				return id, data
			},
			AssertFunc: func(id any, retrieved any) {
				result, ok := retrieved.(model.KeyLabel)
				assert.True(t, ok)
				assert.Equal(t, id, result.ID)
			},
		},
		{
			name: "Create System Success",
			CreateModel: func() (any, repo.Resource) {
				id := uuid.New()
				data := model.System{ID: id}

				return id, data
			},
			AssertFunc: func(id any, retrieved any) {
				result, ok := retrieved.(model.System)
				assert.True(t, ok)
				assert.Equal(t, id, result.ID)
			},
		},
		{
			name: "Create KeyConfigurationTags Success",
			CreateModel: func() (any, repo.Resource) {
				id := uuid.New()
				data := model.KeyConfigurationTag{
					BaseTag: model.BaseTag{ID: id},
				}

				return id, data
			},
			AssertFunc: func(id any, retrieved any) {
				result, ok := retrieved.(model.KeyConfigurationTag)
				assert.True(t, ok)
				assert.Equal(t, id, result.ID)
			},
		},
		{
			name: "Create Tenant config Success",
			CreateModel: func() (any, repo.Resource) {
				id := "tenant config key 1"
				data := model.TenantConfig{Key: id}

				return id, data
			},
			AssertFunc: func(id any, retrieved any) {
				result, ok := retrieved.(model.TenantConfig)
				assert.True(t, ok)
				assert.Equal(t, id, result.Key)
			},
		},
		{
			name: "Create Workflow Success",
			CreateModel: func() (any, repo.Resource) {
				id := uuid.New()
				data := model.Workflow{ID: id}

				return id, data
			},
			AssertFunc: func(id any, retrieved any) {
				result, ok := retrieved.(model.Workflow)
				assert.True(t, ok)
				assert.Equal(t, id, result.ID)
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			db := mock.NewInMemoryDB()
			id, model := test.CreateModel()

			err := db.Create(model)

			if test.expectedErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)

				retrieved, err := db.Get(model)
				assert.NoError(t, err)
				test.AssertFunc(id, retrieved)
			}
		})
	}
}

//nolint:dupl,gocognit,cyclop,gocyclo
func TestGetAll(t *testing.T) {
	tests := []struct {
		name        string
		CreateModel func() (int, repo.Resource, []repo.Resource)
		AssertFunc  func(expected []repo.Resource, retrieved []repo.Resource)
	}{
		{
			name: "Get All Certificate Success",
			CreateModel: func() (int, repo.Resource, []repo.Resource) {
				data := []model.Certificate{
					{ID: uuid.New(), CommonName: "Test1"},
					{ID: uuid.New(), CommonName: "Test2"},
					{ID: uuid.New(), CommonName: "Test3"},
				}

				result := make([]repo.Resource, len(data))
				for i := range result {
					result[i] = data[i]
				}

				return len(data), model.Certificate{}, result
			},
			AssertFunc: func(expected []repo.Resource, retrieved []repo.Resource) {
				convertExpected, err := mock.ConvertSliceToModel[model.Certificate](expected)
				assert.NoError(t, err)
				converted, err := mock.ConvertSliceToModel[model.Certificate](retrieved)
				assert.NoError(t, err)

				for i := range converted {
					for j := range convertExpected {
						if converted[i].ID == convertExpected[j].ID {
							assert.Equal(t, convertExpected[j], converted[i])
						}
					}
				}
			},
		},
		{
			name: "Get All Group Success",
			CreateModel: func() (int, repo.Resource, []repo.Resource) {
				data := []model.Group{
					{ID: uuid.New(), Name: "Test1"},
					{ID: uuid.New(), Name: "Test2"},
					{ID: uuid.New(), Name: "Test3"},
				}

				result := make([]repo.Resource, len(data))
				for i := range result {
					result[i] = data[i]
				}

				return len(data), model.Group{}, result
			},
			AssertFunc: func(expected []repo.Resource, retrieved []repo.Resource) {
				convertExpected, err := mock.ConvertSliceToModel[model.Group](expected)
				assert.NoError(t, err)
				converted, err := mock.ConvertSliceToModel[model.Group](retrieved)
				assert.NoError(t, err)

				for i := range converted {
					for j := range convertExpected {
						if converted[i].ID == convertExpected[j].ID {
							assert.Equal(t, convertExpected[j], converted[i])
						}
					}
				}
			},
		},
		{
			name: "Get All Key Success",
			CreateModel: func() (int, repo.Resource, []repo.Resource) {
				data := []model.Key{
					{ID: uuid.New(), Name: "key1"},
					{ID: uuid.New(), Name: "key2"},
					{ID: uuid.New(), Name: "key3"},
				}

				result := make([]repo.Resource, len(data))
				for i := range result {
					result[i] = data[i]
				}

				return len(data), model.Key{}, result
			},
			AssertFunc: func(expected []repo.Resource, retrieved []repo.Resource) {
				convertExpected, err := mock.ConvertSliceToModel[model.Key](expected)
				assert.NoError(t, err)
				converted, err := mock.ConvertSliceToModel[model.Key](retrieved)
				assert.NoError(t, err)

				for i := range converted {
					for j := range convertExpected {
						if converted[i].ID == convertExpected[j].ID {
							assert.Equal(t, convertExpected[j], converted[i])
						}
					}
				}
			},
		},
		{
			name: "Get All Key Configuration Success",
			CreateModel: func() (int, repo.Resource, []repo.Resource) {
				data := []model.KeyConfiguration{
					{ID: uuid.New(), Name: "key1"},
					{ID: uuid.New(), Name: "key2"},
					{ID: uuid.New(), Name: "key3"},
				}

				result := make([]repo.Resource, len(data))
				for i := range result {
					result[i] = data[i]
				}

				return len(data), model.KeyConfiguration{}, result
			},
			AssertFunc: func(expected []repo.Resource, retrieved []repo.Resource) {
				convertExpected, err := mock.ConvertSliceToModel[model.KeyConfiguration](expected)
				assert.NoError(t, err)
				converted, err := mock.ConvertSliceToModel[model.KeyConfiguration](retrieved)
				assert.NoError(t, err)

				for i := range converted {
					for j := range convertExpected {
						if converted[i].ID == convertExpected[j].ID {
							assert.Equal(t, convertExpected[j], converted[i])
						}
					}
				}
			},
		},
		{
			name: "Get All Keystore Configuration Success",
			CreateModel: func() (int, repo.Resource, []repo.Resource) {
				data := []model.KeystoreConfiguration{
					{ID: uuid.New(), Provider: "AWS"},
					{ID: uuid.New(), Provider: "AWS"},
					{ID: uuid.New(), Provider: "AWS"},
				}

				result := make([]repo.Resource, len(data))
				for i := range result {
					result[i] = data[i]
				}

				return len(data), model.KeystoreConfiguration{}, result
			},
			AssertFunc: func(expected []repo.Resource, retrieved []repo.Resource) {
				convertExpected, err := mock.ConvertSliceToModel[model.KeystoreConfiguration](expected)
				assert.NoError(t, err)
				converted, err := mock.ConvertSliceToModel[model.KeystoreConfiguration](retrieved)
				assert.NoError(t, err)

				for i := range converted {
					for j := range convertExpected {
						if converted[i].ID == convertExpected[j].ID {
							assert.Equal(t, convertExpected[j], converted[i])
						}
					}
				}
			},
		},
		{
			name: "Get All Key Version Success",
			CreateModel: func() (int, repo.Resource, []repo.Resource) {
				data := []model.KeyVersion{
					{ExternalID: "test version 0", Version: 0},
					{ExternalID: "test version 1", Version: 1},
					{ExternalID: "test version 2", Version: 2},
				}

				result := make([]repo.Resource, len(data))
				for i := range result {
					result[i] = data[i]
				}

				return len(data), model.KeyVersion{}, result
			},
			AssertFunc: func(expected []repo.Resource, retrieved []repo.Resource) {
				convertExpected, err := mock.ConvertSliceToModel[model.KeyVersion](expected)
				assert.NoError(t, err)
				converted, err := mock.ConvertSliceToModel[model.KeyVersion](retrieved)
				assert.NoError(t, err)

				for i := range converted {
					for j := range convertExpected {
						if converted[i].ExternalID == convertExpected[j].ExternalID {
							assert.Equal(t, convertExpected[j], converted[i])
						}
					}
				}
			},
		},
		{
			name: "Get All Key Label Success",
			CreateModel: func() (int, repo.Resource, []repo.Resource) {
				data := []model.KeyLabel{
					{
						BaseLabel: model.BaseLabel{
							ID:    uuid.New(),
							Value: "Value1",
						},
					},
					{
						BaseLabel: model.BaseLabel{
							ID:    uuid.New(),
							Value: "Value2",
						},
					},
					{
						BaseLabel: model.BaseLabel{
							ID:    uuid.New(),
							Value: "Value3",
						},
					},
				}

				result := make([]repo.Resource, len(data))
				for i := range result {
					result[i] = data[i]
				}

				return len(data), model.KeyLabel{}, result
			},
			AssertFunc: func(expected []repo.Resource, retrieved []repo.Resource) {
				convertExpected, err := mock.ConvertSliceToModel[model.KeyLabel](expected)
				assert.NoError(t, err)
				converted, err := mock.ConvertSliceToModel[model.KeyLabel](retrieved)
				assert.NoError(t, err)

				for i := range converted {
					for j := range convertExpected {
						if converted[i].ID == convertExpected[j].ID {
							assert.Equal(t, convertExpected[j], converted[i])
						}
					}
				}
			},
		},
		{
			name: "Get All System Success",
			CreateModel: func() (int, repo.Resource, []repo.Resource) {
				data := []model.System{
					{ID: uuid.New()},
					{ID: uuid.New()},
					{ID: uuid.New()},
				}

				result := make([]repo.Resource, len(data))
				for i := range result {
					result[i] = data[i]
				}

				return len(data), model.System{}, result
			},
			AssertFunc: func(expected []repo.Resource, retrieved []repo.Resource) {
				convertExpected, err := mock.ConvertSliceToModel[model.System](expected)
				assert.NoError(t, err)
				converted, err := mock.ConvertSliceToModel[model.System](retrieved)
				assert.NoError(t, err)

				for i := range converted {
					for j := range convertExpected {
						if converted[i].ID == convertExpected[j].ID {
							assert.Equal(t, convertExpected[j], converted[i])
						}
					}
				}
			},
		},
		{
			name: "Get All KeyConfigurationTag Success",
			CreateModel: func() (int, repo.Resource, []repo.Resource) {
				data := []model.KeyConfigurationTag{
					{
						BaseTag: model.BaseTag{
							ID: uuid.New(), Value: "test1",
						},
					},
					{
						BaseTag: model.BaseTag{
							ID: uuid.New(), Value: "test2",
						},
					},
					{
						BaseTag: model.BaseTag{
							ID: uuid.New(), Value: "test3",
						},
					},
				}

				result := make([]repo.Resource, len(data))
				for i := range result {
					result[i] = data[i]
				}

				return len(data), model.KeyConfigurationTag{}, result
			},
			AssertFunc: func(expected []repo.Resource, retrieved []repo.Resource) {
				convertExpected, err := mock.ConvertSliceToModel[model.KeyConfigurationTag](expected)
				assert.NoError(t, err)
				converted, err := mock.ConvertSliceToModel[model.KeyConfigurationTag](retrieved)
				assert.NoError(t, err)

				for i := range converted {
					for j := range convertExpected {
						if converted[i].ID == convertExpected[j].ID {
							assert.Equal(t, convertExpected[j], converted[i])
						}
					}
				}
			},
		},
		{
			name: "Get All Tenant ConfigSuccess",
			CreateModel: func() (int, repo.Resource, []repo.Resource) {
				data := []model.TenantConfig{
					{Key: "key1"},
					{Key: "key2"},
					{Key: "key3"},
				}

				result := make([]repo.Resource, len(data))
				for i := range result {
					result[i] = data[i]
				}

				return len(data), model.TenantConfig{}, result
			},
			AssertFunc: func(expected []repo.Resource, retrieved []repo.Resource) {
				convertExpected, err := mock.ConvertSliceToModel[model.TenantConfig](expected)
				assert.NoError(t, err)
				converted, err := mock.ConvertSliceToModel[model.TenantConfig](retrieved)
				assert.NoError(t, err)

				for i := range converted {
					for j := range convertExpected {
						if converted[i].Key == convertExpected[j].Key {
							assert.Equal(t, convertExpected[j], converted[i])
						}
					}
				}
			},
		},
		{
			name: "Get All Workflow Success",
			CreateModel: func() (int, repo.Resource, []repo.Resource) {
				data := []model.Workflow{
					{ID: uuid.New(), State: "state1"},
					{ID: uuid.New(), State: "state2"},
					{ID: uuid.New(), State: "state3"},
				}

				result := make([]repo.Resource, len(data))
				for i := range result {
					result[i] = data[i]
				}

				return len(data), model.Workflow{}, result
			},
			AssertFunc: func(expected []repo.Resource, retrieved []repo.Resource) {
				convertExpected, err := mock.ConvertSliceToModel[model.Workflow](expected)
				assert.NoError(t, err)
				converted, err := mock.ConvertSliceToModel[model.Workflow](retrieved)
				assert.NoError(t, err)

				for i := range converted {
					for j := range convertExpected {
						if converted[i].ID == convertExpected[j].ID {
							assert.Equal(t, convertExpected[j], converted[i])
						}
					}
				}
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			db := mock.NewInMemoryDB()
			expectedCount, model, datas := test.CreateModel()

			for i := range datas {
				err := db.Create(datas[i])
				assert.NoError(t, err)
			}

			results, count := db.GetAll(model)
			assert.Equal(t, expectedCount, count)
			assert.NotNil(t, results)

			test.AssertFunc(datas, results)
		})
	}
}

func TestGet(t *testing.T) {
	// Arrange
	db := mock.NewInMemoryDB()
	keyID := uuid.New()
	key := model.Key{ID: keyID}
	err := db.Create(key)
	assert.NoError(t, err)

	// Act
	retrievedKey, err := db.Get(key)

	// Assert
	assert.NoError(t, err)

	result, ok := retrievedKey.(model.Key)
	assert.True(t, ok)
	assert.Equal(t, keyID, result.ID)
}

func TestUpdate(t *testing.T) {
	tests := []struct {
		name        string
		CreateModel func() (repo.Resource, repo.Resource, repo.Resource)
		AssertFunc  func(any, any)
		ExpectedErr bool
	}{
		{
			name: "Update Certificate Success",
			CreateModel: func() (repo.Resource, repo.Resource, repo.Resource) {
				certID := uuid.New()
				cert := model.Certificate{ID: certID, CommonName: "test1"}
				newCert := model.Certificate{ID: certID, CommonName: "test2"}
				getCert := model.Certificate{ID: certID}

				return cert, newCert, getCert
			},
			AssertFunc: func(expected any, retrieved any) {
				resultRetrieved, ok := retrieved.(model.Certificate)
				assert.True(t, ok)
				resultExpected, ok := expected.(model.Certificate)
				assert.True(t, ok)
				assert.Equal(t, resultExpected.ID, resultRetrieved.ID)
				assert.Equal(t, resultExpected.CommonName, resultRetrieved.CommonName)
			},
		},
		{
			name: "Update Certificate Failure",
			CreateModel: func() (repo.Resource, repo.Resource, repo.Resource) {
				cert := model.Certificate{ID: uuid.New(), CommonName: "test1"}
				newCert := model.Certificate{ID: uuid.New(), CommonName: "test2"}
				getCert := model.Certificate{ID: uuid.New()}

				return cert, newCert, getCert
			},
			ExpectedErr: true,
		},
		{
			name: "Update Group Success",
			CreateModel: func() (repo.Resource, repo.Resource, repo.Resource) {
				id := uuid.New()
				data := model.Group{ID: id, Name: "test1"}
				newData := model.Group{ID: id, Name: "test2"}
				getData := model.Group{ID: id}

				return data, newData, getData
			},
			AssertFunc: func(expected any, retrieved any) {
				resultRetrieved, ok := retrieved.(model.Group)
				assert.True(t, ok)
				resultExpected, ok := expected.(model.Group)
				assert.True(t, ok)
				assert.Equal(t, resultExpected.ID, resultRetrieved.ID)
				assert.Equal(t, resultExpected.Name, resultRetrieved.Name)
			},
		},
		{
			name: "Update Group Failure",
			CreateModel: func() (repo.Resource, repo.Resource, repo.Resource) {
				data := model.Group{ID: uuid.New(), Name: "test1"}
				newData := model.Group{ID: uuid.New(), Name: "test2"}
				getData := model.Group{ID: uuid.New()}

				return data, newData, getData
			},
			ExpectedErr: true,
		},
		{
			name: "Update Key Success",
			CreateModel: func() (repo.Resource, repo.Resource, repo.Resource) {
				keyID := uuid.New()
				key := model.Key{ID: keyID, Name: "test1"}
				newKey := model.Key{ID: keyID, Name: "test2"}
				getKey := model.Key{ID: keyID}

				return key, newKey, getKey
			},
			AssertFunc: func(expected any, retrieved any) {
				resultRetrieved, ok := retrieved.(model.Key)
				assert.True(t, ok)
				resultExpected, ok := expected.(model.Key)
				assert.True(t, ok)
				assert.Equal(t, resultExpected.ID, resultRetrieved.ID)
				assert.Equal(t, resultExpected.Name, resultRetrieved.Name)
			},
		},
		{
			name: "Update Key Failure",
			CreateModel: func() (repo.Resource, repo.Resource, repo.Resource) {
				key := model.Key{ID: uuid.New(), Name: "test1"}
				newKey := model.Key{ID: uuid.New(), Name: "test2"}
				getKey := model.Key{ID: uuid.New()}

				return key, newKey, getKey
			},
			ExpectedErr: true,
		},
		{
			name: "Update Key Configuration Success",
			CreateModel: func() (repo.Resource, repo.Resource, repo.Resource) {
				id := uuid.New()
				data := model.KeyConfiguration{ID: id, Name: "test1"}
				newData := model.KeyConfiguration{ID: id, Name: "test2"}
				getData := model.KeyConfiguration{ID: id}

				return data, newData, getData
			},
			AssertFunc: func(expected any, retrieved any) {
				resultRetrieved, ok := retrieved.(model.KeyConfiguration)
				assert.True(t, ok)
				resultExpected, ok := expected.(model.KeyConfiguration)
				assert.True(t, ok)
				assert.Equal(t, resultExpected.ID, resultRetrieved.ID)
				assert.Equal(t, resultExpected.Name, resultRetrieved.Name)
			},
		},
		{
			name: "Update Key Configuration Failure",
			CreateModel: func() (repo.Resource, repo.Resource, repo.Resource) {
				data := model.KeyConfiguration{ID: uuid.New(), Name: "test1"}
				newData := model.KeyConfiguration{ID: uuid.New(), Name: "test2"}
				getData := model.KeyConfiguration{ID: uuid.New()}

				return data, newData, getData
			},
			ExpectedErr: true,
		},
		{
			name: "Update Keystore Configuration Success",
			CreateModel: func() (repo.Resource, repo.Resource, repo.Resource) {
				id := uuid.New()
				data := model.KeystoreConfiguration{ID: id, Provider: "AWS"}
				newData := model.KeystoreConfiguration{ID: id, Provider: "AWS"}
				getData := model.KeystoreConfiguration{ID: id}

				return data, newData, getData
			},
			AssertFunc: func(expected any, retrieved any) {
				resultRetrieved, ok := retrieved.(model.KeystoreConfiguration)
				assert.True(t, ok)
				resultExpected, ok := expected.(model.KeystoreConfiguration)
				assert.True(t, ok)
				assert.Equal(t, resultExpected.ID, resultRetrieved.ID)
				assert.Equal(t, resultExpected.Provider, resultRetrieved.Provider)
			},
		},
		{
			name: "Update Keystore Configuration Failure",
			CreateModel: func() (repo.Resource, repo.Resource, repo.Resource) {
				data := model.KeystoreConfiguration{ID: uuid.New(), Provider: "AWS"}
				newData := model.KeystoreConfiguration{ID: uuid.New(), Provider: "AWS"}
				getData := model.KeystoreConfiguration{ID: uuid.New()}

				return data, newData, getData
			},
			ExpectedErr: true,
		},
		{
			name: "Update Key Version Success",
			CreateModel: func() (repo.Resource, repo.Resource, repo.Resource) {
				id := "testExternalID"
				data := model.KeyVersion{ExternalID: id, Version: 0}
				newData := model.KeyVersion{ExternalID: id, Version: 1}
				getData := model.KeyVersion{ExternalID: id}

				return data, newData, getData
			},
			AssertFunc: func(expected any, retrieved any) {
				resultRetrieved, ok := retrieved.(model.KeyVersion)
				assert.True(t, ok)
				resultExpected, ok := expected.(model.KeyVersion)
				assert.True(t, ok)
				assert.Equal(t, resultExpected.ExternalID, resultRetrieved.ExternalID)
				assert.Equal(t, resultExpected.Version, resultRetrieved.Version)
			},
		},
		{
			name: "Update Key Version Failure",
			CreateModel: func() (repo.Resource, repo.Resource, repo.Resource) {
				data := model.KeyVersion{ExternalID: "testExternalID1", Version: 0}
				newData := model.KeyVersion{ExternalID: "testExternalID2", Version: 1}
				getData := model.KeyVersion{ExternalID: "testExternalID3"}

				return data, newData, getData
			},
			ExpectedErr: true,
		},
		//nolint:dupl
		{
			name: "Update Key Label Success",
			CreateModel: func() (repo.Resource, repo.Resource, repo.Resource) {
				id := uuid.New()
				data := model.KeyLabel{
					BaseLabel: model.BaseLabel{ID: id, Value: "value1"},
				}
				newData := model.KeyLabel{
					BaseLabel: model.BaseLabel{ID: id, Value: "value2"},
				}
				getData := model.KeyLabel{
					BaseLabel: model.BaseLabel{ID: id},
				}

				return data, newData, getData
			},
			AssertFunc: func(expected any, retrieved any) {
				resultRetrieved, ok := retrieved.(model.KeyLabel)
				assert.True(t, ok)
				resultExpected, ok := expected.(model.KeyLabel)
				assert.True(t, ok)
				assert.Equal(t, resultExpected.ID, resultRetrieved.ID)
				assert.Equal(t, resultExpected.Value, resultRetrieved.Value)
			},
		},
		{
			name: "Update Key Label Failure",
			CreateModel: func() (repo.Resource, repo.Resource, repo.Resource) {
				data := model.KeyLabel{
					BaseLabel: model.BaseLabel{ID: uuid.New(), Value: "value1"},
				}
				newData := model.KeyLabel{
					BaseLabel: model.BaseLabel{ID: uuid.New(), Value: "value2"},
				}
				getData := model.KeyLabel{
					BaseLabel: model.BaseLabel{ID: uuid.New()},
				}

				return data, newData, getData
			},
			ExpectedErr: true,
		},
		{
			name: "Update System Success",
			CreateModel: func() (repo.Resource, repo.Resource, repo.Resource) {
				keyID := uuid.New()
				key := model.System{ID: keyID}
				newKey := model.System{ID: keyID}
				getKey := model.System{ID: keyID}

				return key, newKey, getKey
			},
			AssertFunc: func(expected any, retrieved any) {
				resultRetrieved, ok := retrieved.(model.System)
				assert.True(t, ok)
				resultExpected, ok := expected.(model.System)
				assert.True(t, ok)
				assert.Equal(t, resultExpected.ID, resultRetrieved.ID)
			},
		},
		{
			name: "Update System Failure",
			CreateModel: func() (repo.Resource, repo.Resource, repo.Resource) {
				key := model.System{ID: uuid.New()}
				newKey := model.System{ID: uuid.New()}
				getKey := model.System{ID: uuid.New()}

				return key, newKey, getKey
			},
			ExpectedErr: true,
		},
		//nolint:dupl
		{
			name: "Update Tag Success",
			CreateModel: func() (repo.Resource, repo.Resource, repo.Resource) {
				keyID := uuid.New()
				key := model.KeyConfigurationTag{BaseTag: model.BaseTag{
					ID: keyID, Value: "test1",
				}}
				newKey := model.KeyConfigurationTag{BaseTag: model.BaseTag{
					ID: keyID, Value: "test2",
				}}
				getKey := model.KeyConfigurationTag{BaseTag: model.BaseTag{
					ID: keyID,
				}}

				return key, newKey, getKey
			},
			AssertFunc: func(expected any, retrieved any) {
				resultRetrieved, ok := retrieved.(model.KeyConfigurationTag)
				assert.True(t, ok)
				resultExpected, ok := expected.(model.KeyConfigurationTag)
				assert.True(t, ok)
				assert.Equal(t, resultExpected.ID, resultRetrieved.ID)
				assert.Equal(t, resultExpected.Value, resultRetrieved.Value)
			},
		},
		{
			name: "Update Tag Failure",
			CreateModel: func() (repo.Resource, repo.Resource, repo.Resource) {
				key := model.KeyConfigurationTag{BaseTag: model.BaseTag{
					ID: uuid.New(), Value: "test1",
				}}
				newKey := model.KeyConfigurationTag{BaseTag: model.BaseTag{
					ID: uuid.New(), Value: "test2",
				}}
				getKey := model.KeyConfigurationTag{BaseTag: model.BaseTag{
					ID: uuid.New(),
				}}

				return key, newKey, getKey
			},
			ExpectedErr: true,
		},
		{
			name: "Update Tenant config Success",
			CreateModel: func() (repo.Resource, repo.Resource, repo.Resource) {
				keyID := uuid.New().String()
				key := model.TenantConfig{Key: keyID, Value: json.RawMessage("test1")}
				newKey := model.TenantConfig{Key: keyID, Value: json.RawMessage("test2")}
				getKey := model.TenantConfig{Key: keyID}

				return key, newKey, getKey
			},
			AssertFunc: func(expected any, retrieved any) {
				resultRetrieved, ok := retrieved.(model.TenantConfig)
				assert.True(t, ok)
				resultExpected, ok := expected.(model.TenantConfig)
				assert.True(t, ok)
				assert.Equal(t, resultExpected.Key, resultRetrieved.Key)
				assert.Equal(t, resultExpected.Value, resultRetrieved.Value)
			},
		},
		{
			name: "Update Tenant config Failure",
			CreateModel: func() (repo.Resource, repo.Resource, repo.Resource) {
				key := model.TenantConfig{Key: uuid.New().String(), Value: json.RawMessage("test1")}
				newKey := model.TenantConfig{Key: uuid.New().String(), Value: json.RawMessage("test2")}
				getKey := model.TenantConfig{Key: uuid.New().String()}

				return key, newKey, getKey
			},
			ExpectedErr: true,
		},
		{
			name: "Update Workflow Success",
			CreateModel: func() (repo.Resource, repo.Resource, repo.Resource) {
				keyID := uuid.New()
				key := model.Workflow{ID: keyID, State: "test1"}
				newKey := model.Workflow{ID: keyID, State: "test2"}
				getKey := model.Workflow{ID: keyID}

				return key, newKey, getKey
			},
			AssertFunc: func(expected any, retrieved any) {
				resultRetrieved, ok := retrieved.(model.Workflow)
				assert.True(t, ok)
				resultExpected, ok := expected.(model.Workflow)
				assert.True(t, ok)
				assert.Equal(t, resultExpected.ID, resultRetrieved.ID)
				assert.Equal(t, resultExpected.State, resultRetrieved.State)
			},
		},
		{
			name: "Update Workflow Failure",
			CreateModel: func() (repo.Resource, repo.Resource, repo.Resource) {
				key := model.Workflow{ID: uuid.New(), State: "test1"}
				newKey := model.Workflow{ID: uuid.New(), State: "test2"}
				getKey := model.Workflow{ID: uuid.New()}

				return key, newKey, getKey
			},
			ExpectedErr: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			db := mock.NewInMemoryDB()
			data, newData, getData := test.CreateModel()

			err := db.Create(data)
			assert.NoError(t, err)

			err = db.Update(newData)
			if test.ExpectedErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)

				retrievedData, err := db.Get(getData)
				assert.NoError(t, err)

				test.AssertFunc(newData, retrievedData)
			}
		})
	}
}

func TestDelete(t *testing.T) {
	tests := []struct {
		name        string
		CreateModel func() (repo.Resource, repo.Resource, repo.Resource)
	}{
		{
			name: "Delete Certificate Success",
			CreateModel: func() (repo.Resource, repo.Resource, repo.Resource) {
				id := uuid.New()
				data := model.Certificate{ID: id, CommonName: "test1"}
				dataToDelete := model.Certificate{ID: id}
				getData := model.Certificate{ID: id}

				return data, dataToDelete, getData
			},
		},
		{
			name: "Delete Group Success",
			CreateModel: func() (repo.Resource, repo.Resource, repo.Resource) {
				id := uuid.New()
				data := model.Group{ID: id, Name: "test1"}
				dataToDelete := model.Group{ID: id}
				getData := model.Group{ID: id}

				return data, dataToDelete, getData
			},
		},
		{
			name: "Delete Key Success",
			CreateModel: func() (repo.Resource, repo.Resource, repo.Resource) {
				id := uuid.New()
				data := model.Key{ID: id, Name: "test1"}
				dataToDelete := model.Key{ID: id}
				getData := model.Key{ID: id}

				return data, dataToDelete, getData
			},
		},
		{
			name: "Delete Key Configuration Success",
			CreateModel: func() (repo.Resource, repo.Resource, repo.Resource) {
				id := uuid.New()
				data := model.KeyConfiguration{ID: id, Name: "test1"}
				dataToDelete := model.KeyConfiguration{ID: id}
				getData := model.KeyConfiguration{ID: id}

				return data, dataToDelete, getData
			},
		},
		{
			name: "Delete Keystore Configuration Success",
			CreateModel: func() (repo.Resource, repo.Resource, repo.Resource) {
				id := uuid.New()
				data := model.KeystoreConfiguration{ID: id, Provider: "AWS"}
				dataToDelete := model.KeystoreConfiguration{ID: id}
				getData := model.KeystoreConfiguration{ID: id}

				return data, dataToDelete, getData
			},
		},
		{
			name: "Delete Key Version Success",
			CreateModel: func() (repo.Resource, repo.Resource, repo.Resource) {
				id := uuid.New().String()
				data := model.KeyVersion{ExternalID: id, Version: 0}
				dataToDelete := model.KeyVersion{ExternalID: id}
				getData := model.KeyVersion{ExternalID: id}

				return data, dataToDelete, getData
			},
		},
		{
			name: "Delete Key Label Success",
			CreateModel: func() (repo.Resource, repo.Resource, repo.Resource) {
				id := uuid.New()
				data := model.KeyLabel{BaseLabel: model.BaseLabel{ID: id, Value: "test1"}}
				dataToDelete := model.KeyLabel{BaseLabel: model.BaseLabel{ID: id}}
				getData := model.KeyLabel{BaseLabel: model.BaseLabel{ID: id}}

				return data, dataToDelete, getData
			},
		},
		{
			name: "Delete System Success",
			CreateModel: func() (repo.Resource, repo.Resource, repo.Resource) {
				id := uuid.New()
				data := model.System{ID: id}
				dataToDelete := model.System{ID: id}
				getData := model.System{ID: id}

				return data, dataToDelete, getData
			},
		},
		{
			name: "Delete Tag Success",
			CreateModel: func() (repo.Resource, repo.Resource, repo.Resource) {
				id := uuid.New()
				data := model.KeyConfigurationTag{BaseTag: model.BaseTag{
					ID: id, Value: "test1",
				}}
				dataToDelete := model.KeyConfigurationTag{BaseTag: model.BaseTag{
					ID: id,
				}}
				getData := model.KeyConfigurationTag{BaseTag: model.BaseTag{
					ID: id,
				}}

				return data, dataToDelete, getData
			},
		},
		{
			name: "Delete TenantConfig Success",
			CreateModel: func() (repo.Resource, repo.Resource, repo.Resource) {
				id := uuid.New().String()
				data := model.TenantConfig{Key: id, Value: json.RawMessage("test1")}
				dataToDelete := model.TenantConfig{Key: id}
				getData := model.TenantConfig{Key: id}

				return data, dataToDelete, getData
			},
		},
		{
			name: "Delete Workflow Success",
			CreateModel: func() (repo.Resource, repo.Resource, repo.Resource) {
				id := uuid.New()
				data := model.Workflow{ID: id, State: "test1"}
				dataToDelete := model.Workflow{ID: id}
				getData := model.Workflow{ID: id}

				return data, dataToDelete, getData
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			db := mock.NewInMemoryDB()
			data, toDelete, getData := test.CreateModel()
			err := db.Create(data)
			assert.NoError(t, err)

			err = db.Delete(toDelete)
			assert.NoError(t, err)

			retrievedKey, err := db.Get(getData)
			assert.Error(t, err)
			assert.Nil(t, retrievedKey)
		})
	}
}
