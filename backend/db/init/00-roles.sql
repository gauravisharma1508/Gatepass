-- Runs once, when the Postgres volume is first created (local development only).
-- Production roles are created by infra with secrets from the secret store.
CREATE ROLE gatepass_owner LOGIN PASSWORD 'gatepass_owner';
CREATE ROLE gatepass_app   LOGIN PASSWORD 'gatepass_app';

CREATE DATABASE gatepass      OWNER gatepass_owner ENCODING 'UTF8';
CREATE DATABASE gatepass_test OWNER gatepass_owner ENCODING 'UTF8';

GRANT CONNECT ON DATABASE gatepass      TO gatepass_app;
GRANT CONNECT ON DATABASE gatepass_test TO gatepass_app;

ALTER ROLE gatepass_app SET statement_timeout = '10s';
ALTER ROLE gatepass_app SET idle_in_transaction_session_timeout = '30s';
ALTER ROLE gatepass_app SET timezone = 'UTC';
