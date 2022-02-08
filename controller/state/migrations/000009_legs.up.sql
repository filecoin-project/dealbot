BEGIN;

CREATE TABLE legs_data (
    key varchar(255) NOT NULL,
    data text NOT NULL CHECK (data != ''),
    created timestamp NOT NULL,
    PRIMARY KEY(Key)
);

COMMIT;
