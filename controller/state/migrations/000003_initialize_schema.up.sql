BEGIN;

CREATE TABLE drainedWorkers (
    worked_by text,
    PRIMARY KEY(worked_by)
);

COMMIT;