-- +goose Up
CREATE TABLE visit_photos (
    id             uuid PRIMARY KEY,
    visit_id       uuid        NOT NULL REFERENCES visits (id) ON DELETE CASCADE,
    kind           text        NOT NULL CHECK (kind IN ('VISITOR', 'ID_DOCUMENT')),
    storage_key    text        NOT NULL,
    mime_type      varchar(20) NOT NULL,
    size_bytes     int         NOT NULL CHECK (size_bytes <= 512000),
    id_type        text CHECK (id_type IN ('AADHAAR', 'PAN', 'PASSPORT', 'DRIVING_LICENCE', 'VOTER_ID', 'OTHER')),
    id_last4       char(4),
    captured_by_id uuid        NOT NULL REFERENCES users (id),
    captured_at    timestamptz NOT NULL,
    UNIQUE (visit_id, kind),
    CHECK (kind <> 'ID_DOCUMENT' OR id_type IS NOT NULL)
);

-- +goose Down
DROP TABLE visit_photos;
