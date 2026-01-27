# Client Headers Generator

This utility generates authentication headers for testing CMK API endpoints based on client data. 
This client data must be attributes of the HTTP header of your request.
It is only intended to be used for local testing.

As successful authorization check requires the groups which are within the client data also within the DB of your local CMK tenant.

## Usage

```bash
go run generate_client_headers.go -json <path-to-tenantAuditor_clientData.json>
```

## Example

```bash
# Using the provided example file
go run generate_client_headers.go -json tenantAdmin_clientData.json
```

## Client Data JSON Format

The JSON file should contain the following fields:

```json
{
  "identifier": "user123@example.com",   // User identifier (subject)
  "type": "user",                        // Client type
  "mail": "user123@example.com",         // Email address
  "reg": "us-east-1",                    // Region
  "AuthContext": {                       // Authentication context
    "issuer": "https://auth.example.com", // Token issuer
    "client_id": "client-12345"          // Client ID (optional)
  },
  "groups": ["admin", "users"],          // User groups (array)
  "kid": "key001",                       // Key ID
  "alg": "RS256"                         // Signature algorithm
}
```

## Output

The program will output:
- `x-client-data` header value
- `x-client-data-signature` header value

## Prerequisites

- The private key file must exist at `../../env/secret/signing-keys/private_key01.pem`, this can be generated using `generate-signing-keys` in `Makefile`.
