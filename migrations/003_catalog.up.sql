BEGIN;
CREATE TABLE users (
    id UUID PRIMARY KEY,
    email TEXT NOT NULL UNIQUE,
    display_name TEXT NOT NULL DEFAULT '',
    issuer TEXT,
    subject TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (issuer, subject),
    CHECK ((issuer IS NULL) = (subject IS NULL))
);
CREATE TABLE organizers (
    id UUID PRIMARY KEY,
    name TEXT NOT NULL CHECK (btrim(name) <> ''),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE TABLE organizer_memberships (
    organizer_id UUID NOT NULL REFERENCES organizers(id),
    user_id UUID NOT NULL REFERENCES users(id),
    role TEXT NOT NULL CHECK (role IN ('organizer','controller')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (organizer_id,user_id)
);
-- Old events have no owner. Keep them as drafts of a migration-only organizer.
INSERT INTO organizers (id,name)
VALUES ('ffffffff-ffff-4fff-8fff-ffffffffffff','Legacy events (migration)');
ALTER TABLE events
    ADD COLUMN organizer_id UUID REFERENCES organizers(id),
    ADD COLUMN description TEXT NOT NULL DEFAULT '' CHECK (char_length(description) <= 10000),
    ADD COLUMN category TEXT NOT NULL DEFAULT '' CHECK (char_length(category) <= 100),
    ADD COLUMN poster_key TEXT NOT NULL DEFAULT '',
    ADD COLUMN status TEXT NOT NULL DEFAULT 'draft' CHECK (status IN ('draft','published','archived')),
    ADD COLUMN updated_at TIMESTAMPTZ NOT NULL DEFAULT now();
UPDATE events SET organizer_id='ffffffff-ffff-4fff-8fff-ffffffffffff';
ALTER TABLE events ALTER COLUMN organizer_id SET NOT NULL;
-- Preserve the historical counters; new events do not use them.
-- Inventory for the product belongs to ticket_types.
ALTER TABLE events ALTER COLUMN total_tickets DROP NOT NULL;
ALTER TABLE events ALTER COLUMN available_tickets DROP NOT NULL;
CREATE INDEX events_organizer_id_idx ON events(organizer_id);
CREATE TABLE venues (
    id UUID PRIMARY KEY,
    name TEXT NOT NULL CHECK (btrim(name) <> ''),
    city TEXT NOT NULL CHECK (btrim(city) <> ''),
    address TEXT NOT NULL CHECK (btrim(address) <> ''),
    timezone TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE TABLE sessions (
    id UUID PRIMARY KEY,
    event_id UUID NOT NULL REFERENCES events(id),
    venue_id UUID NOT NULL REFERENCES venues(id),
    starts_at TIMESTAMPTZ NOT NULL,
    ends_at TIMESTAMPTZ,
    sales_start_at TIMESTAMPTZ NOT NULL,
    sales_end_at TIMESTAMPTZ NOT NULL,
    status TEXT NOT NULL DEFAULT 'draft' CHECK (status IN ('draft','scheduled','cancelled','completed')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CHECK (ends_at IS NULL OR ends_at > starts_at),
    CHECK (sales_start_at < sales_end_at AND sales_end_at <= starts_at)
);
CREATE INDEX sessions_event_id_idx ON sessions(event_id);
CREATE TABLE ticket_types (
    id UUID PRIMARY KEY,
    session_id UUID NOT NULL REFERENCES sessions(id),
    name TEXT NOT NULL CHECK (btrim(name) <> ''),
    price NUMERIC(18,2) NOT NULL CHECK (price >= 0),
    currency TEXT NOT NULL DEFAULT 'RUB' CHECK (currency='RUB'),
    capacity INTEGER NOT NULL CHECK (capacity > 0),
    available INTEGER NOT NULL CHECK (available >= 0),
    held INTEGER NOT NULL DEFAULT 0 CHECK (held >= 0),
    sold INTEGER NOT NULL DEFAULT 0 CHECK (sold >= 0),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CHECK (available::BIGINT + held::BIGINT + sold::BIGINT = capacity)
);
CREATE INDEX ticket_types_session_id_idx ON ticket_types(session_id);
COMMIT;
