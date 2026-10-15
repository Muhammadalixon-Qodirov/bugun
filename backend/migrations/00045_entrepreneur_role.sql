-- +goose Up
-- Sellers and buyers become one role, "entrepreneur" (Tadbirkor): anyone may
-- both sell and ask for quotes. The analyst role is retired and its users
-- become entrepreneurs too. market_admin and admin stay as they are.
--
-- 'seller', 'buyer' and 'org_analyst' are legacy values, still accepted for
-- one release so the previous code keeps working after a rollback (code is
-- rolled back without its migrations). Drop them in a later migration.
ALTER TABLE users DROP CONSTRAINT IF EXISTS users_role_check;
ALTER TABLE users ADD CONSTRAINT users_role_check CHECK (role IN
    ('entrepreneur', 'market_admin', 'admin',
     'seller', 'buyer', 'org_analyst'));

UPDATE users SET role = 'entrepreneur', role_approved = TRUE, updated_at = now()
WHERE role IN ('seller', 'buyer', 'org_analyst');

-- One person can now be on both sides of the market, so the database itself
-- refuses an offer on the offerer's own request (deals already refuse a
-- buyer who is the seller: deals_not_with_oneself).
-- +goose StatementBegin
CREATE FUNCTION offers_not_own_rfq() RETURNS trigger AS $$
BEGIN
    IF EXISTS (SELECT 1 FROM rfqs WHERE id = NEW.rfq_id AND buyer_id = NEW.seller_id) THEN
        RAISE EXCEPTION 'an offer on one''s own request' USING ERRCODE = 'check_violation';
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;
-- +goose StatementEnd

CREATE TRIGGER offers_not_own_rfq BEFORE INSERT OR UPDATE OF rfq_id, seller_id ON offers
    FOR EACH ROW EXECUTE FUNCTION offers_not_own_rfq();

-- +goose Down
DROP TRIGGER IF EXISTS offers_not_own_rfq ON offers;
DROP FUNCTION IF EXISTS offers_not_own_rfq();

ALTER TABLE users DROP CONSTRAINT IF EXISTS users_role_check;
UPDATE users SET role = 'seller', updated_at = now() WHERE role = 'entrepreneur';
ALTER TABLE users ADD CONSTRAINT users_role_check CHECK (role IN
    ('seller', 'buyer', 'market_admin', 'org_analyst', 'admin'));
