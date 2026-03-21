-- Migration: 003_add_task_id_to_agents (rollback)
-- Description: Remove task_id column from destination_agents

ALTER TABLE destination_agents DROP CONSTRAINT IF EXISTS fk_destination_agents_task_id;
DROP INDEX IF EXISTS idx_destination_agents_task_id;
ALTER TABLE destination_agents DROP COLUMN IF EXISTS task_id;