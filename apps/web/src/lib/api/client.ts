/**
 * API Client for UTA Travel Agent
 */

const API_BASE_URL = process.env.NEXT_PUBLIC_API_URL || 'http://localhost:8080';

export interface Agent {
  id: string;
  user_id: string;
  name: string;
  description: string;
  destination: string;
  status: 'creating' | 'ready' | 'failed';
  document_count: number;
  chunk_count?: number;
}

export interface CreateAgentRequest {
  destination: string;
  theme?: string;
  languages?: string[];
  tags?: string[];
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

export interface AgentStatus {
  agent_id: string;
  type: string;
  state: string;
  subagents: string[];
  memory_size: number;
}

export interface CreateDestinationAgentRequest {
  destination: string;
  theme?: string;
  languages?: string[];
  tags?: string[];
}

export interface CreateDestinationAgentResponse {
  agent_id: string;
  destination: string;
  status: string;
  message: string;
}

export interface QueryRequest {
  question: string;
  top_k?: number;
}

export interface QueryResponse {
  answer: string;
  sources: Array<{
    content: string;
    score: number;
    document_id?: string;
  }>;
  confidence: number;
  question: string;
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
    const error = await response.json().catch(() => ({ detail: 'Unknown error' }));
    throw new ApiError(response.status, error.detail || 'Request failed');
  }

  return response.json();
}

export const api = {
  // Health check
  async health() {
    return fetchApi<{ status: string; timestamp: number; service: string }>('/health');
  },

  // Chat with MainAgent
  async chat(data: ChatRequest): Promise<ChatResponse> {
    return fetchApi<ChatResponse>('/api/v1/chat', {
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
          if (data.startsWith('{') && data.includes('error')) {
            throw new Error(data);
          }
          yield data;
        }
      }
    }
  },

  // Get agent status
  async getAgentStatus(): Promise<AgentStatus> {
    return fetchApi<AgentStatus>('/api/v1/agent/status');
  },

  // Create destination agent
  async createDestinationAgent(data: CreateDestinationAgentRequest): Promise<CreateDestinationAgentResponse> {
    return fetchApi<CreateDestinationAgentResponse>('/api/v1/agent/create', {
      method: 'POST',
      body: JSON.stringify(data),
    });
  },
};

export { ApiError };
