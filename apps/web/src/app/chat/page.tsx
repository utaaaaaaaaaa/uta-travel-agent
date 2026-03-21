"use client";

import { useState, useEffect, useRef, useCallback, Suspense } from "react";
import { useRouter, useSearchParams } from "next/navigation";
import Link from "next/link";
import {
  SessionSidebar,
  ChatMessages,
  ChatInput,
  ChatHeader,
} from "@/components/chat";
import type { SessionMessage, Session } from "@/types/session";
import { Button } from "@/components/ui/button";
import { Globe, Loader2 } from "lucide-react";

const API_URL = process.env.NEXT_PUBLIC_API_URL || "http://localhost:8080";

// Simple hook for single session
function useSessionData(sessionId: string | null) {
  const [session, setSession] = useState<Session | null>(null);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    if (!sessionId) {
      setSession(null);
      setLoading(false);
      return;
    }

    const fetchSession = async () => {
      try {
        setLoading(true);
        const response = await fetch(`${API_URL}/api/v1/sessions/${sessionId}`);
        if (response.ok) {
          const data = await response.json();
          setSession(data);
        }
      } catch (error) {
        console.error("Failed to fetch session:", error);
      } finally {
        setLoading(false);
      }
    };

    fetchSession();
  }, [sessionId]);

  return { session, loading };
}

// Hook for sessions list
function useSessionsData() {
  const [sessions, setSessions] = useState<Session[]>([]);
  const [grouped, setGrouped] = useState<{ today: Session[]; yesterday: Session[]; previous: Session[] }>({
    today: [],
    yesterday: [],
    previous: [],
  });
  const [loading, setLoading] = useState(true);

  const fetchSessions = useCallback(async () => {
    try {
      setLoading(true);
      const response = await fetch(`${API_URL}/api/v1/sessions`);
      if (response.ok) {
        const data = await response.json();
        setSessions(data.sessions || []);
        setGrouped(data.grouped || { today: [], yesterday: [], previous: [] });
      }
    } catch (error) {
      console.error("Failed to fetch sessions:", error);
    } finally {
      setLoading(false);
    }
  }, []);

  const createSession = useCallback(async (request: { agent_type: string }): Promise<Session | null> => {
    try {
      const response = await fetch(`${API_URL}/api/v1/sessions`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(request),
      });
      if (response.ok) {
        const session = await response.json();
        await fetchSessions();
        return session;
      }
    } catch (error) {
      console.error("Failed to create session:", error);
    }
    return null;
  }, [fetchSessions]);

  const updateSession = useCallback(async (id: string, request: { title?: string; state?: string }): Promise<boolean> => {
    try {
      const response = await fetch(`${API_URL}/api/v1/sessions/${id}`, {
        method: "PATCH",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(request),
      });
      if (response.ok) {
        await fetchSessions();
        return true;
      }
    } catch (error) {
      console.error("Failed to update session:", error);
    }
    return false;
  }, [fetchSessions]);

  const deleteSession = useCallback(async (id: string): Promise<boolean> => {
    try {
      const response = await fetch(`${API_URL}/api/v1/sessions/${id}`, {
        method: "DELETE",
      });
      if (response.ok) {
        await fetchSessions();
        return true;
      }
    } catch (error) {
      console.error("Failed to delete session:", error);
    }
    return false;
  }, [fetchSessions]);

  useEffect(() => {
    fetchSessions();
  }, [fetchSessions]);

  return { sessions, grouped, loading, createSession, updateSession, deleteSession };
}

function ChatPageContent() {
  const router = useRouter();
  const searchParams = useSearchParams();
  const sessionId = searchParams.get("session");

  const {
    sessions,
    grouped,
    loading: sessionsLoading,
    createSession,
    updateSession,
    deleteSession,
  } = useSessionsData();

  const { session, loading: sessionLoading } = useSessionData(sessionId);

  const [messages, setMessages] = useState<SessionMessage[]>([]);
  const [messagesLoading, setMessagesLoading] = useState(false);
  const [chatLoading, setChatLoading] = useState(false);
  const messagesEndRef = useRef<HTMLDivElement>(null);

  // Fetch messages when session changes
  const fetchMessages = useCallback(async () => {
    if (!sessionId) {
      setMessages([]);
      return;
    }

    try {
      setMessagesLoading(true);
      const response = await fetch(`${API_URL}/api/v1/sessions/${sessionId}/messages`);
      if (response.ok) {
        const data = await response.json();
        setMessages(data.messages || []);
      }
    } catch (error) {
      console.error("Failed to fetch messages:", error);
    } finally {
      setMessagesLoading(false);
    }
  }, [sessionId]);

  useEffect(() => {
    fetchMessages();
  }, [fetchMessages]);

  // Scroll to bottom on new messages
  useEffect(() => {
    messagesEndRef.current?.scrollIntoView({ behavior: "smooth" });
  }, [messages]);

  // Handle session selection
  const handleSelectSession = (id: string) => {
    router.push(`/chat?session=${id}`);
  };

  // Create new session
  const handleCreateSession = async () => {
    const newSession = await createSession({ agent_type: "main" });
    if (newSession) {
      router.push(`/chat?session=${newSession.id}`);
    }
  };

  // Delete session
  const handleDeleteSession = async (id: string) => {
    if (confirm("确定要删除这个对话吗？")) {
      await deleteSession(id);
      if (sessionId === id) {
        router.push("/chat");
      }
    }
  };

  // Rename session
  const handleRenameSession = async (id: string, title: string) => {
    await updateSession(id, { title });
  };

  // Archive session
  const handleArchiveSession = async () => {
    if (sessionId) {
      await updateSession(sessionId, { state: "archived" });
      router.push("/chat");
    }
  };

  // Send message with streaming
  const handleSendMessage = async (content: string) => {
    if (!sessionId || !content.trim()) return;

    // Add user message optimistically
    const userMessage: SessionMessage = {
      id: `temp-user-${Date.now()}`,
      role: "user",
      content: content.trim(),
      created_at: new Date().toISOString(),
    };
    setMessages((prev) => [...prev, userMessage]);
    setChatLoading(true);

    // Add placeholder for assistant response
    const assistantId = `temp-assistant-${Date.now()}`;
    setMessages((prev) => [
      ...prev,
      {
        id: assistantId,
        role: "assistant",
        content: "",
        created_at: new Date().toISOString(),
      },
    ]);

    try {
      // Use existing agent chat endpoint for now
      const eventSource = new EventSource(
        `${API_URL}/api/v1/agents/default/chat/stream?message=${encodeURIComponent(content.trim())}`
      );

      let fullContent = "";

      eventSource.addEventListener("chunk", (event) => {
        try {
          const data = JSON.parse(event.data);
          fullContent += data.content || "";
          setMessages((prev) =>
            prev.map((m) =>
              m.id === assistantId ? { ...m, content: fullContent } : m
            )
          );
        } catch (e) {
          console.error("Parse error:", e);
        }
      });

      eventSource.addEventListener("complete", () => {
        eventSource.close();
        setChatLoading(false);
      });

      eventSource.onerror = () => {
        eventSource.close();
        setChatLoading(false);
      };

    } catch (error) {
      console.error("Chat error:", error);
      setMessages((prev) => prev.filter((m) => m.id !== assistantId));
      setChatLoading(false);
    }
  };

  return (
    <div className="flex h-screen bg-gray-950">
      {/* Left Sidebar */}
      <SessionSidebar
        sessions={sessions}
        grouped={grouped}
        activeId={sessionId}
        onSelect={handleSelectSession}
        onCreate={handleCreateSession}
        onDelete={handleDeleteSession}
        onRename={handleRenameSession}
        loading={sessionsLoading}
      />

      {/* Main Content */}
      <main className="flex-1 flex flex-col min-w-0">
        {/* Header */}
        <div className="h-14 border-b border-gray-700 bg-gray-900 px-4 flex items-center justify-between">
          <Link href="/" className="flex items-center gap-2">
            <Globe className="h-5 w-5 text-emerald-500" />
            <span className="font-bold text-white">UTA Travel</span>
          </Link>
          <div className="flex items-center gap-2">
            <Link href="/destinations">
              <Button variant="ghost" size="sm" className="text-gray-400 hover:text-white">
                我的目的地
              </Button>
            </Link>
            <Link href="/destinations/create">
              <Button variant="outline" size="sm">
                创建 Agent
              </Button>
            </Link>
          </div>
        </div>

        {/* Session Header */}
        <ChatHeader
          session={session}
          onArchive={handleArchiveSession}
          onDelete={() => sessionId && handleDeleteSession(sessionId)}
        />

        {/* Messages */}
        <ChatMessages
          messages={messages}
          loading={messagesLoading || chatLoading}
        />
        <div ref={messagesEndRef} />

        {/* Input */}
        {sessionId && (
          <ChatInput
            onSend={handleSendMessage}
            disabled={!session || sessionLoading}
            loading={chatLoading}
          />
        )}

        {/* Empty state */}
        {!sessionId && (
          <div className="flex-1 flex items-center justify-center">
            <div className="text-center">
              <div className="h-16 w-16 rounded-full bg-emerald-500/10 flex items-center justify-center mx-auto mb-4">
                <Globe className="h-8 w-8 text-emerald-500" />
              </div>
              <h2 className="text-xl font-semibold text-white mb-2">
                欢迎使用 UTA Travel Agent
              </h2>
              <p className="text-gray-400 mb-6">
                选择一个对话或创建新对话开始聊天
              </p>
              <button
                onClick={handleCreateSession}
                className="px-6 py-2.5 bg-blue-600 hover:bg-blue-700 text-white rounded-lg transition-colors font-medium"
              >
                开始新对话
              </button>
            </div>
          </div>
        )}
      </main>
    </div>
  );
}

function LoadingFallback() {
  return (
    <div className="flex h-screen bg-gray-950 items-center justify-center">
      <Loader2 className="h-8 w-8 text-emerald-500 animate-spin" />
    </div>
  );
}

export default function ChatPage() {
  return (
    <Suspense fallback={<LoadingFallback />}>
      <ChatPageContent />
    </Suspense>
  );
}
