[![REUSE status](https://api.reuse.software/badge/github.com/openkcm/cmk-core)](https://api.reuse.software/info/github.com/openkcm/cmk-core)

# Customer Managed Key (CMK)

This repository contains the application and business logic for the
CMK (Customer-Managed-Keys) layer of Key Management Service.

<a name="contents"></a>

## Contents

- [CMK (Customer-Managed-Keys)](#cmk-customer-managed-keys)
    - [Contents](#contents)
    - [Dependencies](#dependencies)
    - [Prerequisite](#prerequisite)
    - [Local Execution](#local-execution)
        - [K3d Environment](#k3d-environment)
            - [Key Features](#key-features)
            - [Running](#running)
            - [Helm chart directory](#helm-chart-directory)
            - [Running aws-kms local mock](#running-aws-kms-local-mock)
            - [Troubleshooting](#troubleshooting)
                - [Credentials Issues](#credentials-issues)
                - [Application Startup Delay](#application-startup-delay)
                - [Application Startup Failure](#application-startup-failure)
        - [Swagger UI](#swagger-ui)
    - [Development](#development)
        - [Building](#building)
        - [Unit tests](#unit-tests)
            - [How to write Unit Tests](#how-to-write-unit-tests)
        - [Integration tests](#integration-tests)
        - [Debugging](#debugging)
        - [API Implementations](#api-implementations)
        - [HTTP Error Mapping](#http-error-mapping)
            - [How Error Mapping Works](#how-error-mapping-works)
            - [Adding New Error Mappings](#adding-new-error-mappings)
    - [Authors](#authors)
    - [Version History](#version-history)
    - [License](#license)

<a name="dependencies"></a>

## Dependencies

* [Go v1.23.0+](https://golang.org/)
* [GORM](https://github.com/go-gorm/gorm)
* [Docker](https://docs.docker.com/get-docker/) or [Colima](https://github.com/abiosoft/colima)
* [Docker-Compose](https://docs.docker.com/compose/) -> Currently only used for integration test setup. TODO Remove
* [Helm](https://helm.sh/docs/intro/install/)
* [K3d](https://k3d.io)

Note that not all of these programs may be required depending on your environment

<a name="prerequisite"></a>

## Prerequisite

CMK has external dependencies which require credentials. These are stored at `env/secret` which are created from `env/blueprints`.

Run the following command to generate the `env/secret` files to configure

```
make create-empty-secrets
```

### Keystore plugins
We need to set up keystore operations plugin (currently only AWS KMS is
supported) to run the application. The plugin is retrieved via a dependency
of the make command (as a cmk submodule).

The plugin will use certificate-based authentication to connect to AWS KMS
(for SYSTEM_MANAGED and Bring Your Own Key - BYOK key types) with pre-configured
ARNs of the Trust Anchor, Role and Profile.

There are two ways to configure keystore plugins:

#### Using pre-configured keystore configuration
ARNs for Trust Anchor, Role and Profile. These ARNs will be loaded from the
config file, and the initial key store configuration will be added to the
keystore pool, ready to be used.

To configure it, replace the values in `charts/values-dev.yaml` with real
ARNs for Trust Anchor, Role and Profile stored [here](https://password.wdf.sap.corp/passvault/#/pwd/0000556959).
```
initKeystoreConfig:
  enabled: true
  provider: AWS
  value:
    localityId: 12345678-90ab-cdef-1234-567890abcdef
    commonName: example.kms.aws
    managementAccessData: |
      roleArn: arn:aws:iam::123456789012:role/KMSServiceRoleAnywhereRole
      trustAnchorArn: arn:aws:rolesanywhere:eu-west-2:123456789012:trust-anchor/12345678-90ab-cdef-1234-567890abcdef
      profileArn: arn:aws:rolesanywhere:eu-west-2:123456789012:profile/12345678-90ab-cdef-1234-567890abcdef
```

**NOTE**:
- When the real AWS credentials are used, real keys will be created
  in the AWS account. **Remember to remove** them after the tests.
- The managementAccessData field must be formatted as a YAML block scalar (using `|`),
  preserving line breaks and indentation.

#### Using automated keystore pool
Keystore configuration will be created and added to a pool by calling
CreateKeystore method of the management keystore plugin. For AWS, this will call
out to OpenKCM Hyperscaler Management Service to create a new AWS account, set up
the required IAM roles, trust anchors and profiles, and return the ARNs to be
used for the keystore configuration.

*(NOTE: For local setup it is recommended to use pre-configured keystore pool
method, as keystore filling job will create real AWS accounts, which consumes
quotas and requires approval process to clean up.)*

To configure this, replace the values in `env/secret/keystore-plugins/management/`
folder with the following files:

* `certificate-service.json`: The content should match
  `env/secret/cert-issuer-plugins/service.json`. Landscape and subaccount ID
  are the second and third OU from the cert issuer subject pattern, respectively.
  The `rootCAUrl` can be filled with the link to
  [Root CA](.....)
* `iam-service.json`: Should include the AWS credentials for any existing AWS
  IAM service user that has permissions to assume role, i.e. with a policy
```
{
    "Version": "2012-10-17",
    "Statement": [
        {
            "Action": "sts:AssumeRole",
            "Effect": "Allow",
            "Resource": "*"
        }
    ]
}
```
* `hyperscaler.json`: First, populate the `responsibleUser` with your own
  email address. The `serviceAccount` field should contain the AWS IAM user
  whose credentials have been provided in `iam-service-user.json`. The rest of
  the fields can be retrieved
  [here](https://password.wdf.sap.corp/passvault/#/pwd/0000558799).

* `supported-regions.json`: Populate with the list of AWS regions that are
  supported for keystore creation. Technical names of the regions are for
  example `eu-west-1`, `us-east-1`, etc., and the names are human-readable
  identifiers, for example `EU (Ireland)`, `US East (N. Virginia)`, etc.

### Certificate Issuer

We need to set up certificate issuer to manage the creation of default
tenant certificates for HYOK and other client certificates.

To configure it, replace the values for real credentials for both `env/secret/cert-issuer-plugins/uaa.json` stored [here](https://password.wdf.sap.corp/passvault/#/pwd/0000530468).
and `env/secret/cert-issuer-plugins/service.json` stored [here](https://password.wdf.sap.corp/passvault/#/pwd/0000547449).

### System Information Plugin

There are two implementations of System Information Plugin available:
* ULI (Unified Landscape Introspection)
* CLD (Cloud Landscape Directory)

In order to run the full CMK workflow and correctly start the task-worker, one of the implementations has to be configured.
To select which plugin is used, one can specify `SIS_PLUGIN` in the Make target:

```sh
$ make start-cmk SIS_PLUGIN=uli
# or
$ make start-cmk SIS_PLUGIN=cld
```

> :warning: **If `SIS_PLUGIN` is not specified, the default value is `cld`**

1. CLD Plugin

   To configure the CLD plugin, download `uaa-config.json` file from [PassVault](https://password.wdf.sap.corp/passvault/index.html#/pwd/0000546858) and put it into `env/secret/uaa-config.json`.
   An empty `uaa-config.json` generated by `make create-empty-secrets` initially contains dummy credentials, which have to be replaced with real values from PassVault, otherwise some operations will fail.

2. ULI plugin
   To configure the ULI plugin, create credentials with [Broker Tools](https://github.tools.sap/DataCustodian/broker-tools) and place your certificate chain and private key into `secrets/sis-plugins/uli` folder, replacing `cert.pem` and `key.pem` files.

### Event Processor

Event processing utilizes the [Orbital](https://github.com/openkcm/orbital) to send
and process events. Orbital requires target AMQP message brokers to be configured.
Additionally, if mTLS is used, certificate files need to be provided in the `env/secret/event-processor` directory.
They include a CA certificate to verify the server, a client certificate, and a private key.

### Identity Management

We need to set up identity management for obtaining information
relating to the identities (eg user groups)

To configure it, replace the values in `env/secret/identity-management.json`
with real credentials generated from OpenKCM Identity Services, in an
Application, under "Client Authentication" Certificates.

### Notification

We need to set up notification service to manage the creation of notifications
with other services.

To configure it, replace the values in `env/secret/notification-plugins/endpoints.json`
real credentials stored[here](https://password.wdf.sap.corp/passvault/#/pwd/0000552536);
and `env/secret/notification-plugins/uua.json` with [here](https://password.wdf.sap.corp/passvault/#/pwd/0000542784);

### Client Data Signing

To sign data for client data within the HTTP requests, we need to set up a private key and public key, both can be found for local setup and testing in the `env/secret/signing-keys` directory after running

```shell
make generate-signing-keys
```

That key pair will be used to secure the requests. The private key can be used to sign the requests for tests and the related public key will be used to verify the signatures. How the sign a header can be found [here](https://github.com/openkcm/common-sdk/blob/main/pkg/auth/clientdata.go) function `Encode`.

<a name="localexe"></a>

## Local Execution

Please also see section [Debugging](#debugging) for details on how to debug
these environments.

<a name="withkd3"></a>

### K3d Environment

#### Key Features

1. **Clean Namespace**: Deletes all resources in the `cmk` namespace to ensure a clean environment.
2. **Install k3d**: Checks if `k3d` is installed; if not, it automatically installs it.
3. **Create/Recreate Cluster**: Creates or recreates a k3d cluster named `cmkcluster`.
4. **Import Docker Image**: Imports a Docker image into the k3d cluster's internal registry.
5. **Helm Release Management**: Automatically installs or upgrades the Helm release.
6. **Namespace Creation**: If the specified namespace does not exist, the command creates it automatically.
7. **Set up Postgresql database**: Applys Postgresql set up from bitnami repository.
8. **Import test data**: Import test data.
9. **Set up port forwarding**: Set up port forwarding so that the application is accessible on localhost.

#### Running

```bash
make start-cmk 
```

The application should be accessible on [http://localhost:8080](http://localhost:8080)
For example [http://localhost:8080/keys](http://localhost:8080/keys)

#### Helm chart directory

The Helm charts required for deployment are located in the ./chart directory.

#### Running aws-kms local mock

Pull Helm chart repository. The Helm charts required for deployment are located in the following repository:
[https://github.tools.sap/kms/helm-charts.git](https://github.tools.sap/kms/helm-charts.git)

set up env varible `CMK_HELM_CHART`, to point to 'charts' directory of helm-chart repository.
Example:

```bash
export CMK_HELM_CHART=/helm-charts/charts
```

Run:

```bash
make apply-kms-local-chart
```

#### Troubleshooting

##### Credentials Issues

If you encounter problems with Docker credentials (e.g., login or authentication
issues), you can modify the Docker configuration file to resolve them. The
credentials store used by Docker is specified in the `~/.docker/config.json` file.

1. Open the `~/.docker/config.json` file in a text editor.
2. Locate the `credsStore` field. It should look like this:

```json
{
  "credsStore": "osxkeychain" // for macOS
}
```

##### Application Startup Delay

The `cmk` application may take some time to fully start after deployment.
This is because it waits for the PostgreSQL database to become available.

##### Application Startup Failure

If the application does not start as expected:

1. Check the logs of the `cmk` application for messages about the database connection.

```bash
kubectl logs <cmk-pod-name> -n cmk
```

2. If running with Colima ensure that resources are sufficient. The following
   command has been deemed sufficient:

```bash
colima start --memory 4 --disk 150
```

<a name="swaggerui"></a>

### Swagger UI

Swagger UI allows to visualize and interact with the API’s resources. It is containerized and can be setup via:
`make swagger-ui`.

This will simply run a docker image which serves swagger-ui. It can be found at `localhost:8087/swagger`

<a name="development"></a>

## Development

<a name="building"></a>

### Building

Building can be via the following Make command:

```bash
make build
```

<a name="unittests"></a>

### Unit tests

Running tests can be done through a Make command:

```go
make test
```

#### How to write Unit Tests
Guidelines:
- **Should** test a small section of code, usually a function
- **Should** be idempotent and independent of other test input/outputs
- **Shouldn't** make calls to external services, if so it should use mock clients

[!NOTE]
Currently there are tests that are not following the guidelines mentioned.
Please fix them or create an enhancement ticket

To ensure consistency testutils where created. Please use them and enhance if needed in your use-case.
Refer to code documentation on the following functions for it's usage and available options.
- `testutils.NewTestDB(tb testing.TB, cfg TestDBConfig, opts ...TestDBConfigOpt) (*multitenancy.DB, []string)`
- `testutils.NewAPIServer(tb testing.TB, db *multitenancy.DB, testCfg TestAPIServerConfig) *http.ServeMux`
- `testutils.MakeHTTPRequest(tb testing.TB, server *http.ServeMux, opt RequestOptions) *httptest.ResponseRecorder`
    - `testutils.WithJSON(tb testing.TB, i any) io.Reader`
    - `testutils.WithString(tb testing.TB, i any) io.Reader`
    - `testutils.GetJSONBody\[t any\](tb testing.TB, w *httptest.ResponseRecorder)`
- `testutils.New<modelType>(m func(*model.<modelType>) *model<modelType>)`
- `testutils.NewGRPCSuite(tb testing.TB, services ...systemsgrpc.ServiceServer)`

<a name="integration-tests"></a>

### Integration tests

Running integration tests can be done through a Make command:

```go
make integration_test
```

NOTE: Some integration tests require credentials. You can refer to [Prerequisite chapter](#prerequisite) to setup those.
If no credentials are provided the tests are skipped!
<a name="debugging"></a>

### Debugging


Run the following command to get a list of your pods:

```bash
sudo kubectl get pod --all-namespaces
```

Then, using the relevant pod (usually of form `cmk-XXX-YYY`):

```bash
sudo kubectl logs -n cmk cmk-XXX-YYY
```

This should display any logs from the cmk application.

<a name="apiimplementations"></a>

### API Implementations

The API clients required for CMK can be generated from the OpenAPI spec.
We use [oapi-codegen](https://github.com/oapi-codegen/oapi-codegen) to generate Go Code based on the OpenAPI spec

In order to generate the clients, execute `make codegen` with one of the listed  `api` flag on `make codegen`

Example: `make codegen api=cmk`
<a name="httperrormapping"></a>

### Logging

CMK uses context-based logging via [slogctx](https://github.com/veqryn/slog-context), injecting a logger onto the context.

On API Requests, the logger is injected with default information on the logging middleware, and in other scenarios also later injected with relevant information.

- Static information can be added to all logs via values.yaml labels as [documentated](https://github.com/openkcm/common-sdk/blob/main/pkg/logger/logging.md) (ex. Target: CMK)
- Dynamic Information that's repeatable in a certain context should be injected into the logger, otherwise added as an attribute on the specific log

### HTTP Error Mapping

Our error mapping system automatically converts internal errors to structured API responses with appropriate HTTP status codes and meaningful error messages.
Each operation in our API has specific error mappings that are automatically
selected based on the operation ID.

#### How Error Mapping Works

The core of our error mapping system is the ErrorMap struct which associates internal errors
with standardized API responses:

```go
type ErrorMap struct {
 Error  []error              // Internal errors to match against
 Detail cmkapi.DetailedError // API response details
}
```

When an error occurs, the system:

* Finds the appropriate error mappings for that operation
* Matches the encountered error against all possible mappings
* Selects the best matching error response
* Returns a standardized error response to the client

#### Adding New Error Mappings

To add new error mappings for your feature, follow these steps:

1. Define Error Constants
   First, define your error constants in the apierrors package:

```go
var (
 ErrMyNewError = errors.New("description of the new error")
)
```

2. Create Error Mappings
   Add mappings to the appropriate entity's mapping slice (e.g., system, key, keyConfiguration):

```go
var system = []ErrorMap{
 // Existing mappings...
 {
  Error: []error{ErrMyNewError},
  Detail: cmkapi.DetailedError{
   Code:    "MY_NEW_ERROR_CODE",
   Message: "User-friendly error message",
   Status:  http.StatusBadRequest,
  },
 },
 // More specific mapping with multiple errors
 {
  Error: []error{ErrMyNewError, repo.ErrNotFound},
  Detail: cmkapi.DetailedError{
   Code:    "MY_NEW_ERROR_NOT_FOUND",
   Message: "Resource not found: detailed message",
   Status:  http.StatusNotFound,
  },
 },
}
```

How Errors Are Matched

* If there is an high prio API Error on the error chain, that API Error is selected
* If API Error chain contains errors not existing in the error they are ignored
* Mapping is done with the most number of matching errors
* If no matches are found, it returns a default internal server error

This allows for precise error handling when errors are wrapped or combined.

<a name="authors"></a>

## Support, Feedback, Contributing

This project is open to feature requests/suggestions, bug reports etc. via [GitHub issues](https://github.com/openkcm/cmk-core/issues). Contribution and feedback are encouraged and always welcome. For more information about how to contribute, the project structure, as well as additional contribution information, see our [Contribution Guidelines](CONTRIBUTING.md).

## Security / Disclosure
If you find any bug that may be a security problem, please follow our instructions at [in our security policy](https://github.com/openkcm/cmk-core/security/policy) on how to report it. Please do not create GitHub issues for security-related doubts or problems.

## Code of Conduct

We as members, contributors, and leaders pledge to make participation in our community a harassment-free experience for everyone. By participating in this project, you agree to abide by its [Code of Conduct](https://github.com/openkcm/.github/blob/main/CODE_OF_CONDUCT.md) at all times.

## Licensing

Copyright 2025 SAP SE or an SAP affiliate company and cmk contributors. Please see our [LICENSE](LICENSE) for copyright and license information. Detailed information including third-party components and their licensing/copyright information is available [via the REUSE tool](https://api.reuse.software/info/github.com/openkcm/cmk-core).
