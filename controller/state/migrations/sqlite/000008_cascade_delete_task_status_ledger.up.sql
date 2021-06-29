-- this should be simpler but sqlite does not support alter table add constraint

PRAGMA foreign_keys=off;
BEGIN TRANSACTION;

DROP INDEX idx_task_status_ledger_by_stage;

CREATE TABLE task_status_ledger_new (
    uuid varchar(36) NOT NULL,
    status int,
    stage varchar(255),
    ts timestamp NOT NULL,
    run int NOT NULL DEFAULT 0,
    PRIMARY KEY(uuid, ts),
    CONSTRAINT fk_status_ledger_uuid FOREIGN KEY (uuid) REFERENCES tasks ON DELETE CASCADE
);

INSERT INTO task_status_ledger_new(uuid, status, stage, ts, run)
SELECT uuid, status, stage, ts, run
FROM task_status_ledger;

DROP TABLE task_status_ledger;

ALTER TABLE task_status_ledger_new RENAME TO task_status_ledger; 
CREATE INDEX idx_task_status_ledger_by_stage ON task_status_ledger (uuid, status, stage, run);

COMMIT;

PRAGMA foreign_keys=on;