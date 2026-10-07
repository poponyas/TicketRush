-- Local development fixtures. Run only against a development database.
BEGIN;
INSERT INTO users(id,email,display_name)
VALUES ('22222222-2222-4222-8222-222222222222','organizer@example.test','Dev organizer')
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
