CREATE TABLE request (
    owner VARCHAR NOT NULL UNIQUE,
    email VARCHAR NOT NULL UNIQUE,
    address VARCHAR NOT NULL UNIQUE,
    code VARCHAR NOT NULL,
    created_at TIMESTAMP NOT NULL,
    confirmed_at TIMESTAMP
);
