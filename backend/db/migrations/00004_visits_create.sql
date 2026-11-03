-- +goose Up
CREATE TABLE visits (
    id                    uuid PRIMARY KEY,
    code                  varchar(16)  NOT NULL UNIQUE,
    type                  text         NOT NULL CHECK (type IN ('PLANNED', 'UNPLANNED')),
    status                text         NOT NULL CHECK (status IN (
        'SCHEDULED', 'PENDING_APPROVAL', 'APPROVED', 'CHECKED_IN', 'MEETING_OVER',
        'CHECKED_OUT', 'REJECTED', 'EXPIRED', 'CANCELLED', 'NO_SHOW', 'DENIED')),
    visitor_id            uuid         NOT NULL REFERENCES visitors (id),
    visitor_name          varchar(100) NOT NULL,
    host_id               uuid         NOT NULL REFERENCES users (id),
    created_by_id         uuid         NOT NULL REFERENCES users (id),
    purpose               varchar(250) NOT NULL,
    visit_date            date         NOT NULL,
    expected_arrival      time,
    expected_duration_min int          NOT NULL CHECK (expected_duration_min BETWEEN 30 AND 480),
    approval_deadline     timestamptz,
    reminder_sent_at      timestamptz,
    decided_at            timestamptz,
    decision_reason       varchar(250),
    checked_in_at         timestamptz,
    checked_in_by_id      uuid REFERENCES users (id),
    meeting_over_at       timestamptz,
    checked_out_at        timestamptz,
    checked_out_by_id     uuid REFERENCES users (id),
    overstay_notified_at  timestamptz,
    closed_reason         varchar(250),
    photos_purged_at      timestamptz,
    version               int          NOT NULL DEFAULT 1,
    created_at            timestamptz  NOT NULL,
    updated_at            timestamptz  NOT NULL
);
CREATE INDEX visits_status_date_idx  ON visits (status, visit_date);
CREATE INDEX visits_host_date_idx    ON visits (host_id, visit_date);
CREATE INDEX visits_visitor_status_idx ON visits (visitor_id, status);
CREATE INDEX visits_pending_deadline_idx ON visits (approval_deadline) WHERE status = 'PENDING_APPROVAL';

CREATE TABLE visit_code_counters (
    date       date PRIMARY KEY,
    last_value int NOT NULL
);

-- +goose Down
DROP TABLE visit_code_counters;
DROP TABLE visits;
