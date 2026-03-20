-- Migration: 001_create_destination_agents
-- Description: Rollback - drop destination_agents table

DROP TRIGGER IF EXISTS trigger_update_destination_agents_updated_at ON destination_agents;
DROP FUNCTION IF EXISTS update_destination_agents_updated_at();
DROP TABLE IF EXISTS destination_agents;