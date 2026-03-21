-- Create sessions table for persistent session management
CREATE TABLE IF NOT EXISTS sessions (
    id VARCHAR(64) PRIMARY KEY,
    agent_type VARCHAR(50) NOT NULL,
    state VARCHAR(20) DEFAULT 'active',
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    last_active_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    title VARCHAR(255),
    message_count INTEGER DEFAULT 0,
    metadata JSONB DEFAULT '{}'
);

-- Create indexes for common queries
CREATE INDEX IF NOT EXISTS idx_sessions_agent_type ON sessions(agent_type);
CREATE INDEX IF NOT EXISTS idx_sessions_state ON sessions(state);
CREATE INDEX IF NOT EXISTS idx_sessions_last_active ON sessions(last_active_at DESC);
CREATE INDEX IF NOT EXISTS idx_sessions_created ON sessions(created_at DESC);
CREATE INDEX IF NOT EXISTS idx_sessions_user_id ON sessions((metadata->>'user_id'));

-- Create session_memory table for persistent memory storage
CREATE TABLE IF NOT EXISTS session_memory (
    id VARCHAR(64) PRIMARY KEY,
    session_id VARCHAR(64) NOT NULL REFERENCES sessions(id) ON DELETE CASCADE,
    type VARCHAR(50) NOT NULL,
    content TEXT NOT NULL,
    role VARCHAR(20),
    metadata JSONB DEFAULT '{}',
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Indexes for session_memory
CREATE INDEX IF NOT EXISTS idx_session_memory_session_id ON session_memory(session_id);
CREATE INDEX IF NOT EXISTS idx_session_memory_type ON session_memory(type);
CREATE INDEX IF NOT EXISTS idx_session_memory_created ON session_memory(created_at);

-- Function to update updated_at timestamp
CREATE OR REPLACE FUNCTION update_session_timestamp()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- Trigger for automatic timestamp update
DROP TRIGGER IF EXISTS trigger_sessions_updated_at ON sessions;
CREATE TRIGGER trigger_sessions_updated_at
    BEFORE UPDATE ON sessions
    FOR EACH ROW
    EXECUTE FUNCTION update_session_timestamp();
