CREATE TABLE IF NOT EXISTS ticketrush.booking_events
(
    message_id UUID,
    booking_id UUID,
    event_id UUID,
    session_id UUID,
    ticket_type_id UUID,

    event_type LowCardinality(String),
    schema_version UInt32,
    quantity UInt32,
    occurred_at DateTime64(6, 'UTC')
)
ENGINE = MergeTree
ORDER BY (event_id, occurred_at, message_id);