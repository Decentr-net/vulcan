CREATE EXTENSION IF NOT EXISTS "pgcrypto";

ALTER TABLE request
    ADD COLUMN own_referral_code TEXT NULL UNIQUE,
    ADD COLUMN registered_by_referral_code TEXT NULL REFERENCES request (own_referral_code);

CREATE OR REPLACE FUNCTION unique_referral_code()
    RETURNS TRIGGER AS $$

DECLARE
    key TEXT;
    qry TEXT;
    found TEXT;
BEGIN

    -- generate the first part of a query as a string with safely
    -- escaped table name, using || to concat the parts
    qry := 'SELECT own_referral_code FROM ' || quote_ident(TG_TABLE_NAME) || ' WHERE own_referral_code=';

    -- This loop will probably only run once per call until we've generated
    -- millions of ids.
    LOOP

        -- Generate our string bytes and re-encode as a base64 string.
        key := encode(gen_random_bytes(6), 'base64');

        -- Base64 encoding contains 2 URL unsafe characters by default.
        -- The URL-safe version has these replacements.
        key := replace(key, '/', '_'); -- url safe replacement
        key := replace(key, '+', '-'); -- url safe replacement

        -- Concat the generated key (safely quoted) with the generated query
        -- and run it.
        -- SELECT id FROM "test" WHERE id='blahblah' INTO found
        -- Now "found" will be the duplicated id or NULL.
        EXECUTE qry || quote_literal(key) INTO found;

        -- Check to see if found is NULL.
        -- If we checked to see if found = NULL it would always be FALSE
        -- because (NULL = NULL) is always FALSE.
        IF found IS NULL THEN

            -- If we didn't find a collision then leave the LOOP.
            EXIT;
        END IF;

        -- We haven't EXITed yet, so return to the top of the LOOP
        -- and try again.
    END LOOP;

    NEW.own_referral_code = key;

    -- The RECORD returned here is what will actually be INSERTed,
    -- or what the next trigger will get if there is one.
    RETURN NEW;
END;
$$ language 'plpgsql';

CREATE TRIGGER trigger_request_unique_referral_code
    BEFORE UPDATE ON request
    FOR EACH ROW EXECUTE PROCEDURE unique_referral_code();

-- Dummy update to invoke the trigger_request_unique_referral_code
UPDATE request SET owner = owner;

-- Existing request updated, drop the trigger
DROP TRIGGER trigger_request_unique_referral_code ON request;

-- Recreate the trigger but apply it only on INSERT
CREATE TRIGGER trigger_request_unique_referral_code
    BEFORE INSERT ON request
    FOR EACH ROW EXECUTE PROCEDURE unique_referral_code();

ALTER TABLE request ALTER COLUMN own_referral_code SET NOT NULL;

-- Referral
CREATE TYPE REFERRAL_STATUS AS ENUM ('registered', 'installed', 'confirmed');

CREATE TABLE referral_tracking (
    sender VARCHAR NOT NULL,
    receiver VARCHAR NOT NULL,
    status REFERRAL_STATUS NOT NULL DEFAULT ('registered'),
    registered_at TIMESTAMP NOT NULL,
    installed_at TIMESTAMP,
    confirmed_at TIMESTAMP,
    sender_reward INT,
    receiver_reward INT,
    PRIMARY KEY (sender, receiver)
);