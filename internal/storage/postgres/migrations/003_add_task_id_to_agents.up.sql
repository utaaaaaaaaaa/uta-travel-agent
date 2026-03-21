-- Migration: 003_add_task_id_to_agents
-- Description: Add task_id column to destination_agents to link to creation task

ALTER TABLE destination_agents ADD COLUMN IF NOT EXISTS task_id VARCHAR(36);

-- Create index for task lookup
CREATE INDEX IF NOT EXISTS idx_destination_agents_task_id ON destination_agents(task_id);

-- Add foreign key constraint
ALTER TABLE destination_agents
    ADD CONSTRAINT fk_destination_agents_task_id
    FOREIGN KEY (task_id) REFERENCES agent_tasks(id) ON DELETE SET NULL;

COMMENT ON COLUMN destination_agents.task_id IS 'Reference to the agent creation task';