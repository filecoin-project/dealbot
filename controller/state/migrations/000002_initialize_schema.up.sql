BEGIN;

ALTER TABLE tasks ADD COLUMN cid string;

CREATE INDEX ix_tasks_cid ON tasks (cid);

CREATE TABLE finalizedData (
    cid string NOT NULL,
    data text NOT NULL CHECK (data != ''),
    created timestamp NOT NULL,
    PRIMARY KEY(cid)
);

COMMIT;
