"use client";

import { useState, useRef, useEffect } from "react";
import { useParams, useRouter } from "next/navigation";
import Link from "next/link";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Card, CardContent } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
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
} from "lucide-react";
import { api, Agent, Attraction, SourceInfo } from "@/lib/api/client";

// Simple markdown content renderer
function MarkdownContent({ content }: { content: string }) {
  if (!content) return null;

  const renderContent = content
    .replace(/\*\*(.*?)\*\*/g, "<strong>$1</strong>")
    .replace(/\*(.*?)\*/g, "<em>$1</em>")
    .replace(/```([\s\S]*?)```/g, "<pre class=\"bg-muted p-2 rounded my-2 overflow-x-auto text-sm\"><code>$1</code></pre>")
    .replace(/`(.*?)`/g, "<code class=\"bg-muted px-1 rounded text-sm\">$1</code>")
    .replace(/^### (.*$)/gm, "<h3 class=\"font-bold text-base my-1\">$1</h3>")
    .replace(/^## (.*$)/gm, "<h2 class=\"font-bold text-lg my-2\">$1</h2>")
    .replace(/^# (.*$)/gm, "<h1 class=\"font-bold text-xl my-2\">$1</h1>")
    .replace(/^\- (.*$)/gm, "<li class=\"ml-4\">$1</li>")
    .replace(/^\d+\. (.*$)/gm, "<li class=\"ml-4 list-decimal\">$1</li>")
    .replace(/\n/g, "<br/>");

  return <div dangerouslySetInnerHTML={{ __html: renderContent }} />;
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
  "景点": "bg-blue-500/10 text-blue-500",
  "美食": "bg-orange-500/10 text-orange-500",
  "购物": "bg-pink-500/10 text-pink-500",
  "交通": "bg-green-500/10 text-green-500",
  "住宿": "bg-purple-500/10 text-purple-500",
};

// Source link component
function SourceLink({ source }: { source: SourceInfo }) {
  if (source.url) {
    return (
      <a
        href={source.url}
        target="_blank"
        rel="noopener noreferrer"
        className="inline-flex items-center gap-1 text-xs text-primary hover:underline bg-primary/5 px-2 py-0.5 rounded"
      >
        <ExternalLink className="h-3 w-3" />
        {source.title || source.url}
      </a>
    );
  }
  return (
    <span className="text-xs text-muted-foreground bg-muted px-2 py-0.5 rounded">
      {source.title}
    </span>
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

  const messagesEndRef = useRef<HTMLDivElement>(null);
  const inputRef = useRef<HTMLInputElement>(null);

  const filteredAttractions = selectedCategory
    ? attractions.filter((a) => a.category === selectedCategory)
    : attractions;

  const categories = [...new Set(attractions.map((a) => a.category))];

  // Scroll to bottom when messages change
  useEffect(() => {
    messagesEndRef.current?.scrollIntoView({ behavior: "smooth" });
  }, [messages]);

  // Focus input on load
  useEffect(() => {
    inputRef.current?.focus();
  }, []);

  // Fetch agent info and attractions
  useEffect(() => {
    const fetchAgent = async () => {
      try {
        const agent = await api.getAgent(agentId);
        setAgentInfo(agent);

        // Try to get task ID from agent, or fetch by agent ID
        if (agent.task_id) {
          setTaskId(agent.task_id);
        } else {
          // Try to fetch task by agent ID
          try {
            const task = await api.getTaskByAgent(agentId);
            if (task?.id) {
              setTaskId(task.id);
            }
          } catch (e) {
            // Task not found, that's okay
            console.log("No task found for this agent");
          }
        }

        // Add welcome message
        setMessages([
          {
            id: "welcome",
            role: "assistant",
            content: `你好！我是${agent.destination}导游助手。\n\n我可以为你介绍当地景点、推荐美食、规划行程等。\n\n左侧是${agent.destination}的热门推荐，点击可快速了解详情。有什么想问的吗？`,
            timestamp: Date.now(),
          },
        ]);

        // Load attractions from API
        try {
          const attractionsData = await api.getAttractions(agentId);
          if (attractionsData.attractions && attractionsData.attractions.length > 0) {
            setAttractions(attractionsData.attractions);
          } else {
            // Use fallback if no attractions
            setAttractions(getFallbackAttractions(agent.destination));
          }
        } catch (e) {
          console.log("Failed to load attractions from API, using fallback");
          setAttractions(getFallbackAttractions(agent.destination));
        }
        setLoadingAttractions(false);
      } catch (error) {
        console.error("Failed to fetch agent:", error);
        // Use default agent info
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

        // Add welcome message for fallback
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

    const userMessage: Message = {
      id: `user-${Date.now()}`,
      role: "user",
      content: input.trim(),
      timestamp: Date.now(),
    };

    setMessages((prev) => [...prev, userMessage]);
    setInput("");
    setIsLoading(true);

    // Add streaming placeholder
    const streamingId = `streaming-${Date.now()}`;
    setMessages((prev) => [
      ...prev,
      { id: streamingId, role: "assistant", content: "", timestamp: Date.now(), isStreaming: true },
    ]);

    try {
      // Try streaming first
      await streamChat(input.trim(), streamingId);
    } catch (error) {
      console.error("Chat error:", error);
      // Remove streaming message and show error
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

  const streamChat = async (message: string, streamingId: string) => {
    const apiUrl = process.env.NEXT_PUBLIC_API_URL || "http://localhost:8080";
    const eventSource = new EventSource(
      `${apiUrl}/api/v1/agents/${agentId}/chat/stream?message=${encodeURIComponent(message)}`
    );

    let completed = false;
    let sources: SourceInfo[] = [];
    let searchType: "rag" | "realtime" | undefined;

    return new Promise<void>((resolve, reject) => {
      eventSource.addEventListener("chunk", (event) => {
        const data = JSON.parse(event.data);
        setMessages((prev) =>
          prev.map((m) =>
            m.id === streamingId
              ? { ...m, content: m.content + data.content }
              : m
          )
        );
      });

      eventSource.addEventListener("sources", (event) => {
        try {
          const data = JSON.parse(event.data);
          if (data.sources) {
            sources = data.sources;
          }
          if (data.search_type) {
            searchType = data.search_type;
          }
        } catch (e) {
          console.error("Failed to parse sources event:", e);
        }
      });

      eventSource.addEventListener("complete", () => {
        completed = true;
        setMessages((prev) =>
          prev.map((m) =>
            m.id === streamingId
              ? { ...m, isStreaming: false, sources, searchType }
              : m
          )
        );
        eventSource.close();
        resolve();
      });

      eventSource.onerror = () => {
        eventSource.close();
        if (!completed) {
          reject(new Error("Stream connection failed"));
        }
      };
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
    <div className="flex h-screen overflow-hidden">
      {/* Left Sidebar - Attractions (Fixed) */}
      <aside className="hidden md:flex w-80 flex-col border-r bg-muted/30 flex-shrink-0">
        {/* Header - Fixed */}
        <div className="flex-shrink-0 p-4 border-b bg-background/95 backdrop-blur">
          <h2 className="font-semibold flex items-center gap-2">
            <MapPin className="h-4 w-4 text-primary" />
            {agentInfo?.destination || "目的地"}推荐
          </h2>
          {/* Category Buttons */}
          <div className="flex gap-2 mt-2 flex-wrap">
            <Button
              key="all"
              variant={selectedCategory === null ? "default" : "outline"}
              size="sm"
              onClick={() => setSelectedCategory(null)}
            >
              全部
            </Button>
            {categories.map((cat, idx) => (
              <Button
                key={`cat-${idx}-${cat}`}
                variant={selectedCategory === cat ? "default" : "outline"}
                size="sm"
                onClick={() => setSelectedCategory(cat)}
              >
                {cat}
              </Button>
            ))}
          </div>
        </div>

        {/* Attractions List - Scrollable */}
        <div className="flex-1 overflow-y-auto p-4 space-y-3">
          {loadingAttractions ? (
            <div className="flex justify-center py-8">
              <Loader2 className="h-6 w-6 animate-spin text-muted-foreground" />
            </div>
          ) : filteredAttractions.length > 0 ? (
            filteredAttractions.map((attraction, idx) => (
              <Card
                key={`attraction-${idx}`}
                className="cursor-pointer hover:bg-muted/50 transition-colors"
                onClick={() => handleAttractionClick(attraction)}
              >
                <CardContent className="p-3">
                  <div className="flex items-start gap-3">
                    <div className="flex-shrink-0 w-8 h-8 rounded-full bg-primary/10 flex items-center justify-center">
                      {getCategoryIcon(attraction.category)}
                    </div>
                    <div className="flex-1 min-w-0">
                      <div className="flex items-center gap-2">
                        <span className="font-medium truncate">{attraction.name}</span>
                        <Badge variant="secondary" className={`text-xs ${categoryColors[attraction.category] || ""}`}>
                          {attraction.category}
                        </Badge>
                      </div>
                      <p className="text-xs text-muted-foreground mt-1 line-clamp-2">
                        {attraction.description}
                      </p>
                    </div>
                  </div>
                </CardContent>
              </Card>
            ))
          ) : (
            <p className="text-sm text-muted-foreground text-center py-8">
              暂无推荐
            </p>
          )}
        </div>
      </aside>

      {/* Main Chat Area */}
      <div className="flex-1 flex flex-col min-w-0 overflow-hidden">
        {/* Header */}
        <header className="flex-shrink-0 h-16 border-b bg-background/95 backdrop-blur px-4 flex items-center justify-between">
          <div className="flex items-center gap-4">
            <Button variant="ghost" size="icon" onClick={() => router.back()}>
              <ArrowLeft className="h-5 w-5" />
            </Button>
            <div className="flex items-center gap-2">
              <Globe className="h-6 w-6 text-primary" />
              <div>
                <h1 className="text-lg font-semibold">
                  {agentInfo?.name || "导游助手"}
                </h1>
                <p className="text-xs text-muted-foreground">
                  {agentInfo?.status === "ready" ? (
                    <span className="flex items-center gap-1">
                      <span className="w-2 h-2 bg-green-500 rounded-full"></span>
                      在线
                    </span>
                  ) : (
                    "连接中..."
                  )}
                </p>
              </div>
            </div>
          </div>
          <div className="flex items-center gap-2">
            {taskId && (
              <Link href={`/tasks/${taskId}`}>
                <Button variant="outline" size="sm" className="gap-1">
                  <FileText className="h-4 w-4" />
                  <span className="hidden sm:inline">查看任务详情</span>
                </Button>
              </Link>
            )}
            <Link href="/" className="flex items-center gap-2">
              <span className="text-xl font-bold hidden sm:inline">UTA Travel</span>
            </Link>
          </div>
        </header>

        {/* Messages Area - Scrollable */}
        <main className="flex-1 overflow-y-auto p-4">
          <div className="max-w-3xl mx-auto space-y-4">
            {messages.map((message) => (
              <div
                key={message.id}
                className={`flex gap-3 ${
                  message.role === "user" ? "justify-end" : "justify-start"
                }`}
              >
                {message.role === "assistant" && (
                  <div className="flex-shrink-0 w-8 h-8 rounded-full bg-primary/10 flex items-center justify-center">
                    <Bot className="h-5 w-5 text-primary" />
                  </div>
                )}
                <div className={`max-w-[80%] ${message.role === "user" ? "" : "flex flex-col gap-2"}`}>
                  <Card
                    className={`${
                      message.role === "user"
                        ? "bg-primary text-primary-foreground"
                        : "bg-muted"
                    }`}
                  >
                    <CardContent className="p-3">
                      {message.role === "user" ? (
                        <p className="text-sm whitespace-pre-wrap">
                          {message.content}
                        </p>
                      ) : (
                        <div className="text-sm">
                          <MarkdownContent content={message.content} />
                          {message.isStreaming && (
                            <span className="inline-block w-1 h-4 ml-1 bg-current animate-pulse"></span>
                          )}
                        </div>
                      )}
                    </CardContent>
                  </Card>

                  {/* Show sources if available */}
                  {message.role === "assistant" && message.sources && message.sources.length > 0 && !message.isStreaming && (
                    <div className="mt-1">
                      <div className="flex items-center gap-1 text-xs text-muted-foreground mb-1">
                        {message.searchType === "realtime" ? (
                          <Badge variant="outline" className="text-xs bg-green-500/10 text-green-600 border-green-200">
                            实时搜索
                          </Badge>
                        ) : (
                          <Badge variant="outline" className="text-xs bg-blue-500/10 text-blue-600 border-blue-200">
                            知识库
                          </Badge>
                        )}
                        <span>信息来源:</span>
                      </div>
                      <div className="flex flex-wrap gap-1">
                        {message.sources.map((source, idx) => (
                          <SourceLink key={`${message.id}-source-${idx}`} source={source} />
                        ))}
                      </div>
                    </div>
                  )}
                </div>
                {message.role === "user" && (
                  <div className="flex-shrink-0 w-8 h-8 rounded-full bg-muted flex items-center justify-center">
                    <User className="h-5 w-5" />
                  </div>
                )}
              </div>
            ))}

            {isLoading && messages[messages.length - 1]?.isStreaming !== true && (
              <div className="flex gap-3 justify-start">
                <div className="flex-shrink-0 w-8 h-8 rounded-full bg-primary/10 flex items-center justify-center">
                  <Bot className="h-5 w-5 text-primary" />
                </div>
                <Card className="bg-muted">
                  <CardContent className="p-3">
                    <Loader2 className="h-5 w-5 animate-spin text-muted-foreground" />
                  </CardContent>
                </Card>
              </div>
            )}

            <div ref={messagesEndRef} />
          </div>
        </main>

        {/* Input Area - Fixed */}
        <footer className="flex-shrink-0 border-t bg-background p-4">
          <div className="max-w-3xl mx-auto">
            {/* Quick Action Buttons */}
            <div className="flex gap-2 mb-3 overflow-x-auto pb-2">
              <Button
                variant="outline"
                size="sm"
                className="flex-shrink-0"
                onClick={() => setInput("推荐必去的景点")}
              >
                <MapPin className="h-4 w-4 mr-1" />
                必去景点
              </Button>
              <Button
                variant="outline"
                size="sm"
                className="flex-shrink-0"
                onClick={() => setInput("当地美食推荐")}
              >
                <Utensils className="h-4 w-4 mr-1" />
                美食推荐
              </Button>
              <Button
                variant="outline"
                size="sm"
                className="flex-shrink-0"
                onClick={() => setInput("交通指南")}
              >
                <Train className="h-4 w-4 mr-1" />
                交通指南
              </Button>
              <Button
                variant="outline"
                size="sm"
                className="flex-shrink-0"
                onClick={() => setInput("购物攻略")}
              >
                <ShoppingBag className="h-4 w-4 mr-1" />
                购物攻略
              </Button>
            </div>

            {/* Main Input */}
            <form
              onSubmit={(e) => {
                e.preventDefault();
                handleSend();
              }}
              className="flex gap-2"
            >
              <div className="flex gap-1">
                <Button
                  type="button"
                  variant="ghost"
                  size="icon"
                  className="hidden sm:flex"
                  title="拍照识别 (开发中)"
                  disabled
                >
                  <Camera className="h-4 w-4" />
                </Button>
                <Button
                  type="button"
                  variant="ghost"
                  size="icon"
                  className="hidden sm:flex"
                  title="语音输入 (开发中)"
                  disabled
                >
                  <Mic className="h-4 w-4" />
                </Button>
              </div>
              <Input
                ref={inputRef}
                value={input}
                onChange={(e) => setInput(e.target.value)}
                onKeyDown={handleKeyDown}
                placeholder="输入你的问题..."
                disabled={isLoading}
                className="flex-1"
              />
              <Button type="submit" disabled={!input.trim() || isLoading}>
                <Send className="h-4 w-4" />
              </Button>
            </form>
          </div>
        </footer>
      </div>
    </div>
  );
}
