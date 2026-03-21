-- Drop triggers
DROP TRIGGER IF EXISTS trigger_sessions_updated_at ON sessions;

-- Drop function
DROP FUNCTION IF EXISTS update_session_timestamp();

-- Drop tables
DROP TABLE IF EXISTS session_memory;
DROP TABLE IF EXISTS sessions;
