-- Extends 00012 with CHECK constraints for the remaining enum-typed columns,
-- mirroring the Go-side Valuer/Scanner in internal/model and internal/api/cmkapi.
-- NOT VALID skips the full-table scan on rollout; new writes are still checked.

-- +goose Up
ALTER TABLE keys DROP CONSTRAINT IF EXISTS chk_keys_key_type;
ALTER TABLE keys ADD CONSTRAINT chk_keys_key_type
    CHECK (key_type IN ('BYOK', 'HYOK')) NOT VALID;

ALTER TABLE keys DROP CONSTRAINT IF EXISTS chk_keys_algorithm;
ALTER TABLE keys ADD CONSTRAINT chk_keys_algorithm
    CHECK (algorithm IN ('AES256')) NOT VALID;

ALTER TABLE import_params DROP CONSTRAINT IF EXISTS chk_import_params_wrapping_alg;
ALTER TABLE import_params ADD CONSTRAINT chk_import_params_wrapping_alg
    CHECK (wrapping_alg IN ('CKM_RSA_AES_KEY_WRAP', 'CKM_RSA_PKCS_OAEP')) NOT VALID;

ALTER TABLE import_params DROP CONSTRAINT IF EXISTS chk_import_params_hash_function;
ALTER TABLE import_params ADD CONSTRAINT chk_import_params_hash_function
    CHECK (hash_function IN ('SHA1', 'SHA256')) NOT VALID;

ALTER TABLE workflows DROP CONSTRAINT IF EXISTS chk_workflows_parameters_resource_type;
ALTER TABLE workflows ADD CONSTRAINT chk_workflows_parameters_resource_type
    CHECK (parameters_resource_type IS NULL OR parameters_resource_type IN ('KEY', 'KEY_CONFIGURATION')) NOT VALID;

ALTER TABLE systems DROP CONSTRAINT IF EXISTS chk_systems_type;
ALTER TABLE systems ADD CONSTRAINT chk_systems_type
    CHECK (type IN ('SYSTEM', 'SUBACCOUNT')) NOT VALID;

ALTER TABLE certificates DROP CONSTRAINT IF EXISTS chk_certificates_state;
ALTER TABLE certificates ADD CONSTRAINT chk_certificates_state
    CHECK (state IS NULL OR state IN ('ACTIVE', 'EXPIRED')) NOT VALID;

ALTER TABLE certificates DROP CONSTRAINT IF EXISTS chk_certificates_purpose;
ALTER TABLE certificates ADD CONSTRAINT chk_certificates_purpose
    CHECK (purpose IS NULL OR purpose IN ('GENERIC', 'TENANT_DEFAULT', 'ROLE_MANAGEMENT', 'KEY_MANAGEMENT', 'CRYPTO')) NOT VALID;

-- +goose Down
ALTER TABLE keys DROP CONSTRAINT IF EXISTS chk_keys_key_type;
ALTER TABLE keys DROP CONSTRAINT IF EXISTS chk_keys_algorithm;
ALTER TABLE import_params DROP CONSTRAINT IF EXISTS chk_import_params_wrapping_alg;
ALTER TABLE import_params DROP CONSTRAINT IF EXISTS chk_import_params_hash_function;
ALTER TABLE workflows DROP CONSTRAINT IF EXISTS chk_workflows_parameters_resource_type;
ALTER TABLE systems DROP CONSTRAINT IF EXISTS chk_systems_type;
ALTER TABLE certificates DROP CONSTRAINT IF EXISTS chk_certificates_state;
ALTER TABLE certificates DROP CONSTRAINT IF EXISTS chk_certificates_purpose;
