BEGIN;

ALTER TABLE task_status_ledger DROP 
   CONSTRAINT fk_status_ledger_uuid;
ALTER TABLE task_status_ledger ADD
  CONSTRAINT fk_status_ledger_uuid FOREIGN KEY (uuid) REFERENCES tasks(uuid) ON DELETE CASCADE;

COMMIT;