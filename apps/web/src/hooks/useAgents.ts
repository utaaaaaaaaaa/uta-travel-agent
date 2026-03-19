/**
 * React hooks for Destination Agent API
 */

'use client';

import { useState, useCallback } from 'react';
import { api, Agent, CreateAgentRequest, QueryResponse, ApiError } from '@/lib/api/client';

// Temporary user ID (should be from auth later)
const TEMP_USER_ID = 'demo-user';

export function useAgents() {
  const [agents, setAgents] = useState<Agent[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const fetchAgents = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const result = await api.listAgents(TEMP_USER_ID);
      setAgents(result.agents);
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Failed to fetch agents');
    } finally {
      setLoading(false);
    }
  }, []);

  const createAgent = useCallback(async (data: Omit<CreateAgentRequest, 'user_id'>) => {
    setLoading(true);
    setError(null);
    try {
      const agent = await api.createAgent({
        ...data,
        user_id: TEMP_USER_ID,
      });
      setAgents((prev) => [...prev, agent]);
      return agent;
    } catch (e) {
      const message = e instanceof Error ? e.message : 'Failed to create agent';
      setError(message);
      throw e;
    } finally {
      setLoading(false);
    }
  }, []);

  const deleteAgent = useCallback(async (id: string) => {
    setLoading(true);
    setError(null);
    try {
      await api.deleteAgent(id);
      setAgents((prev) => prev.filter((a) => a.id !== id));
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Failed to delete agent');
    } finally {
      setLoading(false);
    }
  }, []);

  return {
    agents,
    loading,
    error,
    fetchAgents,
    createAgent,
    deleteAgent,
  };
}

export function useAgent(agentId: string | null) {
  const [agent, setAgent] = useState<Agent | null>(null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const fetchAgent = useCallback(async () => {
    if (!agentId) return;

    setLoading(true);
    setError(null);
    try {
      const result = await api.getAgent(agentId);
      setAgent(result);
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Failed to fetch agent');
    } finally {
      setLoading(false);
    }
  }, [agentId]);

  const pollUntilReady = useCallback(async (
    onProgress?: (agent: Agent) => void,
    maxAttempts = 60,
    interval = 2000
  ) => {
    if (!agentId) return null;

    for (let i = 0; i < maxAttempts; i++) {
      const result = await api.getAgent(agentId);
      setAgent(result);
      onProgress?.(result);

      if (result.status === 'ready' || result.status === 'failed') {
        return result;
      }

      await new Promise((resolve) => setTimeout(resolve, interval));
    }

    return null;
  }, [agentId]);

  return {
    agent,
    loading,
    error,
    fetchAgent,
    pollUntilReady,
  };
}

export function useAgentQuery(agentId: string | null) {
  const [response, setResponse] = useState<QueryResponse | null>(null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const query = useCallback(async (question: string, topK = 5) => {
    if (!agentId) return;

    setLoading(true);
    setError(null);
    setResponse(null);

    try {
      const result = await api.queryAgent(agentId, { question, top_k: topK });
      setResponse(result);
      return result;
    } catch (e) {
      const message = e instanceof Error ? e.message : 'Query failed';
      setError(message);
      throw e;
    } finally {
      setLoading(false);
    }
  }, [agentId]);

  const queryStream = useCallback(async function* (
    question: string,
    topK = 5
  ): AsyncGenerator<string> {
    if (!agentId) return;

    setLoading(true);
    setError(null);

    try {
      for await (const chunk of api.queryAgentStream(agentId, { question, top_k: topK })) {
        yield chunk;
      }
    } catch (e) {
      const message = e instanceof Error ? e.message : 'Stream query failed';
      setError(message);
      throw e;
    } finally {
      setLoading(false);
    }
  }, [agentId]);

  return {
    response,
    loading,
    error,
    query,
    queryStream,
  };
}
