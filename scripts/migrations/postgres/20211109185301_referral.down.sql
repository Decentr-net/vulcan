DROP TABLE referral;
DROP TYPE REFERRAL_STATUS;

ALTER TABLE request DROP COLUMN referral_code;
DROP TRIGGER trigger_request_unique_referral_code ON request;
DROP FUNCTION IF EXISTS unique_referral_code();