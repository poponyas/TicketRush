-- Adopt the exact schema of immutable migrations 001/002 when it was applied
-- manually before migrate was introduced. Never guess after a later failure.
BEGIN;

CREATE TEMP TABLE expected_events (
    id UUID PRIMARY KEY,
    name TEXT NOT NULL CHECK (btrim(name) <> ''),
    total_tickets INTEGER NOT NULL CHECK (total_tickets > 0),
    available_tickets INTEGER NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CHECK (available_tickets >= 0 AND available_tickets <= total_tickets)
) ON COMMIT DROP;
CREATE TEMP TABLE expected_bookings (
    id UUID PRIMARY KEY,
    event_id UUID NOT NULL REFERENCES expected_events(id),
    user_id TEXT NOT NULL CHECK (btrim(user_id) <> ''),
    quantity INTEGER NOT NULL CHECK (quantity > 0),
    status TEXT NOT NULL DEFAULT 'reserved'
        CHECK (status IN ('reserved', 'confirmed', 'cancelled', 'expired')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
) ON COMMIT DROP;
CREATE INDEX expected_bookings_event_id_idx ON expected_bookings(event_id);

CREATE FUNCTION pg_temp.normalize_definition(definition TEXT) RETURNS TEXT
LANGUAGE SQL IMMUTABLE AS $$
    SELECT regexp_replace(
        replace(replace(definition, 'expected_events', 'events'), 'expected_bookings', 'bookings'),
        '(pg_temp(_[0-9]+)?|public)\.', '', 'g'
    );
$$;

CREATE FUNCTION pg_temp.table_signature(relation REGCLASS) RETURNS JSONB
LANGUAGE SQL STABLE AS $$
    SELECT jsonb_build_object(
        'columns', (
            SELECT jsonb_agg(jsonb_build_array(
                a.attname, format_type(a.atttypid, a.atttypmod), a.attnotnull,
                pg_get_expr(d.adbin, d.adrelid), a.attidentity, a.attgenerated, a.attcollation
            ) ORDER BY a.attnum)
            FROM pg_attribute a
            LEFT JOIN pg_attrdef d ON d.adrelid=a.attrelid AND d.adnum=a.attnum
            WHERE a.attrelid=relation AND a.attnum>0 AND NOT a.attisdropped
        ),
        'constraints', (
            SELECT jsonb_agg(jsonb_build_array(
                pg_temp.normalize_definition(c.conname), c.contype::TEXT,
                pg_temp.normalize_definition(pg_get_constraintdef(c.oid)), c.convalidated
            ) ORDER BY pg_temp.normalize_definition(c.conname))
            FROM pg_constraint c WHERE c.conrelid=relation
        ),
        'indexes', (
            SELECT jsonb_agg(jsonb_build_array(
                pg_temp.normalize_definition(pg_get_indexdef(i.indexrelid)),
                i.indisvalid, i.indisready
            ) ORDER BY pg_temp.normalize_definition(pg_get_indexdef(i.indexrelid)))
            FROM pg_index i WHERE i.indrelid=relation
        )
    );
$$;

DO $prepare$
DECLARE
    current_version BIGINT;
    current_dirty BOOLEAN;
    metadata_rows INTEGER;
    baseline INTEGER := 0;
    events_relation REGCLASS := to_regclass('public.events');
    bookings_relation REGCLASS := to_regclass('public.bookings');
BEGIN
    IF to_regclass('public.schema_migrations') IS NOT NULL THEN
        LOCK TABLE public.schema_migrations IN EXCLUSIVE MODE;
        SELECT count(*) INTO metadata_rows FROM public.schema_migrations;
        IF metadata_rows > 1 THEN
            RAISE EXCEPTION 'Migration metadata has more than one version; inspect it manually.';
        END IF;
        SELECT version,dirty INTO current_version,current_dirty FROM public.schema_migrations;
        IF current_version IS NOT NULL AND NOT current_dirty THEN
            RAISE NOTICE 'Migration version % is clean; nothing to repair.', current_version;
            RETURN;
        END IF;
        IF current_dirty AND current_version <> 1 THEN
            RAISE EXCEPTION 'Dirty migration % needs an explicit inspection; automatic adoption only supports the initial legacy schema.', current_version;
        END IF;
    END IF;

    IF events_relation IS NULL AND bookings_relation IS NULL AND NOT coalesce(current_dirty,FALSE) THEN
        RAISE NOTICE 'Empty database; migrate will apply the schema normally.';
        RETURN;
    END IF;
    IF events_relation IS NULL OR bookings_relation IS NULL THEN
        RAISE EXCEPTION 'Incomplete legacy schema; no migration version has been changed.';
    END IF;
    IF EXISTS (
        SELECT 1 FROM pg_class c JOIN pg_namespace n ON n.oid=c.relnamespace
        WHERE n.nspname='public' AND c.relkind IN ('r','p')
          AND c.relname NOT IN ('events','bookings','schema_migrations')
    ) THEN
        RAISE EXCEPTION 'Unexpected business tables in the untracked schema; inspect them before adopting a migration version.';
    END IF;

    IF pg_temp.table_signature(events_relation)=pg_temp.table_signature('expected_events')
       AND pg_temp.table_signature(bookings_relation)=pg_temp.table_signature('expected_bookings') THEN
        baseline := 1;
    END IF;

    ALTER TABLE expected_events ADD CONSTRAINT events_name_length_check
        CHECK (char_length(name)<=200);
    IF pg_temp.table_signature(events_relation)=pg_temp.table_signature('expected_events')
       AND pg_temp.table_signature(bookings_relation)=pg_temp.table_signature('expected_bookings') THEN
        baseline := 2;
    END IF;

    IF baseline=0 THEN
        RAISE EXCEPTION 'Legacy tables differ from migrations 001/002; no migration version has been changed. Inspect columns, defaults, constraint names and indexes.';
    END IF;

    -- Only the migration journal changes. Business tables and rows are untouched.
    CREATE TABLE IF NOT EXISTS public.schema_migrations (
        version BIGINT PRIMARY KEY,
        dirty BOOLEAN NOT NULL
    );
    DELETE FROM public.schema_migrations;
    INSERT INTO public.schema_migrations(version,dirty) VALUES(baseline,FALSE);
    RAISE NOTICE 'Verified legacy schema; adopted migration version % without changing event or booking data.', baseline;
END
$prepare$;

COMMIT;
