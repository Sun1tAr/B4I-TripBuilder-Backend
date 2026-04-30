SET search_path TO tpb_task;

DROP INDEX IF EXISTS idx_tasks_approved_by_admin;
ALTER TABLE tasks DROP COLUMN IF EXISTS approved_by_admin;