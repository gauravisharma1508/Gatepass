-- +goose Up
CREATE TABLE notifications (
    id         uuid PRIMARY KEY,
    user_id    uuid         NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    visit_id   uuid REFERENCES visits (id) ON DELETE CASCADE,
    type       text         NOT NULL,
    title      varchar(200) NOT NULL,
    body       varchar(500) NOT NULL,
    read_at    timestamptz,
    created_at timestamptz  NOT NULL
);
CREATE INDEX notifications_user_idx ON notifications (user_id, read_at, created_at DESC);

-- +goose Down
DROP TABLE notifications;
