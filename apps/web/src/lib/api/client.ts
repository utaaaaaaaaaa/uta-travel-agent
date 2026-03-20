/**
 * API Client for UTA Travel Agent
 */

const API_BASE_URL = process.env.NEXT_PUBLIC_API_URL || 'http://localhost:8080';

// Types
export interface Agent {
  id: string;
  user_id: string;
  name: string;
  description: string;
  destination: string;
  vector_collection_id?: string;
  document_count: number;
  language: string;
  theme: string;
  status: 'creating' | 'ready' | 'busy' | 'archived' | 'error';
  tags: string[];
  created_at: string;
  updated_at: string;
  last_used_at?: string;
  usage_count: number;
  rating: number;
}

export interface AgentTask {
  id: string;
  agent_id: string;
  user_id: string;
  status: 'pending' | 'running' | 'completed' | 'failed' | 'cancelled';
  goal: string;
  result?: Record<string, unknown>;
  error?: string;
  duration_seconds: number;
  total_tokens: number;
  exploration_log: ExplorationStep[];
  radar_data?: RadarData;
  metadata?: Record<string, unknown>;
  created_at: string;
  started_at?: string;
  completed_at?: string;
}

export interface ExplorationStep {
  timestamp: string;
  direction: string;
  thought: string;
  action: string;
  tool_name?: string;
  tool_args?: Record<string, unknown>;
  result?: string;
  tokens_in: number;
  tokens_out: number;
  duration_ms: number;
  success?: boolean;
}

export interface RadarData {
  directions: RadarDirection[];
}

export interface RadarDirection {
  name: string;
  value: number;
  last_update: string;
}

export interface CreateAgentRequest {
  destination: string;
  name?: string;
  description?: string;
  theme?: string;
  languages?: string[];
  user_id?: string;
}

export interface CreateAgentResponse {
  agent_id: string;
  task_id: string;
  status: string;
  message: string;
}

export interface ChatRequest {
  message: string;
  session_id?: string;
}

export interface ChatResponse {
  response: string;
  session_id: string;
  timestamp: number;
}

class ApiError extends Error {
  constructor(public status: number, message: string) {
    super(message);
    this.name = 'ApiError';
  }
}

async function fetchApi<T>(
  path: string,
  options?: RequestInit
): Promise<T> {
  const url = `${API_BASE_URL}${path}`;

  const response = await fetch(url, {
    ...options,
    headers: {
      'Content-Type': 'application/json',
      ...options?.headers,
    },
  });

  if (!response.ok) {
    const error = await response.json().catch(() => ({ error: 'Unknown error' }));
    throw new ApiError(response.status, error.error || 'Request failed');
  }

  return response.json();
}

export const api = {
  // Health check
  async health() {
    return fetchApi<{ status: string }>('/health');
  },

  // ============ Agent APIs ============

  // List agents
  async listAgents(userId?: string): Promise<{ agents: Agent[]; count: number }> {
    const params = userId ? `?user_id=${userId}` : '';
    return fetchApi<{ agents: Agent[]; count: number }>(`/api/v1/agents${params}`);
  },

  // Get agent by ID
  async getAgent(id: string): Promise<Agent> {
    return fetchApi<Agent>(`/api/v1/agents/${id}`);
  },

  // Create agent
  async createAgent(data: CreateAgentRequest): Promise<CreateAgentResponse> {
    return fetchApi<CreateAgentResponse>('/api/v1/agents', {
      method: 'POST',
      body: JSON.stringify(data),
    });
  },

  // Delete agent
  async deleteAgent(id: string): Promise<void> {
    await fetchApi(`/api/v1/agents/${id}`, { method: 'DELETE' });
  },

  // ============ Task APIs ============

  // Get task by ID
  async getTask(id: string): Promise<AgentTask> {
    return fetchApi<AgentTask>(`/api/v1/tasks/${id}`);
  },

  // Create task for agent
  async createTask(agentId: string, goal: string): Promise<{ task_id: string; status: string }> {
    return fetchApi(`/api/v1/agents/${agentId}/tasks`, {
      method: 'POST',
      body: JSON.stringify({ goal }),
    });
  },

  // Stream task progress (SSE)
  streamTaskProgress(taskId: string, callbacks: {
    onProgress?: (data: { stage: string; step: ExplorationStep; message: string }) => void;
    onComplete?: (data: { task_id: string; status: string; agent_id: string; error?: string }) => void;
    onError?: (error: Error) => void;
  }): () => void {
    const url = `${API_BASE_URL}/api/v1/tasks/${taskId}/stream`;
    const eventSource = new EventSource(url);

    eventSource.onmessage = (event) => {
      try {
        const data = JSON.parse(event.data);
        if (callbacks.onProgress) {
          callbacks.onProgress(data);
        }
      } catch (e) {
        console.error('Failed to parse SSE data:', e);
      }
    };

    eventSource.addEventListener('progress', (event) => {
      try {
        const data = JSON.parse((event as MessageEvent).data);
        if (callbacks.onProgress) {
          callbacks.onProgress(data);
        }
      } catch (e) {
        console.error('Failed to parse progress event:', e);
      }
    });

    eventSource.addEventListener('complete', (event) => {
      try {
        const data = JSON.parse((event as MessageEvent).data);
        if (callbacks.onComplete) {
          callbacks.onComplete(data);
        }
        eventSource.close();
      } catch (e) {
        console.error('Failed to parse complete event:', e);
      }
    });

    eventSource.onerror = (error) => {
      if (callbacks.onError) {
        callbacks.onError(new Error('SSE connection error'));
      }
      eventSource.close();
    };

    // Return cleanup function
    return () => {
      eventSource.close();
    };
  },

  // ============ Chat APIs ============

  // Chat with agent
  async chat(agentId: string, data: ChatRequest): Promise<ChatResponse> {
    return fetchApi<ChatResponse>(`/api/v1/agents/${agentId}/chat`, {
      method: 'POST',
      body: JSON.stringify(data),
    });
  },

  // Streaming chat
  async *chatStream(data: ChatRequest): AsyncGenerator<string> {
    const url = `${API_BASE_URL}/api/v1/chat/stream`;
    const response = await fetch(url, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(data),
    });

    if (!response.ok) {
      throw new ApiError(response.status, 'Stream request failed');
    }

    const reader = response.body?.getReader();
    if (!reader) throw new Error('No reader available');

    const decoder = new TextDecoder();
    let buffer = '';

    while (true) {
      const { done, value } = await reader.read();
      if (done) break;

      buffer += decoder.decode(value, { stream: true });
      const lines = buffer.split('\n\n');
      buffer = lines.pop() || '';

      for (const line of lines) {
        if (line.startsWith('data: ')) {
          const data = line.slice(6);
          if (data === '[DONE]') return;
          yield data;
        }
      }
    }
  },
};

export { ApiError };

// Legacy compatibility exports
export const createDestinationAgent = api.createAgent;
export type { Agent as DestinationAgent };