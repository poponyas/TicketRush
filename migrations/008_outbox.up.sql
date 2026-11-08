BEGIN;

CREATE TABLE outbox_events(
    id UUID PRIMARY KEY,
    aggregate_id UUID NOT NULL,
    type TEXT NOT NULL,
    schema_version INTEGER NOT NULL CHECK(schema_version > 0),
    payload JSONB NOT NULL,
    created_at TIMESTAMPTZ NOT NULL,
    published_at TIMESTAMPTZ,
    attempts INTEGER NOT NULL DEFAULT 0 CHECK(attempts >= 0)
);

CREATE INDEX outbox_events_pending_idx
    ON outbox_events(created_at, id)
    WHERE published_at IS NULL;

COMMIT;