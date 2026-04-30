SET search_path TO tpb_event;

DROP INDEX IF EXISTS idx_event_participants_invite_id;
ALTER TABLE event_participants DROP COLUMN IF EXISTS invite_id;