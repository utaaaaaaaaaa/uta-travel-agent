/**
 * Hook for managing destination agent creation flow
 */

'use client';

import { useState, useCallback, useRef, useEffect } from 'react';
import { api, CreateAgentResponse } from '@/lib/api/client';

export interface CreationStep {
  id: string;
  name: string;
  description: string;
  status: 'pending' | 'running' | 'completed' | 'error';
  details?: string;
}

interface CreationState {
  status: 'idle' | 'creating' | 'completed' | 'error';
  steps: CreationStep[];
  progress: number;
  error: string | null;
  agentId: string | null;
}

const DEFAULT_STEPS: CreationStep[] = [
  {
    id: 'researcher',
    name: '信息研究',
    description: '搜索并收集旅游目的地信息',
    status: 'pending',
  },
  {
    id: 'curator',
    name: '信息整理',
    description: '整理和结构化收集的信息',
    status: 'pending',
  },
  {
    id: 'indexer',
    name: '构建索引',
    description: '创建向量索引用于智能检索',
    status: 'pending',
  },
];

export function useAgentCreation() {
  const [state, setState] = useState<CreationState>({
    status: 'idle',
    steps: DEFAULT_STEPS,
    progress: 0,
    error: null,
    agentId: null,
  });

  const pollIntervalRef = useRef<NodeJS.Timeout | null>(null);
  const wsRef = useRef<WebSocket | null>(null);

  // Clean up on unmount
  useEffect(() => {
    return () => {
      if (pollIntervalRef.current) {
        clearInterval(pollIntervalRef.current);
      }
      if (wsRef.current) {
        wsRef.current.close();
      }
    };
  }, []);

  // Update step status
  const updateStep = useCallback((stepId: string, updates: Partial<CreationStep>) => {
    setState(prev => ({
      ...prev,
      steps: prev.steps.map(step =>
        step.id === stepId ? { ...step, ...updates } : step
      ),
    }));
  }, []);

  // Calculate overall progress
  const calculateProgress = useCallback((steps: CreationStep[]): number => {
    const completedCount = steps.filter(s => s.status === 'completed').length;
    const runningStep = steps.find(s => s.status === 'running');

    // Each completed step is 30%, running step can add up to 10%
    let progress = completedCount * 30;
    if (runningStep) {
      progress += 10; // Assume 10% for running step
    }
    return Math.min(progress, 100);
  }, []);

  // Try to connect to WebSocket for real-time updates
  const connectWebSocket = useCallback((agentId: string) => {
    const wsUrl = `${process.env.NEXT_PUBLIC_WS_URL || 'ws://localhost:8080'}/ws/agent/${agentId}/progress`;

    try {
      wsRef.current = new WebSocket(wsUrl);

      wsRef.current.onmessage = (event) => {
        try {
          const data = JSON.parse(event.data);

          if (data.type === 'step_update') {
            updateStep(data.step_id, {
              status: data.status,
              details: data.details,
            });

            setState(prev => ({
              ...prev,
              progress: calculateProgress(prev.steps),
            }));
          } else if (data.type === 'completion') {
            setState(prev => ({
              ...prev,
              status: 'completed',
              progress: 100,
            }));
            wsRef.current?.close();
          } else if (data.type === 'error') {
            setState(prev => ({
              ...prev,
              status: 'error',
              error: data.message,
            }));
            updateStep(data.step_id, { status: 'error', details: data.message });
          }
        } catch {
          // Ignore parse errors
        }
      };

      wsRef.current.onerror = () => {
        // Fall back to polling if WebSocket fails
        startPolling(agentId);
      };
    } catch {
      // Fall back to polling
      startPolling(agentId);
    }
  }, [updateStep, calculateProgress]);

  // Polling fallback for status updates
  const startPolling = useCallback((agentId: string) => {
    pollIntervalRef.current = setInterval(async () => {
      try {
        const agent = await api.getAgent(agentId);

        // Update steps based on agent status
        if (agent.status === 'ready') {
          setState(prev => ({
            ...prev,
            status: 'completed',
            progress: 100,
            steps: prev.steps.map(s => ({ ...s, status: 'completed' as const })),
          }));
          if (pollIntervalRef.current) {
            clearInterval(pollIntervalRef.current);
          }
        } else if (agent.status === 'error') {
          setState(prev => ({
            ...prev,
            status: 'error',
            error: 'Agent creation failed',
          }));
          if (pollIntervalRef.current) {
            clearInterval(pollIntervalRef.current);
          }
        }
      } catch {
        // Ignore polling errors
      }
    }, 2000);
  }, []);

  // Start creation process
  const create = useCallback(async (data: {
    destination: string;
    theme?: string;
    languages?: string[];
  }) => {
    setState(prev => ({
      ...prev,
      status: 'creating',
      steps: DEFAULT_STEPS.map(s => ({ ...s, status: 'pending' as const })),
      progress: 0,
      error: null,
    }));

    // Step 1: Start with researcher
    updateStep('researcher', { status: 'running' });
    setState(prev => ({ ...prev, progress: 10 }));

    try {
      // Call API to create agent
      const response: CreateAgentResponse = await api.createAgent(data);

      setState(prev => ({ ...prev, agentId: response.agent_id }));

      // Connect to WebSocket or fall back to polling
      connectWebSocket(response.agent_id);

      // Simulate step progression (will be replaced by real WebSocket updates)
      // This is temporary until backend is fully integrated
      simulateProgress(response.agent_id);

      return response;
    } catch (e) {
      const message = e instanceof Error ? e.message : '创建失败';
      setState(prev => ({
        ...prev,
        status: 'error',
        error: message,
      }));
      updateStep('researcher', { status: 'error', details: message });
      throw e;
    }
  }, [updateStep, connectWebSocket]);

  // Simulate progress (temporary, will be replaced by real updates)
  const simulateProgress = useCallback((agentId: string) => {
    const delays = [3000, 4000, 5000]; // Delay for each step
    let currentDelay = 0;

    // Step 1: Researcher completes
    setTimeout(() => {
      updateStep('researcher', { status: 'completed', details: '已收集 15 篇文档' });
      updateStep('curator', { status: 'running' });
      setState(prev => ({ ...prev, progress: 40 }));
    }, delays[0]);

    // Step 2: Curator completes
    setTimeout(() => {
      updateStep('curator', { status: 'completed', details: '已整理 8 个类别' });
      updateStep('indexer', { status: 'running' });
      setState(prev => ({ ...prev, progress: 70 }));
    }, delays[0] + delays[1]);

    // Step 3: Indexer completes
    setTimeout(() => {
      updateStep('indexer', { status: 'completed', details: '已索引 120 个向量' });
      setState(prev => ({
        ...prev,
        status: 'completed',
        progress: 100,
      }));
    }, delays[0] + delays[1] + delays[2]);
  }, [updateStep]);

  // Reset state
  const reset = useCallback(() => {
    setState({
      status: 'idle',
      steps: DEFAULT_STEPS,
      progress: 0,
      error: null,
      agentId: null,
    });
  }, []);

  return {
    ...state,
    create,
    reset,
    updateStep,
  };
}