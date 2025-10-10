# Tenant Manager CLI

A command-line tool for managing tenants in the database.

## Local usage
### Compile the CLI
```shell
go build -o tm ./cmd/tenant-manager-cli/main.go
```

### Requirements
config.yaml file should be present in the same directory as the compiled binary. Example config.yaml:
```yaml
database:
  host:
    source: embedded
    value: localhost
  user:
    source: embedded
    value: postgres
  secret:
    source: embedded
    value: secret
  name: cmk
  port: "5432"
```

### Running commands
```shell
./tm <command> [flags]
```
Run:
```shell
./tm --help
```
to see all available commands.

## Commands

### Create tenant
```shell
./tm create -i <TENANT_ID> -r <REGION> -s <STATUS>
```

#### Parameters
- `-i`, `--id` (string, required): Unique identifier for the tenant.
- `-r`, `--region` (string, required): Region where the tenant is located.
- `-s`, `--status` (string, required): Status of the tenant (e.g., STATUS_ACTIVE, STATUS_BLOCKED).
 
### Get tenant by identifier
```shell    
./tm get -i <TENANT_ID>
```

#### Parameters
- `-i`, `--id` (string, required): Unique identifier for the tenant.

### List tenants
```shell    
./tm list
 ```

### Update tenant
```shell    
./tm update <TENANT_ID> -r <REGION> -s <STATUS>
 ```

#### Parameters
- `-i`, `--id` (string, required): Unique identifier for the tenant.
- `-r`, `--region` (string, required): Region where the tenant is located.
- `-s`, `--status` (string, required): Status of the tenant (e.g., STATUS_ACTIVE, STATUS_BLOCKED).

### Create TenantAdmin and TenantAuditor groups for tenant
```shell
./tm add-default-groups -i <TENANT_ID>
```

#### Parameters
- `-i`, `--id` (string, required): Unique identifier for the tenant.

### Delete tenant
```shell    
./tm delete -i <TENANT_ID>
 ```
#### Parameters
- `-i`, `--id` (string, required): Unique identifier for the tenant.


## Examples
```shell
# Create a tenant in region emea with status enabled
./tm create -i tenant1 -r emea -s STATUS_ACTIVE

# Get tenant details
./tm get -i tenant1

# Update tenant status
./tm update -i tenant1 -s STATUS_BLOCKED

# Add default groups to a tenant
./tm add-default-groups -i tenant1

# Delete a tenant
./tm delete -i tenant1
```

# Makefile Target

Makefile target is prepared, to run the CLI commands in cluster.
```shell
make tenant-cli -- create -i <TENANT_ID> -r <REGION> -s <STATUS>
```