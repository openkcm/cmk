package main

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"os"
	"testing"

	"github.com/openkcm/common-sdk/pkg/auth"
	"github.com/stretchr/testify/assert"
)

func TestGenerateHeaders(t *testing.T) {
	clientData := &auth.ClientData{
		Identifier:         "identifier",
		Type:               "type",
		Email:              "mail@test.com",
		Region:             "reg",
		Groups:             []string{"group1", "group2"},
		KeyID:              "kid",
		SignatureAlgorithm: "RS256",
		AuthContext: map[string]string{
			"issuer":    "https://sso.company.com",
			"client_id": "client-12345",
		},
	}

	// Generate a valid RSA private key for testing
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	assert.NoError(t, err)

	hdr, sig, err := generateHeaders(clientData, privateKey)
	assert.NoError(t, err)
	assert.NotEmpty(t, hdr)
	assert.NotEmpty(t, sig)
}

func TestLoadClientData(t *testing.T) {
	clientDataMap := map[string]any{
		"identifier": "identifier",
		"type":       "admin",
		"mail":       "test@example.com",
		"reg":        "us-east",
		"groups":     []any{"group1", "group2"},
		"kid":        "keyid",
		"alg":        "RS256",
		"AuthContext": map[string]any{
			"issuer":    "https://sso.company.com",
			"client_id": "client-12345",
		},
	}
	jsonBytes, err := json.Marshal(clientDataMap)
	assert.NoError(t, err)

	tmpFile, err := os.CreateTemp(t.TempDir(), "clientData_*.json")
	assert.NoError(t, err)

	defer os.Remove(tmpFile.Name())

	_, err = tmpFile.Write(jsonBytes)
	assert.NoError(t, err)
	tmpFile.Close()

	clientData, err := loadClientData(tmpFile.Name())
	assert.NoError(t, err)
	assert.Equal(t, "identifier", clientData.Identifier)
	assert.Equal(t, "admin", clientData.Type)
	assert.Equal(t, "test@example.com", clientData.Email)
	assert.Equal(t, "us-east", clientData.Region)
	assert.Equal(t, []string{"group1", "group2"}, clientData.Groups)
	assert.Equal(t, "keyid", clientData.KeyID)
	assert.Equal(t, auth.SignatureAlgorithm("RS256"), clientData.SignatureAlgorithm)
	assert.Equal(t, map[string]string{
		"issuer":    "https://sso.company.com",
		"client_id": "client-12345",
	}, clientData.AuthContext)
}

func TestLoadPrivateKey(t *testing.T) {
	// Generate a valid RSA private key
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	assert.NoError(t, err)

	// Marshal to PEM format
	privateKeyBytes := x509.MarshalPKCS1PrivateKey(privateKey)
	pemBytes := pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: privateKeyBytes,
	})

	// Write to temp file
	tmpFile, err := os.CreateTemp(t.TempDir(), "private_key_*.pem")
	assert.NoError(t, err)
	defer os.Remove(tmpFile.Name())

	_, err = tmpFile.Write(pemBytes)
	assert.NoError(t, err)
	tmpFile.Close()

	// Test loading from file
	loadedKey, err := loadPrivateKey(tmpFile.Name())
	assert.NoError(t, err)
	assert.NotNil(t, loadedKey)
	assert.Equal(t, privateKey.N, loadedKey.N)
}

func TestLoadPrivateKeyFromStdin(t *testing.T) {
	// Generate a valid RSA private key
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	assert.NoError(t, err)

	// Marshal to PEM format
	privateKeyBytes := x509.MarshalPKCS1PrivateKey(privateKey)
	pemBytes := pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: privateKeyBytes,
	})

	// Create a pipe to simulate stdin
	r, w, err := os.Pipe()
	assert.NoError(t, err)

	// Save original stdin and restore after test
	originalStdin := os.Stdin
	defer func() { os.Stdin = originalStdin }()

	// Replace stdin with our pipe
	os.Stdin = r

	// Write PEM data to pipe in goroutine
	go func() {
		defer w.Close()
		_, _ = w.Write(pemBytes)
	}()

	// Test loading from stdin (using "-" as path)
	loadedKey, err := loadPrivateKey("-")
	assert.NoError(t, err)
	assert.NotNil(t, loadedKey)
	assert.Equal(t, privateKey.N, loadedKey.N)
}

func TestRun(t *testing.T) {
	// Generate a valid RSA private key
	pkey, err := rsa.GenerateKey(rand.Reader, 2048)
	assert.NoError(t, err)

	// Create client data JSON file
	clientDataMap := map[string]any{
		"identifier": "test-identifier",
		"type":       "admin",
		"mail":       "test@example.com",
		"reg":        "us-east",
		"groups":     []any{"admin", "users"},
		"kid":        "key-123",
		"alg":        "RS256",
		"AuthContext": map[string]any{
			"issuer":    "https://sso.company.com",
			"client_id": "client-12345",
		},
	}
	jsonBytes, err := json.Marshal(clientDataMap)
	assert.NoError(t, err)

	tmpClientData, err := os.CreateTemp(t.TempDir(), "clientData_*.json")
	assert.NoError(t, err)
	defer os.Remove(tmpClientData.Name())

	_, err = tmpClientData.Write(jsonBytes)
	assert.NoError(t, err)
	tmpClientData.Close()

	// Create private key PEM file
	privateKeyBytes := x509.MarshalPKCS1PrivateKey(pkey)
	pemBytes := pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: privateKeyBytes,
	})
	tmpKey, err := os.CreateTemp(t.TempDir(), "private_key_*.pem")
	assert.NoError(t, err)
	defer os.Remove(tmpKey.Name())

	_, err = tmpKey.Write(pemBytes)
	assert.NoError(t, err)
	tmpKey.Close()

	// Set flags
	*clientDataFile = tmpClientData.Name()
	*privateKey = tmpKey.Name()

	// Run the function
	err = run()
	assert.NoError(t, err)
}
