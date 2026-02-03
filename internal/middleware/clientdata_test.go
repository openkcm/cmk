package middleware_test

import (
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/openkcm/common-sdk/pkg/auth"
	"github.com/openkcm/common-sdk/pkg/commonfs/loader"
	"github.com/openkcm/common-sdk/pkg/storage/keyvalue"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/openkcm/cmk/internal/api/cmkapi"
	"github.com/openkcm/cmk/internal/apierrors"
	"github.com/openkcm/cmk/internal/config"
	"github.com/openkcm/cmk/internal/constants"
	"github.com/openkcm/cmk/internal/manager"
	"github.com/openkcm/cmk/internal/middleware"
	"github.com/openkcm/cmk/internal/testutils"
	cmkcontext "github.com/openkcm/cmk/utils/context"
)

// testData holds test setup data
type testData struct {
	privateKeys        map[int]*rsa.PrivateKey
	signingKeysPath    string
	config             *config.Config
	signingKeysStorage keyvalue.ReadOnlyStringToBytesStorage
	signingKeysLoader  *loader.Loader
}

// testScenario defines a test case scenario
type testScenario struct {
	name           string
	setupFunc      func(t *testing.T, td *testData) (clientData, signature string)
	expectError    bool
	expectHttpCode int
}

// mockRoleGetter is a minimal mock implementation of manager.GroupManager for testing
// It can be configured to return specific roles for role validation testing
type mockRoleGetter struct {
	roles []constants.Role
	err   error
}

func (m *mockRoleGetter) GetRoleFromIAM(
	_ context.Context,
	groups []string,
) (constants.Role, error) {
	if m.err != nil {
		return "", m.err
	}

	if len(m.roles) > 1 {
		return "", manager.ErrMultipleRolesInGroups
	}

	if len(m.roles) == 0 {
		return "", nil
	}

	return m.roles[0], nil
}

// setupTestEnvironment creates keys, files, and returns test data
func setupTestEnvironment(t *testing.T) *testData {
	t.Helper()

	td := &testData{
		privateKeys: make(map[int]*rsa.PrivateKey),
	}

	tmpdir := t.TempDir()
	td.signingKeysPath = tmpdir

	// Generate 3 key pairs for testing also in case of key rotation
	// Explicitly using RS256 (RSA keys, SHA-256 for signing)
	for keyID := range 3 {
		privateKey, err := rsa.GenerateKey(rand.Reader, 2048) // RS256: RSA key
		require.NoError(t, err, "failed to generate private key")

		td.privateKeys[keyID] = privateKey

		// Write public key to file (RS256 public key)
		pubASN1, err := x509.MarshalPKIXPublicKey(&privateKey.PublicKey)
		require.NoError(t, err, "failed to marshal public key")

		pubPEM := pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: pubASN1})
		keyFile := filepath.Join(tmpdir, strconv.Itoa(keyID)+".pem")

		err = os.WriteFile(keyFile, pubPEM, 0o600)
		require.NoError(t, err, "failed to write public key file")
	}

	td.config = &config.Config{
		ClientData: config.ClientData{
			SigningKeysPath: td.signingKeysPath,
		},
	}

	// Create and initialize the Loader
	memoryStorage := keyvalue.NewMemoryStorage[string, []byte]()
	signingKeysLoader, err := loader.Create(
		loader.OnPath(td.signingKeysPath),
		loader.WithExtension("pem"),
		loader.WithKeyIDType(loader.FileNameWithoutExtension),
		loader.WithStorage(memoryStorage),
	)
	require.NoError(t, err, "failed to create the signing keys loader")

	td.signingKeysStorage = memoryStorage

	err = signingKeysLoader.Start()
	require.NoError(t, err, "failed to load signing keys")

	td.signingKeysLoader = signingKeysLoader

	defer func() {
		err := signingKeysLoader.Close()
		require.NoError(t, err, "failed to stop watcher")
	}()

	return td
}

// createValidClientData creates properly encoded and signed client data
func (td *testData) createValidClientData(t *testing.T, keyID int) (string, string, error) {
	t.Helper()

	privateKey := td.privateKeys[keyID]
	if privateKey == nil {
		return "", "", os.ErrNotExist
	}

	clientData := auth.ClientData{
		Identifier:         "test-identifier",
		Type:               "test-type",
		Email:              "test@example.com",
		Region:             "test-region",
		Groups:             []string{"group1", "group2"},
		KeyID:              strconv.Itoa(keyID),
		SignatureAlgorithm: auth.SignatureAlgorithmRS256, // Explicitly RS256
		AuthContext: map[string]string{
			"client_id":        "test-client-id",
			"issuer":           "https://example-issuer.com",
			"multitenancy_ref": "some-ref",
			"other_field":      "other-value",
			"irrelevant":       "should-be-ignored",
		},
	}

	jsonBytes, err := json.Marshal(clientData)
	if err != nil {
		return "", "", err
	}

	b64data := base64.RawURLEncoding.EncodeToString(jsonBytes)

	// Sign the data using RS256 (RSA + SHA-256)
	hash := crypto.SHA256.New()
	hash.Write([]byte(b64data))
	digest := hash.Sum(nil)

	sigBytes, err := rsa.SignPKCS1v15(rand.Reader, privateKey, crypto.SHA256, digest)
	if err != nil {
		return "", "", err
	}

	b64sig := base64.RawURLEncoding.EncodeToString(sigBytes)

	return b64data, b64sig, nil
}

// createCustomClientData creates client data with custom fields and signs it
func (td *testData) createCustomClientData(t *testing.T, clientData auth.ClientData) (string, string) {
	t.Helper()

	privateKey := td.privateKeys[0]
	require.NotNil(t, privateKey, "private key not found for keyID %d", 0)

	jsonBytes, err := json.Marshal(clientData)
	require.NoError(t, err)

	b64data := base64.RawURLEncoding.EncodeToString(jsonBytes)
	// Sign the data using RS256 (RSA + SHA-256)
	hash := crypto.SHA256.New()
	hash.Write([]byte(b64data))
	digest := hash.Sum(nil)
	sigBytes, err := rsa.SignPKCS1v15(rand.Reader, privateKey, crypto.SHA256, digest)
	require.NoError(t, err)

	b64sig := base64.RawURLEncoding.EncodeToString(sigBytes)

	return b64data, b64sig
}

func getTestScenarios() []testScenario {
	return []testScenario{
		{
			name: "valid_key_0",
			setupFunc: func(t *testing.T, td *testData) (string, string) {
				t.Helper()
				data, sig, err := td.createValidClientData(t, 0)
				require.NoError(t, err)

				return data, sig
			},
			expectError:    false,
			expectHttpCode: http.StatusOK,
		},
		{
			name: "valid_key_1",
			setupFunc: func(t *testing.T, td *testData) (string, string) {
				t.Helper()
				data, sig, err := td.createValidClientData(t, 1)
				require.NoError(t, err)

				return data, sig
			},
			expectError:    false,
			expectHttpCode: http.StatusOK,
		},
		{
			name: "valid_key_2",
			setupFunc: func(t *testing.T, td *testData) (string, string) {
				t.Helper()
				data, sig, err := td.createValidClientData(t, 2)
				require.NoError(t, err)

				return data, sig
			},
			expectError:    false,
			expectHttpCode: http.StatusOK,
		},
		{
			name: "no_client_data_header",
			setupFunc: func(_ *testing.T, _ *testData) (string, string) {
				return "", "" // No headers set
			},
			expectError:    true,
			expectHttpCode: http.StatusInternalServerError,
		},
		{
			name: "missing_signature_header",
			setupFunc: func(t *testing.T, td *testData) (string, string) {
				t.Helper()
				data, _, err := td.createValidClientData(t, 0)
				require.NoError(t, err)

				return data, "" // No signature
			},
			expectError:    true,
			expectHttpCode: http.StatusInternalServerError,
		},
		{
			name: "malformed_base64",
			setupFunc: func(_ *testing.T, _ *testData) (string, string) {
				return "not_base64!!", ""
			},
			expectError:    true,
			expectHttpCode: http.StatusInternalServerError,
		},
		{
			name: "malformed_json",
			setupFunc: func(_ *testing.T, _ *testData) (string, string) {
				return base64.RawURLEncoding.EncodeToString([]byte("not_json")), ""
			},
			expectError:    true,
			expectHttpCode: http.StatusInternalServerError,
		},
		{
			name: "signature_mismatch",
			setupFunc: func(t *testing.T, td *testData) (string, string) {
				t.Helper()
				// Create data with different content than signature
				wrongData, _, err := td.createValidClientData(t, 0)
				require.NoError(t, err)

				// Create signature for different data
				_, correctSig, err := td.createValidClientData(t, 0)
				require.NoError(t, err)

				// Modify the data slightly to make signature invalid
				return wrongData[:len(wrongData)-5] + "XXXXX", correctSig
			},
			expectError:    true,
			expectHttpCode: http.StatusInternalServerError,
		},
		{
			name: "public_key_missing",
			setupFunc: func(t *testing.T, td *testData) (string, string) {
				t.Helper()
				// Use keyID that doesn't exist
				td.privateKeys[99] = td.privateKeys[0] // Fake key for signing
				data, sig, err := td.createValidClientData(t, 99)
				require.NoError(t, err)
				delete(td.privateKeys, 99) // Remove it

				return data, sig
			},
			expectError:    true,
			expectHttpCode: http.StatusInternalServerError,
		},
		{
			name: "unsupported_algorithm",
			setupFunc: func(t *testing.T, td *testData) (string, string) {
				t.Helper()

				clientData := auth.ClientData{
					Identifier:         "test-identifier",
					Type:               "test-type",
					Email:              "test@example.com",
					Region:             "test-region",
					AuthContext:        map[string]string{"issuer": "test-issuer"},
					Groups:             []string{"group1", "group2"},
					KeyID:              "0",
					SignatureAlgorithm: "UNSUPPORTED",
				}

				return td.createCustomClientData(t, clientData)
			},
			expectError:    true,
			expectHttpCode: http.StatusInternalServerError,
		},
		{
			name: "try_to_be_system",
			setupFunc: func(t *testing.T, td *testData) (string, string) {
				t.Helper()

				clientData := auth.ClientData{
					Identifier:         constants.SystemUser.String(),
					Type:               "test-type",
					Email:              "test@example.com",
					Region:             "test-region",
					AuthContext:        map[string]string{"issuer": "test-issuer"},
					Groups:             []string{"group1", "group2"},
					KeyID:              "0",
					SignatureAlgorithm: auth.SignatureAlgorithmRS256,
				}

				return td.createCustomClientData(t, clientData)
			},
			expectError:    true,
			expectHttpCode: http.StatusInternalServerError,
		},
		{
			name: "missing_keyid",
			setupFunc: func(t *testing.T, td *testData) (string, string) {
				t.Helper()

				clientData := auth.ClientData{
					Identifier:         "test-identifier",
					Type:               "test-type",
					Email:              "test@example.com",
					Region:             "test-region",
					AuthContext:        map[string]string{"issuer": "test-issuer"},
					Groups:             []string{"group1", "group2"},
					SignatureAlgorithm: auth.SignatureAlgorithmRS256, // KeyID is missing
				}

				return td.createCustomClientData(t, clientData)
			},
			expectError:    true,
			expectHttpCode: http.StatusInternalServerError,
		},
	}
}

func TestClientDataMiddleware(t *testing.T) {
	td := setupTestEnvironment(t)
	scenarios := getTestScenarios()
	authContextFields := []string{"client_id", "issuer", "multitenancy_ref"}
	roleGetter := &mockRoleGetter{} // Empty roles for basic tests

	for _, scenario := range scenarios {
		t.Run(
			scenario.name, func(t *testing.T) {
				// Set up the test scenario
				clientData, signature := scenario.setupFunc(t, td)

				// Create middleware
				middlewareFunc := middleware.ClientDataMiddleware(
					td.signingKeysStorage, authContextFields, roleGetter,
				)

				// Create test handler
				var clientDataFromContext *auth.ClientData

				testHandler := http.HandlerFunc(
					func(w http.ResponseWriter, r *http.Request) {
						if !scenario.expectError {
							// Extract client data from context using the new approach
							var err error

							clientDataFromContext, err = cmkcontext.ExtractClientData(r.Context())
							if err != nil {
								clientDataFromContext = nil
							}
						}

						w.WriteHeader(http.StatusOK)
					},
				)

				// Apply middleware
				handler := middlewareFunc(testHandler)

				// Create request
				req := httptest.NewRequest(http.MethodGet, "https://example.com", nil)
				if clientData != "" {
					req.Header.Set(auth.HeaderClientData, clientData)
				}

				if signature != "" {
					req.Header.Set(auth.HeaderClientDataSignature, signature)
				}

				// Execute request
				w := httptest.NewRecorder()
				handler.ServeHTTP(w, req)

				// Assertions
				assert.Equal(t, scenario.expectHttpCode, w.Result().StatusCode)

				if scenario.expectError {
					// For error cases, context should not be populated with client data
					assert.Nil(t, clientDataFromContext)

					// Also verify that extracting from a fresh context returns an error
					req := httptest.NewRequest(http.MethodGet, "https://example.com", nil)
					ctx := req.Context()
					_, err := cmkcontext.ExtractClientData(ctx)
					assert.Error(t, err)
				} else {
					// For successful cases, verify context is properly populated with client data
					require.NotNil(t, clientDataFromContext)
					assert.Equal(t, "test@example.com", clientDataFromContext.Email)
					assert.Equal(t, []string{"group1", "group2"}, clientDataFromContext.Groups)
					assert.Equal(t, "test-region", clientDataFromContext.Region)
					assert.Equal(t, "test-type", clientDataFromContext.Type)
					assert.Equal(t, "test-identifier", clientDataFromContext.Identifier)
					assert.Equal(t, auth.SignatureAlgorithmRS256, clientDataFromContext.SignatureAlgorithm)
					assert.Equal(
						t, map[string]string{
							"client_id":        "test-client-id",
							"issuer":           "https://example-issuer.com",
							"multitenancy_ref": "some-ref",
						}, clientDataFromContext.AuthContext,
					)
				}
			},
		)
	}
}

func TestClientDataMiddleware_RoleValidation(t *testing.T) {
	td := setupTestEnvironment(t)
	authContextFields := []string{"client_id", "issuer", "multitenancy_ref"}

	testCases := []struct {
		name            string
		roles           []constants.Role
		clientGroups    []string
		expectError     bool
		expectHttpCode  int
		expectErrorCode string
	}{
		{
			name:           "single_role_key_admin",
			roles:          []constants.Role{constants.KeyAdminRole},
			clientGroups:   []string{"group1", "group2"},
			expectError:    false,
			expectHttpCode: http.StatusOK,
		},
		{
			name:           "single_role_tenant_admin",
			roles:          []constants.Role{constants.TenantAdminRole},
			clientGroups:   []string{"group1", "group2"},
			expectError:    false,
			expectHttpCode: http.StatusOK,
		},
		{
			name:           "single_role_tenant_auditor",
			roles:          []constants.Role{constants.TenantAuditorRole},
			clientGroups:   []string{"group1", "group2"},
			expectError:    false,
			expectHttpCode: http.StatusOK,
		},
		{
			name:            "mixed_roles_key_admin_and_tenant_admin",
			roles:           []constants.Role{constants.KeyAdminRole, constants.TenantAdminRole},
			clientGroups:    []string{"group1", "group2"},
			expectError:     true,
			expectHttpCode:  http.StatusForbidden,
			expectErrorCode: apierrors.MultipleRolesInGroupsCode,
		},
		{
			name:            "mixed_roles_key_admin_and_tenant_auditor",
			roles:           []constants.Role{constants.KeyAdminRole, constants.TenantAuditorRole},
			clientGroups:    []string{"group1", "group2"},
			expectError:     true,
			expectHttpCode:  http.StatusForbidden,
			expectErrorCode: apierrors.MultipleRolesInGroupsCode,
		},
		{
			name:            "mixed_roles_tenant_admin_and_tenant_auditor",
			roles:           []constants.Role{constants.TenantAdminRole, constants.TenantAuditorRole},
			clientGroups:    []string{"group1", "group2"},
			expectError:     true,
			expectHttpCode:  http.StatusForbidden,
			expectErrorCode: apierrors.MultipleRolesInGroupsCode,
		},
		{
			name:           "single_group_no_validation_needed",
			roles:          []constants.Role{},
			clientGroups:   []string{"group1"},
			expectError:    false,
			expectHttpCode: http.StatusOK,
		},
		{
			name:           "no_groups_access_denied",
			roles:          []constants.Role{},
			clientGroups:   []string{},
			expectError:    false,
			expectHttpCode: http.StatusForbidden,
		},
	}

	for _, tc := range testCases {
		t.Run(
			tc.name, func(t *testing.T) {
				// Create mock group manager with predefined roles
				mockGroupMgr := &mockRoleGetter{roles: tc.roles}

				// Create client data with the specified groups
				clientData := auth.ClientData{
					Identifier:         "test-identifier",
					Type:               "test-type",
					Email:              "test@example.com",
					Region:             "test-region",
					Groups:             tc.clientGroups,
					KeyID:              "0",
					SignatureAlgorithm: auth.SignatureAlgorithmRS256,
					AuthContext: map[string]string{
						"client_id":        "test-client-id",
						"issuer":           "https://example-issuer.com",
						"multitenancy_ref": "some-ref",
					},
				}

				clientDataStr, signature := td.createCustomClientData(t, clientData)

				// Create middleware
				middlewareFunc := middleware.ClientDataMiddleware(
					td.signingKeysStorage, authContextFields, mockGroupMgr,
				)

				// Create test handler
				var clientDataFromContext *auth.ClientData
				testHandler := http.HandlerFunc(
					func(w http.ResponseWriter, r *http.Request) {
						if !tc.expectError {
							var err error
							clientDataFromContext, err = cmkcontext.ExtractClientData(r.Context())
							if err != nil {
								clientDataFromContext = nil
							}
						}
						w.WriteHeader(http.StatusOK)
					},
				)

				// Apply middleware
				handler := middlewareFunc(testHandler)

				// Create request
				req := httptest.NewRequest(http.MethodGet, "https://example.com", nil)
				req.Header.Set(auth.HeaderClientData, clientDataStr)
				req.Header.Set(auth.HeaderClientDataSignature, signature)

				// Execute request
				w := httptest.NewRecorder()
				handler.ServeHTTP(w, req)

				// Assertions
				assert.Equal(t, tc.expectHttpCode, w.Result().StatusCode)
				if tc.expectErrorCode != "" {
					response := testutils.GetJSONBody[cmkapi.ErrorMessage](t, w)
					assert.Equal(t, apierrors.MultipleRolesInGroupsCode, response.Error.Code)
				}

				if tc.expectError {
					// For error cases, context should not be populated with client data
					require.Nil(t, clientDataFromContext)
				} else
				// For successful cases, verify context is properly populated with client data
				if len(tc.clientGroups) > 0 {
					require.NotNil(t, clientDataFromContext)
					assert.Equal(t, tc.clientGroups, clientDataFromContext.Groups)
				}
			},
		)
	}
}
