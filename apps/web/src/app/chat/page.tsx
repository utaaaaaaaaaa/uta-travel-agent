"use client";

import { useState, useRef, useEffect } from "react";
import Link from "next/link";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Globe, Send, Loader2, Trash2 } from "lucide-react";
import { useChat } from "@/hooks/useAgents";

interface Message {
  role: "user" | "assistant";
  content: string;
  timestamp: number;
}

export default function ChatPage() {
  const [messages, setMessages] = useState<Message[]>([]);
  const [input, setInput] = useState("");
  const messagesEndRef = useRef<HTMLDivElement>(null);
  const { loading, error, chatStream, clearSession } = useChat();

  const scrollToBottom = () => {
    messagesEndRef.current?.scrollIntoView({ behavior: "smooth" });
  };

  useEffect(() => {
    scrollToBottom();
  }, [messages]);

  const handleSend = async () => {
    if (!input.trim() || loading) return;

    const userMessage: Message = {
      role: "user",
      content: input.trim(),
      timestamp: Date.now(),
    };

    setMessages((prev) => [...prev, userMessage]);
    setInput("");

    // Create empty assistant message for streaming
    const assistantIdx = messages.length + 1;
    const assistantMessage: Message = {
      role: "assistant",
      content: "",
      timestamp: Date.now(),
    };
    setMessages((prev) => [...prev, assistantMessage]);

    try {
      // Use streaming
      for await (const chunk of chatStream(userMessage.content)) {
        setMessages((prev) => {
          const updated = [...prev];
          if (updated[assistantIdx]) {
            updated[assistantIdx] = {
              ...updated[assistantIdx],
              content: updated[assistantIdx].content + chunk,
            };
          }
          return updated;
        });
      }
    } catch (e) {
      setMessages((prev) => {
        const updated = [...prev];
        if (updated[assistantIdx]) {
          updated[assistantIdx] = {
            ...updated[assistantIdx],
            content: `Error: ${e instanceof Error ? e.message : "Something went wrong"}`,
          };
        }
        return updated;
      });
    }
  };

  const handleClear = () => {
    setMessages([]);
    clearSession();
  };

  const handleKeyDown = (e: React.KeyboardEvent) => {
    if (e.key === "Enter" && !e.shiftKey) {
      e.preventDefault();
      handleSend();
    }
  };

  return (
    <div className="min-h-screen flex flex-col">
      {/* Header */}
      <header className="sticky top-0 z-50 w-full border-b bg-background/95 backdrop-blur">
        <div className="container flex h-16 items-center justify-between">
          <Link href="/" className="flex items-center gap-2">
            <Globe className="h-6 w-6 text-primary" />
            <span className="text-xl font-bold">UTA Travel</span>
          </Link>
          <div className="flex items-center gap-4">
            <Link href="/destinations">
              <Button variant="ghost">我的目的地</Button>
            </Link>
            <Link href="/destinations/create">
              <Button variant="outline">创建 Agent</Button>
            </Link>
          </div>
        </div>
      </header>

      {/* Main Content */}
      <main className="flex-1 container max-w-4xl py-6 flex flex-col">
        {/* Chat Header */}
        <div className="flex items-center justify-between mb-6">
          <div>
            <h1 className="text-2xl font-bold">AI 旅游助手</h1>
            <p className="text-muted-foreground">问我任何关于旅行的问题</p>
          </div>
          {messages.length > 0 && (
            <Button variant="outline" size="sm" onClick={handleClear}>
              <Trash2 className="h-4 w-4 mr-2" />
              清空对话
            </Button>
          )}
        </div>

        {/* Messages */}
        <div className="flex-1 overflow-y-auto mb-4 space-y-4">
          {messages.length === 0 ? (
            <div className="text-center py-12">
              <div className="h-16 w-16 rounded-full bg-primary/10 flex items-center justify-center mx-auto mb-4">
                <Globe className="h-8 w-8 text-primary" />
              </div>
              <h2 className="text-xl font-semibold mb-2">你好！我是 UTA Travel 智能助手</h2>
              <p className="text-muted-foreground mb-6">
                我可以帮助你规划旅行、了解目的地信息、创建专属导游 Agent
              </p>
              <div className="grid grid-cols-1 md:grid-cols-2 gap-4 max-w-2xl mx-auto">
                <Card className="cursor-pointer hover:bg-muted/50 transition-colors" onClick={() => setInput("介绍一下京都的旅游景点")}>
                  <CardContent className="p-4">
                    <p className="font-medium">🏯 京都旅游</p>
                    <p className="text-sm text-muted-foreground">介绍京都的热门景点</p>
                  </CardContent>
                </Card>
                <Link href="/destinations/create">
                  <Card className="cursor-pointer hover:bg-muted/50 transition-colors h-full">
                    <CardContent className="p-4">
                      <p className="font-medium">🗼 创建 Agent</p>
                      <p className="text-sm text-muted-foreground">创建专属目的地 Agent</p>
                    </CardContent>
                  </Card>
                </Link>
                <Card className="cursor-pointer hover:bg-muted/50 transition-colors" onClick={() => setInput("推荐一个3天的京都行程")}>
                  <CardContent className="p-4">
                    <p className="font-medium">📅 行程规划</p>
                    <p className="text-sm text-muted-foreground">规划个性化旅行行程</p>
                  </CardContent>
                </Card>
                <Card className="cursor-pointer hover:bg-muted/50 transition-colors" onClick={() => setInput("日本旅游需要注意什么？")}>
                  <CardContent className="p-4">
                    <p className="font-medium">💡 旅行建议</p>
                    <p className="text-sm text-muted-foreground">获取实用的旅行建议</p>
                  </CardContent>
                </Card>
              </div>
            </div>
          ) : (
            messages.map((msg, idx) => (
              <div
                key={idx}
                className={`flex ${msg.role === "user" ? "justify-end" : "justify-start"}`}
              >
                <div
                  className={`max-w-[80%] rounded-lg px-4 py-2 ${
                    msg.role === "user"
                      ? "bg-primary text-primary-foreground"
                      : "bg-muted"
                  }`}
                >
                  <p className="whitespace-pre-wrap">{msg.content}</p>
                  <p className="text-xs opacity-70 mt-1">
                    {new Date(msg.timestamp).toLocaleTimeString()}
                  </p>
                </div>
              </div>
            ))
          )}

          {loading && (
            <div className="flex justify-start">
              <div className="bg-muted rounded-lg px-4 py-2">
                <Loader2 className="h-4 w-4 animate-spin" />
              </div>
            </div>
          )}

          {error && (
            <div className="flex justify-center">
              <div className="bg-destructive/10 text-destructive rounded-lg px-4 py-2">
                {error}
              </div>
            </div>
          )}

          <div ref={messagesEndRef} />
        </div>

        {/* Input */}
        <div className="border-t pt-4">
          <div className="flex gap-2">
            <Input
              value={input}
              onChange={(e) => setInput(e.target.value)}
              onKeyDown={handleKeyDown}
              placeholder="输入你的问题..."
              disabled={loading}
              className="flex-1"
            />
            <Button onClick={handleSend} disabled={!input.trim() || loading}>
              {loading ? (
                <Loader2 className="h-4 w-4 animate-spin" />
              ) : (
                <Send className="h-4 w-4" />
              )}
            </Button>
          </div>
        </div>
      </main>
    </div>
  );
}