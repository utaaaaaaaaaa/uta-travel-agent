"use client";

import { useState, useEffect, useRef, useMemo } from "react";
import { useParams, useRouter } from "next/navigation";
import Link from "next/link";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Globe, ArrowLeft, Clock, Zap, Loader2, ChevronDown, ChevronUp, Users, FileText, Search, Database, Layers } from "lucide-react";

interface ExplorationStep {
  timestamp: string;
  direction: string;
  thought: string;
  action: string;
  tool_name: string;
  result: string;
  tokens_in: number;
  tokens_out: number;
  duration_ms: number;
  success?: boolean;
}

interface AgentProgress {
  ID: string;
  CurrentRound: number;
  MaxRounds: number;
  CurrentTopic: string;
  DocumentsFound: number;
  Status: string;
}

interface TaskDetails {
  task_id: string;
  agent_id: string;
  status: string;
  duration_seconds: number;
  total_tokens: number;
  exploration_log: ExplorationStep[];
  result?: {
    document_count?: number;
    covered_topics?: Record<string, number>;
    missing_topics?: string[];
    researchers?: AgentProgress[];
    collection_id?: string;
    errors?: string[];
  };
}

// Topic labels
const topicLabels: Record<string, string> = {
  attractions: "景点",
  food: "美食",
  history: "历史",
  transport: "交通",
  accommodation: "住宿",
  entertainment: "娱乐",
  shopping: "购物",
  practical: "实用信息",
};

// Simple Agent Visualization - One point per agent, all moving
function SimpleAgentRadar({
  coveredTopics,
  agents,
  isRunning
}: {
  coveredTopics: Record<string, number>;
  agents: AgentProgress[];
  isRunning: boolean;
}) {
  const size = 320;
  const center = size / 2;
  const radius = 120;
  const [tick, setTick] = useState(0);

  // Animation loop - ALWAYS run so agents move
  useEffect(() => {
    const interval = setInterval(() => setTick(t => t + 1), 80);
    return () => clearInterval(interval);
  }, []);

  // Default topics if coveredTopics is empty
  const defaultTopics = ["attractions", "food", "history", "transport"];
  const topics = Object.keys(coveredTopics).length > 0
    ? Object.keys(coveredTopics).filter(t => !["curating", "indexing"].includes(t))
    : defaultTopics;
  const numAxes = topics.length;
  const axisAngle = (2 * Math.PI) / numAxes;

  // Topic to axis index
  const topicAxisIndex: Record<string, number> = {};
  topics.forEach((t, i) => { topicAxisIndex[t] = i; });

  // Agent colors - DISTINCT for each agent
  const getAgentColor = (id: string): string => {
    if (id === "researcher-1") return "#3b82f6"; // blue
    if (id === "researcher-2") return "#f97316"; // orange
    if (id === "researcher-3") return "#a855f7"; // purple
    if (id === "researcher-4") return "#22c55e"; // green
    if (id.startsWith("curator")) return "#ec4899"; // pink
    if (id.startsWith("indexer")) return "#06b6d4"; // cyan
    return "#6b7280"; // gray
  };

  const getAgentLabel = (id: string): string => {
    if (id === "researcher-1") return "R1";
    if (id === "researcher-2") return "R2";
    if (id === "researcher-3") return "R3";
    if (id === "researcher-4") return "R4";
    if (id.startsWith("curator")) return "C";
    if (id.startsWith("indexer")) return "I";
    return "?";
  };

  // Calculate position for each agent
  const getAgentPosition = (agent: AgentProgress, time: number) => {
    const isActive = isRunning && agent.Status !== "complete";
    const progress = agent.CurrentRound / Math.max(agent.MaxRounds, 1);
    const num = agent.ID.includes("-") ? parseInt(agent.ID.split("-")[1]) : 1;

    let angle = 0;
    let dist = radius * 0.5;

    if (agent.ID.startsWith("researcher")) {
      // Map topic to angle: attractions=0, food=90, history=180, transport=270 degrees
      const topicAngles: Record<string, number> = {
        attractions: -Math.PI / 2,  // top
        food: 0,                    // right
        history: Math.PI / 2,       // bottom
        transport: Math.PI,         // left
      };
      angle = topicAngles[agent.CurrentTopic] ?? (num - 1) * (Math.PI / 2);

      // Add wobble movement
      const wobble = isActive ? 0.15 : 0.06;
      angle += Math.sin(time * 0.08 + num * 2) * wobble;

      // Distance varies slightly
      dist = radius * (0.35 + progress * 0.35 + Math.sin(time * 0.05 + num) * 0.05);

    } else if (agent.ID.startsWith("curator")) {
      // Curator circles the center
      angle = time * 0.03;
      dist = radius * 0.2 + Math.sin(time * 0.04) * radius * 0.03;

    } else if (agent.ID.startsWith("indexer")) {
      // Indexer circles in opposite direction
      angle = -time * 0.04 + Math.PI;
      dist = radius * 0.25 + Math.cos(time * 0.05) * radius * 0.03;
    }

    return {
      x: center + Math.cos(angle) * dist,
      y: center + Math.sin(angle) * dist,
      isActive,
    };
  };

  return (
    <div className="relative">
      <svg width={size} height={size} viewBox={`0 0 ${size} ${size}`}>
        {/* Background circles */}
        {[25, 50, 75, 100].map((pct) => (
          <circle key={pct} cx={center} cy={center} r={radius * pct / 100}
            fill="none" stroke="#334155" strokeOpacity="0.3" />
        ))}

        {/* Radar axes with labels */}
        {topics.map((topic, i) => {
          const angle = i * axisAngle - Math.PI / 2;
          return (
            <g key={topic}>
              <line x1={center} y1={center}
                x2={center + radius * Math.cos(angle)}
                y2={center + radius * Math.sin(angle)}
                stroke="#334155" strokeOpacity="0.4" />
              <text
                x={center + (radius + 18) * Math.cos(angle)}
                y={center + (radius + 18) * Math.sin(angle)}
                textAnchor="middle" dominantBaseline="middle"
                className="text-[10px] fill-slate-400">
                {topicLabels[topic] || topic}
              </text>
            </g>
          );
        })}

        {/* All 6 agent points */}
        {agents.map((agent) => {
          const color = getAgentColor(agent.ID);
          const label = getAgentLabel(agent.ID);
          const pos = getAgentPosition(agent, tick);
          const r = pos.isActive ? 12 : 9;

          return (
            <g key={agent.ID}>
              {/* Glow ring when active */}
              {pos.isActive && (
                <circle cx={pos.x} cy={pos.y} r={r + 6}
                  fill={color} fillOpacity="0.15" />
              )}
              {/* Main circle */}
              <circle cx={pos.x} cy={pos.y} r={r}
                fill={color} stroke="#fff" strokeWidth="2" />
              {/* Label inside */}
              <text x={pos.x} y={pos.y}
                textAnchor="middle" dominantBaseline="middle"
                className="text-[9px] font-bold" fill="#fff">
                {label}
              </text>
            </g>
          );
        })}
      </svg>

      {/* Legend */}
      <div className="flex flex-wrap justify-center gap-3 mt-2 text-xs">
        {[["R1", "#3b82f6"], ["R2", "#f97316"], ["R3", "#a855f7"], ["R4", "#22c55e"], ["C", "#ec4899"], ["I", "#06b6d4"]].map(([lbl, clr]) => (
          <div key={lbl} className="flex items-center gap-1">
            <div className="w-2.5 h-2.5 rounded-full" style={{ backgroundColor: clr }} />
            <span className="text-slate-400">{lbl}</span>
          </div>
        ))}
      </div>
    </div>
  );
}

// Agent Card Component
function AgentCard({ agent }: { agent: AgentProgress }) {
  const statusColors: Record<string, string> = {
    ready: "bg-gray-500",
    searching: "bg-blue-500 animate-pulse",
    processing: "bg-yellow-500 animate-pulse",
    complete: "bg-green-500",
  };

  const progress = (agent.CurrentRound / Math.max(agent.MaxRounds, 1)) * 100;
  const topicLabel = topicLabels[agent.CurrentTopic] || agent.CurrentTopic;

  const getAgentInfo = (id: string) => {
    if (id.includes("curator")) return { Icon: Database, label: "策展" };
    if (id.includes("indexer")) return { Icon: Layers, label: "索引" };
    return { Icon: Search, label: "研究" };
  };

  const { Icon, label } = getAgentInfo(agent.ID);

  return (
    <Card className="relative overflow-hidden">
      <div className={`absolute top-0 left-0 h-1 ${statusColors[agent.Status] || "bg-gray-500"}`}
        style={{ width: `${progress}%` }} />
      <CardContent className="p-2.5">
        <div className="flex items-center justify-between mb-1">
          <div className="flex items-center gap-1.5">
            <div className={`w-2 h-2 rounded-full ${statusColors[agent.Status] || "bg-gray-500"}`} />
            <Icon className="h-3 w-3 text-muted-foreground" />
            <span className="text-xs font-medium">{agent.ID}</span>
          </div>
          <span className="text-[10px] text-muted-foreground">
            {agent.Status === "complete" ? "✓" : `${agent.CurrentRound}/${agent.MaxRounds}`}
          </span>
        </div>
        <div className="flex items-center justify-between text-[10px] text-muted-foreground">
          <span>{topicLabel}</span>
          {agent.DocumentsFound > 0 && <span>{agent.DocumentsFound}篇</span>}
        </div>
      </CardContent>
    </Card>
  );
}

// Timeline Item
function TimelineItem({ step, index }: { step: ExplorationStep; index: number }) {
  const [expanded, setExpanded] = useState(false);
  const time = new Date(step.timestamp).toLocaleTimeString("zh-CN");

  const stageColors: Record<string, string> = {
    researching: "bg-blue-500",
    curating: "bg-pink-500",
    indexing: "bg-cyan-500",
  };

  return (
    <div className="relative pl-6 pb-3">
      {index > 0 && <div className="absolute left-[9px] top-0 w-px h-full bg-border" />}
      <div className={`absolute left-0 w-5 h-5 rounded-full flex items-center justify-center text-[10px] font-bold text-white ${stageColors[step.action] || "bg-gray-500"}`}>
        {index + 1}
      </div>
      <Card className="cursor-pointer hover:bg-muted/50" onClick={() => setExpanded(!expanded)}>
        <CardContent className="p-2">
          <div className="flex items-center justify-between gap-2">
            <div className="flex-1 min-w-0">
              <div className="flex items-center gap-2 text-[10px] text-muted-foreground">
                <span>{time}</span>
                <span className="font-medium text-foreground">[{step.direction}]</span>
              </div>
              <p className="text-xs truncate">{step.thought}</p>
            </div>
            {expanded ? <ChevronUp className="h-3 w-3" /> : <ChevronDown className="h-3 w-3" />}
          </div>
          {expanded && step.result && (
            <p className="text-[10px] mt-2 p-1.5 bg-muted rounded max-h-20 overflow-y-auto">{step.result}</p>
          )}
        </CardContent>
      </Card>
    </div>
  );
}

export default function TaskDetailsPage() {
  const params = useParams();
  const router = useRouter();
  const taskId = params.id as string;

  const [task, setTask] = useState<TaskDetails | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [isConnected, setIsConnected] = useState(false);
  const seenStepsRef = useRef<Set<string>>(new Set());

  const isRunning = task?.status === "running" || task?.status === "creating" || task?.status === "pending";

  const fetchTask = async () => {
    try {
      const res = await fetch(`http://localhost:8080/api/v1/tasks/${taskId}`);
      if (!res.ok) throw new Error("Failed to fetch");
      const data = await res.json();
      setTask(data);
      return data;
    } catch (err) {
      setError(err instanceof Error ? err.message : "加载失败");
      return null;
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => { fetchTask(); }, [taskId]);

  // SSE for running tasks only
  useEffect(() => {
    if (!task || !isRunning) return;

    const es = new EventSource(`http://localhost:8080/api/v1/tasks/${taskId}/stream`);
    es.onopen = () => setIsConnected(true);

    es.addEventListener("step", (e) => {
      try {
        const data = JSON.parse(e.data);
        const key = `${data.timestamp}-${data.direction}`;
        if (!seenStepsRef.current.has(key)) {
          seenStepsRef.current.add(key);
          setTask(prev => prev ? { ...prev, exploration_log: [...(prev.exploration_log || []), data] } : prev);
        }
      } catch {}
    });

    es.addEventListener("progress", (e) => {
      try {
        const data = JSON.parse(e.data);
        setTask(prev => prev ? {
          ...prev,
          result: { ...prev.result, ...data, researchers: data.researchers ?? prev.result?.researchers }
        } : prev);
      } catch {}
    });

    es.addEventListener("complete", () => { es.close(); setIsConnected(false); fetchTask(); });
    es.onerror = () => { es.close(); setIsConnected(false); };

    return () => es.close();
  }, [taskId, task?.status, isRunning]);

  const agents = useMemo(() => task?.result?.researchers || [], [task?.result?.researchers]);
  const steps = useMemo(() =>
    [...(task?.exploration_log || [])].sort((a, b) =>
      new Date(a.timestamp).getTime() - new Date(b.timestamp).getTime()),
    [task?.exploration_log]
  );

  if (loading && !task) return <div className="min-h-screen flex items-center justify-center"><Loader2 className="h-8 w-8 animate-spin text-primary" /></div>;
  if (error || !task) return <div className="min-h-screen flex items-center justify-center"><Card className="max-w-md"><CardContent className="p-6 text-center"><p className="text-destructive">{error || "任务不存在"}</p><Button className="mt-4" onClick={() => router.push("/")}>返回首页</Button></CardContent></Card></div>;

  return (
    <div className="min-h-screen">
      <header className="sticky top-0 z-50 w-full border-b bg-background/95 backdrop-blur">
        <div className="container flex h-14 items-center justify-between">
          <div className="flex items-center gap-3">
            <Button variant="ghost" size="icon" onClick={() => router.back()}><ArrowLeft className="h-4 w-4" /></Button>
            <Globe className="h-5 w-5 text-primary" />
            <span className="font-bold">任务详情</span>
            {isConnected && isRunning && <span className="px-1.5 py-0.5 text-[10px] bg-green-500/20 text-green-500 rounded">实时</span>}
            {!isRunning && <span className="px-1.5 py-0.5 text-[10px] bg-gray-500/20 text-gray-500 rounded">完成</span>}
          </div>
          <Link href="/" className="font-bold">UTA Travel</Link>
        </div>
      </header>

      <main className="container py-6">
        {/* Stats */}
        <div className="grid grid-cols-4 gap-3 mb-6">
          {[
            { icon: Clock, label: "耗时", value: task.duration_seconds > 0 ? `${task.duration_seconds.toFixed(1)}s` : (isRunning ? "..." : "-") },
            { icon: Zap, label: "Tokens", value: task.total_tokens },
            { icon: FileText, label: "文档", value: task.result?.document_count || 0 },
            { icon: Users, label: "Agent", value: agents.length },
          ].map((s, i) => (
            <Card key={i}>
              <CardContent className="p-3 text-center">
                <s.icon className="h-4 w-4 mx-auto mb-1 text-muted-foreground" />
                <p className="text-lg font-bold">{s.value}</p>
                <p className="text-[10px] text-muted-foreground">{s.label}</p>
              </CardContent>
            </Card>
          ))}
        </div>

        {/* Agent Cards */}
        {agents.length > 0 && (
          <Card className="mb-6">
            <CardHeader className="py-3"><CardTitle className="text-sm flex items-center gap-2"><Users className="h-4 w-4" />Agent 状态</CardTitle></CardHeader>
            <CardContent className="pb-3">
              <div className="grid grid-cols-3 md:grid-cols-6 gap-2">
                {agents.map(a => <AgentCard key={a.ID} agent={a} />)}
              </div>
            </CardContent>
          </Card>
        )}

        <div className="grid md:grid-cols-2 gap-6">
          {/* Radar Chart */}
          <Card>
            <CardHeader className="py-3">
              <CardTitle className="text-sm">Agent 活动图</CardTitle>
              <p className="text-xs text-muted-foreground">每个点代表一个 Agent 在实时探索</p>
            </CardHeader>
            <CardContent className="flex justify-center py-4">
              <SimpleAgentRadar coveredTopics={task.result?.covered_topics || {}} agents={agents} isRunning={isRunning} />
            </CardContent>
          </Card>

          {/* Timeline */}
          <Card>
            <CardHeader className="py-3">
              <CardTitle className="text-sm">时间线 ({steps.length} 步)</CardTitle>
            </CardHeader>
            <CardContent>
              <div className="max-h-[350px] overflow-y-auto">
                {steps.length > 0 ? steps.map((s, i) => <TimelineItem key={i} step={s} index={i} />) :
                  <p className="text-center text-muted-foreground py-8">{isRunning ? "等待..." : "无记录"}</p>}
              </div>
            </CardContent>
          </Card>
        </div>

        {!isRunning && task.status === "completed" && task.agent_id && (
          <div className="mt-6 text-center">
            <Button onClick={() => router.push(`/guide/${task.agent_id}`)} size="lg">开始对话</Button>
          </div>
        )}
      </main>
    </div>
  );
}