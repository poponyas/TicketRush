BEGIN;

-- The legacy model has no session, tariff or price to map automatically.
-- Lock before checking so a concurrent insert cannot be lost.
LOCK TABLE bookings IN ACCESS EXCLUSIVE MODE;
DO $guard$
BEGIN
    IF EXISTS (SELECT 1 FROM bookings) THEN
        RAISE EXCEPTION 'Cannot migrate booking model: legacy bookings contain data. Migrate those records explicitly before applying migration 006.';
    END IF;
END
$guard$;

DROP TABLE bookings;

CREATE TABLE booking (
    id UUID PRIMARY KEY,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    session_id UUID NOT NULL REFERENCES sessions(id) ON DELETE RESTRICT,
    status VARCHAR(50) NOT NULL CHECK(status IN ('held', 'confirmed', 'cancelled', 'expired')),
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT booking_expires_after_creation CHECK(created_at < expires_at)
);

CREATE TABLE booking_item (
    id UUID PRIMARY KEY,
    booking_id UUID NOT NULL REFERENCES booking(id) ON DELETE CASCADE,
    ticket_type_id UUID NOT NULL REFERENCES ticket_types(id) ON DELETE RESTRICT,
    quantity INT NOT NULL CHECK(quantity > 0),
    unit_price NUMERIC(18, 2) NOT NULL CHECK (unit_price >= 0),
    currency TEXT NOT NULL CHECK (currency = 'RUB')
);

CREATE INDEX booking_user_id_idx ON booking(user_id);
CREATE INDEX booking_session_id_idx ON booking(session_id);
CREATE INDEX booking_item_booking_id_idx ON booking_item(booking_id);
CREATE INDEX booking_item_ticket_type_id_idx ON booking_item(ticket_type_id);

COMMIT;
