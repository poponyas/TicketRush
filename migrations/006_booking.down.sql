BEGIN;

DROP TABLE booking_item;
DROP TABLE booking;

CREATE TABLE bookings (
    id UUID PRIMARY KEY,
    event_id UUID NOT NULL REFERENCES events(id),
    user_id TEXT NOT NULL CHECK (btrim(user_id) <> ''),
    quantity INTEGER NOT NULL CHECK (quantity > 0),
    status TEXT NOT NULL DEFAULT 'reserved'
        CHECK (status IN ('reserved', 'confirmed', 'cancelled', 'expired')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX bookings_event_id_idx ON bookings(event_id);

COMMIT;
