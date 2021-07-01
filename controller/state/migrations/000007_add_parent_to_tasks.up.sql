BEGIN;

ALTER TABLE tasks ADD COLUMN parent varchar(36);

COMMIT;
