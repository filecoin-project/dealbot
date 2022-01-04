BEGIN;

CREATE TABLE legs_data (
    dsKey varchar(255) NOT NULL,
    data text NOT NULL CHECK (data != ''),
    created timestamp NOT NULL,
    PRIMARY KEY(dsKey)
);

COMMIT;
