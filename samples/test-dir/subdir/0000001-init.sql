CREATE SCHEMA IF NOT EXISTS test_schema;

CREATE TABLE IF NOT EXISTS test_schema.test_table (
    id SERIAL PRIMARY KEY,
    name TEXT NOT NULL
);
