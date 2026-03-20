"use client";

import { useState, useEffect, useRef, useCallback } from "react";
import { useRouter } from "next/navigation";
import Link from "next/link";
import { Button } from "@/components/ui/button";
import { Card, CardContent } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Globe, ArrowRight, Check, Loader2, AlertCircle, Sparkles, Bot, Zap, Clock } from "lucide-react";

// API base URL
const API_BASE_URL = process.env.NEXT_PUBLIC_API_URL || 'http://localhost:8080';

interface ExplorationPoint {
  id: string;
  direction: string;
  agentType: string;
  thought: string;
  action: string;
  timestamp: number;
  angle: number;
  distance: number;
  intensity: number;
  fadeStart: number;
}

interface AgentState {
  id: string;
  name: string;
  status: 'idle' | 'running' | 'completed';
  angle: number;
  targetAngle: number;
  color: string;
}

// Real-time Radar Component
function RadarVisualization({
  explorationPoints,
  agents,
  scanAngle
}: {
  explorationPoints: ExplorationPoint[];
  agents: AgentState[];
  scanAngle: number;
}) {
  const size = 320;
  const center = size / 2;
  const radius = 140;

  // Direction labels
  const directions = [
    { name: "景点", angle: -90 },
    { name: "美食", angle: -30 },
    { name: "文化", angle: 30 },
    { name: "交通", angle: 90 },
    { name: "住宿", angle: 150 },
    { name: "购物", angle: 210 },
  ];

  // Convert direction to angle
  const getDirectionAngle = (dir: string): number => {
    const dirMap: Record<string, number> = {
      "景点": -90,
      "美食": -30,
      "文化": 30,
      "交通": 90,
      "住宿": 150,
      "购物": 210,
      "综合": 0,
    };
    return dirMap[dir] ?? 0;
  };

  // Get color for agent type
  const getAgentColor = (agentType: string): string => {
    const colors: Record<string, string> = {
      "researcher": "#3b82f6", // blue
      "curator": "#8b5cf6",    // purple
      "indexer": "#10b981",    // green
      "planner": "#f59e0b",    // amber
    };
    return colors[agentType] || "#6b7280";
  };

  // Calculate point position
  const getPointPosition = (point: ExplorationPoint) => {
    const angleRad = (point.angle * Math.PI) / 180;
    const x = center + point.distance * Math.cos(angleRad);
    const y = center + point.distance * Math.sin(angleRad);
    return { x, y };
  };

  // Check if point should be visible (within scan sweep)
  const isPointVisible = (point: ExplorationPoint): boolean => {
    const angleDiff = ((scanAngle - point.angle) % 360 + 360) % 360;
    return angleDiff < 60 || angleDiff > 300;
  };

  // Calculate point opacity based on time since last scan
  const getPointOpacity = (point: ExplorationPoint): number => {
    const age = Date.now() - point.timestamp;
    const maxAge = 8000;
    if (age > maxAge) return 0.1;

    const angleDiff = ((scanAngle - point.angle) % 360 + 360) % 360;
    if (angleDiff < 60) {
      return 1 - (angleDiff / 60) * 0.5;
    }
    return 0.3;
  };

  return (
    <div className="relative" style={{ width: size, height: size }}>
      <svg width={size} height={size} viewBox={`0 0 ${size} ${size}`}>
        {/* Background */}
        <defs>
          <radialGradient id="radarGradient" cx="50%" cy="50%" r="50%">
            <stop offset="0%" stopColor="currentColor" stopOpacity="0.05" />
            <stop offset="100%" stopColor="currentColor" stopOpacity="0.02" />
          </radialGradient>
          <linearGradient id="sweepGradient" x1="0%" y1="0%" x2="100%" y2="0%">
            <stop offset="0%" stopColor="currentColor" stopOpacity="0" />
            <stop offset="70%" stopColor="currentColor" stopOpacity="0.3" />
            <stop offset="100%" stopColor="currentColor" stopOpacity="0.5" />
          </linearGradient>
        </defs>

        {/* Radar background */}
        <circle cx={center} cy={center} r={radius} fill="url(#radarGradient)" className="text-primary" />

        {/* Concentric circles */}
        {[0.25, 0.5, 0.75, 1].map((r, i) => (
          <circle
            key={i}
            cx={center}
            cy={center}
            r={radius * r}
            fill="none"
            stroke="currentColor"
            strokeOpacity="0.15"
            strokeWidth="1"
            className="text-primary"
          />
        ))}

        {/* Cross lines */}
        <line x1={center} y1={center - radius} x2={center} y2={center + radius} stroke="currentColor" strokeOpacity="0.15" className="text-primary" />
        <line x1={center - radius} y1={center} x2={center + radius} y2={center} stroke="currentColor" strokeOpacity="0.15" className="text-primary" />

        {/* Direction labels */}
        {directions.map((dir, i) => {
          const angleRad = (dir.angle * Math.PI) / 180;
          const labelRadius = radius + 25;
          const x = center + labelRadius * Math.cos(angleRad);
          const y = center + labelRadius * Math.sin(angleRad);

          return (
            <text
              key={i}
              x={x}
              y={y}
              textAnchor="middle"
              dominantBaseline="middle"
              className="text-xs fill-current font-medium"
            >
              {dir.name}
            </text>
          );
        })}

        {/* Exploration points (blips) */}
        {explorationPoints.map((point) => {
          const pos = getPointPosition(point);
          const opacity = getPointOpacity(point);
          const color = getAgentColor(point.agentType);

          return (
            <g key={point.id}>
              {/* Glow effect */}
              <circle
                cx={pos.x}
                cy={pos.y}
                r="8"
                fill={color}
                opacity={opacity * 0.3}
              />
              {/* Point */}
              <circle
                cx={pos.x}
                cy={pos.y}
                r="4"
                fill={color}
                opacity={opacity}
              />
            </g>
          );
        })}

        {/* Agent positions */}
        {agents.filter(a => a.status === 'running').map((agent) => {
          const angleRad = (agent.angle * Math.PI) / 180;
          const distance = radius * 0.7;
          const x = center + distance * Math.cos(angleRad);
          const y = center + distance * Math.sin(angleRad);

          return (
            <g key={agent.id}>
              {/* Agent glow */}
              <circle
                cx={x}
                cy={y}
                r="12"
                fill={agent.color}
                opacity="0.3"
              >
                <animate
                  attributeName="r"
                  values="12;16;12"
                  dur="1.5s"
                  repeatCount="indefinite"
                />
                <animate
                  attributeName="opacity"
                  values="0.3;0.5;0.3"
                  dur="1.5s"
                  repeatCount="indefinite"
                />
              </circle>
              {/* Agent dot */}
              <circle
                cx={x}
                cy={y}
                r="6"
                fill={agent.color}
              />
              {/* Agent label */}
              <text
                x={x}
                y={y - 18}
                textAnchor="middle"
                className="text-[10px] fill-current font-medium"
                fill={agent.color}
              >
                {agent.name}
              </text>
            </g>
          );
        })}

        {/* Scan line (sweep) */}
        <path
          d={`M ${center} ${center} L ${center} ${center - radius} A ${radius} ${radius} 0 0 1 ${center + radius * Math.sin((scanAngle * Math.PI) / 180)} ${center - radius * Math.cos((scanAngle * Math.PI) / 180)} Z`}
          fill="url(#sweepGradient)"
          className="text-primary"
          opacity="0.6"
        />

        {/* Center point */}
        <circle cx={center} cy={center} r="4" fill="currentColor" className="text-primary" />
      </svg>

      {/* Legend */}
      <div className="absolute -bottom-8 left-0 right-0 flex justify-center gap-4 text-xs">
        <div className="flex items-center gap-1">
          <div className="w-2 h-2 rounded-full bg-blue-500" />
          <span>研究员</span>
        </div>
        <div className="flex items-center gap-1">
          <div className="w-2 h-2 rounded-full bg-purple-500" />
          <span>整理员</span>
        </div>
        <div className="flex items-center gap-1">
          <div className="w-2 h-2 rounded-full bg-green-500" />
          <span>索引员</span>
        </div>
      </div>
    </div>
  );
}

// Mini Agent Card
function AgentCard({ agent }: { agent: AgentState }) {
  const statusColors = {
    idle: "bg-muted text-muted-foreground",
    running: "bg-primary/10 text-primary border-primary/30",
    completed: "bg-green-500/10 text-green-600 border-green-500/30",
  };

  const statusIcons = {
    idle: <div className="w-3 h-3 rounded-full bg-muted-foreground/30" />,
    running: <Loader2 className="w-3 h-3 animate-spin" />,
    completed: <Check className="w-3 h-3" />,
  };

  return (
    <div className={`flex items-center gap-2 p-2 rounded-lg border ${statusColors[agent.status]}`}>
      <Bot className="w-4 h-4" />
      <span className="text-sm font-medium">{agent.name}</span>
      {statusIcons[agent.status]}
    </div>
  );
}

// Stats Bar
function StatsBar({ tokens, duration }: { tokens: number; duration: number }) {
  return (
    <div className="flex items-center gap-4 text-sm text-muted-foreground">
      <div className="flex items-center gap-1">
        <Zap className="w-4 h-4" />
        <span>{tokens.toLocaleString()} tokens</span>
      </div>
      <div className="flex items-center gap-1">
        <Clock className="w-4 h-4" />
        <span>{duration}s</span>
      </div>
    </div>
  );
}

const themes = [
  { id: "cultural", label: "文化之旅", icon: "🏛️", description: "历史遗迹、博物馆、传统文化" },
  { id: "food", label: "美食探索", icon: "🍜", description: "当地美食、餐厅推荐、烹饪体验" },
  { id: "adventure", label: "户外探险", icon: "🏔️", description: "徒步、攀岩、自然风光" },
  { id: "art", label: "艺术之旅", icon: "🎨", description: "美术馆、街头艺术、设计" },
];

const languages = [
  { code: "zh", label: "中文" },
  { code: "en", label: "English" },
  { code: "ja", label: "日本語" },
];

export default function CreateDestinationPage() {
  const router = useRouter();

  const [destination, setDestination] = useState("");
  const [selectedTheme, setSelectedTheme] = useState("cultural");
  const [selectedLanguages, setSelectedLanguages] = useState<string[]>(["zh"]);

  // Creation state
  const [status, setStatus] = useState<'idle' | 'creating' | 'completed' | 'error'>('idle');
  const [error, setError] = useState<string | null>(null);
  const [agentId, setAgentId] = useState<string | null>(null);

  // Radar state
  const [scanAngle, setScanAngle] = useState(0);
  const [explorationPoints, setExplorationPoints] = useState<ExplorationPoint[]>([]);
  const [agents, setAgents] = useState<AgentState[]>([
    { id: "researcher", name: "研究员", status: "idle", angle: -90, targetAngle: -90, color: "#3b82f6" },
    { id: "curator", name: "整理员", status: "idle", angle: 90, targetAngle: 90, color: "#8b5cf6" },
    { id: "indexer", name: "索引员", status: "idle", angle: 180, targetAngle: 180, color: "#10b981" },
  ]);
  const [totalTokens, setTotalTokens] = useState(0);
  const [duration, setDuration] = useState(0);

  // Animation frame for radar sweep
  const animationRef = useRef<number | null>(null);
  const startTimeRef = useRef<number>(0);

  // Radar sweep animation
  useEffect(() => {
    if (status === 'creating') {
      const animate = () => {
        setScanAngle((prev) => (prev + 1) % 360);
        animationRef.current = requestAnimationFrame(animate);
      };
      animationRef.current = requestAnimationFrame(animate);

      return () => {
        if (animationRef.current) {
          cancelAnimationFrame(animationRef.current);
        }
      };
    }
  }, [status]);

  // Update duration
  useEffect(() => {
    if (status === 'creating') {
      const interval = setInterval(() => {
        setDuration(Math.floor((Date.now() - startTimeRef.current) / 1000));
      }, 1000);
      return () => clearInterval(interval);
    }
  }, [status]);

  // Simulate exploration points
  const addExplorationPoint = (agentType: string, direction: string, thought: string) => {
    const dirAngles: Record<string, number> = {
      "景点": -90 + (Math.random() - 0.5) * 30,
      "美食": -30 + (Math.random() - 0.5) * 30,
      "文化": 30 + (Math.random() - 0.5) * 30,
      "交通": 90 + (Math.random() - 0.5) * 30,
      "住宿": 150 + (Math.random() - 0.5) * 30,
      "购物": 210 + (Math.random() - 0.5) * 30,
      "综合": Math.random() * 360,
    };

    const point: ExplorationPoint = {
      id: `${Date.now()}-${Math.random()}`,
      direction,
      agentType,
      thought,
      action: "探索中",
      timestamp: Date.now(),
      angle: dirAngles[direction] || Math.random() * 360,
      distance: 40 + Math.random() * 80,
      intensity: 0.5 + Math.random() * 0.5,
      fadeStart: Date.now(),
    };

    setExplorationPoints((prev) => [...prev.slice(-50), point]);
    setTotalTokens((prev) => prev + Math.floor(100 + Math.random() * 200));
  };

  // Simulate agent movement
  const moveAgent = (agentId: string, direction: string) => {
    const dirAngles: Record<string, number> = {
      "景点": -90,
      "美食": -30,
      "文化": 30,
      "交通": 90,
      "住宿": 150,
      "购物": 210,
    };

    setAgents((prev) => prev.map((a) =>
      a.id === agentId
        ? { ...a, targetAngle: dirAngles[direction] || a.targetAngle }
        : a
    ));
  };

  // Animate agents toward target
  useEffect(() => {
    if (status !== 'creating') return;

    const interval = setInterval(() => {
      setAgents((prev) => prev.map((agent) => {
        const diff = agent.targetAngle - agent.angle;
        if (Math.abs(diff) < 1) return agent;
        return { ...agent, angle: agent.angle + diff * 0.1 };
      }));
    }, 50);

    return () => clearInterval(interval);
  }, [status]);

  const handleCreate = async () => {
    setStatus('creating');
    setError(null);
    setExplorationPoints([]);
    setTotalTokens(0);
    setDuration(0);
    startTimeRef.current = Date.now();

    // Set researcher as running
    setAgents((prev) => prev.map((a) =>
      a.id === "researcher" ? { ...a, status: "running" } : a
    ));

    try {
      // 1. Create agent via API
      const response = await fetch(`${API_BASE_URL}/api/v1/agents`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          destination,
          theme: selectedTheme,
          languages: selectedLanguages,
        }),
      });

      if (!response.ok) {
        throw new Error(`创建失败: ${response.status}`);
      }

      const data = await response.json();
      const { agent_id, task_id } = data;
      setAgentId(agent_id);

      // 2. Connect to SSE for progress updates
      const eventSource = new EventSource(`${API_BASE_URL}/api/v1/tasks/${task_id}/stream`);

      eventSource.onmessage = (event) => {
        try {
          const progressData = JSON.parse(event.data);
          handleProgressUpdate(progressData);
        } catch (e) {
          console.error('Failed to parse SSE data:', e);
        }
      };

      eventSource.addEventListener('progress', (event) => {
        try {
          const progressData = JSON.parse((event as MessageEvent).data);
          handleProgressUpdate(progressData);
        } catch (e) {
          console.error('Failed to parse progress event:', e);
        }
      });

      eventSource.addEventListener('complete', (event) => {
        try {
          const completeData = JSON.parse((event as MessageEvent).data);
          console.log('Task completed:', completeData);

          // Update tokens and duration
          if (completeData.tokens) {
            setTotalTokens(completeData.tokens);
          }
          if (completeData.duration_sec) {
            setDuration(Math.round(completeData.duration_sec));
          }

          // Mark all agents as completed
          setAgents((prev) => prev.map((a) => ({ ...a, status: "completed" })));
          setStatus('completed');

          eventSource.close();
        } catch (e) {
          console.error('Failed to parse complete event:', e);
        }
      });

      eventSource.onerror = (err) => {
        console.error('SSE error:', err);
        eventSource.close();

        // If connection fails, still check task status via polling
        pollTaskStatus(task_id, agent_id);
      };

      // Cleanup on unmount
      return () => {
        eventSource.close();
      };

    } catch (e) {
      console.error('Create agent error:', e);
      setError(e instanceof Error ? e.message : "创建失败，请稍后重试");
      setStatus('error');
      setAgents((prev) => prev.map((a) => ({ ...a, status: "idle" })));
    }
  };

  // Handle progress updates from SSE
  const handleProgressUpdate = useCallback((data: {
    stage?: string;
    step?: {
      direction: string;
      thought: string;
      action: string;
      tool_name?: string;
    };
    message?: string;
  }) => {
    if (data.step) {
      // Add exploration point
      addExplorationPoint(
        getAgentTypeFromStage(data.stage),
        data.step.direction,
        data.step.thought
      );

      // Move agent to direction
      moveAgent(getAgentTypeFromStage(data.stage), data.step.direction);
    }

    if (data.stage) {
      // Update agent status based on stage
      updateAgentStatus(data.stage);
    }
  }, []);

  // Poll task status as fallback
  const pollTaskStatus = async (taskId: string, agentId: string) => {
    const maxPolls = 60; // 30 seconds max
    let pollCount = 0;

    const poll = async () => {
      try {
        const response = await fetch(`${API_BASE_URL}/api/v1/tasks/${taskId}`);
        if (!response.ok) return;

        const task = await response.json();

        if (task.status === 'completed') {
          setAgentId(agentId);
          setTotalTokens(task.total_tokens || 0);
          setDuration(Math.round(task.duration_seconds || 0));
          setAgents((prev) => prev.map((a) => ({ ...a, status: "completed" })));
          setStatus('completed');
          return;
        }

        if (task.status === 'failed') {
          setError(task.error || "创建失败");
          setStatus('error');
          return;
        }

        // Update exploration log
        if (task.exploration_log && task.exploration_log.length > 0) {
          const latestStep = task.exploration_log[task.exploration_log.length - 1];
          addExplorationPoint(
            getAgentTypeFromStage(task.status),
            latestStep.direction,
            latestStep.thought
          );
        }

        // Continue polling
        pollCount++;
        if (pollCount < maxPolls && task.status === 'running') {
          setTimeout(poll, 500);
        }
      } catch (e) {
        console.error('Poll error:', e);
      }
    };

    poll();
  };

  // Helper: get agent type from stage
  const getAgentTypeFromStage = (stage?: string): string => {
    if (!stage) return "researcher";
    if (stage.includes('research') || stage === 'exploring') return "researcher";
    if (stage.includes('curat')) return "curator";
    if (stage.includes('index')) return "indexer";
    return "researcher";
  };

  // Helper: update agent status based on stage
  const updateAgentStatus = (stage: string) => {
    setAgents((prev) => {
      const newAgents = [...prev];

      if (stage.includes('research')) {
        return newAgents.map((a) =>
          a.id === "researcher" ? { ...a, status: "running" } :
          a.status === "running" ? { ...a, status: "completed" } : a
        );
      }

      if (stage.includes('curat')) {
        return newAgents.map((a) =>
          a.id === "curator" ? { ...a, status: "running" } :
          a.id === "researcher" ? { ...a, status: "completed" } : a
        );
      }

      if (stage.includes('index')) {
        return newAgents.map((a) =>
          a.id === "indexer" ? { ...a, status: "running" } :
          a.id === "curator" ? { ...a, status: "completed" } : a
        );
      }

      return newAgents;
    });
  };

  const handleViewAgent = () => {
    if (agentId) {
      router.push(`/guide/${agentId}`);
    }
  };

  const handleViewTaskDetails = () => {
    if (agentId) {
      router.push(`/tasks/${agentId}`);
    }
  };

  const handleCreateAnother = () => {
    setStatus('idle');
    setDestination("");
    setSelectedTheme("cultural");
    setSelectedLanguages(["zh"]);
    setExplorationPoints([]);
    setAgents([
      { id: "researcher", name: "研究员", status: "idle", angle: -90, targetAngle: -90, color: "#3b82f6" },
      { id: "curator", name: "整理员", status: "idle", angle: 90, targetAngle: 90, color: "#8b5cf6" },
      { id: "indexer", name: "索引员", status: "idle", angle: 180, targetAngle: 180, color: "#10b981" },
    ]);
  };

  const toggleLanguage = (code: string) => {
    setSelectedLanguages((prev) =>
      prev.includes(code) ? prev.filter((l) => l !== code) : [...prev, code]
    );
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
        </div>
      </header>

      <main className="container py-8 max-w-5xl">
        {/* Step 1: Form */}
        {status === 'idle' && (
          <div className="max-w-2xl mx-auto space-y-8">
            <div>
              <h1 className="text-3xl font-bold">创建你的专属导游 Agent</h1>
              <p className="text-muted-foreground mt-2">
                输入目的地信息，AI 将自动为你构建专属知识库
              </p>
            </div>

            {/* Destination Input */}
            <div className="space-y-2">
              <label className="text-sm font-medium">目的地名称</label>
              <Input
                placeholder="例如：京都, Japan"
                value={destination}
                onChange={(e) => setDestination(e.target.value)}
                className="text-lg"
              />
            </div>

            {/* Theme Selection */}
            <div className="space-y-3">
              <label className="text-sm font-medium">主题选择</label>
              <div className="grid grid-cols-2 gap-3">
                {themes.map((theme) => (
                  <Card
                    key={theme.id}
                    className={`cursor-pointer transition-all ${
                      selectedTheme === theme.id
                        ? "border-primary bg-primary/5"
                        : "hover:border-gray-300"
                    }`}
                    onClick={() => setSelectedTheme(theme.id)}
                  >
                    <CardContent className="p-4">
                      <div className="flex items-center gap-3">
                        <span className="text-2xl">{theme.icon}</span>
                        <div>
                          <p className="font-medium">{theme.label}</p>
                          <p className="text-xs text-muted-foreground">
                            {theme.description}
                          </p>
                        </div>
                      </div>
                    </CardContent>
                  </Card>
                ))}
              </div>
            </div>

            {/* Language Selection */}
            <div className="space-y-3">
              <label className="text-sm font-medium">支持语言</label>
              <div className="flex gap-2">
                {languages.map((lang) => (
                  <Button
                    key={lang.code}
                    variant={selectedLanguages.includes(lang.code) ? "default" : "outline"}
                    onClick={() => toggleLanguage(lang.code)}
                  >
                    {lang.label}
                  </Button>
                ))}
              </div>
            </div>

            {/* Submit */}
            <Button
              size="lg"
              className="w-full"
              disabled={!destination.trim() || selectedLanguages.length === 0}
              onClick={handleCreate}
            >
              开始创建
              <ArrowRight className="ml-2 h-4 w-4" />
            </Button>
          </div>
        )}

        {/* Step 2: Creation Progress with Radar */}
        {status === 'creating' && (
          <div className="space-y-6">
            <div className="text-center">
              <h1 className="text-2xl font-bold">正在创建中...</h1>
              <p className="text-muted-foreground mt-1">
                为 <span className="font-medium text-foreground">{destination}</span> 构建导游 Agent
              </p>
            </div>

            {/* Main visualization area */}
            <div className="grid md:grid-cols-2 gap-6">
              {/* Left: Agent Status */}
              <Card>
                <CardContent className="p-6">
                  <h3 className="font-semibold mb-4 flex items-center gap-2">
                    <Bot className="w-5 h-5" />
                    Agent 状态
                  </h3>
                  <div className="space-y-3">
                    {agents.map((agent) => (
                      <AgentCard key={agent.id} agent={agent} />
                    ))}
                  </div>

                  <div className="mt-6 pt-4 border-t">
                    <StatsBar tokens={totalTokens} duration={duration} />
                  </div>
                </CardContent>
              </Card>

              {/* Right: Radar Visualization */}
              <Card className="bg-gradient-to-br from-slate-900 to-slate-800 border-slate-700">
                <CardContent className="p-6 flex flex-col items-center justify-center">
                  <h3 className="font-semibold mb-4 text-white">探索雷达</h3>
                  <RadarVisualization
                    explorationPoints={explorationPoints}
                    agents={agents}
                    scanAngle={scanAngle}
                  />
                  <p className="text-xs text-slate-400 mt-8">
                    实时显示 Agent 探索方向
                  </p>
                </CardContent>
              </Card>
            </div>

            {/* Recent exploration log */}
            <Card>
              <CardContent className="p-4">
                <h4 className="text-sm font-medium mb-2">最近活动</h4>
                <div className="space-y-1 max-h-32 overflow-y-auto">
                  {explorationPoints.slice(-5).reverse().map((point) => (
                    <div key={point.id} className="text-xs text-muted-foreground flex items-center gap-2">
                      <span className="font-medium text-foreground">[{point.direction}]</span>
                      <span>{point.thought}</span>
                    </div>
                  ))}
                </div>
              </CardContent>
            </Card>
          </div>
        )}

        {/* Step 3: Completion */}
        {status === 'completed' && (
          <div className="max-w-2xl mx-auto space-y-8">
            <div className="text-center">
              <div className="h-20 w-20 rounded-full bg-green-100 dark:bg-green-900/30 flex items-center justify-center mx-auto mb-4">
                <Check className="h-10 w-10 text-green-600 dark:text-green-400" />
              </div>
              <h1 className="text-2xl font-bold text-green-700 dark:text-green-400">创建完成！</h1>
              <p className="text-muted-foreground mt-2">
                <span className="font-medium text-foreground">{destination}</span> 导游 Agent 已准备就绪
              </p>
            </div>

            {/* Summary Card */}
            <Card className="bg-gradient-to-br from-primary/5 to-primary/10 border-primary/20">
              <CardContent className="p-6">
                <div className="flex items-center gap-4 mb-4">
                  <div className="h-12 w-12 rounded-lg bg-primary/20 flex items-center justify-center">
                    <Sparkles className="h-6 w-6 text-primary" />
                  </div>
                  <div>
                    <h3 className="font-semibold">{destination}</h3>
                    <p className="text-sm text-muted-foreground">
                      用时 {duration}s | 消耗 {totalTokens.toLocaleString()} tokens
                    </p>
                  </div>
                </div>

                <div className="grid grid-cols-3 gap-4 text-center">
                  {agents.map((agent) => (
                    <div key={agent.id} className="space-y-1">
                      <p className="text-xs text-muted-foreground">{agent.name}</p>
                      <p className="text-sm font-medium text-green-600 dark:text-green-400">
                        <Check className="h-3 w-3 inline mr-1" />
                        完成
                      </p>
                    </div>
                  ))}
                </div>
              </CardContent>
            </Card>

            {/* Action Buttons */}
            <div className="flex gap-3">
              <Button
                size="lg"
                className="flex-1"
                onClick={handleViewAgent}
              >
                开始使用
                <ArrowRight className="ml-2 h-4 w-4" />
              </Button>
              <Button
                size="lg"
                variant="outline"
                onClick={handleViewTaskDetails}
              >
                任务详情
              </Button>
              <Button
                size="lg"
                variant="outline"
                onClick={handleCreateAnother}
              >
                创建新 Agent
              </Button>
            </div>
          </div>
        )}

        {/* Error State */}
        {status === 'error' && (
          <div className="max-w-2xl mx-auto space-y-8 text-center">
            <div className="h-20 w-20 rounded-full bg-destructive/10 flex items-center justify-center mx-auto">
              <AlertCircle className="h-10 w-10 text-destructive" />
            </div>
            <div>
              <h1 className="text-2xl font-bold text-destructive">创建失败</h1>
              <p className="text-muted-foreground mt-2">{error}</p>
            </div>
            <Button variant="outline" size="lg" onClick={handleCreateAnother}>
              重试
            </Button>
          </div>
        )}
      </main>
    </div>
  );
}