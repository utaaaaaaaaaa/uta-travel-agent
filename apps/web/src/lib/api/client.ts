/**
 * API Client for Destination Agent Service
 */

const API_BASE_URL = process.env.NEXT_PUBLIC_API_URL || 'http://localhost:8001';

export interface Agent {
  id: string;
  user_id: string;
  name: string;
  description: string;
  destination: string;
  status: 'creating' | 'ready' | 'failed';
  document_count: number;
  chunk_count: number;
}

export interface CreateAgentRequest {
  user_id: string;
  destination: string;
  theme?: string;
  languages?: string[];
  tags?: string[];
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
    return fetchApi<{ status: string }>('/health');
  },

  // Agent CRUD
  async createAgent(data: CreateAgentRequest): Promise<Agent> {
    return fetchApi<Agent>('/agents', {
      method: 'POST',
      body: JSON.stringify(data),
    });
  },

  async getAgent(id: string): Promise<Agent> {
    return fetchApi<Agent>(`/agents/${id}`);
  },

  async listAgents(userId: string): Promise<{ agents: Agent[] }> {
    return fetchApi<{ agents: Agent[] }>(`/agents?user_id=${encodeURIComponent(userId)}`);
  },

  async deleteAgent(id: string): Promise<void> {
    await fetchApi(`/agents/${id}`, { method: 'DELETE' });
  },

  // Query
  async queryAgent(id: string, data: QueryRequest): Promise<QueryResponse> {
    return fetchApi<QueryResponse>(`/agents/${id}/query`, {
      method: 'POST',
      body: JSON.stringify(data),
    });
  },

  // Streaming query
  async *queryAgentStream(
    id: string,
    data: QueryRequest
  ): AsyncGenerator<string> {
    const url = `${API_BASE_URL}/agents/${id}/query/stream`;
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
          if (data.startsWith('[ERROR]')) {
            throw new Error(data.slice(8));
          }
          yield data;
        }
      }
    }
  },

  // Stats
  async getAgentStats(id: string) {
    return fetchApi(`/agents/${id}/stats`);
  },
};

export { ApiError };
