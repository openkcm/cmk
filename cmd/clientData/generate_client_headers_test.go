package main

import (
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"flag"
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

	hdr, sig := generateHeaders(clientData, privateKey)
	assert.NotEmpty(t, hdr)
	assert.NotEmpty(t, sig)
}

func TestOutputHeaders(t *testing.T) {
	outputFile := "test_headers.txt"
	defer os.Remove(outputFile)

	outputHeaders("header-data", "header-signature", outputFile)

	b, err := os.ReadFile(outputFile)
	assert.NoError(t, err)
	assert.Contains(t, string(b), "x-client-data: header-data")
	assert.Contains(t, string(b), "x-client-data-signature: header-signature")
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

	clientData := loadClientData(tmpFile.Name())
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

func TestParseArguments(t *testing.T) {
	// Save and restore os.Args and flag.CommandLine
	origArgs := os.Args
	origFlag := flag.CommandLine

	defer func() {
		os.Args = origArgs
		flag.CommandLine = origFlag
	}()

	// Valid case
	os.Args = []string{"cmd", "-json", "client.json", "-out", "out.txt"}
	flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ExitOnError)
	jsonPath, outPath := parseArguments()
	//nolint:testifylint
	assert.Equal(t, "client.json", jsonPath)
	assert.Equal(t, "out.txt", outPath)

	// Error case: missing -json, should call os.Exit(1)
	os.Args = []string{"cmd"}
	flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ExitOnError)
	calledExit := false
	// Patch os.Exit
	origExit := osExit
	osExit = func(_ int) { calledExit = true; panic("exit") }

	defer func() { osExit = origExit }()
	defer func() {
		if r := recover(); r != nil {
			assert.True(t, calledExit, "os.Exit should be called when -json is missing")
		}
	}()

	parseArguments()
}
