DROP FUNCTION referral_tracking_sender_stats (addr VARCHAR, since INTERVAL);

ALTER TABLE referral_tracking
    ALTER COLUMN sender_reward TYPE INT,
    ALTER COLUMN receiver_reward TYPE INT;

CREATE FUNCTION referral_tracking_sender_stats(addr VARCHAR, since INTERVAL)
    RETURNS TABLE
            (
                registered INT,
                installed  INT,
                confirmed  INT,
                reward     INT
            )
AS
$$
BEGIN
    RETURN QUERY
        SELECT COALESCE(
                       (SELECT COUNT(*)
                        FROM referral_tracking
                        WHERE sender = addr
                          AND CASE WHEN since IS NULL THEN TRUE ELSE registered_at > NOW() - since END),
                       0)::INT AS registered,
               COALESCE(
                       (SELECT COUNT(*)
                        FROM referral_tracking
                        WHERE sender = addr
                          AND installed_at IS NOT NULL
                          AND CASE WHEN since IS NULL THEN TRUE ELSE registered_at > NOW() - since END),
                       0)::INT AS installed,
               COALESCE(
                       (SELECT COUNT(*)
                        FROM referral_tracking
                        WHERE sender = addr
                          AND confirmed_at IS NOT NULL
                          AND CASE WHEN since IS NULL THEN TRUE ELSE registered_at > NOW() - since END),
                       0)::INT AS confirmed,
               COALESCE(
                       (SELECT SUM(COALESCE(sender_reward, 0))
                        FROM referral_tracking
                        WHERE sender = addr
                          AND CASE WHEN since IS NULL THEN TRUE ELSE registered_at > NOW() - since END),
                       0)::INT AS reward;
END;
$$ LANGUAGE 'plpgsql';