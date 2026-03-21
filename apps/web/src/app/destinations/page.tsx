"use client";

import { useState, useEffect } from "react";
import Link from "next/link";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Plus, MapPin, Globe, Loader2, AlertCircle, RefreshCw, Trash2, ArrowLeft } from "lucide-react";
import { DeleteConfirmModal } from "@/components/ui/delete-confirm-modal";

// API base URL
const API_BASE_URL = process.env.NEXT_PUBLIC_API_URL || 'http://localhost:8080';

interface Agent {
  id: string;
  user_id: string;
  name: string;
  description: string;
  destination: string;
  theme: string;
  status: string;
  document_count: number;
  language: string;
  tags: string[];
  created_at: string;
  updated_at: string;
  usage_count: number;
  rating: number;
}

export default function DestinationsPage() {
  const [agents, setAgents] = useState<Agent[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [deleting, setDeleting] = useState<string | null>(null);
  const [deleteModal, setDeleteModal] = useState<{ open: boolean; agentId: string; agentName: string }>({
    open: false,
    agentId: "",
    agentName: ""
  });

  const fetchAgents = async () => {
    setLoading(true);
    setError(null);
    try {
      const response = await fetch(`${API_BASE_URL}/api/v1/agents`);
      if (!response.ok) {
        throw new Error(`获取列表失败: ${response.status}`);
      }
      const data = await response.json();
      // Sort by created_at descending (newest first) for consistent order
      const sortedAgents = (data.agents || []).sort((a: Agent, b: Agent) =>
        new Date(b.created_at).getTime() - new Date(a.created_at).getTime()
      );
      setAgents(sortedAgents);
    } catch (err) {
      console.error('Failed to fetch agents:', err);
      setError(err instanceof Error ? err.message : '获取 Agent 列表失败');
    } finally {
      setLoading(false);
    }
  };

  const handleDeleteClick = (agentId: string, agentName: string, e: React.MouseEvent) => {
    e.preventDefault();
    e.stopPropagation();
    setDeleteModal({ open: true, agentId, agentName });
  };

  const handleDeleteConfirm = async () => {
    const agentId = deleteModal.agentId;
    setDeleteModal({ open: false, agentId: "", agentName: "" });
    setDeleting(agentId);

    try {
      const response = await fetch(`${API_BASE_URL}/api/v1/agents/${agentId}`, {
        method: 'DELETE',
      });
      if (!response.ok) {
        throw new Error(`删除失败: ${response.status}`);
      }
      // Remove from local state
      setAgents(agents.filter(a => a.id !== agentId));
    } catch (err) {
      console.error('Failed to delete agent:', err);
      alert(err instanceof Error ? err.message : '删除失败');
    } finally {
      setDeleting(null);
    }
  };

  useEffect(() => {
    fetchAgents();
  }, []);

  const getStatusLabel = (status: string) => {
    const labels: Record<string, string> = {
      creating: "创建中",
      ready: "就绪",
      busy: "忙碌",
      archived: "已归档",
      error: "错误",
    };
    return labels[status] || status;
  };

  const getStatusColor = (status: string) => {
    const colors: Record<string, string> = {
      creating: "bg-yellow-100 text-yellow-700",
      ready: "bg-green-100 text-green-700",
      busy: "bg-blue-100 text-blue-700",
      archived: "bg-gray-100 text-gray-700",
      error: "bg-red-100 text-red-700",
    };
    return colors[status] || "bg-gray-100 text-gray-700";
  };

  const formatDate = (dateStr: string) => {
    const date = new Date(dateStr);
    return date.toLocaleDateString('zh-CN', {
      year: 'numeric',
      month: 'short',
      day: 'numeric',
    });
  };

  return (
    <div className="min-h-screen">
      {/* Header */}
      <header className="sticky top-0 z-50 w-full border-b bg-background/95 backdrop-blur">
        <div className="container flex h-16 items-center justify-between">
          <Link href="/" className="flex items-center gap-2">
            <Globe className="h-6 w-6 text-primary" />
            <span className="text-xl font-bold">UTA Travel</span>
          </Link>
          <div className="flex items-center gap-2">
            <Button variant="outline" size="icon" onClick={fetchAgents} disabled={loading}>
              <RefreshCw className={`h-4 w-4 ${loading ? 'animate-spin' : ''}`} />
            </Button>
            <Link href="/destinations/create">
              <Button>
                <Plus className="h-4 w-4 mr-2" />
                创建 Agent
              </Button>
            </Link>
          </div>
        </div>
      </header>

      {/* Main Content */}
      <main className="container py-8">
        <div className="flex items-center justify-between mb-8">
          <div>
            <h1 className="text-3xl font-bold">我的目的地</h1>
            <p className="text-muted-foreground mt-1">
              管理你创建的目的地 Agent
            </p>
          </div>
        </div>

        {/* Loading State */}
        {loading && agents.length === 0 && (
          <div className="flex flex-col items-center justify-center py-20">
            <Loader2 className="h-10 w-10 animate-spin text-primary mb-4" />
            <p className="text-muted-foreground">加载中...</p>
          </div>
        )}

        {/* Error State */}
        {error && (
          <div className="flex flex-col items-center justify-center py-20">
            <AlertCircle className="h-10 w-10 text-destructive mb-4" />
            <p className="text-destructive font-medium mb-2">加载失败</p>
            <p className="text-muted-foreground text-sm mb-4">{error}</p>
            <Button variant="outline" onClick={fetchAgents}>
              重试
            </Button>
          </div>
        )}

        {/* Agent Grid */}
        {!loading && !error && (
          <div className="grid md:grid-cols-2 lg:grid-cols-3 gap-6">
            {/* Create New Card */}
            <Link href="/destinations/create">
              <Card className="h-full cursor-pointer hover:border-primary transition-colors border-dashed">
                <CardContent className="flex flex-col items-center justify-center h-full py-12">
                  <div className="h-16 w-16 rounded-full bg-primary/10 flex items-center justify-center mb-4">
                    <Plus className="h-8 w-8 text-primary" />
                  </div>
                  <p className="font-medium">创建新目的地 Agent</p>
                  <p className="text-sm text-muted-foreground mt-1">
                    输入目的地，AI 自动构建知识库
                  </p>
                </CardContent>
              </Card>
            </Link>

            {/* Agent Cards */}
            {agents.map((agent) => (
              <Card key={agent.id} className="h-full cursor-pointer hover:border-primary transition-colors relative group">
                <Link href={`/guide/${agent.id}`} className="block">
                  <CardHeader>
                    <div className="flex items-start justify-between">
                      <div className="h-12 w-12 rounded-lg bg-primary/10 flex items-center justify-center">
                        <MapPin className="h-6 w-6 text-primary" />
                      </div>
                      <span className={`text-xs px-2 py-1 rounded-full ${getStatusColor(agent.status)}`}>
                        {getStatusLabel(agent.status)}
                      </span>
                    </div>
                    <CardTitle className="mt-4">{agent.name || agent.destination}</CardTitle>
                    <CardDescription>{agent.destination}</CardDescription>
                  </CardHeader>
                  <CardContent>
                    <div className="flex items-center gap-4 text-sm text-muted-foreground">
                      <span>{agent.document_count || 0} 篇文档</span>
                      <span>•</span>
                      <span className="capitalize">{agent.theme || 'cultural'}</span>
                      {agent.usage_count > 0 && (
                        <>
                          <span>•</span>
                          <span>使用 {agent.usage_count} 次</span>
                        </>
                      )}
                    </div>
                    {agent.description && (
                      <p className="text-sm text-muted-foreground mt-2 line-clamp-2">
                        {agent.description}
                      </p>
                    )}
                    <p className="text-xs text-muted-foreground mt-2">
                      创建于 {formatDate(agent.created_at)}
                    </p>
                  </CardContent>
                </Link>
                {/* Delete Button */}
                <button
                  onClick={(e) => handleDeleteClick(agent.id, agent.name || agent.destination, e)}
                  disabled={deleting === agent.id}
                  className="absolute top-3 right-3 p-2 rounded-full bg-background/80 hover:bg-destructive/10 hover:text-destructive opacity-0 group-hover:opacity-100 transition-all disabled:opacity-50"
                  title="删除 Agent"
                >
                  {deleting === agent.id ? (
                    <Loader2 className="h-4 w-4 animate-spin" />
                  ) : (
                    <Trash2 className="h-4 w-4" />
                  )}
                </button>
              </Card>
            ))}
          </div>
        )}

        {/* Empty State */}
        {!loading && !error && agents.length === 0 && (
          <div className="text-center py-12">
            <div className="h-20 w-20 rounded-full bg-muted flex items-center justify-center mx-auto mb-4">
              <MapPin className="h-10 w-10 text-muted-foreground" />
            </div>
            <h3 className="text-lg font-medium mb-2">还没有创建任何 Agent</h3>
            <p className="text-muted-foreground mb-4">
              创建你的第一个目的地 Agent，开始智能导游之旅
            </p>
            <Link href="/destinations/create">
              <Button>
                <Plus className="h-4 w-4 mr-2" />
                创建 Agent
              </Button>
            </Link>
          </div>
        )}
      </main>

      {/* Delete Confirmation Modal */}
      <DeleteConfirmModal
        isOpen={deleteModal.open}
        agentName={deleteModal.agentName}
        onConfirm={handleDeleteConfirm}
        onCancel={() => setDeleteModal({ open: false, agentId: "", agentName: "" })}
      />
    </div>
  );
}
