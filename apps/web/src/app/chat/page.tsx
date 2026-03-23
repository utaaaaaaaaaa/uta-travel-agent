"use client";

import { useState, useEffect, useRef, useCallback, Suspense } from "react";
import { useRouter, useSearchParams } from "next/navigation";
import Link from "next/link";
import type { SessionMessage, Session } from "@/types/session";
import {
  Compass,
  Plus,
  Send,
  Loader2,
  MapPin,
  Plane,
  Sparkles,
  User,
  Bot,
  Trash2,
  Edit3,
  Check,
  X,
  MessageSquare,
  Archive,
  MoreHorizontal,
  AlertTriangle,
  Globe,
  ChevronDown,
  ChevronRight,
} from "lucide-react";
import "./chat.css";

const API_URL = process.env.NEXT_PUBLIC_API_URL || "http://localhost:8080";

// Agent info interface
interface AgentInfo {
  id: string;
  name: string;
  destination: string;
}

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
  const [agents, setAgents] = useState<AgentInfo[]>([]);
  const [grouped, setGrouped] = useState<{ today: Session[]; yesterday: Session[]; previous: Session[] }>({
    today: [],
    yesterday: [],
    previous: [],
  });
  const [loading, setLoading] = useState(true);

  const fetchAgents = useCallback(async () => {
    try {
      const response = await fetch(`${API_URL}/api/v1/agents`);
      if (response.ok) {
        const data = await response.json();
        setAgents(data.agents || []);
      }
    } catch (error) {
      console.error("Failed to fetch agents:", error);
    }
  }, []);

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
    fetchAgents();
    fetchSessions();
  }, [fetchAgents, fetchSessions]);

  return { sessions, grouped, agents, loading, createSession, updateSession, deleteSession };
}

// Format time ago
function formatTimeAgo(dateString: string): string {
  const date = new Date(dateString);
  const now = new Date();
  const diffMs = now.getTime() - date.getTime();
  const diffMins = Math.floor(diffMs / 60000);
  const diffHours = Math.floor(diffMs / 3600000);
  const diffDays = Math.floor(diffMs / 86400000);

  if (diffMins < 1) return "刚刚";
  if (diffMins < 60) return `${diffMins} 分钟前`;
  if (diffHours < 24) return `${diffHours} 小时前`;
  if (diffDays < 7) return `${diffDays} 天前`;
  return date.toLocaleDateString("zh-CN");
}

// Session Item Component
function SessionItem({
  session,
  isActive,
  onSelect,
  onDelete,
  onRename,
}: {
  session: Session;
  isActive: boolean;
  onSelect: () => void;
  onDelete: () => void;
  onRename: (title: string) => void;
}) {
  const [isEditing, setIsEditing] = useState(false);
  const [editTitle, setEditTitle] = useState(session.title || "新对话");
  const [showActions, setShowActions] = useState(false);

  const timeAgo = formatTimeAgo(session.last_active_at || session.created_at);

  const handleRename = () => {
    if (editTitle.trim() && editTitle !== session.title) {
      onRename(editTitle.trim());
    }
    setIsEditing(false);
  };

  return (
    <div
      className={`session-item relative flex items-center gap-3 px-3 py-2.5 cursor-pointer group ${
        isActive ? "active" : ""
      }`}
      onClick={() => !isEditing && onSelect()}
      onMouseEnter={() => setShowActions(true)}
      onMouseLeave={() => {
        setShowActions(false);
        if (isEditing) {
          setEditTitle(session.title || "新对话");
          setIsEditing(false);
        }
      }}
    >
      <div className={`w-8 h-8 rounded-lg flex items-center justify-center flex-shrink-0 ${
        isActive ? "bg-[var(--chat-primary)]" : "bg-[var(--chat-hover)]"
      }`}>
        <MessageSquare className={`w-4 h-4 ${isActive ? "text-white" : "text-[var(--chat-text-muted)]"}`} />
      </div>

      <div className="flex-1 min-w-0">
        {isEditing ? (
          <div className="flex items-center gap-1">
            <input
              type="text"
              value={editTitle}
              onChange={(e) => setEditTitle(e.target.value)}
              onKeyDown={(e) => {
                if (e.key === "Enter") handleRename();
                else if (e.key === "Escape") {
                  setEditTitle(session.title || "新对话");
                  setIsEditing(false);
                }
              }}
              className="w-full bg-white text-sm px-2 py-1 rounded border border-[var(--chat-primary)] focus:outline-none"
              autoFocus
              onClick={(e) => e.stopPropagation()}
            />
            <button onClick={(e) => { e.stopPropagation(); handleRename(); }} className="p-1 hover:bg-[var(--chat-hover)] rounded">
              <Check className="w-3 h-3 text-[var(--chat-primary)]" />
            </button>
            <button onClick={(e) => { e.stopPropagation(); setEditTitle(session.title || "新对话"); setIsEditing(false); }} className="p-1 hover:bg-[var(--chat-hover)] rounded">
              <X className="w-3 h-3 text-red-500" />
            </button>
          </div>
        ) : (
          <>
            <p className="session-title truncate">{session.title || "新对话"}</p>
            <p className="session-time truncate">{timeAgo}</p>
          </>
        )}
      </div>

      {showActions && !isEditing && (
        <div className="flex items-center gap-1 absolute right-2 top-1/2 -translate-y-1/2 bg-[var(--chat-card)] rounded-lg shadow-sm border border-[var(--chat-border)] p-1">
          <button
            onClick={(e) => { e.stopPropagation(); setEditTitle(session.title || "新对话"); setIsEditing(true); }}
            className="p-1.5 hover:bg-[var(--chat-hover)] rounded transition-colors"
            title="重命名"
          >
            <Edit3 className="w-3.5 h-3.5 text-[var(--chat-text-muted)]" />
          </button>
          <button
            onClick={(e) => { e.stopPropagation(); onDelete(); }}
            className="p-1.5 hover:bg-red-50 rounded transition-colors"
            title="删除"
          >
            <Trash2 className="w-3.5 h-3.5 text-red-500" />
          </button>
        </div>
      )}
    </div>
  );
}

// Delete Confirmation Modal
function DeleteConfirmModal({
  isOpen,
  title,
  onConfirm,
  onCancel,
}: {
  isOpen: boolean;
  title: string;
  onConfirm: () => void;
  onCancel: () => void;
}) {
  if (!isOpen) return null;

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center">
      {/* Backdrop */}
      <div
        className="absolute inset-0 bg-black/50 backdrop-blur-sm"
        onClick={onCancel}
      />

      {/* Modal */}
      <div className="relative bg-white rounded-2xl shadow-2xl w-full max-w-sm mx-4 overflow-hidden animate-fade-in">
        {/* Icon */}
        <div className="flex justify-center pt-6">
          <div className="w-14 h-14 rounded-full bg-red-100 flex items-center justify-center">
            <AlertTriangle className="w-7 h-7 text-red-500" />
          </div>
        </div>

        {/* Content */}
        <div className="px-6 py-4 text-center">
          <h3 className="text-lg font-semibold text-gray-900 mb-2">删除对话</h3>
          <p className="text-sm text-gray-500">
            确定要删除 "<span className="font-medium text-gray-700">{title || "新对话"}</span>" 吗？此操作无法撤销。
          </p>
        </div>

        {/* Actions */}
        <div className="flex border-t border-gray-100">
          <button
            onClick={onCancel}
            className="flex-1 py-3.5 text-sm font-medium text-gray-600 hover:bg-gray-50 transition-colors"
          >
            取消
          </button>
          <button
            onClick={onConfirm}
            className="flex-1 py-3.5 text-sm font-medium text-red-500 hover:bg-red-50 transition-colors border-l border-gray-100"
          >
            删除
          </button>
        </div>
      </div>
    </div>
  );
}

// Sidebar Component with agent grouping
function SessionSidebar({
  sessions,
  grouped,
  agents,
  activeId,
  onSelect,
  onCreate,
  onDelete,
  onRename,
  loading,
}: {
  sessions: Session[];
  grouped: { today: Session[]; yesterday: Session[]; previous: Session[] };
  agents: AgentInfo[];
  activeId: string | null;
  onSelect: (id: string) => void;
  onCreate: () => void;
  onDelete: (id: string) => void;
  onRename: (id: string, title: string) => void;
  loading?: boolean;
}) {
  const [expandedGroups, setExpandedGroups] = useState<Record<string, boolean>>({
    main: true,
  });

  // Group sessions by agent
  const sessionsByAgent = useCallback(() => {
    const groups: Record<string, { agent: AgentInfo | null; sessions: Session[] }> = {
      main: { agent: null, sessions: [] },
    };

    // Create groups for each agent
    agents.forEach(agent => {
      groups[agent.id] = { agent, sessions: [] };
    });

    // Sort sessions into groups
    sessions.forEach(session => {
      if (session.agent_type === "main" || !session.agent_id) {
        groups.main.sessions.push(session);
      } else if (groups[session.agent_id]) {
        groups[session.agent_id].sessions.push(session);
      } else {
        // Unknown agent, add to a misc group
        if (!groups[`unknown-${session.agent_id}`]) {
          groups[`unknown-${session.agent_id}`] = {
            agent: { id: session.agent_id, name: "未知导游", destination: "" },
            sessions: []
          };
        }
        groups[`unknown-${session.agent_id}`].sessions.push(session);
      }
    });

    return groups;
  }, [sessions, agents]);

  const toggleGroup = (groupId: string) => {
    setExpandedGroups(prev => ({
      ...prev,
      [groupId]: prev[groupId] === undefined ? false : !prev[groupId]
    }));
  };

  const groups = sessionsByAgent();

  return (
    <div className="chat-sidebar w-72 h-full flex flex-col">
      {/* Header */}
      <div className="p-4">
        <Link href="/" className="flex items-center gap-2.5 mb-4 group">
          <div className="w-9 h-9 rounded-xl bg-gradient-to-br from-[var(--chat-primary)] to-[var(--chat-primary-light)] flex items-center justify-center shadow-lg shadow-[var(--chat-primary)]/20 group-hover:scale-105 transition-transform">
            <Compass className="w-5 h-5 text-white" />
          </div>
          <span className="text-lg font-semibold text-[var(--chat-text)]" style={{ fontFamily: "'Crimson Pro', serif" }}>
            UTA Travel
          </span>
        </Link>
        <button onClick={onCreate} className="new-session-btn w-full flex items-center justify-center gap-2">
          <Plus className="w-4 h-4" />
          <span>开始新对话</span>
        </button>
      </div>

      {/* Session List - Grouped by Agent */}
      <div className="flex-1 overflow-y-auto chat-scroll">
        {loading ? (
          <div className="flex items-center justify-center py-12">
            <Loader2 className="w-5 h-5 animate-spin text-[var(--chat-text-muted)]" />
          </div>
        ) : sessions.length === 0 ? (
          <div className="flex flex-col items-center justify-center py-12 px-4 text-center">
            <div className="w-12 h-12 rounded-full bg-[var(--chat-hover)] flex items-center justify-center mb-3">
              <MessageSquare className="w-6 h-6 text-[var(--chat-text-muted)]" />
            </div>
            <p className="text-sm text-[var(--chat-text-muted)]">暂无对话记录</p>
            <p className="text-xs text-[var(--chat-text-muted)] mt-1 opacity-70">点击上方按钮开始</p>
          </div>
        ) : (
          <div className="px-2">
            {/* Main Agent Group */}
            {groups.main.sessions.length > 0 && (
              <AgentGroup
                groupId="main"
                title="旅行助手"
                icon={<Bot className="w-4 h-4" />}
                color="var(--chat-primary)"
                sessions={groups.main.sessions}
                expanded={expandedGroups.main !== false}
                onToggle={() => toggleGroup("main")}
                activeId={activeId}
                onSelect={onSelect}
                onDelete={onDelete}
                onRename={onRename}
              />
            )}

            {/* Guide Agent Groups */}
            {Object.entries(groups)
              .filter(([id, group]) => id !== "main" && group.sessions.length > 0)
              .sort(([, a], [, b]) => (a.agent?.destination || "").localeCompare(b.agent?.destination || ""))
              .map(([groupId, group]) => (
                <AgentGroup
                  key={groupId}
                  groupId={groupId}
                  title={group.agent?.destination || "导游助手"}
                  subtitle={group.agent?.name}
                  icon={<MapPin className="w-4 h-4" />}
                  color="var(--chat-accent)"
                  sessions={group.sessions}
                  expanded={expandedGroups[groupId] !== false}
                  onToggle={() => toggleGroup(groupId)}
                  activeId={activeId}
                  onSelect={onSelect}
                  onDelete={onDelete}
                  onRename={onRename}
                />
              ))}
          </div>
        )}
      </div>

      {/* Footer */}
      <div className="p-4 border-t border-[var(--chat-border)]">
        <div className="flex items-center justify-between text-xs text-[var(--chat-text-muted)]">
          <span>v0.6.0-alpha</span>
          <Link href="/destinations" className="hover:text-[var(--chat-primary)] transition-colors">
            我的目的地
          </Link>
        </div>
      </div>
    </div>
  );
}

// Agent Group Component
function AgentGroup({
  groupId,
  title,
  subtitle,
  icon,
  color,
  sessions,
  expanded,
  onToggle,
  activeId,
  onSelect,
  onDelete,
  onRename,
}: {
  groupId: string;
  title: string;
  subtitle?: string;
  icon: React.ReactNode;
  color: string;
  sessions: Session[];
  expanded: boolean;
  onToggle: () => void;
  activeId: string | null;
  onSelect: (id: string) => void;
  onDelete: (id: string) => void;
  onRename: (id: string, title: string) => void;
}) {
  return (
    <div className="mb-2">
      {/* Group Header */}
      <button
        onClick={onToggle}
        className="w-full flex items-center gap-2 px-3 py-2 rounded-lg hover:bg-[var(--chat-hover)] transition-colors"
      >
        <div
          className="w-6 h-6 rounded-lg flex items-center justify-center flex-shrink-0"
          style={{ backgroundColor: `color-mix(in srgb, ${color} 20%, transparent)` }}
        >
          {icon}
        </div>
        <div className="flex-1 text-left">
          <div className="text-sm font-medium text-[var(--chat-text)]">{title}</div>
          {subtitle && <div className="text-xs text-[var(--chat-text-muted)]">{subtitle}</div>}
        </div>
        <div className="flex items-center gap-2">
          <span className="text-xs text-[var(--chat-text-muted)] bg-[var(--chat-hover)] px-2 py-0.5 rounded-full">
            {sessions.length}
          </span>
          {expanded ? (
            <ChevronDown className="w-4 h-4 text-[var(--chat-text-muted)]" />
          ) : (
            <ChevronRight className="w-4 h-4 text-[var(--chat-text-muted)]" />
          )}
        </div>
      </button>

      {/* Sessions in Group */}
      {expanded && (
        <div className="ml-2 mt-1 space-y-1">
          {sessions.map((session) => (
            <SessionItem
              key={session.id}
              session={session}
              isActive={activeId === session.id}
              onSelect={() => onSelect(session.id)}
              onDelete={() => onDelete(session.id)}
              onRename={(title) => onRename(session.id, title)}
            />
          ))}
        </div>
      )}
    </div>
  );
}

// Markdown renderer
function renderMarkdown(content: string): string {
  return content
    .replace(/\*\*(.*?)\*\*/g, "<strong>$1</strong>")
    .replace(/\*(.*?)\*/g, "<em>$1</em>")
    .replace(/```([\s\S]*?)```/g, '<pre class="bg-[var(--chat-sidebar)] p-3 rounded-lg my-2 overflow-x-auto text-sm"><code>$1</code></pre>')
    .replace(/`(.*?)`/g, '<code class="bg-[var(--chat-sidebar)] px-1.5 py-0.5 rounded text-sm">$1</code>')
    .replace(/^### (.*$)/gm, '<h3 class="font-semibold text-base my-2">$1</h3>')
    .replace(/^## (.*$)/gm, '<h2 class="font-semibold text-lg my-2">$1</h2>')
    .replace(/^# (.*$)/gm, '<h1 class="font-semibold text-xl my-3">$1</h1>')
    .replace(/^\- (.*$)/gm, '<div class="ml-4 my-1">• $1</div>')
    .replace(/^(\d+)\. (.*$)/gm, '<div class="ml-4 my-1">$1. $2</div>')
    .replace(/\n/g, "<br/>");
}

// Message Component
function ChatMessage({ message, isStreaming, agentName }: { message: SessionMessage; isStreaming?: boolean; agentName?: string }) {
  const isUser = message.role === "user";

  return (
    <div className={`flex gap-4 px-6 py-5 animate-fade-in ${isUser ? "justify-end" : ""}`}>
      {!isUser && (
        <div className="avatar-assistant w-9 h-9 rounded-xl flex items-center justify-center flex-shrink-0 shadow-lg shadow-[var(--chat-primary)]/10">
          <Bot className="w-5 h-5 text-white" />
        </div>
      )}
      <div className={isUser ? "message-user" : "message-assistant"}>
        <div className="flex items-center gap-2 mb-1.5">
          <span className="text-sm font-medium text-[var(--chat-text)]">
            {isUser ? "你" : (agentName || "旅行助手")}
          </span>
          <span className="text-xs text-[var(--chat-text-muted)]">
            {new Date(message.created_at).toLocaleTimeString("zh-CN", { hour: "2-digit", minute: "2-digit" })}
          </span>
        </div>
        <div
          className="text-sm text-[var(--chat-text)] leading-relaxed"
          style={{ fontFamily: "'DM Sans', sans-serif" }}
          dangerouslySetInnerHTML={{ __html: renderMarkdown(message.content) + (isStreaming ? '<span class="streaming-cursor">▊</span>' : '') }}
        />
      </div>
      {isUser && (
        <div className="avatar-user w-9 h-9 rounded-xl flex items-center justify-center flex-shrink-0 shadow-lg">
          <User className="w-5 h-5 text-white" />
        </div>
      )}
    </div>
  );
}

// Messages List Component
function ChatMessages({ messages, loading, containerRef, onScroll, agentName }: {
  messages: SessionMessage[];
  loading?: boolean;
  containerRef?: React.RefObject<HTMLDivElement | null>;
  onScroll?: () => void;
  agentName?: string;
}) {
  // Check if last message is from assistant (streaming in progress)
  const lastMessage = messages[messages.length - 1];
  const isAssistantStreaming = lastMessage?.role === "assistant";
  // Only show loading animation if loading AND no assistant message yet
  const showLoadingAnimation = loading && !isAssistantStreaming;

  return (
    <div ref={containerRef} onScroll={onScroll} className="flex-1 overflow-y-auto chat-scroll">
      {messages.length === 0 ? (
        <div className="flex flex-col items-center justify-center h-full text-[var(--chat-text-muted)]">
          <Bot className="w-12 h-12 mb-3 opacity-30" />
          <p className="text-base font-medium">开始新对话</p>
          <p className="text-sm mt-1 opacity-70">输入消息开始与助手交流</p>
        </div>
      ) : (
        <>
          {messages.map((message) => (
            <ChatMessage key={message.id} message={message} agentName={agentName} isStreaming={loading && message.role === "assistant" && message === lastMessage} />
          ))}
          {showLoadingAnimation && (
            <div className="flex gap-4 px-6 py-5">
              <div className="avatar-assistant w-9 h-9 rounded-xl flex items-center justify-center flex-shrink-0 shadow-lg shadow-[var(--chat-primary)]/10">
                <Bot className="w-5 h-5 text-white" />
              </div>
              <div className="flex items-center gap-1.5 px-4 py-3 bg-[var(--chat-sidebar)] rounded-2xl">
                <div className="loading-dot w-2 h-2 bg-[var(--chat-primary)] rounded-full" />
                <div className="loading-dot w-2 h-2 bg-[var(--chat-primary)] rounded-full" />
                <div className="loading-dot w-2 h-2 bg-[var(--chat-primary)] rounded-full" />
              </div>
            </div>
          )}
        </>
      )}
    </div>
  );
}

// Input Component
function ChatInput({
  onSend,
  disabled,
  loading,
  placeholder = "询问任何旅行相关的问题...",
}: {
  onSend: (message: string) => void;
  disabled?: boolean;
  loading?: boolean;
  placeholder?: string;
}) {
  const [input, setInput] = useState("");
  const textareaRef = useRef<HTMLTextAreaElement>(null);

  useEffect(() => {
    if (textareaRef.current) {
      textareaRef.current.style.height = "auto";
      textareaRef.current.style.height = `${Math.min(textareaRef.current.scrollHeight, 150)}px`;
    }
  }, [input]);

  const handleSend = () => {
    if (input.trim() && !disabled && !loading) {
      onSend(input.trim());
      setInput("");
      if (textareaRef.current) {
        textareaRef.current.style.height = "auto";
      }
    }
  };

  return (
    <div className="chat-input-container">
      <div className="max-w-3xl mx-auto">
        <div className="chat-input-wrapper flex items-end gap-2 px-4 py-3">
          <textarea
            ref={textareaRef}
            value={input}
            onChange={(e) => setInput(e.target.value)}
            onKeyDown={(e) => {
              if (e.key === "Enter" && !e.shiftKey) {
                e.preventDefault();
                handleSend();
              }
            }}
            placeholder={placeholder}
            disabled={disabled || loading}
            rows={1}
            className="flex-1 bg-transparent resize-none focus:outline-none disabled:opacity-50"
          />
          <button
            onClick={handleSend}
            disabled={!input.trim() || disabled || loading}
            className="send-btn flex-shrink-0 p-2.5 text-white disabled:opacity-40 disabled:cursor-not-allowed transition-all"
          >
            {loading ? (
              <Loader2 className="w-5 h-5 animate-spin" />
            ) : (
              <Send className="w-5 h-5" />
            )}
          </button>
        </div>
        <p className="text-xs text-[var(--chat-text-muted)] mt-2 text-center opacity-60">
          按 Enter 发送 · Shift + Enter 换行
        </p>
      </div>
    </div>
  );
}

// Welcome Screen Component
function WelcomeScreen({ onCreate }: { onCreate: () => void }) {
  const features = [
    { icon: MapPin, title: "目的地探索", desc: "了解任何城市的景点与文化" },
    { icon: Plane, title: "行程规划", desc: "智能规划个性化旅行路线" },
    { icon: Sparkles, title: "文化讲解", desc: "深度了解景点背后的故事" },
  ];

  return (
    <div className="flex-1 flex items-center justify-center chat-topo-bg p-8">
      <div className="welcome-container max-w-lg">
        <div className="welcome-icon">
          <Compass className="w-14 h-14 text-white" />
        </div>
        <h1 className="welcome-title">你好，旅行者</h1>
        <p className="welcome-subtitle">
          我是你的智能旅行助手，可以帮助你规划行程、探索目的地、了解当地文化。
          今天想去哪里？
        </p>

        <div className="grid gap-4 mb-8">
          {features.map((feature, index) => (
            <div
              key={index}
              className="feature-card flex items-center gap-4 text-left"
              onClick={onCreate}
              style={{ animationDelay: `${index * 100}ms` }}
            >
              <div className="w-11 h-11 rounded-xl bg-gradient-to-br from-[var(--chat-primary)]/10 to-[var(--chat-primary-light)]/10 flex items-center justify-center flex-shrink-0">
                <feature.icon className="w-5 h-5 text-[var(--chat-primary)]" />
              </div>
              <div>
                <h3 className="font-medium text-[var(--chat-text)]">{feature.title}</h3>
                <p className="text-sm text-[var(--chat-text-muted)]">{feature.desc}</p>
              </div>
            </div>
          ))}
        </div>

        <button onClick={onCreate} className="new-session-btn inline-flex items-center gap-2">
          <Plus className="w-4 h-4" />
          <span>开始新对话</span>
        </button>
      </div>
    </div>
  );
}

// Chat Header Component
function ChatHeader({
  session,
  agents,
  onArchive,
  onDelete,
}: {
  session: Session | null;
  agents: AgentInfo[];
  onArchive: () => void;
  onDelete: () => void;
}) {
  const [showMenu, setShowMenu] = useState(false);

  // Get agent info for this session
  const agentInfo = session?.agent_id
    ? agents.find(a => a.id === session.agent_id)
    : null;

  const agentName = agentInfo?.name || "旅行助手";
  const agentDestination = agentInfo?.destination;

  return (
    <div className="chat-header flex items-center justify-between">
      <div className="flex items-center gap-3">
        <div className="w-10 h-10 rounded-xl bg-gradient-to-br from-[var(--chat-primary)] to-[var(--chat-primary-light)] flex items-center justify-center shadow-lg shadow-[var(--chat-primary)]/10">
          {agentDestination ? <MapPin className="w-5 h-5 text-white" /> : <Bot className="w-5 h-5 text-white" />}
        </div>
        <div>
          <h2 className="chat-header-title">
            {session?.title || "新对话"}
          </h2>
          <p className="text-xs text-[var(--chat-text-muted)]">{agentName}</p>
        </div>
      </div>

      <div className="flex items-center gap-2">
        <Link
          href="/destinations/create"
          className="px-4 py-2 text-sm font-medium text-[var(--chat-primary)] hover:bg-[var(--chat-primary)]/5 rounded-lg transition-colors"
        >
          创建 Agent
        </Link>
        <div className="relative">
          <button
            onClick={() => setShowMenu(!showMenu)}
            className="p-2 hover:bg-[var(--chat-hover)] rounded-lg transition-colors"
          >
            <MoreHorizontal className="w-5 h-5 text-[var(--chat-text-muted)]" />
          </button>
          {showMenu && (
            <>
              <div className="fixed inset-0 z-10" onClick={() => setShowMenu(false)} />
              <div className="absolute right-0 top-full mt-1 w-40 bg-white rounded-xl shadow-xl border border-[var(--chat-border)] py-1 z-20">
                <button
                  onClick={() => { onArchive(); setShowMenu(false); }}
                  className="w-full flex items-center gap-2 px-4 py-2 text-sm text-[var(--chat-text)] hover:bg-[var(--chat-hover)] transition-colors"
                >
                  <Archive className="w-4 h-4" />
                  归档对话
                </button>
                <button
                  onClick={() => { onDelete(); setShowMenu(false); }}
                  className="w-full flex items-center gap-2 px-4 py-2 text-sm text-red-500 hover:bg-red-50 transition-colors"
                >
                  <Trash2 className="w-4 h-4" />
                  删除对话
                </button>
              </div>
            </>
          )}
        </div>
      </div>
    </div>
  );
}

// Main Chat Content
function ChatPageContent() {
  const router = useRouter();
  const searchParams = useSearchParams();
  const sessionId = searchParams.get("session");

  const {
    sessions,
    grouped,
    agents,
    loading: sessionsLoading,
    createSession,
    updateSession,
    deleteSession,
  } = useSessionsData();

  const { session, loading: sessionLoading } = useSessionData(sessionId);

  // Get agent name for current session
  const currentAgent = session?.agent_id
    ? agents.find(a => a.id === session.agent_id)
    : null;
  const agentName = currentAgent?.name || "旅行助手";

  const [messages, setMessages] = useState<SessionMessage[]>([]);
  const [messagesLoading, setMessagesLoading] = useState(false);
  const [chatLoading, setChatLoading] = useState(false);
  const messagesEndRef = useRef<HTMLDivElement>(null);
  const messagesContainerRef = useRef<HTMLDivElement>(null);
  const shouldAutoScrollRef = useRef(true);

  // Delete modal state
  const [deleteModal, setDeleteModal] = useState<{
    isOpen: boolean;
    sessionId: string | null;
    sessionTitle: string;
  }>({ isOpen: false, sessionId: null, sessionTitle: "" });

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

  // Check if user is near bottom of scroll container
  const isNearBottom = () => {
    if (!messagesContainerRef.current) return true;
    const { scrollTop, scrollHeight, clientHeight } = messagesContainerRef.current;
    return scrollHeight - scrollTop - clientHeight < 100;
  };

  // Handle scroll events to track user position
  const handleScroll = () => {
    shouldAutoScrollRef.current = isNearBottom();
  };

  // Scroll to bottom only if user was already at bottom
  useEffect(() => {
    if (shouldAutoScrollRef.current) {
      messagesEndRef.current?.scrollIntoView({ behavior: "smooth" });
    }
  }, [messages]);

  const handleSelectSession = (id: string) => {
    router.push(`/chat?session=${id}`);
  };

  const handleCreateSession = async () => {
    const newSession = await createSession({ agent_type: "main" });
    if (newSession) {
      router.push(`/chat?session=${newSession.id}`);
    }
  };

  const handleDeleteSession = (id: string) => {
    const sessionToDelete = sessions.find(s => s.id === id);
    setDeleteModal({
      isOpen: true,
      sessionId: id,
      sessionTitle: sessionToDelete?.title || "新对话",
    });
  };

  const confirmDeleteSession = async () => {
    if (deleteModal.sessionId) {
      await deleteSession(deleteModal.sessionId);
      if (sessionId === deleteModal.sessionId) {
        router.push("/chat");
      }
    }
    setDeleteModal({ isOpen: false, sessionId: null, sessionTitle: "" });
  };

  const cancelDelete = () => {
    setDeleteModal({ isOpen: false, sessionId: null, sessionTitle: "" });
  };

  const handleRenameSession = async (id: string, title: string) => {
    await updateSession(id, { title });
  };

  const handleArchiveSession = async () => {
    if (sessionId) {
      await updateSession(sessionId, { state: "archived" });
      router.push("/chat");
    }
  };

  const handleSendMessage = async (content: string) => {
    if (!sessionId || !content.trim()) return;

    const userMessage: SessionMessage = {
      id: `temp-user-${Date.now()}`,
      role: "user",
      content: content.trim(),
      created_at: new Date().toISOString(),
    };
    setMessages((prev) => [...prev, userMessage]);
    setChatLoading(true);

    const assistantId = `temp-assistant-${Date.now()}`;
    // Don't add empty assistant message - loading state will show the animation

    try {
      // Use streaming chat endpoint
      const response = await fetch(`${API_URL}/api/v1/sessions/${sessionId}/chat/stream`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ message: content.trim() }),
      });

      if (!response.ok) {
        throw new Error(`Chat failed: ${response.status}`);
      }

      // Now add the assistant message for streaming
      setMessages((prev) => [
        ...prev,
        { id: assistantId, role: "assistant", content: "", created_at: new Date().toISOString() },
      ]);

      // Read as SSE stream
      const reader = response.body?.getReader();
      const decoder = new TextDecoder();
      let fullContent = "";

      if (reader) {
        while (true) {
          const { done, value } = await reader.read();
          if (done) break;

          const chunk = decoder.decode(value, { stream: true });
          const lines = chunk.split('\n');

          for (const line of lines) {
            if (line.startsWith('data: ')) {
              const data = line.slice(6);
              if (data === '[DONE]') break;

              try {
                const parsed = JSON.parse(data);
                // Handle both object with content and primitive values (numbers, strings)
                if (parsed !== null && typeof parsed === "object" && parsed.content) {
                  fullContent += parsed.content;
                  setMessages((prev) =>
                    prev.map((m) => (m.id === assistantId ? { ...m, content: fullContent } : m))
                  );
                } else if (typeof parsed !== "object") {
                  // Primitive value (number, string, boolean) - treat as plain text
                  fullContent += String(parsed);
                  setMessages((prev) =>
                    prev.map((m) => (m.id === assistantId ? { ...m, content: fullContent } : m))
                  );
                }
              } catch {
                // If not JSON, treat as plain text
                if (data && data !== '[DONE]') {
                  fullContent += data;
                  setMessages((prev) =>
                    prev.map((m) => (m.id === assistantId ? { ...m, content: fullContent } : m))
                  );
                }
              }
            }
          }
        }
      }

      // If no content was streamed, use a fallback
      if (!fullContent) {
        setMessages((prev) =>
          prev.map((m) => (m.id === assistantId ? { ...m, content: "收到回复" } : m))
        );
      }

      setChatLoading(false);
    } catch (error) {
      console.error("Chat error:", error);
      // Add error message
      setMessages((prev) => [
        ...prev,
        { id: assistantId, role: "assistant", content: "抱歉，发生错误。请重试。", created_at: new Date().toISOString() },
      ]);
      setChatLoading(false);
    }
  };

  return (
    <div className="flex h-screen bg-[var(--chat-bg)]">
      {/* Left Sidebar */}
      <SessionSidebar
        sessions={sessions}
        grouped={grouped}
        agents={agents}
        activeId={sessionId}
        onSelect={handleSelectSession}
        onCreate={handleCreateSession}
        onDelete={handleDeleteSession}
        onRename={handleRenameSession}
        loading={sessionsLoading}
      />

      {/* Main Content */}
      <main className="flex-1 flex flex-col min-w-0">
        {sessionId ? (
          <>
            {/* Header */}
            <ChatHeader
              session={session}
              agents={agents}
              onArchive={handleArchiveSession}
              onDelete={() => sessionId && handleDeleteSession(sessionId)}
            />

            {/* Messages */}
            <ChatMessages
              messages={messages}
              loading={messagesLoading || chatLoading}
              containerRef={messagesContainerRef}
              onScroll={handleScroll}
              agentName={agentName}
            />
            <div ref={messagesEndRef} />

            {/* Input */}
            <ChatInput
              onSend={handleSendMessage}
              disabled={!session || sessionLoading}
              loading={chatLoading}
            />
          </>
        ) : (
          <WelcomeScreen onCreate={handleCreateSession} />
        )}
      </main>

      {/* Delete Confirmation Modal */}
      <DeleteConfirmModal
        isOpen={deleteModal.isOpen}
        title={deleteModal.sessionTitle}
        onConfirm={confirmDeleteSession}
        onCancel={cancelDelete}
      />
    </div>
  );
}

function LoadingFallback() {
  return (
    <div className="flex h-screen bg-[var(--chat-bg)] items-center justify-center">
      <div className="text-center">
        <div className="w-16 h-16 rounded-2xl bg-gradient-to-br from-[var(--chat-primary)] to-[var(--chat-primary-light)] flex items-center justify-center mx-auto mb-4 shadow-xl">
          <Compass className="w-8 h-8 text-white animate-pulse" />
        </div>
        <Loader2 className="w-6 h-6 animate-spin text-[var(--chat-text-muted)] mx-auto" />
      </div>
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
