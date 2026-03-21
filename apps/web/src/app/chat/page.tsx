"use client";

import { useState, useRef, useEffect } from "react";
import Link from "next/link";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Card, CardContent } from "@/components/ui/card";
import { Globe, Send, Loader2, Trash2 } from "lucide-react";

interface Message {
  id: string;
  role: "user" | "assistant";
  content: string;
  timestamp: number;
}

// Simple markdown content renderer
function MarkdownContent({ content }: { content: string }) {
  if (!content) return null;

  const renderContent = content
    .replace(/\*\*(.*?)\*\*/g, "<strong>$1</strong>")
    .replace(/\*(.*?)\*/g, "<em>$1</em>")
    .replace(/```([\s\S]*?)```/g, '<pre class="bg-muted p-2 rounded my-2 overflow-x-auto"><code>$1</code></pre>')
    .replace(/`(.*?)`/g, '<code class="bg-muted px-1 rounded">$1</code>')
    .replace(/^### (.*$)/gm, '<h3 class="font-bold text-base my-1">$1</h3>')
    .replace(/^## (.*$)/gm, '<h2 class="font-bold text-lg my-2">$1</h2>')
    .replace(/^# (.*$)/gm, '<h1 class="font-bold text-xl my-2">$1</h1>')
    .replace(/^\- (.*$)/gm, '<li class="ml-4">$1</li>')
    .replace(/^\d+\. (.*$)/gm, '<li class="ml-4 list-decimal">$1</li>')
    .replace(/\n/g, "<br/>");

  return <div dangerouslySetInnerHTML={{ __html: renderContent }} />;
}

export default function ChatPage() {
  const [messages, setMessages] = useState<Message[]>([]);
  const [input, setInput] = useState("");
  const [isLoading, setIsLoading] = useState(false);
  const messagesEndRef = useRef<HTMLDivElement>(null);

  const scrollToBottom = () => {
    messagesEndRef.current?.scrollIntoView({ behavior: "smooth" });
  };

  useEffect(() => {
    scrollToBottom();
  }, [messages]);

  const handleSend = async () => {
    if (!input.trim() || isLoading) return;

    const userMessage: Message = {
      id: `user-${Date.now()}`,
      role: "user",
      content: input.trim(),
      timestamp: Date.now(),
    };

    setMessages((prev) => [...prev, userMessage]);
    const messageText = input.trim();
    setInput("");
    setIsLoading(true);

    const streamingId = `streaming-${Date.now()}`;
    setMessages((prev) => [
      ...prev,
      { id: streamingId, role: "assistant", content: "", timestamp: Date.now() },
    ]);

    try {
      await streamChat(messageText, streamingId);
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

  const streamChat = (message: string, streamingId: string): Promise<void> => {
    return new Promise((resolve, reject) => {
      const apiUrl = process.env.NEXT_PUBLIC_API_URL || "http://localhost:8080";
      const eventSource = new EventSource(
        `${apiUrl}/api/v1/agents/default/chat/stream?message=${encodeURIComponent(message)}`
      );

      let completed = false;

      eventSource.addEventListener("chunk", (event) => {
        try {
          const data = JSON.parse(event.data);
          setMessages((prev) =>
            prev.map((m) =>
              m.id === streamingId
                ? { ...m, content: m.content + data.content }
                : m
            )
          );
        } catch (e) {
          console.error("Parse error:", e);
        }
      });

      eventSource.addEventListener("complete", () => {
        completed = true;
        eventSource.close();
        resolve();
      });

      eventSource.onerror = () => {
        eventSource.close();
        // Only reject if we haven't received complete event
        if (!completed) {
          reject(new Error("Stream connection failed"));
        }
      };

      // Timeout after 60 seconds
      setTimeout(() => {
        if (!completed) {
          eventSource.close();
          resolve();
        }
      }, 60000);
    });
  };

  const handleClear = () => {
    setMessages([]);
  };

  const handleKeyDown = (e: React.KeyboardEvent) => {
    if (e.key === "Enter" && !e.shiftKey) {
      e.preventDefault();
      handleSend();
    }
  };

  return (
    <div className="min-h-screen flex flex-col">
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

      <main className="flex-1 container max-w-4xl py-6 flex flex-col">
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
                    <p className="font-medium">京都旅游</p>
                    <p className="text-sm text-muted-foreground">介绍京都的热门景点</p>
                  </CardContent>
                </Card>
                <Link href="/destinations/create">
                  <Card className="cursor-pointer hover:bg-muted/50 transition-colors h-full">
                    <CardContent className="p-4">
                      <p className="font-medium">创建 Agent</p>
                      <p className="text-sm text-muted-foreground">创建专属目的地 Agent</p>
                    </CardContent>
                  </Card>
                </Link>
                <Card className="cursor-pointer hover:bg-muted/50 transition-colors" onClick={() => setInput("推荐一个3天的京都行程")}>
                  <CardContent className="p-4">
                    <p className="font-medium">行程规划</p>
                    <p className="text-sm text-muted-foreground">规划个性化旅行行程</p>
                  </CardContent>
                </Card>
                <Card className="cursor-pointer hover:bg-muted/50 transition-colors" onClick={() => setInput("日本旅游需要注意什么？")}>
                  <CardContent className="p-4">
                    <p className="font-medium">旅行建议</p>
                    <p className="text-sm text-muted-foreground">获取实用的旅行建议</p>
                  </CardContent>
                </Card>
              </div>
            </div>
          ) : (
            messages.map((msg) => (
              <div
                key={msg.id}
                className={`flex ${msg.role === "user" ? "justify-end" : "justify-start"}`}
              >
                <div
                  className={`max-w-[80%] rounded-lg px-4 py-2 ${
                    msg.role === "user"
                      ? "bg-primary text-primary-foreground"
                      : "bg-muted"
                  }`}
                >
                  {msg.role === "user" ? (
                    <p className="whitespace-pre-wrap">{msg.content}</p>
                  ) : (
                    <MarkdownContent content={msg.content} />
                  )}
                  <p className="text-xs opacity-70 mt-1">
                    {new Date(msg.timestamp).toLocaleTimeString()}
                  </p>
                </div>
              </div>
            ))
          )}

          {isLoading && messages[messages.length - 1]?.role !== "assistant" && (
            <div className="flex justify-start">
              <div className="bg-muted rounded-lg px-4 py-2">
                <Loader2 className="h-4 w-4 animate-spin" />
              </div>
            </div>
          )}

          <div ref={messagesEndRef} />
        </div>

        <div className="border-t pt-4">
          <div className="flex gap-2">
            <Input
              value={input}
              onChange={(e) => setInput(e.target.value)}
              onKeyDown={handleKeyDown}
              placeholder="输入你的问题..."
              disabled={isLoading}
              className="flex-1"
            />
            <Button onClick={handleSend} disabled={!input.trim() || isLoading}>
              {isLoading ? (
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