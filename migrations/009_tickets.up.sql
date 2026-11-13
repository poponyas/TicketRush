BEGIN;
CREATE TABLE tickets (
    id UUID PRIMARY KEY,
    booking_item_id UUID NOT NULL REFERENCES booking_item(id) ON DELETE RESTRICT,
    number INTEGER NOT NULL CHECK (number > 0),
    status TEXT NOT NULL DEFAULT 'active' CHECK (status IN ('active','used','revoked')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
    used_at TIMESTAMPTZ,
    checked_in_by UUID REFERENCES users(id) ON DELETE RESTRICT,
    UNIQUE (booking_item_id, number),
    CHECK ((used_at IS NULL) = (checked_in_by IS NULL)),
    CHECK (status <> 'used' OR used_at IS NOT NULL)
);
CREATE INDEX tickets_created_id_idx ON tickets(created_at DESC, id DESC);
COMMIT;
