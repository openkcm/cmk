-- This migration creates the workflow_key_configurations table
-- It stores the key configurations related to a workflow at creation time
-- +goose Up

CREATE TABLE IF NOT EXISTS workflow_key_configurations (
	id uuid PRIMARY KEY,
	workflow_id uuid NOT NULL,
	key_configuration_id uuid NOT NULL,
	CONSTRAINT fk_workflow_key_configurations_workflow
		FOREIGN KEY (workflow_id) REFERENCES workflows(id) ON DELETE CASCADE,
	CONSTRAINT fk_workflow_key_configurations_key_configuration
		FOREIGN KEY (key_configuration_id) REFERENCES key_configurations(id) ON DELETE CASCADE,
	CONSTRAINT unique_workflow_key_configuration UNIQUE (workflow_id, key_configuration_id)
);

CREATE INDEX idx_workflow_key_configurations_workflow_id ON workflow_key_configurations(workflow_id);
CREATE INDEX idx_workflow_key_configurations_key_configuration_id ON workflow_key_configurations(key_configuration_id);

-- +goose Down
DROP TABLE IF EXISTS workflow_key_configurations;
