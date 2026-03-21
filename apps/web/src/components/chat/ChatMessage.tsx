"use client";

import { User, Bot } from "lucide-react";
import type { SessionMessage } from "@/types/session";

interface ChatMessageProps {
  message: SessionMessage;
}

// Simple markdown renderer
function renderMarkdown(content: string): string {
  return content
    .replace(/\*\*(.*?)\*\*/g, "<strong>$1</strong>")
    .replace(/\*(.*?)\*/g, "<em>$1</em>")
    .replace(/```([\s\S]*?)```/g, '<pre class="bg-gray-800 p-3 rounded-lg my-2 overflow-x-auto"><code>$1</code></pre>')
    .replace(/`(.*?)`/g, '<code class="bg-gray-800 px-1.5 py-0.5 rounded text-sm">$1</code>')
    .replace(/^### (.*$)/gm, '<h3 class="font-bold text-base my-2">$1</h3>')
    .replace(/^## (.*$)/gm, '<h2 class="font-bold text-lg my-2">$1</h2>')
    .replace(/^# (.*$)/gm, '<h1 class="font-bold text-xl my-3">$1</h1>')
    .replace(/^\- (.*$)/gm, '<li class="ml-4 my-1">$1</li>')
    .replace(/^\d+\. (.*$)/gm, '<li class="ml-4 my-1 list-decimal">$1</li>')
    .replace(/\n/g, "<br/>");
}

export function ChatMessage({ message }: ChatMessageProps) {
  const isUser = message.role === "user";

  return (
    <div
      className={`flex gap-3 px-4 py-4 ${isUser ? "bg-transparent" : "bg-gray-800/30"}`}
    >
      {/* Avatar */}
      <div
        className={`
          flex-shrink-0 w-8 h-8 rounded-full flex items-center justify-center
          ${isUser ? "bg-blue-600" : "bg-emerald-600"}
        `}
      >
        {isUser ? (
          <User className="w-5 h-5 text-white" />
        ) : (
          <Bot className="w-5 h-5 text-white" />
        )}
      </div>

      {/* Content */}
      <div className="flex-1 min-w-0">
        <div className="flex items-center gap-2 mb-1">
          <span className="text-sm font-medium text-gray-300">
            {isUser ? "你" : "助手"}
          </span>
          <span className="text-xs text-gray-500">
            {new Date(message.created_at).toLocaleTimeString("zh-CN", {
              hour: "2-digit",
              minute: "2-digit",
            })}
          </span>
        </div>
        <div
          className="text-sm text-gray-200 leading-relaxed prose prose-invert max-w-none"
          dangerouslySetInnerHTML={{ __html: renderMarkdown(message.content) }}
        />
      </div>
    </div>
  );
}

interface ChatMessagesProps {
  messages: SessionMessage[];
  loading?: boolean;
}

export function ChatMessages({ messages, loading }: ChatMessagesProps) {
  return (
    <div className="flex-1 overflow-y-auto">
      {messages.length === 0 ? (
        <div className="flex flex-col items-center justify-center h-full text-gray-500">
          <Bot className="w-12 h-12 mb-3 opacity-30" />
          <p className="text-lg font-medium">开始新对话</p>
          <p className="text-sm mt-1">输入消息开始与助手交流</p>
        </div>
      ) : (
        <>
          {messages.map((message) => (
            <ChatMessage key={message.id} message={message} />
          ))}
          {loading && (
            <div className="flex gap-3 px-4 py-4 bg-gray-800/30">
              <div className="w-8 h-8 rounded-full bg-emerald-600 flex items-center justify-center">
                <Bot className="w-5 h-5 text-white" />
              </div>
              <div className="flex items-center gap-1">
                <div className="w-2 h-2 bg-gray-400 rounded-full animate-bounce" style={{ animationDelay: "0ms" }} />
                <div className="w-2 h-2 bg-gray-400 rounded-full animate-bounce" style={{ animationDelay: "150ms" }} />
                <div className="w-2 h-2 bg-gray-400 rounded-full animate-bounce" style={{ animationDelay: "300ms" }} />
              </div>
            </div>
          )}
        </>
      )}
    </div>
  );
}