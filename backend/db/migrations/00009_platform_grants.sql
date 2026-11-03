-- +goose Up
-- The application role gets data access only; the audit log is append-only (FR-9.2).
-- Skipped when the role does not exist (e.g. a throw-away database).
-- +goose StatementBegin
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'gatepass_app') AND current_user <> 'gatepass_app' THEN
        GRANT USAGE ON SCHEMA public TO gatepass_app;
        GRANT SELECT, INSERT, UPDATE, DELETE ON ALL TABLES IN SCHEMA public TO gatepass_app;
        REVOKE UPDATE, DELETE, TRUNCATE ON audit_logs FROM gatepass_app;
        ALTER DEFAULT PRIVILEGES IN SCHEMA public GRANT SELECT, INSERT, UPDATE, DELETE ON TABLES TO gatepass_app;
    END IF;
END $$;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'gatepass_app') AND current_user <> 'gatepass_app' THEN
        ALTER DEFAULT PRIVILEGES IN SCHEMA public REVOKE SELECT, INSERT, UPDATE, DELETE ON TABLES FROM gatepass_app;
        REVOKE ALL ON ALL TABLES IN SCHEMA public FROM gatepass_app;
    END IF;
END $$;
-- +goose StatementEnd
