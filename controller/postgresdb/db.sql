create database dealbot;

-- Revoke public permissions?
-- REVOKE ALL ON ALL TABLES IN SCHEMA public FROM PUBLIC;
-- REVOKE CONNECT ON DATABASE dealbot FROM PUBLIC;

-- Create the dealbot role for the dealbot service to use for DB access
DO
$body$
BEGIN
    IF NOT EXISTS (SELECT * FROM pg_catalog.pg_user WHERE usename = 'dealbot') THEN
        CREATE ROLE "dealbot" WITH LOGIN;
    END IF;
END
$body$
;

GRANT CONNECT ON DATABASE dealbot TO "dealbot";
