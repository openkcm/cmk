package testutils

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/openkcm/common-sdk/pkg/commonfs/loader"
	"github.com/openkcm/common-sdk/pkg/storage/keyvalue"
	"github.com/stretchr/testify/require"

	"github.com/openkcm/cmk/internal/constants"
)

// GenerateTestKeyPair generates an RSA key pair for testing
// Returns a 2048-bit RSA private key suitable for RS256 signing
func GenerateTestKeyPair() (*rsa.PrivateKey, *rsa.PublicKey, error) {
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, nil, err
	}
	return privateKey, &privateKey.PublicKey, nil
}

// TestSigningKeyStorage holds test signing keys in memory
type TestSigningKeyStorage struct {
	storage        keyvalue.ReadOnlyStringToBytesStorage
	loader         *loader.Loader
	tempDir        string
	privateKeys    map[int]*rsa.PrivateKey
	cleanupFunc    func()
}

// Get retrieves a public key by ID
// Returns (value, found) to match keyvalue.ReadStorage interface
func (t *TestSigningKeyStorage) Get(keyID string) ([]byte, bool) {
	return t.storage.Get(keyID)
}

// IsEmpty returns whether the storage is empty
func (t *TestSigningKeyStorage) IsEmpty() bool {
	return t.storage.IsEmpty()
}

// List returns all key IDs in the storage
func (t *TestSigningKeyStorage) List() []string {
	return t.storage.List()
}

// GetPrivateKey retrieves a private key by ID for test signing
func (t *TestSigningKeyStorage) GetPrivateKey(keyID int) (*rsa.PrivateKey, bool) {
	key, ok := t.privateKeys[keyID]
	return key, ok
}

// Cleanup stops the loader and removes temporary files
func (t *TestSigningKeyStorage) Cleanup() {
	if t.cleanupFunc != nil {
		t.cleanupFunc()
	}
}

// NewTestSigningKeyStorage creates a signing key storage with pre-generated test keys
// Generates 3 key pairs (keyID 0, 1, 2) to simulate key rotation scenarios
// Returns storage that implements keyvalue.ReadOnlyStringToBytesStorage interface
func NewTestSigningKeyStorage(tb testing.TB) *TestSigningKeyStorage {
	tb.Helper()

	tmpDir := tb.TempDir()
	privateKeys := make(map[int]*rsa.PrivateKey)

	// Generate 3 key pairs for testing key rotation scenarios
	for keyID := range 3 {
		privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
		require.NoError(tb, err, "failed to generate private key")

		privateKeys[keyID] = privateKey

		// Write public key to PEM file
		pubASN1, err := x509.MarshalPKIXPublicKey(&privateKey.PublicKey)
		require.NoError(tb, err, "failed to marshal public key")

		pubPEM := pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: pubASN1})
		keyFile := filepath.Join(tmpDir, strconv.Itoa(keyID)+".pem")

		err = os.WriteFile(keyFile, pubPEM, 0o600)
		require.NoError(tb, err, "failed to write public key file")
	}

	// Create memory storage and loader for public keys
	memoryStorage := keyvalue.NewMemoryStorage[string, []byte]()
	signingKeysLoader, err := loader.Create(
		loader.OnPath(tmpDir),
		loader.WithExtension("pem"),
		loader.WithKeyIDType(loader.FileNameWithoutExtension),
		loader.WithStorage(memoryStorage),
	)
	require.NoError(tb, err, "failed to create signing keys loader")

	err = signingKeysLoader.Start()
	require.NoError(tb, err, "failed to load signing keys")

	storage := &TestSigningKeyStorage{
		storage:     memoryStorage,
		loader:      signingKeysLoader,
		tempDir:     tmpDir,
		privateKeys: privateKeys,
		cleanupFunc: func() {
			_ = signingKeysLoader.Close()
		},
	}

	tb.Cleanup(storage.Cleanup)

	return storage
}

// TestRoleGetter is a mock RoleGetter for testing that always returns a default role
type TestRoleGetter struct {
	DefaultRole constants.Role
}

// GetRoleFromIAM returns the configured default role (or TenantAdminRole if not set)
func (t *TestRoleGetter) GetRoleFromIAM(ctx context.Context, iamIdentifiers []string) (constants.Role, error) {
	if t.DefaultRole != "" {
		return t.DefaultRole, nil
	}
	return constants.TenantAdminRole, nil
}

// NewTestRoleGetter creates a TestRoleGetter with the default role set to TenantAdminRole
func NewTestRoleGetter() *TestRoleGetter {
	return &TestRoleGetter{
		DefaultRole: constants.TenantAdminRole,
	}
}

