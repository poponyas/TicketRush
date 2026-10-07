ALTER TABLE events
ADD CONSTRAINT events_name_length_check
CHECK (char_length(name) <= 200);