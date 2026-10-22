BEGIN;

CREATE INDEX booking_held_expires_at_idx
    ON booking(expires_at, id) WHERE status = 'held';

COMMIT;
