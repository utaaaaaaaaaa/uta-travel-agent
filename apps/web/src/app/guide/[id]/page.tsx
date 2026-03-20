"use client";

import { useState, useRef, useEffect } from "react";
import { useParams, useRouter } from "next/navigation";
import Link from "next/link";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { ScrollArea } from "@/components/ui/scroll-area";
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
} from "lucide-react";
import { api, Agent } from "@/lib/api/client";

interface Message {
  id: string;
  role: "user" | "assistant";
  content: string;
  timestamp: number;
  isStreaming?: boolean;
}

interface Attraction {
  id: string;
  name: string;
  category: string;
  description: string;
  icon: React.ReactNode;
}

// Mock attractions data - will be loaded from agent's knowledge base
const getMockAttractions = (destination: string): Attraction[] => {
  const attractions: Record<string, Attraction[]> = {
    "京都": [
      { id: "1", name: "金阁寺", category: "景点", description: "世界文化遗产，黄金色的寺院倒映在镜湖池中", icon: <Landmark className="h-4 w-4" /> },
      { id: "2", name: "清水寺", category: "景点", description: "著名的悬空舞台和音羽瀑布", icon: <Landmark className="h-4 w-4" /> },
      { id: "3", name: "伏见稻荷大社", category: "景点", description: "千本�的居，壮观的朱红色隧道", icon: <Landmark className="h-4 w-4" /> },
      { id: "4", name: "岚山竹林", category: "景点", description: "静谧的竹林小径", icon: <MapPin className="h-4 w-4" /> },
      { id: "5", name: "抹茶甜点", category: "美食", description: "宇治抹茶冰淇淋、抹茶蛋糕", icon: <Utensils className="h-4 w-4" /> },
      { id: "6", name: "京料理", category: "美食", description: "传统的怀石料理体验", icon: <Utensils className="h-4 w-4" /> },
      { id: "7", name: "锦市场", category: "购物", description: "京都的厨房，400年老街", icon: <ShoppingBag className="h-4 w-4" /> },
    ],
    "东京": [
      { id: "1", name: "东京塔", category: "景点", description: "东京地标，可俯瞰城市全景", icon: <Building className="h-4 w-4" /> },
      { id: "2", name: "浅草寺", category: "景点", description: "东京最古老的寺院", icon: <Landmark className="h-4 w-4" /> },
      { id: "3", name: "涩谷十字路口", category: "景点", description: "世界最繁忙的十字路口", icon: <MapPin className="h-4 w-4" /> },
      { id: "4", name: "明治神宫", category: "景点", description: "闹市中的宁静神社", icon: <Landmark className="h-4 w-4" /> },
      { id: "5", name: "寿司", category: "美食", description: "筑地/丰洲新鲜寿司", icon: <Utensils className="h-4 w-4" /> },
      { id: "6", name: "拉面", category: "美食", description: "一�的、阿夫利等名店", icon: <Utensils className="h-4 w-4" /> },
      { id: "7", name: "秋叶原", category: "购物", description: "电器街、动漫圣地", icon: <ShoppingBag className="h-4 w-4" /> },
    ],
  };

  return attractions[destination] || [
    { id: "1", name: `${destination}市中心`, category: "景点", description: "探索城市中心", icon: <MapPin className="h-4 w-4" /> },
    { id: "2", name: "当地美食", category: "美食", description: "品尝特色料理", icon: <Utensils className="h-4 w-4" /> },
  ];
};

const categoryColors: Record<string, string> = {
  "景点": "bg-blue-500/10 text-blue-500",
  "美食": "bg-orange-500/10 text-orange-500",
  "购物": "bg-pink-500/10 text-pink-500",
  "交通": "bg-green-500/10 text-green-500",
  "住宿": "bg-purple-500/10 text-purple-500",
};

export default function GuidePage() {
  const params = useParams();
  const router = useRouter();
  const agentId = params.id as string;

  const [messages, setMessages] = useState<Message[]>([]);
  const [input, setInput] = useState("");
  const [isLoading, setIsLoading] = useState(false);
  const [agentInfo, setAgentInfo] = useState<Agent | null>(null);
  const [selectedCategory, setSelectedCategory] = useState<string | null>(null);

  const messagesEndRef = useRef<HTMLDivElement>(null);
  const inputRef = useRef<HTMLInputElement>(null);

  // Get attractions based on destination
  const attractions = getMockAttractions(agentInfo?.destination || "");
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

  // Fetch agent info
  useEffect(() => {
    const fetchAgent = async () => {
      try {
        const agent = await api.getAgent(agentId);
        setAgentInfo(agent);

        // Add welcome message
        setMessages([
          {
            id: "welcome",
            role: "assistant",
            content: `你好！我是${agent.destination}导游助手。\n\n我可以为你介绍当地景点、推荐美食、规划行程等。\n\n左侧是${agent.destination}的热门推荐，点击可快速了解详情。有什么想问的吗？`,
            timestamp: Date.now(),
          },
        ]);
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
    const eventSource = new EventSource(
      `${process.env.NEXT_PUBLIC_API_URL || "http://localhost:8080"}/api/v1/agents/${agentId}/chat/stream?message=${encodeURIComponent(message)}`
    );

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

      eventSource.addEventListener("complete", () => {
        setMessages((prev) =>
          prev.map((m) =>
            m.id === streamingId ? { ...m, isStreaming: false } : m
          )
        );
        eventSource.close();
        resolve();
      });

      eventSource.onerror = (error) => {
        eventSource.close();
        reject(error);
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
    <div className="min-h-screen flex flex-col md:flex-row">
      {/* Left Sidebar - Attractions */}
      <aside className="w-full md:w-80 border-r bg-muted/30 flex-shrink-0">
        <div className="sticky top-0 p-4 border-b bg-background/95 backdrop-blur">
          <h2 className="font-semibold flex items-center gap-2">
            <MapPin className="h-4 w-4 text-primary" />
            {agentInfo?.destination || "目的地"}推荐
          </h2>
          <div className="flex gap-2 mt-2 flex-wrap">
            <Button
              variant={selectedCategory === null ? "default" : "outline"}
              size="sm"
              onClick={() => setSelectedCategory(null)}
            >
              全部
            </Button>
            {categories.map((cat) => (
              <Button
                key={cat}
                variant={selectedCategory === cat ? "default" : "outline"}
                size="sm"
                onClick={() => setSelectedCategory(cat)}
              >
                {cat}
              </Button>
            ))}
          </div>
        </div>

        <ScrollArea className="h-[calc(100vh-180px)]">
          <div className="p-4 space-y-3">
            {filteredAttractions.map((attraction) => (
              <Card
                key={attraction.id}
                className="cursor-pointer hover:bg-muted/50 transition-colors"
                onClick={() => handleAttractionClick(attraction)}
              >
                <CardContent className="p-3">
                  <div className="flex items-start gap-3">
                    <div className="flex-shrink-0 w-8 h-8 rounded-full bg-primary/10 flex items-center justify-center">
                      {attraction.icon}
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
            ))}
          </div>
        </ScrollArea>
      </aside>

      {/* Main Chat Area */}
      <div className="flex-1 flex flex-col min-w-0">
        {/* Header */}
        <header className="sticky top-0 z-50 w-full border-b bg-background/95 backdrop-blur">
          <div className="container flex h-16 items-center justify-between px-4">
            <div className="flex items-center gap-4">
              <Button variant="ghost" size="icon" onClick={() => router.push("/")}>
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
            <Link href="/" className="flex items-center gap-2">
              <span className="text-xl font-bold hidden sm:inline">UTA Travel</span>
            </Link>
          </div>
        </header>

        {/* Messages Area */}
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
                <Card
                  className={`max-w-[80%] ${
                    message.role === "user"
                      ? "bg-primary text-primary-foreground"
                      : "bg-muted"
                  }`}
                >
                  <CardContent className="p-3">
                    <p className="text-sm whitespace-pre-wrap">
                      {message.content}
                      {message.isStreaming && (
                        <span className="inline-block w-1 h-4 ml-1 bg-current animate-pulse"></span>
                      )}
                    </p>
                  </CardContent>
                </Card>
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

        {/* Input Area */}
        <footer className="sticky bottom-0 border-t bg-background p-4">
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
