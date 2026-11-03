-- +goose Up
CREATE TABLE visitors (
    id         uuid PRIMARY KEY,
    mobile     varchar(13)  NOT NULL UNIQUE,
    name       varchar(100) NOT NULL,
    email      varchar(254),
    company    varchar(100),
    created_at timestamptz  NOT NULL,
    updated_at timestamptz  NOT NULL
);

-- +goose Down
DROP TABLE visitors;
