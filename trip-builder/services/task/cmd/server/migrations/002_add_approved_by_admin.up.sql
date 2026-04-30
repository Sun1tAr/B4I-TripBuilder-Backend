SET search_path TO tpb_task;

ALTER TABLE tasks ADD COLUMN approved_by_admin BOOLEAN DEFAULT true;

-- Индекс для поиска неподтвержденных задач
CREATE INDEX idx_tasks_approved_by_admin ON tasks(approved_by_admin);