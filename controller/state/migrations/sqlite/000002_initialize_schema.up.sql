BEGIN;

ALTER TABLE tasks ADD COLUMN cid varchar(255);

CREATE INDEX ix_tasks_cid ON tasks (cid);

CREATE TABLE finalizedData (
    cid varchar(255) NOT NULL,
    data text NOT NULL CHECK (data != ''),
    created timestamp NOT NULL,
    PRIMARY KEY(cid)
);

COMMIT;
