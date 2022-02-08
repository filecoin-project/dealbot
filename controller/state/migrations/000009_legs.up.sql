BEGIN;

CREATE TABLE legs_data (
    key varchar(255) NOT NULL,
    data text NOT NULL CHECK (data != ''),
    PRIMARY KEY(Key)
);

COMMIT;
