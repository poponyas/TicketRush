BEGIN;
-- Global venue-creation permission is separate from organizer membership.
CREATE TABLE user_roles (
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    role TEXT NOT NULL CHECK(role='ownerVenue'),
    PRIMARY KEY(user_id,role)
);
COMMIT;
