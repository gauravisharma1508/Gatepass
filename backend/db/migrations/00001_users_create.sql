-- +goose Up
CREATE TABLE users (
    id                   uuid PRIMARY KEY,
    employee_code        varchar(20)  NOT NULL UNIQUE,
    name                 varchar(100) NOT NULL,
    email                varchar(254) NOT NULL,
    phone                varchar(15),
    department           varchar(100),
    role                 text         NOT NULL CHECK (role IN ('EMPLOYEE', 'SECURITY', 'ADMIN')),
    password_hash        text         NOT NULL,
    must_change_password boolean      NOT NULL DEFAULT false,
    is_active            boolean      NOT NULL DEFAULT true,
    failed_login_count   int          NOT NULL DEFAULT 0,
    locked_until         timestamptz,
    created_at           timestamptz  NOT NULL,
    updated_at           timestamptz  NOT NULL
);
CREATE UNIQUE INDEX users_email_lower_uq ON users (lower(email));

-- +goose Down
DROP TABLE users;
