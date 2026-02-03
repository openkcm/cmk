# Load Tests with k6

This directory contains load tests for the CMK registry service using k6.

## Prerequisites

1. **k6 installed**: [Installation guide](https://k6.io/docs/getting-started/installation/)
2. **Registry service running**: Make sure `make start-cmk` and `make helm-install-registry` have been executed
3. **Port-forward active**: Registry service should be accessible on `localhost:9092`

## Files

- `register-tenant-same.js`: Load test for concurrent tenant registration with fixed tenant ID
- `grpc_test.js`: Basic gRPC load test with TLS support (creates unique tenants)
- `grpc_test.sh`: Script to run gRPC test without TLS
- `grpc_test_with_tls.sh`: Script to run gRPC test with TLS
- `registry.proto`: Protocol buffer definition for the registry service (optional local copy, see below)

## Running the Tests

### Basic execution

```bash
cd test/load_tests
k6 run register-tenant-same.js
```

### Using helper scripts

```bash
# Run without TLS
./grpc_test.sh

# Run with TLS (requires certificates)
./grpc_test_with_tls.sh
```

### With custom parameters

```bash
k6 run register-tenant-same.js \
  --env TARGET=localhost:9092 \
  --env VUS=150 \
  --env DURATION=45s \
  --env TENANT_ID=test-tenant-001 \
  --env REGION=emea
```

### gRPC test with custom parameters

```bash
k6 run grpc_test.js \
  --env BROKER=localhost:9092 \
  --env USE_TLS=false \
  --env VUS=50 \
  --env DURATION=20s
```

## Environment Variables

### For `register-tenant-same.js`:
- `TARGET`: Registry service address (default: `localhost:9092`)
- `VUS`: Number of virtual users (default: `120`)
- `DURATION`: Test duration (default: `30s`)
- `TENANT_ID`: Tenant ID to use for testing (default: `race-acme-fixed-001`)
- `TENANT_NAME`: Tenant name (default: `Acme Corp Fixed`)
- `REGION`: Tenant region (default: `eu10`)
- `PROTO_PATH`: Path to proto file (default: `../../vendor/github.com/openkcm/api-sdk/proto/kms/api/cmk/registry/tenant/v1/tenant.proto`)

### For `grpc_test.js`:
- `BROKER`: Registry service address (default: `localhost:9092`)
- `USE_TLS`: Enable TLS (default: `false`)
- `CA_FILE`: Path to CA certificate file
- `CERT_FILE`: Path to client certificate file
- `KEY_FILE`: Path to client key file
- `INSECURE_SKIP_VERIFY`: Skip TLS verification (default: `false`)
- `VUS`: Number of virtual users (default: `30`)
- `DURATION`: Test duration (default: `10s`)
- `PROTO_PATH`: Path to proto file (default: `../../vendor/github.com/openkcm/api-sdk/proto/kms/api/cmk/registry/tenant/v1/tenant.proto`)

## Test Configuration

### `register-tenant-same.js`
- **Executor**: `constant-vus` - maintains constant virtual users
- **Thresholds**:
  - `grpc_req_duration`: 95th percentile should be under 1 second
  - `checks`: 98% of checks should pass
- **Purpose**: Tests concurrent registration of the same tenant (race condition testing)

### `grpc_test.js`
- **Executor**: Simple VUs and duration
- **Features**: Supports TLS with certificates
- **Purpose**: Basic gRPC load test with unique tenant creation

## Expected Behavior

- The test registers the same tenant multiple times concurrently
- `ALREADY_EXISTS` errors are expected and acceptable
- `INTERNAL` and `UNKNOWN` errors are not acceptable and will cause the test to fail

## Troubleshooting

### Port-forward not active

If you get connection errors, ensure the registry port-forward is active:

```bash
kubectl port-forward -n cmk svc/registry 9092:9092 &
```

### Proto file not found

The tests reference the proto file directly from the vendor directory (populated by `go mod vendor`). If you encounter proto file errors:

1. **Ensure vendor directory exists**: Run `go mod vendor` to populate the vendor directory
2. **Use local copy as fallback**: If vendor directory is not available, you can create a local copy:
   ```bash
   cp vendor/github.com/openkcm/api-sdk/proto/kms/api/cmk/registry/tenant/v1/tenant.proto test/load_tests/registry.proto
   ```
   Then set `PROTO_PATH=./registry.proto` when running the tests.

**Note**: Using the vendor directory directly ensures the proto file stays in sync with the `api-sdk` dependency version specified in `go.mod`.

