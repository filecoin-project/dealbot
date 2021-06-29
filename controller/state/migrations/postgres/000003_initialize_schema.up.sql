BEGIN;

CREATE TABLE drained_workers (
    worked_by text,
    PRIMARY KEY(worked_by)
);

COMMIT;