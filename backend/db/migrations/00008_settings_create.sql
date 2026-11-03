-- +goose Up
CREATE TABLE settings (
    id                    smallint PRIMARY KEY CHECK (id = 1),
    approval_window_min   int NOT NULL,
    approval_reminder_min int NOT NULL,
    max_advance_days      int NOT NULL,
    photo_retention_days  int NOT NULL,
    record_retention_days int NOT NULL,
    updated_at            timestamptz NOT NULL
);
INSERT INTO settings VALUES (1, 30, 10, 30, 90, 365, now());

-- +goose Down
DROP TABLE settings;
