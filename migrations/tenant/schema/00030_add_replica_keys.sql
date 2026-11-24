-- +goose Up
CREATE TABLE IF NOT EXISTS replica_keys (
	id uuid PRIMARY KEY,
	key_id uuid NOT NULL,
	native_id varchar(255) NOT NULL,
	CONSTRAINT fk_keys_native_entries FOREIGN KEY (key_id) REFERENCES keys(id) ON DELETE CASCADE
);

CREATE INDEX idx_replica_keys_key_id ON replica_keys(key_id);

-- +goose Down
DROP TABLE IF EXISTS replica_keys;
