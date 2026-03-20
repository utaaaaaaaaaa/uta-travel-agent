/**
 * React hooks for UTA Travel Agent API
 */

'use client';

import { useState, useCallback } from 'react';
import { api, ChatResponse, CreateAgentRequest, CreateAgentResponse } from '@/lib/api/client';

export function useChat() {
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [sessionId, setSessionId] = useState<string | null>(null);

  const chat = useCallback(async (message: string): Promise<string> => {
    setLoading(true);
    setError(null);
    try {
      const response: ChatResponse = await api.chat(sessionId || 'default', {
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

  const createAgent = useCallback(async (data: CreateAgentRequest): Promise<CreateAgentResponse> => {
    setLoading(true);
    setError(null);
    try {
      const response = await api.createAgent(data);
      return response;
    } catch (e) {
      const message = e instanceof Error ? e.message : 'Failed to create agent';
      setError(message);
      throw e;
    } finally {
      setLoading(false);
    }
  }, []);

  const listAgents = useCallback(async (userId?: string) => {
    setLoading(true);
    setError(null);
    try {
      const result = await api.listAgents(userId);
      return result;
    } catch (e) {
      const message = e instanceof Error ? e.message : 'Failed to list agents';
      setError(message);
      throw e;
    } finally {
      setLoading(false);
    }
  }, []);

  const getAgent = useCallback(async (id: string) => {
    setLoading(true);
    setError(null);
    try {
      const agent = await api.getAgent(id);
      return agent;
    } catch (e) {
      const message = e instanceof Error ? e.message : 'Failed to get agent';
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
    listAgents,
    getAgent,
  };
}