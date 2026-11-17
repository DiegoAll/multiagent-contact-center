-- +goose Up
-- Datos ficticios que simulan un CRM. No contienen información real.
INSERT INTO customers (name, email, phone, segment) VALUES
    ('Ana Gómez',    'ana.gomez@example.com',    '+57 300 000 0001', 'premium'),
    ('Carlos Ruiz',  'carlos.ruiz@example.com',  '+57 300 000 0002', 'standard'),
    ('Laura Mejía',  'laura.mejia@example.com',  '+57 300 000 0003', 'standard');

INSERT INTO orders (customer_id, status, total_cents, currency, items, estimated_delivery) VALUES
    (1, 'shipped',    19990000, 'COP', 'Audífonos inalámbricos x1',            now() + interval '2 days'),
    (1, 'delivered',   4590000, 'COP', 'Cable USB-C x2',                       NULL),
    (2, 'processing', 89900000, 'COP', 'Monitor 27" x1',                       now() + interval '6 days'),
    (2, 'cancelled',   2990000, 'COP', 'Soporte de laptop x1',                 NULL),
    (3, 'shipped',    12990000, 'COP', 'Teclado mecánico x1',                  now() + interval '1 day');

INSERT INTO tickets (customer_id, subject, description, priority) VALUES
    (2, 'Retraso en pedido', 'El monitor aún no sale del almacén.', 'medium');

-- +goose Down
DELETE FROM tickets;
DELETE FROM orders;
DELETE FROM customers;
