BEGIN;
-- Restoring NOT NULL must not invent an inventory for new product events.
-- Remove/backup new events explicitly before rolling this schema back.
DO $$ BEGIN
    IF EXISTS (SELECT 1 FROM events WHERE total_tickets IS NULL OR available_tickets IS NULL) THEN
        RAISE EXCEPTION 'Cannot rollback catalog: new events have no legacy inventory. Back up and remove them first.';
    END IF;
END $$;
DROP TABLE ticket_types;
DROP TABLE sessions;
DROP TABLE venues;
DROP INDEX events_organizer_id_idx;
ALTER TABLE events
    DROP COLUMN organizer_id,
    DROP COLUMN description,
    DROP COLUMN category,
    DROP COLUMN poster_key,
    DROP COLUMN status,
    DROP COLUMN updated_at;
ALTER TABLE events ALTER COLUMN total_tickets SET NOT NULL;
ALTER TABLE events ALTER COLUMN available_tickets SET NOT NULL;
DROP TABLE organizer_memberships;
DROP TABLE organizers;
DROP TABLE users;
COMMIT;
