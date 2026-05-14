-- Enable pg_stat_statements
CREATE EXTENSION IF NOT EXISTS pg_stat_statements;

-- Create some dummy tables
CREATE TABLE users (
    id SERIAL, -- Missing Primary Key explicitly to trigger SchemaAnalyzer
    username VARCHAR(50),
    email VARCHAR(100),
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    data TEXT -- To simulate TOAST data
);

CREATE TABLE orders (
    id SERIAL PRIMARY KEY,
    user_id INT,
    total_amount DECIMAL(10, 2),
    status VARCHAR(20),
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Generate a lot of data for sequential scan triggers
INSERT INTO users (username, email, data)
SELECT 
    'user_' || i, 
    'user_' || i || '@example.com',
    repeat('this is some long text data to force toast storage and increase table size ', 100)
FROM generate_series(1, 500000) i;

-- Create an unused index
CREATE INDEX idx_users_email ON users(email);
-- We will never query by email, making it an unused index

-- Do some updates/deletes to create dead tuples (trigger VacuumAnalyzer)
UPDATE users SET username = username || '_updated' WHERE id % 5 = 0;
DELETE FROM users WHERE id % 7 = 0;

-- Do some sequential scans manually to populate pg_stat_statements and pg_stat_user_tables
SELECT count(*) FROM users WHERE username LIKE '%99%';
SELECT count(*) FROM users WHERE username LIKE '%88%';
SELECT count(*) FROM users WHERE username LIKE '%77%';
SELECT count(*) FROM users WHERE username LIKE '%66%';

-- Keep a connection idle in transaction to trigger ConnectionAnalyzer
-- (This is hard to do purely from init.sql because init.sql runs to completion.
-- We can simulate it by leaving a background job, but it's okay if not all analyzers trigger)
