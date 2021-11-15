DROP TABLE referral;
DROP TYPE REFERRAL_STATUS;

ALTER TABLE request DROP COLUMN own_referral_code,
                    DROP COLUMN registration_referral_code;
DROP TRIGGER trigger_request_unique_referral_code ON request;
DROP FUNCTION IF EXISTS unique_referral_code();