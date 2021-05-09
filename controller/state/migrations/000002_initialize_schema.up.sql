BEGIN;

ALTER TABLE tasks ADD COLUMN cid string;

CREATE INDEX ix_tasks_cid ON tasks (cid);

COMMIT;
