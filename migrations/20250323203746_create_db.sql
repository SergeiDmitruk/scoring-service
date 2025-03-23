-- +goose Up
-- +goose StatementBegin
-- 001_create_database_and_tables.up.sql


CREATE TABLE IF NOT EXISTS users (
    id SERIAL PRIMARY KEY,
    login VARCHAR(255) UNIQUE NOT NULL,
    password_hash TEXT NOT NULL
);


CREATE TABLE IF NOT EXISTS orders (
    id SERIAL PRIMARY KEY,
    user_id INT REFERENCES users(id),
    number VARCHAR(20) UNIQUE NOT NULL,
    status VARCHAR(50) NOT NULL,
    accrual NUMERIC(10,2) DEFAULT 0,
    uploaded_at TIMESTAMP DEFAULT now()
);


CREATE TABLE IF NOT EXISTS balances (
    id SERIAL PRIMARY KEY,
    user_id INT REFERENCES users(id),
    current_balance NUMERIC(10,2) DEFAULT 0,
    withdrawn NUMERIC(10,2) DEFAULT 0
);


CREATE TABLE IF NOT EXISTS withdrawals (
    id SERIAL PRIMARY KEY,
    user_id INT REFERENCES users(id),
    order_number VARCHAR(20) NOT NULL,
    sum NUMERIC(10,2) NOT NULL,
    uploaded_at TIMESTAMP DEFAULT now()
);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
SELECT 'down SQL query';
-- +goose StatementEnd
