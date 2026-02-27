-- +goose Up
ALTER TABLE events ADD error_message varchar(255);
ALTER TABLE events ADD error_code varchar(255);

-- +goose Down
ALTER TABLE events DROP error_message;
ALTER TABLE events DROP error_code;
