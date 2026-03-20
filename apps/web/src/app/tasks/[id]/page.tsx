"use client";

import { useState, useEffect } from "react";
import { useParams, useRouter } from "next/navigation";
import Link from "next/link";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Globe, ArrowLeft, Clock, Zap, MapPin, Loader2, ChevronDown, ChevronUp } from "lucide-react";

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
}

interface RadarDirection {
  name: string;
  value: number;
  last_update: string;
}

interface TaskDetails {
  task_id: string;
  agent_id: string;
  status: string;
  duration_seconds: number;
  total_tokens: number;
  exploration_log: ExplorationStep[];
  radar_data: {
    directions: RadarDirection[];
  };
  metadata: {
    memory_size: number;
    subagent_count: number;
    exploration_count: number;
  };
}

// Radar Chart Component
function RadarChart({ directions }: { directions: RadarDirection[] }) {
  const size = 300;
  const center = size / 2;
  const radius = 120;
  const angleStep = (2 * Math.PI) / directions.length;

  // Calculate polygon points
  const points = directions.map((dir, i) => {
    const angle = i * angleStep - Math.PI / 2; // Start from top
    const r = (dir.value / 100) * radius;
    return {
      x: center + r * Math.cos(angle),
      y: center + r * Math.sin(angle),
      labelX: center + (radius + 30) * Math.cos(angle),
      labelY: center + (radius + 30) * Math.sin(angle),
    };
  });

  const polygonPoints = points.map((p) => `${p.x},${p.y}`).join(" ");

  return (
    <svg width={size} height={size} viewBox={`0 0 ${size} ${size}`}>
      {/* Background circles */}
      {[20, 40, 60, 80, 100].map((percent) => (
        <circle
          key={percent}
          cx={center}
          cy={center}
          r={(percent / 100) * radius}
          fill="none"
          stroke="currentColor"
          strokeOpacity="0.1"
          className="text-muted-foreground"
        />
      ))}

      {/* Axis lines */}
      {directions.map((_, i) => {
        const angle = i * angleStep - Math.PI / 2;
        return (
          <line
            key={i}
            x1={center}
            y1={center}
            x2={center + radius * Math.cos(angle)}
            y2={center + radius * Math.sin(angle)}
            stroke="currentColor"
            strokeOpacity="0.2"
            className="text-muted-foreground"
          />
        );
      })}

      {/* Data polygon */}
      <polygon
        points={polygonPoints}
        fill="currentColor"
        fillOpacity="0.2"
        stroke="currentColor"
        strokeWidth="2"
        className="text-primary"
      />

      {/* Data points */}
      {points.map((p, i) => (
        <circle
          key={i}
          cx={p.x}
          cy={p.y}
          r="4"
          fill="currentColor"
          className="text-primary"
        />
      ))}

      {/* Labels */}
      {directions.map((dir, i) => {
        const angle = i * angleStep - Math.PI / 2;
        const labelRadius = radius + 25;
        const x = center + labelRadius * Math.cos(angle);
        const y = center + labelRadius * Math.sin(angle);

        return (
          <text
            key={i}
            x={x}
            y={y}
            textAnchor="middle"
            dominantBaseline="middle"
            className="text-xs fill-slate-300 dark:fill-slate-400"
          >
            <tspan className="font-medium" fill="#94a3b8">{dir.name}</tspan>
            <tspan x={x} dy="14" fill="#64748b">
              {Math.round(dir.value)}%
            </tspan>
          </text>
        );
      })}
    </svg>
  );
}

// Timeline Item Component
function TimelineItem({ step, index }: { step: ExplorationStep; index: number }) {
  const [expanded, setExpanded] = useState(false);

  const time = new Date(step.timestamp).toLocaleTimeString("zh-CN");
  const directionColors: Record<string, string> = {
    景点: "text-blue-500",
    美食: "text-orange-500",
    文化: "text-purple-500",
    交通: "text-green-500",
    住宿: "text-cyan-500",
    购物: "text-pink-500",
    综合: "text-gray-500",
  };

  return (
    <div className="relative pl-8 pb-4">
      {/* Timeline line */}
      {index > 0 && (
        <div className="absolute left-[11px] top-0 w-0.5 h-full bg-border" />
      )}

      {/* Timeline dot */}
      <div
        className={`absolute left-0 w-6 h-6 rounded-full flex items-center justify-center text-xs font-bold ${
          step.tool_name ? "bg-primary text-primary-foreground" : "bg-muted"
        }`}
      >
        {index + 1}
      </div>

      {/* Content */}
      <Card className="cursor-pointer hover:border-primary/50" onClick={() => setExpanded(!expanded)}>
        <CardContent className="p-3">
          <div className="flex items-start justify-between gap-2">
            <div className="flex-1 min-w-0">
              <div className="flex items-center gap-2 mb-1">
                <span className="text-xs text-muted-foreground">{time}</span>
                <span
                  className={`text-xs font-medium ${
                    directionColors[step.direction] || "text-gray-500"
                  }`}
                >
                  [{step.direction}]
                </span>
              </div>
              <p className="text-sm line-clamp-2">{step.thought}</p>
            </div>
            <div className="flex items-center gap-2 text-xs text-muted-foreground">
              {step.tokens_in + step.tokens_out} tokens
              {expanded ? (
                <ChevronUp className="h-4 w-4" />
              ) : (
                <ChevronDown className="h-4 w-4" />
              )}
            </div>
          </div>

          {expanded && (
            <div className="mt-3 pt-3 border-t space-y-2">
              {step.tool_name && (
                <div>
                  <span className="text-xs text-muted-foreground">工具: </span>
                  <code className="text-xs bg-muted px-1 rounded">{step.tool_name}</code>
                </div>
              )}
              {step.result && (
                <div>
                  <span className="text-xs text-muted-foreground">结果:</span>
                  <p className="text-xs mt-1 bg-muted/50 p-2 rounded max-h-40 overflow-y-auto">
                    {step.result}
                  </p>
                </div>
              )}
              <div className="flex gap-4 text-xs text-muted-foreground">
                <span>输入: {step.tokens_in} tokens</span>
                <span>输出: {step.tokens_out} tokens</span>
                <span>耗时: {step.duration_ms}ms</span>
              </div>
            </div>
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

  useEffect(() => {
    const fetchTaskDetails = async () => {
      try {
        const response = await fetch(
          `http://localhost:8080/api/v1/agent/task/${taskId}`
        );
        if (!response.ok) {
          throw new Error("Failed to fetch task details");
        }
        const data = await response.json();
        setTask(data);
      } catch (err) {
        setError(err instanceof Error ? err.message : "加载失败");
      } finally {
        setLoading(false);
      }
    };

    fetchTaskDetails();
  }, [taskId]);

  if (loading) {
    return (
      <div className="min-h-screen flex items-center justify-center">
        <Loader2 className="h-8 w-8 animate-spin text-primary" />
      </div>
    );
  }

  if (error || !task) {
    return (
      <div className="min-h-screen flex items-center justify-center">
        <Card className="max-w-md">
          <CardContent className="p-6 text-center">
            <p className="text-destructive">{error || "任务不存在"}</p>
            <Button className="mt-4" onClick={() => router.push("/")}>
              返回首页
            </Button>
          </CardContent>
        </Card>
      </div>
    );
  }

  return (
    <div className="min-h-screen">
      {/* Header */}
      <header className="sticky top-0 z-50 w-full border-b bg-background/95 backdrop-blur">
        <div className="container flex h-16 items-center justify-between">
          <div className="flex items-center gap-4">
            <Button variant="ghost" size="icon" onClick={() => router.back()}>
              <ArrowLeft className="h-5 w-5" />
            </Button>
            <div className="flex items-center gap-2">
              <Globe className="h-6 w-6 text-primary" />
              <span className="text-xl font-bold">任务详情</span>
            </div>
          </div>
          <Link href="/" className="flex items-center gap-2">
            <span className="text-xl font-bold">UTA Travel</span>
          </Link>
        </div>
      </header>

      <main className="container py-8">
        {/* Stats Cards */}
        <div className="grid grid-cols-2 md:grid-cols-4 gap-4 mb-8">
          <Card>
            <CardContent className="p-4 text-center">
              <Clock className="h-5 w-5 mx-auto mb-2 text-muted-foreground" />
              <p className="text-2xl font-bold">
                {task.duration_seconds > 0
                  ? `${task.duration_seconds.toFixed(1)}s`
                  : task.exploration_log.length > 0
                    ? `${((new Date(task.exploration_log[task.exploration_log.length - 1].timestamp).getTime() -
                            new Date(task.exploration_log[0].timestamp).getTime()) / 1000).toFixed(1)}s`
                    : "-"}
              </p>
              <p className="text-xs text-muted-foreground">总耗时</p>
            </CardContent>
          </Card>
          <Card>
            <CardContent className="p-4 text-center">
              <Zap className="h-5 w-5 mx-auto mb-2 text-muted-foreground" />
              <p className="text-2xl font-bold">{task.total_tokens}</p>
              <p className="text-xs text-muted-foreground">Token 消耗</p>
            </CardContent>
          </Card>
          <Card>
            <CardContent className="p-4 text-center">
              <MapPin className="h-5 w-5 mx-auto mb-2 text-muted-foreground" />
              <p className="text-2xl font-bold">{task.exploration_log.length}</p>
              <p className="text-xs text-muted-foreground">探索步骤</p>
            </CardContent>
          </Card>
          <Card>
            <CardContent className="p-4 text-center">
              <Globe className="h-5 w-5 mx-auto mb-2 text-muted-foreground" />
              <p className="text-2xl font-bold">{task.metadata.subagent_count}</p>
              <p className="text-xs text-muted-foreground">协作 Agent</p>
            </CardContent>
          </Card>
        </div>

        <div className="grid md:grid-cols-2 gap-8">
          {/* Radar Chart */}
          <Card>
            <CardHeader>
              <CardTitle className="text-lg">探索雷达图</CardTitle>
              <p className="text-sm text-muted-foreground">
                展示 Agent 在各个方向的探索深度
              </p>
            </CardHeader>
            <CardContent className="flex justify-center">
              {task.radar_data?.directions ? (
                <RadarChart directions={task.radar_data.directions} />
              ) : (
                <p className="text-muted-foreground">暂无雷达数据</p>
              )}
            </CardContent>
          </Card>

          {/* Exploration Timeline */}
          <Card>
            <CardHeader>
              <CardTitle className="text-lg">探索时间线</CardTitle>
              <p className="text-sm text-muted-foreground">
                记录 Agent 的思考和行动过程
              </p>
            </CardHeader>
            <CardContent>
              <div className="max-h-[400px] overflow-y-auto">
                {task.exploration_log.length > 0 ? (
                  task.exploration_log.map((step, index) => (
                    <TimelineItem key={index} step={step} index={index} />
                  ))
                ) : (
                  <p className="text-muted-foreground text-center py-8">
                    暂无探索记录
                  </p>
                )}
              </div>
            </CardContent>
          </Card>
        </div>
      </main>
    </div>
  );
}
