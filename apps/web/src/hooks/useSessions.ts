"use client";

import { useState, useEffect, useCallback } from "react";
import type {
  Session,
  SessionGroup,
  SessionListResponse,
  CreateSessionRequest,
  UpdateSessionRequest,
} from "@/types/session";

const API_URL = process.env.NEXT_PUBLIC_API_URL || "http://localhost:8080";

// Group sessions by date
function groupSessionsByDate(sessions: Session[]): SessionGroup {
  const now = new Date();
  const today = new Date(now.getFullYear(), now.getMonth(), now.getDate());
  const yesterday = new Date(today);
  yesterday.setDate(yesterday.getDate() - 1);

  const grouped: SessionGroup = {
    today: [],
    yesterday: [],
    previous: [],
  };

  for (const session of sessions) {
    const created = new Date(session.last_active_at || session.created_at);
    if (created >= today) {
      grouped.today.push(session);
    } else if (created >= yesterday) {
      grouped.yesterday.push(session);
    } else {
      grouped.previous.push(session);
    }
  }

  return grouped;
}

export function useSessions() {
  const [sessions, setSessions] = useState<Session[]>([]);
  const [grouped, setGrouped] = useState<SessionGroup>({
    today: [],
    yesterday: [],
    previous: [],
  });
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const fetchSessions = useCallback(async () => {
    try {
      setLoading(true);
      const response = await fetch(`${API_URL}/api/v1/sessions`);
      if (!response.ok) throw new Error("Failed to fetch sessions");

      const data: SessionListResponse = await response.json();
      setSessions(data.sessions || []);
      setGrouped(data.grouped || groupSessionsByDate(data.sessions || []));
      setError(null);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Unknown error");
    } finally {
      setLoading(false);
    }
  }, []);

  const createSession = useCallback(async (request: CreateSessionRequest): Promise<Session | null> => {
    try {
      const response = await fetch(`${API_URL}/api/v1/sessions`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(request),
      });
      if (!response.ok) throw new Error("Failed to create session");

      const session: Session = await response.json();
      await fetchSessions(); // Refresh list
      return session;
    } catch (err) {
      setError(err instanceof Error ? err.message : "Unknown error");
      return null;
    }
  }, [fetchSessions]);

  const updateSession = useCallback(async (id: string, request: UpdateSessionRequest): Promise<boolean> => {
    try {
      const response = await fetch(`${API_URL}/api/v1/sessions/${id}`, {
        method: "PATCH",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(request),
      });
      if (!response.ok) throw new Error("Failed to update session");

      await fetchSessions(); // Refresh list
      return true;
    } catch (err) {
      setError(err instanceof Error ? err.message : "Unknown error");
      return false;
    }
  }, [fetchSessions]);

  const deleteSession = useCallback(async (id: string): Promise<boolean> => {
    try {
      const response = await fetch(`${API_URL}/api/v1/sessions/${id}`, {
        method: "DELETE",
      });
      if (!response.ok) throw new Error("Failed to delete session");

      await fetchSessions(); // Refresh list
      return true;
    } catch (err) {
      setError(err instanceof Error ? err.message : "Unknown error");
      return false;
    }
  }, [fetchSessions]);

  useEffect(() => {
    fetchSessions();
  }, [fetchSessions]);

  return {
    sessions,
    grouped,
    loading,
    error,
    refetch: fetchSessions,
    createSession,
    updateSession,
    deleteSession,
  };
}

export function useSession(sessionId: string | null) {
  const [session, setSession] = useState<Session | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const fetchSession = useCallback(async () => {
    if (!sessionId) {
      setSession(null);
      setLoading(false);
      return;
    }

    try {
      setLoading(true);
      const response = await fetch(`${API_URL}/api/v1/sessions/${sessionId}`);
      if (!response.ok) throw new Error("Failed to fetch session");

      const data: Session = await response.json();
      setSession(data);
      setError(null);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Unknown error");
    } finally {
      setLoading(false);
    }
  }, [sessionId]);

  useEffect(() => {
    fetchSession();
  }, [fetchSession]);

  return {
    session,
    loading,
    error,
    refetch: fetchSession,
  };
}
