// Session types for multi-session frontend

export type SessionState = "active" | "paused" | "archived";

export interface Session {
  id: string;
  agent_type: "main" | "guide" | "planner";
  title: string;
  state: SessionState;
  created_at: string;
  updated_at: string;
  last_active_at: string;
  message_count: number;
  metadata?: Record<string, unknown>;
}

export interface SessionMessage {
  id: string;
  role: "user" | "assistant" | "system";
  content: string;
  created_at: string;
}

export interface SessionGroup {
  today: Session[];
  yesterday: Session[];
  previous: Session[];
}

export interface SessionListResponse {
  sessions: Session[];
  grouped: SessionGroup;
  total: number;
}

export interface CreateSessionRequest {
  agent_type: "main" | "guide" | "planner";
  destination_id?: string;
  title?: string;
}

export interface UpdateSessionRequest {
  title?: string;
  state?: SessionState;
}

export interface ChatRequest {
  message: string;
}

export interface MessagesResponse {
  messages: SessionMessage[];
  has_more: boolean;
}
