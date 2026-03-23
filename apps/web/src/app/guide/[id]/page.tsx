"use client";

import { useState, useRef, useEffect, useCallback } from "react";
import { useParams, useRouter } from "next/navigation";
import Link from "next/link";
import {
  Globe,
  Send,
  ArrowLeft,
  Loader2,
  Bot,
  User,
  MapPin,
  Camera,
  Mic,
  Utensils,
  ShoppingBag,
  Landmark,
  Train,
  Building,
  ExternalLink,
  FileText,
  Compass,
  Sparkles,
  Plus,
  MessageSquare,
  Trash2,
  Edit3,
  Check,
  X,
  AlertTriangle,
} from "lucide-react";
import { api, Agent, Attraction, SourceInfo } from "@/lib/api/client";
import type { Session, SessionMessage } from "@/types/session";

const API_URL = process.env.NEXT_PUBLIC_API_URL || "http://localhost:8080";

// Tab type for sidebar
type SidebarTab = "attractions" | "sessions";

// Simple markdown content renderer
function MarkdownContent({ content }: { content: string }) {
  if (!content) return null;

  // Process line by line to handle streaming content properly
  const lines = content.split('\n');
  const processedLines: string[] = [];

  for (let i = 0; i < lines.length; i++) {
    const line = lines[i];

    // Headers
    if (line.startsWith('### ')) {
      processedLines.push(`<h3 class="font-display font-semibold text-base my-2 text-[var(--uta-text)]">${escapeHtml(line.slice(4))}</h3>`);
      continue;
    }
    if (line.startsWith('## ')) {
      processedLines.push(`<h2 class="font-display font-semibold text-lg my-2 text-[var(--uta-text)]">${escapeHtml(line.slice(3))}</h2>`);
      continue;
    }
    if (line.startsWith('# ')) {
      processedLines.push(`<h1 class="font-display font-bold text-xl my-2 text-[var(--uta-text)]">${escapeHtml(line.slice(2))}</h1>`);
      continue;
    }

    // Table rows - detect by starting with |
    if (line.startsWith('|')) {
      // Check if it's a separator row (contains only dashes, colons, pipes, whitespace)
      if (/^\|[\s\-:|]+\|?$/.test(line)) {
        processedLines.push('<!-- TABLE_SEPARATOR -->');
        continue;
      }
      // Parse table cells
      const cells = line.replace(/^\||\|$/g, '').split('|').map(cell => {
        let processed = cell.trim();
        processed = escapeHtml(processed);
        processed = processed.replace(/\*\*(.*?)\*\*/g, "<strong>$1</strong>");
        processed = processed.replace(/\*(.*?)\*/g, "<em>$1</em>");
        return `<td class="px-3 py-2 border-b border-[var(--uta-border)]">${processed}</td>`;
      }).join('');
      processedLines.push(`<tr>${cells}</tr>`);
      continue;
    }

    // List items
    if (line.startsWith('- ')) {
      let processed = escapeHtml(line.slice(2));
      processed = processed.replace(/\*\*(.*?)\*\*/g, "<strong>$1</strong>");
      processed = processed.replace(/\*(.*?)\*/g, "<em>$1</em>");
      processedLines.push(`<div class="ml-4 my-1">• ${processed}</div>`);
      continue;
    }
    // Handle numbered lists - show the number from the source, not auto-increment
    const numberedMatch = line.match(/^(\d+)\.\s+(.*)$/);
    if (numberedMatch) {
      const number = numberedMatch[1];
      let processed = escapeHtml(numberedMatch[2]);
      processed = processed.replace(/\*\*(.*?)\*\*/g, "<strong>$1</strong>");
      processed = processed.replace(/\*(.*?)\*/g, "<em>$1</em>");
      processedLines.push(`<div class="ml-4 my-1">${number}. ${processed}</div>`);
      continue;
    }

    // Horizontal rule
    if (line === '---') {
      processedLines.push('<hr class="my-4 border-t border-[var(--uta-border)]" />');
      continue;
    }

    // Regular text - handle inline formatting
    let processed = escapeHtml(line);
    processed = processed.replace(/\*\*(.*?)\*\*/g, "<strong>$1</strong>");
    processed = processed.replace(/\*(.*?)\*/g, "<em>$1</em>");
    processed = processed.replace(/`(.*?)`/g, "<code class=\"bg-[var(--uta-bg)] px-1.5 py-0.5 rounded text-sm border border-[var(--uta-border)]\">$1</code>");

    processedLines.push(processed || '&nbsp;');
  }

  // Post-process: wrap consecutive table rows in table tags
  let result = '';
  let inTable = false;
  let tableRows: string[] = [];
  let isFirstRow = true;

  for (const line of processedLines) {
    if (line === '<tr>' || line.startsWith('<tr>')) {
      if (!inTable) {
        inTable = true;
        tableRows = [];
        isFirstRow = true;
      }
      // Check if next non-separator line is a data row to determine if this is header
      if (line === '<tr>') {
        tableRows.push(isFirstRow ? line.replace('<tr>', '<tr class="bg-[var(--uta-bg)]">') : line);
        isFirstRow = false;
      } else {
        tableRows.push(line);
      }
    } else if (line === '<!-- TABLE_SEPARATOR -->') {
      // Skip separator, but mark that next row is not header
      isFirstRow = false;
      continue;
    } else {
      if (inTable) {
        result += `<table class="w-full my-3 border-collapse border border-[var(--uta-border)] rounded-lg overflow-hidden"><tbody>${tableRows.join('')}</tbody></table>`;
        inTable = false;
        tableRows = [];
      }
      result += line;
      if (line && !line.startsWith('<h') && !line.startsWith('<li') && !line.startsWith('<hr') && !line.startsWith('<!')) {
        result += '<br/>';
      }
    }
  }

  if (inTable) {
    result += `<table class="w-full my-3 border-collapse border border-[var(--uta-border)] rounded-lg overflow-hidden"><tbody>${tableRows.join('')}</tbody></table>`;
  }

  return <div className="markdown-content" dangerouslySetInnerHTML={{ __html: result }} />;
}

// Helper function to escape HTML special characters
function escapeHtml(text: string): string {
  return text
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;');
}

interface Message {
  id: string;
  role: "user" | "assistant";
  content: string;
  timestamp: number;
  isStreaming?: boolean;
  sources?: SourceInfo[];
  searchType?: "rag" | "realtime";
}

// Get icon for category
const getCategoryIcon = (category: string): React.ReactNode => {
  switch (category) {
    case "景点": return <Landmark className="h-4 w-4" />;
    case "美食": return <Utensils className="h-4 w-4" />;
    case "购物": return <ShoppingBag className="h-4 w-4" />;
    case "交通": return <Train className="h-4 w-4" />;
    case "住宿": return <Building className="h-4 w-4" />;
    default: return <MapPin className="h-4 w-4" />;
  }
};

// Fallback attractions data when API is not available
const getFallbackAttractions = (destination: string): Attraction[] => {
  const attractions: Record<string, Attraction[]> = {
    "京都": [
      { id: "kyoto-1", name: "金阁寺", category: "景点", description: "世界文化遗产，黄金色的寺院倒映在镜湖池中" },
      { id: "kyoto-2", name: "清水寺", category: "景点", description: "著名的悬空舞台和音羽瀑布" },
      { id: "kyoto-3", name: "伏见稻荷大社", category: "景点", description: "千本鸟居，壮观的朱红色隧道" },
      { id: "kyoto-4", name: "岚山竹林", category: "景点", description: "静谧的竹林小径" },
      { id: "kyoto-5", name: "抹茶甜点", category: "美食", description: "宇治抹茶冰淇淋、抹茶蛋糕" },
      { id: "kyoto-6", name: "京料理", category: "美食", description: "传统的怀石料理体验" },
      { id: "kyoto-7", name: "锦市场", category: "购物", description: "京都的厨房，400年老街" },
    ],
    "东京": [
      { id: "tokyo-1", name: "东京塔", category: "景点", description: "东京地标，可俯瞰城市全景" },
      { id: "tokyo-2", name: "浅草寺", category: "景点", description: "东京最古老的寺院" },
      { id: "tokyo-3", name: "涩谷十字路口", category: "景点", description: "世界最繁忙的十字路口" },
      { id: "tokyo-4", name: "明治神宫", category: "景点", description: "闹市中的宁静神社" },
      { id: "tokyo-5", name: "寿司", category: "美食", description: "筑地/丰洲新鲜寿司" },
      { id: "tokyo-6", name: "拉面", category: "美食", description: "一兰、阿夫利等名店" },
      { id: "tokyo-7", name: "秋叶原", category: "购物", description: "电器街、动漫圣地" },
    ],
  };

  return attractions[destination] || [
    { id: `${destination}-1`, name: `${destination}市中心`, category: "景点", description: "探索城市中心" },
    { id: `${destination}-2`, name: "当地美食", category: "美食", description: "品尝特色料理" },
  ];
};

const categoryColors: Record<string, string> = {
  "景点": "bg-[var(--uta-primary)]/10 text-[var(--uta-primary)] border-[var(--uta-primary)]/20",
  "美食": "bg-[var(--uta-accent)]/10 text-[var(--uta-accent)] border-[var(--uta-accent)]/20",
  "购物": "bg-pink-50 text-pink-600 border-pink-200",
  "交通": "bg-emerald-50 text-emerald-600 border-emerald-200",
  "住宿": "bg-violet-50 text-violet-600 border-violet-200",
};

// Source link component - Wanderlust Edition
function SourceLink({ source }: { source: SourceInfo }) {
  if (source.url) {
    return (
      <a
        href={source.url}
        target="_blank"
        rel="noopener noreferrer"
        className="inline-flex items-center gap-1.5 text-xs text-[var(--uta-primary)] hover:underline bg-[var(--uta-primary)]/5 px-2.5 py-1 rounded-lg border border-[var(--uta-primary)]/20"
      >
        <ExternalLink className="h-3 w-3" />
        {source.title || source.url}
      </a>
    );
  }
  return (
    <span className="text-xs text-[var(--uta-text-muted)] bg-[var(--uta-bg)] px-2.5 py-1 rounded-lg border border-[var(--uta-border)]">
      {source.title}
    </span>
  );
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
      <div
        className="absolute inset-0 bg-black/50 backdrop-blur-sm"
        onClick={onCancel}
      />
      <div className="relative bg-white rounded-2xl shadow-2xl w-full max-w-sm mx-4 overflow-hidden">
        <div className="flex justify-center pt-6">
          <div className="w-14 h-14 rounded-full bg-red-100 flex items-center justify-center">
            <AlertTriangle className="w-7 h-7 text-red-500" />
          </div>
        </div>
        <div className="px-6 py-4 text-center">
          <h3 className="text-lg font-semibold text-gray-900 mb-2">删除对话</h3>
          <p className="text-sm text-gray-500">
            确定要删除 "{title || "新对话"}" 吗？此操作无法撤销。
          </p>
        </div>
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

// Session Item Component for sidebar
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
      className={`relative flex items-center gap-3 px-3 py-2.5 cursor-pointer group rounded-lg mx-2 ${
        isActive ? "bg-[var(--uta-primary)]/10 border border-[var(--uta-primary)]/20" : "hover:bg-[var(--uta-bg)]"
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
        isActive ? "bg-[var(--uta-primary)]" : "bg-[var(--uta-bg)]"
      }`}>
        <MessageSquare className={`w-4 h-4 ${isActive ? "text-white" : "text-[var(--uta-text-muted)]"}`} />
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
              className="w-full bg-white text-sm px-2 py-1 rounded border border-[var(--uta-primary)] focus:outline-none"
              autoFocus
              onClick={(e) => e.stopPropagation()}
            />
            <button onClick={(e) => { e.stopPropagation(); handleRename(); }} className="p-1 hover:bg-[var(--uta-bg)] rounded">
              <Check className="w-3 h-3 text-[var(--uta-primary)]" />
            </button>
            <button onClick={(e) => { e.stopPropagation(); setEditTitle(session.title || "新对话"); setIsEditing(false); }} className="p-1 hover:bg-[var(--uta-bg)] rounded">
              <X className="w-3 h-3 text-red-500" />
            </button>
          </div>
        ) : (
          <>
            <p className="text-sm font-medium text-[var(--uta-text)] truncate">{session.title || "新对话"}</p>
            <p className="text-xs text-[var(--uta-text-muted)] truncate">{timeAgo}</p>
          </>
        )}
      </div>

      {showActions && !isEditing && (
        <div className="flex items-center gap-1 absolute right-3 top-1/2 -translate-y-1/2 bg-white rounded-lg shadow-sm border border-[var(--uta-border)] p-1">
          <button
            onClick={(e) => { e.stopPropagation(); setEditTitle(session.title || "新对话"); setIsEditing(true); }}
            className="p-1.5 hover:bg-[var(--uta-bg)] rounded transition-colors"
            title="重命名"
          >
            <Edit3 className="w-3.5 h-3.5 text-[var(--uta-text-muted)]" />
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

export default function GuidePage() {
  const params = useParams();
  const router = useRouter();
  const agentId = params.id as string;

  const [messages, setMessages] = useState<Message[]>([]);
  const [input, setInput] = useState("");
  const [isLoading, setIsLoading] = useState(false);
  const [agentInfo, setAgentInfo] = useState<Agent | null>(null);
  const [attractions, setAttractions] = useState<Attraction[]>([]);
  const [selectedCategory, setSelectedCategory] = useState<string | null>(null);
  const [loadingAttractions, setLoadingAttractions] = useState(true);
  const [taskId, setTaskId] = useState<string | null>(null);

  // Session-related state
  const [sidebarTab, setSidebarTab] = useState<SidebarTab>("attractions");
  const [sessions, setSessions] = useState<Session[]>([]);
  const [currentSessionId, setCurrentSessionId] = useState<string | null>(null);
  const [loadingSessions, setLoadingSessions] = useState(false);
  const [deleteModal, setDeleteModal] = useState<{ isOpen: boolean; sessionId: string | null; title: string }>({
    isOpen: false,
    sessionId: null,
    title: "",
  });

  const messagesEndRef = useRef<HTMLDivElement>(null);
  const messagesContainerRef = useRef<HTMLDivElement>(null);
  const inputRef = useRef<HTMLInputElement>(null);
  const shouldAutoScrollRef = useRef(true);

  const filteredAttractions = selectedCategory
    ? attractions.filter((a) => a.category === selectedCategory)
    : attractions;

  const categories = [...new Set(attractions.map((a) => a.category))];

  // Check if user is near bottom of scroll container
  const isNearBottom = () => {
    if (!messagesContainerRef.current) return true;
    const { scrollTop, scrollHeight, clientHeight } = messagesContainerRef.current;
    // Consider "near bottom" if within 100px of bottom
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

  // Focus input on load
  useEffect(() => {
    inputRef.current?.focus();
  }, []);

  // Fetch sessions for this agent
  const fetchSessions = useCallback(async () => {
    try {
      setLoadingSessions(true);
      const response = await fetch(`${API_URL}/api/v1/sessions?agent_id=${agentId}`);
      if (response.ok) {
        const data = await response.json();
        setSessions(data.sessions || []);
      }
    } catch (error) {
      console.error("Failed to fetch sessions:", error);
    } finally {
      setLoadingSessions(false);
    }
  }, [agentId]);

  // Create a new session for this agent
  const createSession = useCallback(async (): Promise<Session | null> => {
    try {
      const response = await fetch(`${API_URL}/api/v1/agents/${agentId}/sessions`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({}),
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
  }, [agentId, fetchSessions]);

  // Delete a session
  const deleteSession = useCallback(async (sessionId: string): Promise<boolean> => {
    try {
      const response = await fetch(`${API_URL}/api/v1/sessions/${sessionId}`, {
        method: "DELETE",
      });
      if (response.ok) {
        await fetchSessions();
        if (currentSessionId === sessionId) {
          setCurrentSessionId(null);
          setMessages([]);
        }
        return true;
      }
    } catch (error) {
      console.error("Failed to delete session:", error);
    }
    return false;
  }, [fetchSessions, currentSessionId]);

  // Rename a session
  const renameSession = useCallback(async (sessionId: string, title: string): Promise<boolean> => {
    try {
      const response = await fetch(`${API_URL}/api/v1/sessions/${sessionId}`, {
        method: "PATCH",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ title }),
      });
      if (response.ok) {
        await fetchSessions();
        return true;
      }
    } catch (error) {
      console.error("Failed to rename session:", error);
    }
    return false;
  }, [fetchSessions]);

  // Load or create session on page load
  const initializeSession = useCallback(async (destination: string) => {
    try {
      // First, try to fetch existing sessions for this agent
      const response = await fetch(`${API_URL}/api/v1/sessions?agent_id=${agentId}`);
      if (response.ok) {
        const data = await response.json();
        const existingSessions = data.sessions || [];

        if (existingSessions.length > 0) {
          // Load the most recent session
          const latestSession = existingSessions[0];
          setCurrentSessionId(latestSession.id);
          setSessions(existingSessions);
          console.log("Loaded existing session:", latestSession.id);
        } else {
          // No existing sessions, create a new one
          const createResponse = await fetch(`${API_URL}/api/v1/agents/${agentId}/sessions`, {
            method: "POST",
            headers: { "Content-Type": "application/json" },
            body: JSON.stringify({}),
          });
          if (createResponse.ok) {
            const newSession = await createResponse.json();
            setCurrentSessionId(newSession.id);
            setSessions([newSession]);
            console.log("Created new session:", newSession.id);

            // For new session, show welcome message immediately
            setMessages([{
              id: "welcome",
              role: "assistant",
              content: `你好！我是${destination}导游助手。\n\n我可以为你介绍当地景点、推荐美食、规划行程等。\n\n左侧是${destination}的热门推荐，点击可快速了解详情。有什么想问的吗？`,
              timestamp: Date.now(),
            }]);
          }
        }
      }
    } catch (error) {
      console.error("Failed to initialize session:", error);
      // Set welcome message as fallback
      setMessages([{
        id: "welcome",
        role: "assistant",
        content: `你好！我是${destination}导游助手。\n\n我可以为你介绍当地景点、推荐美食、规划行程等。\n\n左侧是${destination}的热门推荐，点击可快速了解详情。有什么想问的吗？`,
        timestamp: Date.now(),
      }]);
    }
  }, [agentId]);

  // Load session messages when switching sessions
  useEffect(() => {
    if (!currentSessionId) return;

    const loadSessionMessages = async () => {
      try {
        const response = await fetch(`${API_URL}/api/v1/sessions/${currentSessionId}/messages`);
        if (response.ok) {
          const data = await response.json();
          const loadedMessages: Message[] = (data.messages || []).map((msg: SessionMessage) => ({
            id: msg.id,
            role: msg.role,
            content: msg.content,
            timestamp: new Date(msg.created_at).getTime(),
          }));

          // Always show messages if we have them
          if (loadedMessages.length > 0) {
            setMessages(loadedMessages);
          } else if (agentInfo) {
            // Only show welcome for existing sessions with no messages
            // New sessions already have welcome message set in initializeSession
            const existingSession = sessions.find(s => s.id === currentSessionId);
            if (existingSession && existingSession.message_count === 0) {
              // Check if we already have welcome message set
              setMessages(prev => {
                if (prev.length === 0 || (prev.length === 1 && prev[0].id === "welcome")) {
                  return [{
                    id: "welcome",
                    role: "assistant",
                    content: `你好！我是${agentInfo.destination}导游助手。\n\n我可以为你介绍当地景点、推荐美食、规划行程等。\n\n左侧是${agentInfo.destination}的热门推荐，点击可快速了解详情。有什么想问的吗？`,
                    timestamp: Date.now(),
                  }];
                }
                return prev;
              });
            }
          }
        }
      } catch (error) {
        console.error("Failed to load session messages:", error);
      }
    };

    loadSessionMessages();
  }, [currentSessionId, agentInfo, sessions]);

  // Fetch sessions when tab changes to sessions
  useEffect(() => {
    if (sidebarTab === "sessions") {
      fetchSessions();
    }
  }, [sidebarTab, fetchSessions]);

  // Fetch agent info and attractions
  useEffect(() => {
    const fetchAgent = async () => {
      try {
        const agent = await api.getAgent(agentId);
        setAgentInfo(agent);

        // Try to get task ID from agent
        if (agent.task_id) {
          setTaskId(agent.task_id);
        } else {
          try {
            const task = await api.getTaskByAgent(agentId);
            if (task?.id) {
              setTaskId(task.id);
            }
          } catch (e) {
            console.log("No task found for this agent");
          }
        }

        // Initialize session (load existing or create new)
        await initializeSession(agent.destination);

        // Load attractions from API
        try {
          const attractionsData = await api.getAttractions(agentId);
          if (attractionsData.attractions && attractionsData.attractions.length > 0) {
            setAttractions(attractionsData.attractions);
          } else {
            setAttractions(getFallbackAttractions(agent.destination));
          }
        } catch (e) {
          console.log("Failed to load attractions from API, using fallback");
          setAttractions(getFallbackAttractions(agent.destination));
        }
        setLoadingAttractions(false);
      } catch (error) {
        console.error("Failed to fetch agent:", error);
        const destination = agentId.includes("kyoto") ? "京都" :
                           agentId.includes("tokyo") ? "东京" : "目的地";
        setAgentInfo({
          id: agentId,
          user_id: "default",
          name: `${destination}导游助手`,
          description: `${destination}智能导游`,
          destination: destination,
          status: "ready",
          document_count: 0,
          language: "zh",
          theme: "cultural",
          tags: [],
          created_at: new Date().toISOString(),
          updated_at: new Date().toISOString(),
          usage_count: 0,
          rating: 0,
        });

        setMessages([
          {
            id: "welcome",
            role: "assistant",
            content: `你好！我是${destination}导游助手。\n\n我可以为你介绍当地景点、推荐美食、规划行程等。\n\n左侧是${destination}的热门推荐，点击可快速了解详情。有什么想问的吗？`,
            timestamp: Date.now(),
          },
        ]);
        setAttractions(getFallbackAttractions(destination));
        setLoadingAttractions(false);
      }
    };

    fetchAgent();
  }, [agentId]);

  const handleSend = async () => {
    if (!input.trim() || isLoading) return;

    // If no current session, create one first
    let sessionId = currentSessionId;
    if (!sessionId) {
      const newSession = await createSession();
      if (newSession) {
        sessionId = newSession.id;
        setCurrentSessionId(sessionId);
        // Set welcome message for new session
        setMessages([{
          id: "welcome",
          role: "assistant",
          content: `你好！我是${agentInfo?.destination || "目的地"}导游助手。\n\n我可以为你介绍当地景点、推荐美食、规划行程等。\n\n左侧是${agentInfo?.destination || "目的地"}的热门推荐，点击可快速了解详情。有什么想问的吗？`,
          timestamp: Date.now(),
        }]);
      } else {
        console.error("Failed to create session");
        return;
      }
    }

    const userMessage: Message = {
      id: `user-${Date.now()}`,
      role: "user",
      content: input.trim(),
      timestamp: Date.now(),
    };

    setMessages((prev) => [...prev, userMessage]);
    setInput("");
    setIsLoading(true);

    const streamingId = `streaming-${Date.now()}`;
    setMessages((prev) => [
      ...prev,
      { id: streamingId, role: "assistant", content: "", timestamp: Date.now(), isStreaming: true },
    ]);

    try {
      await streamChat(input.trim(), streamingId, sessionId);
    } catch (error) {
      console.error("Chat error:", error);
      setMessages((prev) => prev.filter((m) => m.id !== streamingId));
      setMessages((prev) => [
        ...prev,
        {
          id: `error-${Date.now()}`,
          role: "assistant",
          content: "抱歉，我遇到了一些问题。请稍后再试。",
          timestamp: Date.now(),
        },
      ]);
    } finally {
      setIsLoading(false);
    }
  };

  const streamChat = async (message: string, streamingId: string, sessionId: string) => {
    let completed = false;
    let sources: SourceInfo[] = [];
    let searchType: "rag" | "realtime" | undefined;

    return new Promise<void>(async (resolve, reject) => {
      try {
        // Use session chat stream API
        const response = await fetch(`${API_URL}/api/v1/sessions/${sessionId}/chat/stream`, {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({ message }),
        });

        if (!response.ok) {
          reject(new Error(`HTTP error: ${response.status}`));
          return;
        }

        const reader = response.body?.getReader();
        if (!reader) {
          reject(new Error("No response body"));
          return;
        }

        const decoder = new TextDecoder();
        let buffer = "";

        while (true) {
          const { done, value } = await reader.read();
          if (done) break;

          buffer += decoder.decode(value, { stream: true });
          const lines = buffer.split("\n");
          buffer = lines.pop() || "";

          for (const line of lines) {
            if (line.startsWith("data: ")) {
              const data = line.slice(6);

              // Check for completion marker
              if (data === "[DONE]") {
                completed = true;
                setMessages((prev) =>
                  prev.map((m) =>
                    m.id === streamingId
                      ? { ...m, isStreaming: false, sources, searchType }
                      : m
                  )
                );
                resolve();
                return;
              }

              // Check for error marker
              if (data.startsWith("[ERROR]")) {
                console.error("Stream error:", data);
                continue;
              }

              // Try to parse as JSON first (for structured responses)
              try {
                const jsonData = JSON.parse(data);

                // Only handle if jsonData is an object (not a primitive like number/string)
                if (jsonData !== null && typeof jsonData === "object") {
                  if (jsonData.type === "chunk" || jsonData.content !== undefined) {
                    setMessages((prev) =>
                      prev.map((m) =>
                        m.id === streamingId
                          ? { ...m, content: m.content + (jsonData.content || "") }
                          : m
                      )
                    );
                  } else if (jsonData.type === "sources" || jsonData.sources) {
                    if (jsonData.sources) sources = jsonData.sources;
                    if (jsonData.search_type) searchType = jsonData.search_type;
                  } else if (jsonData.type === "complete" || jsonData.done) {
                    completed = true;
                    setMessages((prev) =>
                      prev.map((m) =>
                        m.id === streamingId
                          ? { ...m, isStreaming: false, sources, searchType }
                          : m
                      )
                    );
                    resolve();
                    return;
                  }
                } else {
                  // jsonData is a primitive (number, string, boolean), treat as plain text
                  const contentToAdd = String(jsonData);
                  setMessages((prev) =>
                    prev.map((m) =>
                      m.id === streamingId
                        ? { ...m, content: m.content + contentToAdd }
                        : m
                    )
                  );
                }
              } catch (e) {
                // Not JSON, treat as plain text content
                // SSE format: empty "data:" represents a newline character
                // Regular content is sent as-is
                const contentToAdd = data === "" ? "\n" : data;
                setMessages((prev) =>
                  prev.map((m) =>
                    m.id === streamingId
                      ? { ...m, content: m.content + contentToAdd }
                      : m
                  )
                );
              }
            }
          }
        }

        // If we get here without complete, mark as done
        if (!completed) {
          setMessages((prev) =>
            prev.map((m) =>
              m.id === streamingId
                ? { ...m, isStreaming: false }
                : m
            )
          );
        }
        resolve();
      } catch (err) {
        console.error("Stream error:", err);
        reject(err);
      }
    });
  };

  const handleAttractionClick = (attraction: Attraction) => {
    setInput(`请介绍一下${attraction.name}`);
    inputRef.current?.focus();
  };

  const handleKeyDown = (e: React.KeyboardEvent) => {
    if (e.key === "Enter" && !e.shiftKey) {
      e.preventDefault();
      handleSend();
    }
  };

  return (
    <div className="flex h-screen overflow-hidden bg-topo">
      {/* Left Sidebar with Tabs */}
      <aside className="hidden md:flex w-80 flex-col border-r border-[var(--uta-border)] bg-white/80 backdrop-blur-xl flex-shrink-0">
        {/* Tab Header */}
        <div className="flex-shrink-0 p-3 border-b border-[var(--uta-border)]">
          <div className="flex gap-1 p-1 bg-[var(--uta-bg)] rounded-xl">
            <button
              onClick={() => setSidebarTab("attractions")}
              className={`flex-1 px-3 py-2 rounded-lg text-sm font-medium transition-all flex items-center justify-center gap-1.5 ${
                sidebarTab === "attractions"
                  ? "bg-white text-[var(--uta-primary)] shadow-sm"
                  : "text-[var(--uta-text-muted)] hover:text-[var(--uta-text)]"
              }`}
            >
              <MapPin className="h-4 w-4" />
              推荐
            </button>
            <button
              onClick={() => setSidebarTab("sessions")}
              className={`flex-1 px-3 py-2 rounded-lg text-sm font-medium transition-all flex items-center justify-center gap-1.5 ${
                sidebarTab === "sessions"
                  ? "bg-white text-[var(--uta-primary)] shadow-sm"
                  : "text-[var(--uta-text-muted)] hover:text-[var(--uta-text)]"
              }`}
            >
              <MessageSquare className="h-4 w-4" />
              对话
            </button>
          </div>
        </div>

        {/* Attractions Tab Content */}
        {sidebarTab === "attractions" && (
          <>
            {/* Category Filter */}
            <div className="flex-shrink-0 px-5 py-3 border-b border-[var(--uta-border)]">
              <div className="flex gap-2 flex-wrap">
                <button
                  key="all"
                  onClick={() => setSelectedCategory(null)}
                  className={`px-3 py-1.5 rounded-lg text-sm font-medium transition-all ${
                    selectedCategory === null
                      ? "btn-primary py-1.5"
                      : "bg-[var(--uta-bg)] text-[var(--uta-text-muted)] border border-[var(--uta-border)] hover:border-[var(--uta-primary)]"
                  }`}
                >
                  全部
                </button>
                {categories.map((cat, idx) => (
                  <button
                    key={`cat-${idx}-${cat}`}
                    onClick={() => setSelectedCategory(cat)}
                    className={`px-3 py-1.5 rounded-lg text-sm font-medium transition-all ${
                      selectedCategory === cat
                        ? "btn-primary py-1.5"
                        : "bg-[var(--uta-bg)] text-[var(--uta-text-muted)] border border-[var(--uta-border)] hover:border-[var(--uta-primary)]"
                    }`}
                  >
                    {cat}
                  </button>
                ))}
              </div>
            </div>

            {/* Attractions List */}
            <div className="flex-1 overflow-y-auto p-4 space-y-3">
              {loadingAttractions ? (
                <div className="flex justify-center py-8">
                  <Loader2 className="h-6 w-6 animate-spin text-[var(--uta-primary)]" />
                </div>
              ) : filteredAttractions.length > 0 ? (
                filteredAttractions.map((attraction, idx) => (
                  <button
                    key={`attraction-${idx}`}
                    onClick={() => handleAttractionClick(attraction)}
                    className="w-full text-left p-4 rounded-xl bg-[var(--uta-card)] border border-[var(--uta-border)] hover:border-[var(--uta-primary)] hover:shadow-lg hover:shadow-[var(--uta-primary)]/5 transition-all"
                  >
                    <div className="flex items-start gap-3">
                      <div className="flex-shrink-0 w-10 h-10 rounded-xl bg-gradient-to-br from-[var(--uta-primary)] to-[var(--uta-primary-light)] flex items-center justify-center shadow-md shadow-[var(--uta-primary)]/20">
                        {getCategoryIcon(attraction.category)}
                      </div>
                      <div className="flex-1 min-w-0">
                        <div className="flex items-center gap-2 mb-1">
                          <span className="font-medium text-[var(--uta-text)] truncate">{attraction.name}</span>
                          <span className={`text-xs px-2 py-0.5 rounded-full border ${categoryColors[attraction.category] || "bg-[var(--uta-bg)] text-[var(--uta-text-muted)] border-[var(--uta-border)]"}`}>
                            {attraction.category}
                          </span>
                        </div>
                        <p className="text-xs text-[var(--uta-text-muted)] line-clamp-2">
                          {attraction.description}
                        </p>
                      </div>
                    </div>
                  </button>
                ))
              ) : (
                <p className="text-sm text-[var(--uta-text-muted)] text-center py-8">
                  暂无推荐
                </p>
              )}
            </div>
          </>
        )}

        {/* Sessions Tab Content */}
        {sidebarTab === "sessions" && (
          <>
            {/* New Session Button */}
            <div className="flex-shrink-0 p-4 border-b border-[var(--uta-border)]">
              <button
                onClick={async () => {
                  const newSession = await createSession();
                  if (newSession) {
                    setCurrentSessionId(newSession.id);
                    setMessages([{
                      id: "welcome",
                      role: "assistant",
                      content: `你好！我是${agentInfo?.destination || "目的地"}导游助手。\n\n我可以为你介绍当地景点、推荐美食、规划行程等。\n\n有什么想问的吗？`,
                      timestamp: Date.now(),
                    }]);
                  }
                }}
                className="w-full flex items-center justify-center gap-2 px-4 py-2.5 rounded-xl bg-[var(--uta-primary)] text-white font-medium hover:bg-[var(--uta-primary)]/90 transition-all shadow-lg shadow-[var(--uta-primary)]/20"
              >
                <Plus className="h-4 w-4" />
                新对话
              </button>
            </div>

            {/* Sessions List */}
            <div className="flex-1 overflow-y-auto py-2">
              {loadingSessions ? (
                <div className="flex justify-center py-8">
                  <Loader2 className="h-6 w-6 animate-spin text-[var(--uta-primary)]" />
                </div>
              ) : sessions.length === 0 ? (
                <div className="flex flex-col items-center justify-center py-12 px-4 text-center">
                  <div className="w-12 h-12 rounded-full bg-[var(--uta-bg)] flex items-center justify-center mb-3">
                    <MessageSquare className="h-6 w-6 text-[var(--uta-text-muted)]" />
                  </div>
                  <p className="text-sm text-[var(--uta-text-muted)]">暂无对话记录</p>
                  <p className="text-xs text-[var(--uta-text-muted)] mt-1 opacity-70">点击上方按钮开始</p>
                </div>
              ) : (
                sessions.map((session) => (
                  <SessionItem
                    key={session.id}
                    session={session}
                    isActive={currentSessionId === session.id}
                    onSelect={() => setCurrentSessionId(session.id)}
                    onDelete={() => setDeleteModal({ isOpen: true, sessionId: session.id, title: session.title || "新对话" })}
                    onRename={(title) => renameSession(session.id, title)}
                  />
                ))
              )}
            </div>
          </>
        )}
      </aside>

      {/* Delete Confirmation Modal */}
      <DeleteConfirmModal
        isOpen={deleteModal.isOpen}
        title={deleteModal.title}
        onConfirm={async () => {
          if (deleteModal.sessionId) {
            await deleteSession(deleteModal.sessionId);
          }
          setDeleteModal({ isOpen: false, sessionId: null, title: "" });
        }}
        onCancel={() => setDeleteModal({ isOpen: false, sessionId: null, title: "" })}
      />

      {/* Main Chat Area */}
      <div className="flex-1 flex flex-col min-w-0 overflow-hidden">
        {/* Header */}
        <header className="flex-shrink-0 h-16 border-b border-[var(--uta-border)] bg-white/80 backdrop-blur-xl px-4 flex items-center justify-between">
          <div className="flex items-center gap-4">
            <button
              onClick={() => router.back()}
              className="w-10 h-10 rounded-xl bg-[var(--uta-card)] border border-[var(--uta-border)] flex items-center justify-center text-[var(--uta-text-muted)] hover:text-[var(--uta-primary)] hover:border-[var(--uta-primary)] transition-all"
            >
              <ArrowLeft className="h-5 w-5" />
            </button>
            <div className="flex items-center gap-3">
              <div className="w-10 h-10 rounded-xl bg-gradient-to-br from-[var(--uta-primary)] to-[var(--uta-primary-light)] flex items-center justify-center shadow-lg shadow-[var(--uta-primary)]/20">
                <Globe className="h-5 w-5 text-white" />
              </div>
              <div>
                <h1 className="font-display font-semibold text-[var(--uta-text)]">
                  {agentInfo?.name || "导游助手"}
                </h1>
                <p className="text-xs text-[var(--uta-text-muted)]">
                  {agentInfo?.status === "ready" ? (
                    <span className="flex items-center gap-1.5">
                      <span className="w-2 h-2 bg-emerald-500 rounded-full"></span>
                      在线
                    </span>
                  ) : (
                    "连接中..."
                  )}
                </p>
              </div>
            </div>
          </div>
          <div className="flex items-center gap-3">
            {taskId && (
              <Link href={`/tasks/${taskId}`}>
                <button className="btn-secondary text-sm py-2">
                  <FileText className="h-4 w-4" />
                  <span className="hidden sm:inline">查看任务详情</span>
                </button>
              </Link>
            )}
            <Link href="/" className="flex items-center gap-2">
              <div className="w-8 h-8 rounded-lg bg-gradient-to-br from-[var(--uta-primary)] to-[var(--uta-primary-light)] flex items-center justify-center">
                <Compass className="h-4 w-4 text-white" />
              </div>
              <span className="font-display font-semibold text-[var(--uta-text)] hidden sm:inline">UTA Travel</span>
            </Link>
          </div>
        </header>

        {/* Messages Area - Scrollable */}
        <main ref={messagesContainerRef} onScroll={handleScroll} className="flex-1 overflow-y-auto p-6">
          <div className="max-w-3xl mx-auto space-y-6">
            {messages.map((message) => (
              <div
                key={message.id}
                className={`flex gap-3 ${
                  message.role === "user" ? "justify-end" : "justify-start"
                }`}
              >
                {message.role === "assistant" && (
                  <div className="flex-shrink-0 w-9 h-9 rounded-xl bg-gradient-to-br from-[var(--uta-primary)] to-[var(--uta-primary-light)] flex items-center justify-center shadow-lg shadow-[var(--uta-primary)]/20">
                    <Bot className="h-5 w-5 text-white" />
                  </div>
                )}
                <div className={`max-w-[80%] ${message.role === "user" ? "" : "flex flex-col gap-2"}`}>
                  <div
                    className={`${
                      message.role === "user"
                        ? "bg-gradient-to-br from-[var(--uta-primary)] to-[var(--uta-primary-light)] text-white rounded-2xl rounded-tr-md shadow-lg shadow-[var(--uta-primary)]/20"
                        : "bg-[var(--uta-card)] border border-[var(--uta-border)] rounded-2xl rounded-tl-md shadow-sm"
                    } p-4`}
                  >
                    {message.role === "user" ? (
                      <p className="text-sm whitespace-pre-wrap">
                        {message.content}
                      </p>
                    ) : (
                      <div className="text-sm text-[var(--uta-text)]">
                        <MarkdownContent content={message.content} />
                        {message.isStreaming && (
                          <span className="inline-block w-1 h-4 ml-1 bg-[var(--uta-primary)] animate-pulse"></span>
                        )}
                      </div>
                    )}
                  </div>

                  {/* Show sources if available */}
                  {message.role === "assistant" && message.sources && message.sources.length > 0 && !message.isStreaming && (
                    <div className="mt-2">
                      <div className="flex items-center gap-2 text-xs text-[var(--uta-text-muted)] mb-2">
                        {message.searchType === "realtime" ? (
                          <span className="px-2 py-1 rounded-lg text-xs bg-emerald-50 text-emerald-600 border border-emerald-200">
                            实时搜索
                          </span>
                        ) : (
                          <span className="px-2 py-1 rounded-lg text-xs bg-sky-50 text-sky-600 border border-sky-200">
                            知识库
                          </span>
                        )}
                        <span>信息来源:</span>
                      </div>
                      <div className="flex flex-wrap gap-2">
                        {message.sources.map((source, idx) => (
                          <SourceLink key={`${message.id}-source-${idx}`} source={source} />
                        ))}
                      </div>
                    </div>
                  )}
                </div>
                {message.role === "user" && (
                  <div className="flex-shrink-0 w-9 h-9 rounded-xl bg-gradient-to-br from-[var(--uta-accent)] to-[#c9956c] flex items-center justify-center shadow-lg shadow-[var(--uta-accent)]/20">
                    <User className="h-5 w-5 text-white" />
                  </div>
                )}
              </div>
            ))}

            {isLoading && messages[messages.length - 1]?.isStreaming !== true && (
              <div className="flex gap-3 justify-start">
                <div className="flex-shrink-0 w-9 h-9 rounded-xl bg-gradient-to-br from-[var(--uta-primary)] to-[var(--uta-primary-light)] flex items-center justify-center shadow-lg shadow-[var(--uta-primary)]/20">
                  <Bot className="h-5 w-5 text-white" />
                </div>
                <div className="bg-[var(--uta-card)] border border-[var(--uta-border)] rounded-2xl rounded-tl-md p-4 shadow-sm">
                  <Loader2 className="h-5 w-5 animate-spin text-[var(--uta-primary)]" />
                </div>
              </div>
            )}

            <div ref={messagesEndRef} />
          </div>
        </main>

        {/* Input Area - Fixed */}
        <footer className="flex-shrink-0 border-t border-[var(--uta-border)] bg-white/80 backdrop-blur-xl p-4">
          <div className="max-w-3xl mx-auto">
            {/* Quick Action Buttons */}
            <div className="flex gap-2 mb-3 overflow-x-auto pb-2">
              <button
                onClick={() => setInput("推荐必去的景点")}
                className="flex-shrink-0 px-4 py-2 rounded-xl bg-[var(--uta-bg)] border border-[var(--uta-border)] text-sm text-[var(--uta-text-muted)] hover:text-[var(--uta-primary)] hover:border-[var(--uta-primary)] transition-all flex items-center gap-1.5"
              >
                <MapPin className="h-4 w-4" />
                必去景点
              </button>
              <button
                onClick={() => setInput("当地美食推荐")}
                className="flex-shrink-0 px-4 py-2 rounded-xl bg-[var(--uta-bg)] border border-[var(--uta-border)] text-sm text-[var(--uta-text-muted)] hover:text-[var(--uta-primary)] hover:border-[var(--uta-primary)] transition-all flex items-center gap-1.5"
              >
                <Utensils className="h-4 w-4" />
                美食推荐
              </button>
              <button
                onClick={() => setInput("交通指南")}
                className="flex-shrink-0 px-4 py-2 rounded-xl bg-[var(--uta-bg)] border border-[var(--uta-border)] text-sm text-[var(--uta-text-muted)] hover:text-[var(--uta-primary)] hover:border-[var(--uta-primary)] transition-all flex items-center gap-1.5"
              >
                <Train className="h-4 w-4" />
                交通指南
              </button>
              <button
                onClick={() => setInput("购物攻略")}
                className="flex-shrink-0 px-4 py-2 rounded-xl bg-[var(--uta-bg)] border border-[var(--uta-border)] text-sm text-[var(--uta-text-muted)] hover:text-[var(--uta-primary)] hover:border-[var(--uta-primary)] transition-all flex items-center gap-1.5"
              >
                <ShoppingBag className="h-4 w-4" />
                购物攻略
              </button>
            </div>

            {/* Main Input */}
            <form
              onSubmit={(e) => {
                e.preventDefault();
                handleSend();
              }}
              className="flex gap-3"
            >
              <div className="flex gap-2">
                <button
                  type="button"
                  className="hidden sm:flex w-10 h-10 rounded-xl bg-[var(--uta-bg)] border border-[var(--uta-border)] items-center justify-center text-[var(--uta-text-muted)] opacity-50 cursor-not-allowed"
                  title="拍照识别 (开发中)"
                  disabled
                >
                  <Camera className="h-4 w-4" />
                </button>
                <button
                  type="button"
                  className="hidden sm:flex w-10 h-10 rounded-xl bg-[var(--uta-bg)] border border-[var(--uta-border)] items-center justify-center text-[var(--uta-text-muted)] opacity-50 cursor-not-allowed"
                  title="语音输入 (开发中)"
                  disabled
                >
                  <Mic className="h-4 w-4" />
                </button>
              </div>
              <input
                ref={inputRef}
                value={input}
                onChange={(e) => setInput(e.target.value)}
                onKeyDown={handleKeyDown}
                placeholder="输入你的问题..."
                disabled={isLoading}
                className="flex-1 px-4 py-3 rounded-xl bg-[var(--uta-bg)] border-2 border-[var(--uta-border)] text-[var(--uta-text)] placeholder:text-[var(--uta-text-muted)] focus:outline-none focus:border-[var(--uta-primary)] focus:ring-4 focus:ring-[var(--uta-primary)]/10 transition-all"
              />
              <button
                type="submit"
                disabled={!input.trim() || isLoading}
                className="w-12 h-12 rounded-xl bg-gradient-to-br from-[var(--uta-primary)] to-[var(--uta-primary-light)] flex items-center justify-center text-white shadow-lg shadow-[var(--uta-primary)]/25 hover:shadow-xl hover:shadow-[var(--uta-primary)]/30 transition-all disabled:opacity-50 disabled:cursor-not-allowed"
              >
                <Send className="h-5 w-5" />
              </button>
            </form>
          </div>
        </footer>
      </div>
    </div>
  );
}