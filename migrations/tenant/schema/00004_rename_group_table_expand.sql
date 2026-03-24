-- +goose Up
ALTER TABLE "group" RENAME TO "groups";

-- This view can later be deleted on the contract phase
CREATE VIEW "group" AS
SELECT * FROM groups;

-- +goose Down
DROP VIEW "group";
ALTER TABLE "groups" RENAME TO "group";
