BEGIN;

-- This is first migration so start with empty DB
DROP INDEX IF EXISTS idx_task_status_ledger_by_stage;
DROP TABLE IF EXISTS task_status_ledger;
DROP TABLE IF EXISTS tasks;

CREATE TABLE tasks (
    uuid varchar(36) NOT NULL,
    worked_by text,
    data text NOT NULL CHECK (data != ''),
    created timestamp NOT NULL,
    PRIMARY KEY(uuid)
);

CREATE TABLE task_status_ledger (
    uuid varchar(36) NOT NULL,
    status int,
    stage varchar(255),
    ts timestamp NOT NULL,
    PRIMARY KEY(uuid, ts),
    CONSTRAINT fk_status_ledger_uuid FOREIGN KEY (uuid) REFERENCES tasks
);

CREATE UNIQUE INDEX idx_task_status_ledger_by_stage ON task_status_ledger (uuid, status, stage);

--GRANT SELECT, INSERT, UPDATE, DELETE ON ALL TABLES IN SCHEMA public TO "dealbotuser";

COMMIT;
