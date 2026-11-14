-- Local development fixtures. Run only against a development database.
-- Demo login: organizer@example.test / TicketRush-demo-2026!
-- The public demo credentials are for local development only.
-- Backfill missing passwords on legacy demo accounts; keep existing passwords.
BEGIN;
INSERT INTO users(id,email,display_name,password_hash)
VALUES ('22222222-2222-4222-8222-222222222222','organizer@example.test','Dev organizer','$argon2id$v=19$m=19456,t=2,p=1$WqGMhPAZ4ks9oNO0nHg+dg$ohVrMwpHoBDy50Xbf6xgdG8HvOh+VzzJoACstQtSD3U')
ON CONFLICT (id) DO UPDATE
SET password_hash = EXCLUDED.password_hash
WHERE users.email = EXCLUDED.email AND users.password_hash IS NULL;
INSERT INTO organizers(id,name)
VALUES ('33333333-3333-4333-8333-333333333333','TicketRush Demo')
ON CONFLICT (id) DO NOTHING;
INSERT INTO organizer_memberships(organizer_id,user_id,role)
VALUES ('33333333-3333-4333-8333-333333333333','22222222-2222-4222-8222-222222222222','organizer')
ON CONFLICT (organizer_id,user_id) DO NOTHING;
INSERT INTO events(id,organizer_id,name,description,category,status)
VALUES ('11111111-1111-4111-8111-111111111111','33333333-3333-4333-8333-333333333333',
        'Go Live','Demo event','conference','draft')
ON CONFLICT (id) DO NOTHING;

INSERT INTO users(id,email,display_name,password_hash)
VALUES ('88888888-8888-4888-8888-888888888888','venue-owner@example.test','Dev venue owner','$argon2id$v=19$m=19456,t=2,p=1$WqGMhPAZ4ks9oNO0nHg+dg$ohVrMwpHoBDy50Xbf6xgdG8HvOh+VzzJoACstQtSD3U')
ON CONFLICT(id) DO UPDATE
SET password_hash = EXCLUDED.password_hash
WHERE users.email = EXCLUDED.email AND users.password_hash IS NULL;
INSERT INTO user_roles(user_id,role) VALUES('88888888-8888-4888-8888-888888888888','ownerVenue') ON CONFLICT DO NOTHING;

-- A separate published scenario for the end-to-end demo; existing data is kept.
INSERT INTO venues(id,name,city,address) VALUES
 ('44444444-4444-4444-8444-444444444444','Demo Hall','Moscow','Demo address') ON CONFLICT(id) DO NOTHING;
INSERT INTO events(id,organizer_id,name,description,category,status) VALUES
 ('55555555-5555-4555-8555-555555555555','33333333-3333-4333-8333-333333333333','TicketRush demo','End-to-end demo','conference','published') ON CONFLICT(id) DO NOTHING;
INSERT INTO sessions(id,event_id,venue_id,starts_at,sales_start_at,sales_end_at,status) VALUES
 ('66666666-6666-4666-8666-666666666666','55555555-5555-4555-8555-555555555555','44444444-4444-4444-8444-444444444444',now()+interval '30 days',now()-interval '1 hour',now()+interval '29 days','scheduled') ON CONFLICT(id) DO NOTHING;
INSERT INTO ticket_types(id,session_id,name,price,currency,capacity,available,held,sold) VALUES
 ('77777777-7777-4777-8777-777777777777','66666666-6666-4666-8666-666666666666','Standard',1500.25,'RUB',100,100,0,0) ON CONFLICT(id) DO NOTHING;
COMMIT;
