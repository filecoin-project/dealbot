BEGIN;

DROP INDEX IF EXISTS idx_task_status_ledger_by_stage;

ALTER TABLE task_status_ledger ADD COLUMN run int NOT NULL DEFAULT 0;

CREATE INDEX idx_task_status_ledger_by_stage ON task_status_ledger (uuid, status, stage, run);

COMMIT;
