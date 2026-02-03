#!/bin/bash

# Script to generate test certificates for TLS testing
# These certificates are self-signed and should ONLY be used for local testing

set -e

CERT_DIR="./certs"
CA_KEY="$CERT_DIR/ca.key"
CA_CERT="$CERT_DIR/ca.crt"
SERVER_KEY="$CERT_DIR/server.key"
SERVER_CERT="$CERT_DIR/server.crt"
SERVER_CSR="$CERT_DIR/server.csr"
CLIENT_KEY="$CERT_DIR/client.key"
CLIENT_CERT="$CERT_DIR/client.crt"
CLIENT_CSR="$CERT_DIR/client.csr"

# Create certs directory
mkdir -p "$CERT_DIR"

echo "🔐 Generating test certificates for TLS testing..."
echo "⚠️  WARNING: These are self-signed certificates for TESTING ONLY"
echo ""

# Generate CA private key
echo "1. Generating CA private key..."
openssl genrsa -out "$CA_KEY" 2048

# Generate CA certificate (self-signed)
echo "2. Generating CA certificate..."
openssl req -new -x509 -days 365 -key "$CA_KEY" -out "$CA_CERT" \
  -subj "/C=US/ST=Test/L=Test/O=CMK Test/CN=Test CA"

# Generate server private key
echo "3. Generating server private key..."
openssl genrsa -out "$SERVER_KEY" 2048

# Generate server certificate signing request
echo "4. Generating server certificate signing request..."
openssl req -new -key "$SERVER_KEY" -out "$SERVER_CSR" \
  -subj "/C=US/ST=Test/L=Test/O=CMK Test/CN=localhost"

# Sign server certificate with CA
echo "5. Signing server certificate..."
openssl x509 -req -days 365 -in "$SERVER_CSR" -CA "$CA_CERT" -CAkey "$CA_KEY" \
  -CAcreateserial -out "$SERVER_CERT" \
  -extensions v3_req -extfile <(
    echo "[v3_req]"
    echo "subjectAltName=DNS:localhost,DNS:*.localhost,IP:127.0.0.1,IP:0.0.0.0"
  )

# Generate client private key
echo "6. Generating client private key..."
openssl genrsa -out "$CLIENT_KEY" 2048

# Generate client certificate signing request
echo "7. Generating client certificate signing request..."
openssl req -new -key "$CLIENT_KEY" -out "$CLIENT_CSR" \
  -subj "/C=US/ST=Test/L=Test/O=CMK Test Client/CN=test-client"

# Sign client certificate with CA
echo "8. Signing client certificate..."
openssl x509 -req -days 365 -in "$CLIENT_CSR" -CA "$CA_CERT" -CAkey "$CA_KEY" \
  -CAcreateserial -out "$CLIENT_CERT"

# Clean up CSR files
rm -f "$SERVER_CSR" "$CLIENT_CSR"

# Set permissions
chmod 600 "$CA_KEY" "$SERVER_KEY" "$CLIENT_KEY"
chmod 644 "$CA_CERT" "$SERVER_CERT" "$CLIENT_CERT"

echo ""
echo "✅ Certificates generated successfully!"
echo ""
echo "Generated files:"
echo "  - $CA_CERT (CA certificate)"
echo "  - $CA_KEY (CA private key)"
echo "  - $SERVER_CERT (Server certificate)"
echo "  - $SERVER_KEY (Server private key)"
echo "  - $CLIENT_CERT (Client certificate)"
echo "  - $CLIENT_KEY (Client private key)"
echo ""
echo "For k6 tests, you'll need:"
echo "  - CA_FILE: $CA_CERT"
echo "  - CERT_FILE: $CLIENT_CERT"
echo "  - KEY_FILE: $CLIENT_KEY"
echo ""
echo "⚠️  NOTE: These certificates are for TESTING ONLY and should NOT be used in production!"







