BEGIN;

create table record_updates (
    cid varchar(255) NOT NULL,
    created timestamp NOT NULL,
    worked_by text,
    status int,
    PRIMARY KEY (status, created)
);

COMMIT;