SET search_path TO tpb_event;

DROP INDEX IF EXISTS idx_event_participants_unique_user;

ALTER TABLE event_participants DROP CONSTRAINT event_participants_pkey;
ALTER TABLE event_participants ADD PRIMARY KEY (event_id, user_id);