package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/golang-jwt/jwt/v5"
	"github.com/openkcm/common-sdk/pkg/auth"
)

var osExit = os.Exit

const (
	filePermissions = 0600
	privateKeyPath  = "../../env/secret/signing-keys/private_key01.pem"
)

func main() {
	clientDataFile, outputFile := parseArguments()
	clientData := loadClientData(clientDataFile)
	privateKey := loadPrivateKey()

	clientDataHeader, signatureHeader := generateHeaders(clientData, privateKey)
	outputHeaders(clientDataHeader, signatureHeader, outputFile)
}

func parseArguments() (string, string) {
	var clientDataFile, outputFile string
	flag.StringVar(&clientDataFile, "json", "", "Path to the client data JSON file")
	flag.StringVar(&outputFile, "out", "", "Path to output file (optional)")
	flag.Parse()

	if clientDataFile == "" {
		log.Println("Usage: go run generate_client_headers.go -json <path-to-clientData.json> [-out <output-file>]")
		log.Println("Example: go run generate_client_headers.go -json tenantAdmin_clientData.json -out headers.txt")
		osExit(1)
	}

	return clientDataFile, outputFile
}

func loadClientData(clientDataFile string) *auth.ClientData {
	clientDataBytes, err := os.ReadFile(clientDataFile)
	if err != nil {
		log.Fatalf("Failed to read %s: %v", clientDataFile, err)
	}

	var clientDataMap map[string]any

	err = json.Unmarshal(clientDataBytes, &clientDataMap)
	if err != nil {
		log.Fatalf("Failed to parse %s: %v", clientDataFile, err)
	}

	// Build AuthContext map from nested AuthContext object in JSON
	authContext := make(map[string]string)

	if authContextRaw, ok := clientDataMap["AuthContext"]; ok {
		if authContextMap, ok := authContextRaw.(map[string]any); ok {
			for k, v := range authContextMap {
				if strVal, ok := v.(string); ok {
					authContext[k] = strVal
				}
			}
		}
	}

	return &auth.ClientData{
		Identifier:         getString(clientDataMap, "identifier"),
		Type:               getString(clientDataMap, "type"),
		Email:              getString(clientDataMap, "mail"),
		Region:             getString(clientDataMap, "reg"),
		Groups:             getStringArray(clientDataMap, "groups"),
		KeyID:              getString(clientDataMap, "kid"),
		SignatureAlgorithm: auth.SignatureAlgorithm(getString(clientDataMap, "alg")),
		AuthContext:        authContext,
	}
}

func loadPrivateKey() any {
	privateKeyBytes, err := os.ReadFile(privateKeyPath)
	if err != nil {
		log.Fatalf("Failed to read private key file %s: %v", privateKeyPath, err)
	}

	privateKey, err := jwt.ParseRSAPrivateKeyFromPEM(privateKeyBytes)
	if err != nil {
		log.Fatalf("Failed to parse RSA private key: %v", err)
	}

	return privateKey
}

func generateHeaders(clientData *auth.ClientData, privateKey any) (string, string) {
	clientDataHeader, signatureHeader, err := clientData.Encode(privateKey)
	if err != nil {
		log.Fatalf("Failed to encode and sign client data: %v", err)
	}

	return clientDataHeader, signatureHeader
}

func outputHeaders(clientDataHeader, signatureHeader, outputFile string) {
	if outputFile != "" {
		headerOutput := fmt.Sprintf(
			"x-client-data: %s\nx-client-data-signature: %s\n", clientDataHeader, signatureHeader,
		)

		err := os.WriteFile(outputFile, []byte(headerOutput), filePermissions)
		if err != nil {
			log.Fatalf("Failed to write to output file %s: %v", outputFile, err)
		}

		log.Printf("Headers written to %s", outputFile)
	}

	//nolint:forbidigo
	fmt.Println("Headers for testing:")
	//nolint:forbidigo
	fmt.Printf("x-client-data: %s\n", clientDataHeader)
	//nolint:forbidigo
	fmt.Printf("x-client-data-signature: %s\n", signatureHeader)
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
