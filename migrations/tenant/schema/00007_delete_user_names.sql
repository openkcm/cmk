-- This can be safely removed as it can be retrieved from IAM provider
-- +goose Up
ALTER TABLE "workflows" DROP COLUMN initiator_name;
ALTER TABLE "workflow_approvers" DROP COLUMN user_name;
ALTER TABLE "key_configurations" DROP COLUMN creator_name;
-- +goose Down
ALTER TABLE "workflows" ADD COLUMN initiator_name varchar(255);
ALTER TABLE "workflow_approvers" ADD COLUMN user_name varchar(255);
ALTER TABLE "key_configurations" ADD COLUMN creator_name varchar(255);
