package xss_test

import (
	"encoding/json"
	"maps"
	"net/http"
	"testing"

	"github.com/bartventer/gorm-multitenancy/v8/pkg/driver"
	"github.com/stretchr/testify/assert"

	multitenancy "github.com/bartventer/gorm-multitenancy/v8"

	"github.com/openkcm/cmk/internal/api/cmkapi"
	"github.com/openkcm/cmk/internal/config"
	"github.com/openkcm/cmk/internal/constants"
	"github.com/openkcm/cmk/internal/model"
	"github.com/openkcm/cmk/internal/repo/sql"
	"github.com/openkcm/cmk/internal/testutils"
	integrationutils "github.com/openkcm/cmk/test/integration/integration_utils"
	cmkcontext "github.com/openkcm/cmk/utils/context"
	"github.com/openkcm/cmk/utils/ptr"
)

const providerTest = "TEST"

var (
	ksConfig            = testutils.NewKeystoreConfig(func(_ *model.KeystoreConfiguration) {})
	keystoreDefaultCert = testutils.NewCertificate(func(c *model.Certificate) {
		c.Purpose = model.CertificatePurposeKeystoreDefault
		c.CommonName = testutils.TestDefaultKeystoreCommonName
	})
	tenantDefaultCert = testutils.NewCertificate(func(c *model.Certificate) {
		c.Purpose = model.CertificatePurposeTenantDefault
		c.CommonName = testutils.TestDefaultKeystoreCommonName
	})
)

func startAPIAndDBForKey(t *testing.T) (*multitenancy.DB, cmkapi.ServeMux, string) {
	t.Helper()

	cfg := &config.Config{
		Database: integrationutils.DB,
	}
	integrationutils.StartPostgresSQL(t, &cfg.Database)

	dbConfig := testutils.TestDBConfig{
		Models: []driver.TenantTabler{
			&model.Key{},
			&model.KeyVersion{},
			&model.System{},
			&model.KeyConfiguration{},
			&model.TenantConfig{},
			&model.Certificate{},
			&model.ImportParams{},
			&model.KeystoreConfiguration{},
		}}
	db, tenants, _ := testutils.NewTestDB(t, dbConfig,
		testutils.WithDatabase(cfg.Database),
	)

	sv := testutils.NewAPIServer(t, db,
		testutils.TestAPIServerConfig{Plugins: []testutils.MockPlugin{testutils.KeyStorePlugin}})

	return db, sv, tenants[0]
}

func TestKeyController_ForXSS(t *testing.T) {
	db, sv, tenant := startAPIAndDBForKey(t)

	keyConfig := testutils.NewKeyConfig(func(_ *model.KeyConfiguration) {})
	tenantDefaultCert := testutils.NewCertificate(func(_ *model.Certificate) {})

	ctx := cmkcontext.CreateTenantContext(t.Context(), tenant)

	r := sql.NewRepository(db)

	testutils.CreateTestEntities(
		ctx,
		t,
		r,
		tenantDefaultCert,
		keyConfig,
		ksConfig,
		keystoreDefaultCert,
	)

	baseKey := map[string]any{
		"name":               "test-key",
		"type":               string(cmkapi.KeyTypeSYSTEMMANAGED),
		"keyConfigurationID": keyConfig.ID,
		"provider":           providerTest,
		"algorithm":          string(cmkapi.KeyAlgorithmAES256),
		"region":             "us-west-2",
		"description":        "test key",
		"enabled":            true,
	}

	// Create the mutator function
	requestMut := testutils.NewMutator(func() map[string]any {
		// Create a copy of the base map
		baseMap := make(map[string]any)
		maps.Copy(baseMap, baseKey)

		return baseMap
	})

	tests := []struct {
		name      string
		inputMap  map[string]any
		outputMap map[string]any
	}{
		{
			name:      "POST Key - no XSS",
			inputMap:  requestMut(),
			outputMap: requestMut(),
		},
		{
			name: "POST Key - Standard UUID not affected",
			inputMap: requestMut(func(m *map[string]any) {
				(*m)["name"] = "test-key-0"
				(*m)["description"] = "10d90855-cf4a-4396-8db7-caf41171766f"
			}),
			outputMap: requestMut(func(m *map[string]any) {
				(*m)["name"] = "test-key-0"
				(*m)["description"] = "10d90855-cf4a-4396-8db7-caf41171766f"
			}),
		},
		{
			name: "POST Key - XSS on description",
			inputMap: requestMut(func(m *map[string]any) {
				(*m)["name"] = "test-key-1"
				(*m)["description"] = "Hello <STYLE>.XSS{background-image:url" +
					"(\"javascript:alert('XSS')\");}</STYLE><A CLASS=XSS></A>World"
			}),
			outputMap: requestMut(func(m *map[string]any) {
				(*m)["name"] = "test-key-1"
				(*m)["description"] = "Hello World"
			}),
		},
		{
			name: "POST Key - XSS on description - embedded trick",
			inputMap: requestMut(func(m *map[string]any) {
				(*m)["name"] = "test-key-2"
				(*m)["description"] = "Hello <STYLE>.XSS{background-image" +
					"<STYLE>.XSS{background-image:url(\"javascript:alert('XSS')\");}" +
					"</STYLE><A CLASS=XSS></A>:url(\"javascript:alert('XSS')\");}" +
					"</STYLE><A CLASS=XSS></A>World"
			}),
			outputMap: requestMut(func(m *map[string]any) {
				(*m)["name"] = "test-key-2"
				(*m)["description"] = "Hello :url(&#34;javascript:alert(&#39;XSS&#39;)&#34;);}World"
			}),
		},
		{
			name: "POST Key - XSS on description - simple tags",
			inputMap: requestMut(func(m *map[string]any) {
				(*m)["name"] = "test-key-3"
				(*m)["description"] = "<STYLE></STYLE>"
			}),
			outputMap: requestMut(func(m *map[string]any) {
				(*m)["name"] = "test-key-3"
				(*m)["description"] = ""
			}),
		},
		{
			name: "POST Key - XSS on description - simple tags - embedded",
			inputMap: requestMut(func(m *map[string]any) {
				(*m)["name"] = "test-key-4"
				(*m)["description"] = "<ST<STYLE></STYLE>YLE></ST<STYLE></STYLE>YLE>"
			}),
			outputMap: requestMut(func(m *map[string]any) {
				(*m)["name"] = "test-key-4"
				(*m)["description"] = "YLE&gt;YLE&gt;"
			}),
		},
		{
			name: "POST Key - Just javascript",
			inputMap: requestMut(func(m *map[string]any) {
				(*m)["name"] = "test-key-5"
				(*m)["description"] = "url(\"javascript:alert('XSS')\");"
			}),
			outputMap: requestMut(func(m *map[string]any) {
				(*m)["name"] = "test-key-5"
				(*m)["description"] = "url(&#34;javascript:alert(&#39;XSS&#39;)&#34;);"
			}),
		},
		{
			// We don't actually santise anything which is JSON type. The escaping would likely break the JSON.
			name: "POST Key - JSON like",
			inputMap: requestMut(func(m *map[string]any) {
				(*m)["name"] = "test-key-6"
				(*m)["description"] = "{\"str\":\"test\",\"nest\":{\"a\":1,\"b\":\"2\"},\"array\":[\"a\",1,2]}"
			}),
			outputMap: requestMut(func(m *map[string]any) {
				(*m)["name"] = "test-key-6"
				(*m)["description"] = "{&#34;str&#34;:&#34;test&#34;,&#34;nest&#34;" +
					":{&#34;a&#34;:1,&#34;b&#34;:&#34;2&#34;},&#34;array&#34;:[&#34;a&#34;,1,2]}"
			}),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := testutils.MakeHTTPRequest(t, sv, testutils.RequestOptions{
				Method:   http.MethodPost,
				Endpoint: "keys",
				Tenant:   tenant,
				Body:     testutils.WithJSON(t, tt.inputMap),
			})

			assert.Equal(t, http.StatusCreated, w.Code)

			postResponse := testutils.GetJSONBody[cmkapi.Key](t, w)

			w = testutils.MakeHTTPRequest(t, sv, testutils.RequestOptions{
				Method:   http.MethodGet,
				Endpoint: "/keys/" + postResponse.Id.String(),
				Tenant:   tenant,
			})

			assert.Equal(t, http.StatusOK, w.Code)

			getResponse := testutils.GetJSONBody[cmkapi.Key](t, w)
			if tt.outputMap["description"] != "" {
				assert.Equal(t, tt.outputMap["description"],
					*getResponse.Description)
			} else {
				assert.Nil(t, getResponse.Description)
			}
		})
	}
}

func TestKeyController_ForJSONXSS(t *testing.T) {
	db, sv, tenant := startAPIAndDBForKey(t)
	ctx := cmkcontext.CreateTenantContext(t.Context(), tenant)
	r := sql.NewRepository(db)

	kc := testutils.NewKeyConfig(func(_ *model.KeyConfiguration) {})

	key := testutils.NewKey(func(k *model.Key) {
		k.IsPrimary = true
		k.KeyType = constants.KeyTypeHYOK
		k.ManagementAccessData = json.RawMessage("{\"<>\":\"><\"}")
		k.CryptoAccessData = json.RawMessage("{\"<>\":\"test\"}")
		k.KeyConfigurationID = kc.ID
		k.Provider = providerTest
		k.NativeID = ptr.PointTo("sdsad")
	})

	testutils.CreateTestEntities(
		ctx,
		t,
		r,
		keystoreDefaultCert,
		tenantDefaultCert,
		key,
		ksConfig,
	)

	w := testutils.MakeHTTPRequest(t, sv, testutils.RequestOptions{
		Method:   http.MethodGet,
		Endpoint: "/keys/" + key.ID.String(),
		Tenant:   tenant,
	})

	assert.Equal(t, http.StatusOK, w.Code)

	response := testutils.GetJSONBody[cmkapi.Key](t, w)
	responseJSON, err := json.Marshal(response.AccessDetails.Management)
	assert.NoError(t, err)
	assert.JSONEq(t, "{\"\\u0026lt;\\u0026gt;\":\"\\u0026gt;\\u0026lt;\"}", string(responseJSON))
}
