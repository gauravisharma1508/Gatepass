-- +goose Up
-- A monotonic sequence gives the audit log a reliable insertion order (timestamps can tie).
ALTER TABLE audit_logs ADD COLUMN seq bigserial NOT NULL;
CREATE INDEX audit_logs_seq_idx ON audit_logs (seq);

-- +goose StatementBegin
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'gatepass_app') AND current_user <> 'gatepass_app' THEN
        GRANT USAGE, SELECT ON ALL SEQUENCES IN SCHEMA public TO gatepass_app;
        ALTER DEFAULT PRIVILEGES IN SCHEMA public GRANT USAGE, SELECT ON SEQUENCES TO gatepass_app;
    END IF;
END $$;
-- +goose StatementEnd

-- +goose Down
DROP INDEX audit_logs_seq_idx;
ALTER TABLE audit_logs DROP COLUMN seq;
