BEGIN;

DROP INDEX IF EXISTS idx_task_status_ledger_by_stage;

DROP TABLE IF EXISTS task_status_ledger;

DROP TABLE IF EXISTS tasks;

COMMIT;
