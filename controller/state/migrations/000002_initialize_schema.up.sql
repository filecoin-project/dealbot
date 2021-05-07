BEGIN;

ALTER TABLE tasks ADD COLUMN cid BYTEA;

CREATE INDEX ix_tasks_cid ON tasks (cid);

COMMIT;
