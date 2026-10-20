-- Local development fixtures. Run only against a development database.
-- Demo login: organizer@example.test / TicketRush-demo-2026!
-- The public demo credentials are for local development only.
BEGIN;
INSERT INTO users(id,email,display_name,password_hash)
VALUES ('22222222-2222-4222-8222-222222222222','organizer@example.test','Dev organizer','$argon2id$v=19$m=19456,t=2,p=1$WqGMhPAZ4ks9oNO0nHg+dg$ohVrMwpHoBDy50Xbf6xgdG8HvOh+VzzJoACstQtSD3U')
ON CONFLICT (id) DO NOTHING;
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
COMMIT;
