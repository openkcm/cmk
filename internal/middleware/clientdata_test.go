package middleware_test

import (
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
	"github.com/openkcm/common-sdk/pkg/commoncfg"
	"github.com/openkcm/common-sdk/pkg/commonfs/loader"
	"github.com/openkcm/common-sdk/pkg/storage/keyvalue"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/openkcm/cmk-core/internal/config"
	"github.com/openkcm/cmk-core/internal/flags"
	"github.com/openkcm/cmk-core/internal/middleware"
)

// testData holds test setup data
type testData struct {
	privateKeys        map[int]*rsa.PrivateKey
	signingKeysPath    string
	config             *config.Config
	featureGates       commoncfg.FeatureGates
	signingKeysStorage keyvalue.ReadOnlyStringToBytesStorage
	signingKeysLoader  *loader.Loader
}

// testScenario defines a test case scenario
type testScenario struct {
	name        string
	setupFunc   func(t *testing.T, td *testData) (clientData, signature string)
	expectError bool
	expectLog   string // Expected log level/message pattern
}

// setupTestEnvironment creates keys, files, and returns test data
func setupTestEnvironment(t *testing.T) *testData {
	t.Helper()

	td := &testData{
		privateKeys:  make(map[int]*rsa.PrivateKey),
		featureGates: commoncfg.FeatureGates{},
	}

	tmpdir := t.TempDir()
	td.signingKeysPath = tmpdir

	// Generate 3 key pairs for testing also in case of key rotation
	// Explicitly using RS256 (RSA keys, SHA-256 for signing)
	for keyID := range []int{0, 1, 2} {
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
		Subject:            "test-subject",
		Type:               "test-type",
		Email:              "test@example.com",
		Region:             "test-region",
		Groups:             []string{"group1", "group2"},
		KeyID:              strconv.Itoa(keyID),
		SignatureAlgorithm: auth.SignatureAlgorithmRS256, // Explicitly RS256
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
func (td *testData) createCustomClientData(t *testing.T, keyID int, clientData auth.ClientData) (string, string) {
	t.Helper()

	privateKey := td.privateKeys[keyID]
	require.NotNil(t, privateKey, "private key not found for keyID %d", keyID)

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

//nolint:funlen
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
			expectError: false,
		},
		{
			name: "valid_key_1",
			setupFunc: func(t *testing.T, td *testData) (string, string) {
				t.Helper()
				data, sig, err := td.createValidClientData(t, 1)
				require.NoError(t, err)

				return data, sig
			},
			expectError: false,
		},
		{
			name: "valid_key_2",
			setupFunc: func(t *testing.T, td *testData) (string, string) {
				t.Helper()
				data, sig, err := td.createValidClientData(t, 2)
				require.NoError(t, err)

				return data, sig
			},
			expectError: false,
		},
		{
			name: "no_client_data_header",
			setupFunc: func(_ *testing.T, _ *testData) (string, string) {
				return "", "" // No headers set
			},
			expectError: true,
			expectLog:   "INFO",
		},
		{
			name: "missing_signature_header",
			setupFunc: func(t *testing.T, td *testData) (string, string) {
				t.Helper()
				data, _, err := td.createValidClientData(t, 0)
				require.NoError(t, err)

				return data, "" // No signature
			},
			expectError: true,
			expectLog:   "WARN",
		},
		{
			name: "malformed_base64",
			setupFunc: func(_ *testing.T, _ *testData) (string, string) {
				return "not_base64!!", ""
			},
			expectError: true,
			expectLog:   "ERROR",
		},
		{
			name: "malformed_json",
			setupFunc: func(_ *testing.T, _ *testData) (string, string) {
				return base64.RawURLEncoding.EncodeToString([]byte("not_json")), ""
			},
			expectError: true,
			expectLog:   "ERROR",
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
			expectError: true,
			expectLog:   "ERROR",
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
			expectError: true,
			expectLog:   "ERROR",
		},
		{
			name: "public_key_invalid_pem",
			setupFunc: func(t *testing.T, td *testData) (string, string) {
				t.Helper()
				// Corrupt the public key file
				keyPath := filepath.Join(td.signingKeysPath, "0.pem")
				err := os.WriteFile(keyPath, []byte("not_a_pem"), 0o600)
				require.NoError(t, err)

				data, sig, err := td.createValidClientData(t, 0)
				require.NoError(t, err)

				return data, sig
			},
			expectError: true,
			expectLog:   "ERROR",
		},
		{
			name: "unsupported_algorithm",
			setupFunc: func(t *testing.T, td *testData) (string, string) {
				t.Helper()

				clientData := auth.ClientData{
					Subject:            "test-subject",
					Type:               "test-type",
					Email:              "test@example.com",
					Region:             "test-region",
					Groups:             []string{"group1", "group2"},
					KeyID:              "0",
					SignatureAlgorithm: "UNSUPPORTED",
				}

				return td.createCustomClientData(t, 0, clientData)
			},
			expectError: true,
			expectLog:   "ERROR",
		},
		{
			name: "missing_keyid",
			setupFunc: func(t *testing.T, td *testData) (string, string) {
				t.Helper()

				clientData := auth.ClientData{
					Subject:            "test-subject",
					Type:               "test-type",
					Email:              "test@example.com",
					Region:             "test-region",
					Groups:             []string{"group1", "group2"},
					SignatureAlgorithm: auth.SignatureAlgorithmRS256, // KeyID is missing
				}

				return td.createCustomClientData(t, 0, clientData)
			},
			expectError: true,
			expectLog:   "ERROR",
		},
	}
}

func TestClientDataMiddleware(t *testing.T) {
	td := setupTestEnvironment(t)
	scenarios := getTestScenarios()

	for _, scenario := range scenarios {
		t.Run(
			scenario.name, func(t *testing.T) {
				// Set up the test scenario
				clientData, signature := scenario.setupFunc(t, td)

				// Create middleware
				middlewareFunc := middleware.ClientDataMiddleware(
					&td.config.FeatureGates, td.signingKeysStorage,
				)

				// Create test handler
				var contextValues map[string]any

				testHandler := http.HandlerFunc(
					func(w http.ResponseWriter, r *http.Request) {
						if !scenario.expectError {
							// Capture context values for validation
							ctx := r.Context()
							contextValues = map[string]any{
								"subject": ctx.Value(middleware.ClientDataSubject),
								"email":   ctx.Value(middleware.ClientDataEmail),
								"groups":  ctx.Value(middleware.ClientDataGroups),
								"region":  ctx.Value(middleware.ClientDataRegion),
								"type":    ctx.Value(middleware.ClientDataType),
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
				assert.Equal(t, http.StatusOK, w.Result().StatusCode)

				if scenario.expectError {
					// For error cases, context should not be populated with client data values
					// the client data middleware should pass through without setting context
					req := httptest.NewRequest(http.MethodGet, "https://example.com", nil)
					ctx := req.Context()
					assert.Nil(t, ctx.Value(middleware.ClientDataSubject))
					assert.Nil(t, ctx.Value(middleware.ClientDataEmail))
				} else {
					// For successful cases, verify context is properly populated
					assert.Equal(t, "test@example.com", contextValues["email"])
					assert.Equal(t, []string{"group1", "group2"}, contextValues["groups"])
					assert.Equal(t, "test-region", contextValues["region"])
					assert.Equal(t, "test-type", contextValues["type"])
				}
			},
		)
	}
}

func TestClientDataMiddleware_FeatureGateDisabled(t *testing.T) {
	td := setupTestEnvironment(t)

	// Enable the disable feature gate (confusing but correct)
	td.config.FeatureGates = commoncfg.FeatureGates{
		flags.DisableClientDataComputation: true,
	}

	middlewareFunc := middleware.ClientDataMiddleware(&td.config.FeatureGates, td.signingKeysStorage)

	testHandler := http.HandlerFunc(
		func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
		},
	)

	handler := middlewareFunc(testHandler)
	req := httptest.NewRequest(http.MethodGet, "https://example.com", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Result().StatusCode)
}
