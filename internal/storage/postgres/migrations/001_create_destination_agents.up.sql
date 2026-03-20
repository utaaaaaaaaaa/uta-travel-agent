-- Migration: 001_create_destination_agents
-- Description: Create destination_agents table for persisting agent metadata

CREATE TABLE IF NOT EXISTS destination_agents (
    id VARCHAR(36) PRIMARY KEY,
    user_id VARCHAR(36) NOT NULL,
    name VARCHAR(255),
    description TEXT,
    destination VARCHAR(255) NOT NULL,
    vector_collection_id VARCHAR(255),
    document_count INT DEFAULT 0,
    language VARCHAR(10) DEFAULT 'zh',
    theme VARCHAR(50) DEFAULT 'cultural',
    status VARCHAR(20) DEFAULT 'creating' CHECK (status IN ('creating', 'ready', 'busy', 'archived', 'error')),
    tags TEXT[] DEFAULT '{}',
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    last_used_at TIMESTAMP WITH TIME ZONE,
    usage_count INT DEFAULT 0,
    rating FLOAT DEFAULT 0 CHECK (rating >= 0 AND rating <= 5)
);

-- Indexes for common queries
CREATE INDEX IF NOT EXISTS idx_destination_agents_user_id ON destination_agents(user_id);
CREATE INDEX IF NOT EXISTS idx_destination_agents_status ON destination_agents(status);
CREATE INDEX IF NOT EXISTS idx_destination_agents_destination ON destination_agents(destination);

-- Update timestamp trigger
CREATE OR REPLACE FUNCTION update_destination_agents_updated_at()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trigger_update_destination_agents_updated_at
    BEFORE UPDATE ON destination_agents
    FOR EACH ROW
    EXECUTE FUNCTION update_destination_agents_updated_at();

-- Comments
COMMENT ON TABLE destination_agents IS 'Persisted destination agents with metadata';
COMMENT ON COLUMN destination_agents.id IS 'Unique agent identifier';
COMMENT ON COLUMN destination_agents.user_id IS 'Owner user identifier';
COMMENT ON COLUMN destination_agents.vector_collection_id IS 'Qdrant collection ID for RAG';
COMMENT ON COLUMN destination_agents.status IS 'Agent status: creating, ready, busy, archived, error';