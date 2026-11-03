-- +goose Up
-- No foreign keys on purpose: audit rows must outlive the visits the retention job deletes.
CREATE TABLE audit_logs (
    id          uuid PRIMARY KEY,
    visit_id    uuid,
    actor_id    uuid,
    actor_role  text        NOT NULL CHECK (actor_role IN ('EMPLOYEE', 'SECURITY', 'ADMIN', 'SYSTEM')),
    action      text        NOT NULL,
    from_status text,
    to_status   text,
    reason      varchar(250),
    metadata    jsonb,
    created_at  timestamptz NOT NULL
);
CREATE INDEX audit_logs_visit_idx ON audit_logs (visit_id, created_at);

-- +goose Down
DROP TABLE audit_logs;
