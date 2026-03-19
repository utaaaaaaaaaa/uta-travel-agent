/**
 * React hooks for UTA Travel Agent API
 */

'use client';

import { useState, useCallback } from 'react';
import { api, ChatResponse, CreateDestinationAgentRequest, CreateDestinationAgentResponse } from '@/lib/api/client';

export function useChat() {
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [sessionId, setSessionId] = useState<string | null>(null);

  const chat = useCallback(async (message: string): Promise<string> => {
    setLoading(true);
    setError(null);
    try {
      const response: ChatResponse = await api.chat({
        message,
        session_id: sessionId || undefined,
      });
      setSessionId(response.session_id);
      return response.response;
    } catch (e) {
      const message = e instanceof Error ? e.message : 'Chat failed';
      setError(message);
      throw e;
    } finally {
      setLoading(false);
    }
  }, [sessionId]);

  const chatStream = useCallback(async function* (message: string): AsyncGenerator<string> {
    setLoading(true);
    setError(null);

    try {
      for await (const chunk of api.chatStream({
        message,
        session_id: sessionId || undefined,
      })) {
        yield chunk;
      }
    } catch (e) {
      const message = e instanceof Error ? e.message : 'Stream failed';
      setError(message);
      throw e;
    } finally {
      setLoading(false);
    }
  }, [sessionId]);

  const clearSession = useCallback(() => {
    setSessionId(null);
  }, []);

  return {
    loading,
    error,
    sessionId,
    chat,
    chatStream,
    clearSession,
  };
}

export function useAgents() {
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const createAgent = useCallback(async (data: CreateDestinationAgentRequest): Promise<CreateDestinationAgentResponse> => {
    setLoading(true);
    setError(null);
    try {
      const response = await api.createDestinationAgent(data);
      return response;
    } catch (e) {
      const message = e instanceof Error ? e.message : 'Failed to create agent';
      setError(message);
      throw e;
    } finally {
      setLoading(false);
    }
  }, []);

  const getAgentStatus = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const status = await api.getAgentStatus();
      return status;
    } catch (e) {
      const message = e instanceof Error ? e.message : 'Failed to get status';
      setError(message);
      throw e;
    } finally {
      setLoading(false);
    }
  }, []);

  return {
    loading,
    error,
    createAgent,
    getAgentStatus,
  };
}