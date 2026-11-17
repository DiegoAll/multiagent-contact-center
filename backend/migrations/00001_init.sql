-- +goose Up
CREATE TABLE customers (
    id         BIGSERIAL PRIMARY KEY,
    name       TEXT        NOT NULL,
    email      TEXT        NOT NULL UNIQUE,
    phone      TEXT        NOT NULL DEFAULT '',
    segment    TEXT        NOT NULL DEFAULT 'standard',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE orders (
    id                 BIGSERIAL PRIMARY KEY,
    customer_id        BIGINT      NOT NULL REFERENCES customers (id),
    status             TEXT        NOT NULL,
    total_cents        BIGINT      NOT NULL,
    currency           TEXT        NOT NULL DEFAULT 'COP',
    items              TEXT        NOT NULL,
    estimated_delivery TIMESTAMPTZ,
    updated_at         TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_orders_customer ON orders (customer_id);

CREATE TABLE tickets (
    id                BIGSERIAL PRIMARY KEY,
    customer_id       BIGINT      NOT NULL REFERENCES customers (id),
    subject           TEXT        NOT NULL,
    description       TEXT        NOT NULL,
    priority          TEXT        NOT NULL DEFAULT 'medium',
    status            TEXT        NOT NULL DEFAULT 'open',
    escalated         BOOLEAN     NOT NULL DEFAULT FALSE,
    escalation_reason TEXT        NOT NULL DEFAULT '',
    created_at        TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_tickets_customer ON tickets (customer_id);

-- +goose Down
DROP TABLE IF EXISTS tickets;
DROP TABLE IF EXISTS orders;
DROP TABLE IF EXISTS customers;
