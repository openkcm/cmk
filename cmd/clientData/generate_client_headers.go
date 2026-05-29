package main

import (
	"crypto/rsa"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"os"

	"github.com/golang-jwt/jwt/v5"
	"github.com/openkcm/common-sdk/pkg/auth"
)

const (
	filePermissions = 0o600
	privateKeyPath  = "../../env/secret/signing-keys/private_key01.pem"
)

var (
	clientDataFile = flag.String("clientData", "", "Path to the client data JSON file")
	privateKey     = flag.String("key", privateKeyPath, "Path to the private key PEM file (use '-' for stdin)")
)

func main() {
	flag.Parse()
	err := run()
	if err != nil {
		log.Fatal(err)
	}
}

func run() error {
	clientData, err := loadClientData(*clientDataFile)
	if err != nil {
		return err
	}

	privateKey, err := loadPrivateKey(*privateKey)
	if err != nil {
		return err
	}

	clientDataHeader, signatureHeader, err := generateHeaders(clientData, privateKey)
	if err != nil {
		return err
	}
	//nolint:forbidigo // CLI tool outputs to stdout
	fmt.Printf("x-client-data: %s\n", clientDataHeader)
	//nolint:forbidigo // CLI tool outputs to stdout
	fmt.Printf("x-client-data-signature: %s\n", signatureHeader)

	return nil
}

func loadClientData(file string) (*auth.ClientData, error) {
	clientDataBytes, err := os.ReadFile(file)
	if err != nil {
		return nil, fmt.Errorf("failed to read %s: %w", file, err)
	}

	var clientData map[string]any

	err = json.Unmarshal(clientDataBytes, &clientData)
	if err != nil {
		return nil, fmt.Errorf("failed to parse %s: %w", file, err)
	}

	// Build AuthContext map from nested AuthContext object in JSON
	authContext := make(map[string]string)

	if authContextRaw, ok := clientData["AuthContext"]; ok {
		if authContextMap, ok := authContextRaw.(map[string]any); ok {
			for k, v := range authContextMap {
				if strVal, ok := v.(string); ok {
					authContext[k] = strVal
				}
			}
		}
	}

	return &auth.ClientData{
		Identifier:         getString(clientData, "identifier"),
		Type:               getString(clientData, "type"),
		Email:              getString(clientData, "mail"),
		Region:             getString(clientData, "reg"),
		Groups:             getStringArray(clientData, "groups"),
		KeyID:              getString(clientData, "kid"),
		SignatureAlgorithm: auth.SignatureAlgorithm(getString(clientData, "alg")),
		AuthContext:        authContext,
	}, nil
}

func loadPrivateKey(path string) (*rsa.PrivateKey, error) {
	var privateKeyBytes []byte
	var err error

	if path == "-" {
		privateKeyBytes, err = io.ReadAll(os.Stdin)
		if err != nil {
			return nil, fmt.Errorf("failed to read private key from stdin: %w", err)
		}
	} else {
		privateKeyBytes, err = os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("failed to read private key file %s: %w", path, err)
		}
	}

	privateKey, err := jwt.ParseRSAPrivateKeyFromPEM(privateKeyBytes)
	if err != nil {
		return nil, fmt.Errorf("failed to parse RSA private key: %w", err)
	}

	return privateKey, nil
}

func generateHeaders(clientData *auth.ClientData, privateKey any) (string, string, error) {
	clientDataHeader, signatureHeader, err := clientData.Encode(privateKey)
	if err != nil {
		return "", "", fmt.Errorf("failed to encode and sign client data: %w", err)
	}

	return clientDataHeader, signatureHeader, nil
}

// Helper function to get string from map
func getString(m map[string]any, key string) string {
	if val, ok := m[key]; ok {
		if str, ok := val.(string); ok {
			return str
		}
	}

	return ""
}

// Helper function to get string array from map
func getStringArray(m map[string]any, key string) []string {
	if val, ok := m[key]; ok {
		if arr, ok := val.([]any); ok {
			result := make([]string, len(arr))
			for i, v := range arr {
				if str, ok := v.(string); ok {
					result[i] = str
				}
			}

			return result
		}
	}

	return nil
}
