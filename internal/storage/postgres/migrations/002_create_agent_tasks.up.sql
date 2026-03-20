-- Migration: 002_create_agent_tasks
-- Description: Create agent_tasks table for tracking task execution

CREATE TABLE IF NOT EXISTS agent_tasks (
    id VARCHAR(36) PRIMARY KEY,
    agent_id VARCHAR(36) REFERENCES destination_agents(id) ON DELETE CASCADE,
    user_id VARCHAR(36) NOT NULL,
    status VARCHAR(20) DEFAULT 'pending' CHECK (status IN ('pending', 'running', 'completed', 'failed', 'cancelled')),
    goal TEXT,
    result JSONB,
    error TEXT,
    duration_seconds FLOAT DEFAULT 0,
    total_tokens INT DEFAULT 0,
    exploration_log JSONB DEFAULT '[]',
    radar_data JSONB DEFAULT '{"directions": []}',
    metadata JSONB DEFAULT '{}',
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    started_at TIMESTAMP WITH TIME ZONE,
    completed_at TIMESTAMP WITH TIME ZONE
);

-- Indexes for common queries
CREATE INDEX IF NOT EXISTS idx_agent_tasks_agent_id ON agent_tasks(agent_id);
CREATE INDEX IF NOT EXISTS idx_agent_tasks_user_id ON agent_tasks(user_id);
CREATE INDEX IF NOT EXISTS idx_agent_tasks_status ON agent_tasks(status);
CREATE INDEX IF NOT EXISTS idx_agent_tasks_created_at ON agent_tasks(created_at DESC);

-- Comments
COMMENT ON TABLE agent_tasks IS 'Task execution records with exploration logs';
COMMENT ON COLUMN agent_tasks.exploration_log IS 'Array of exploration steps for radar visualization';
COMMENT ON COLUMN agent_tasks.radar_data IS 'Aggregated radar chart data';