CREATE TABLE dloan
(
    id         SERIAL PRIMARY KEY,
    address    TEXT      NOT NULL,
    first_name TEXT      NOT NULL,
    last_name  TEXT      NOT NULL,
    pdv        FLOAT     NOT NULL,
    created_at TIMESTAMP NOT NULL
);