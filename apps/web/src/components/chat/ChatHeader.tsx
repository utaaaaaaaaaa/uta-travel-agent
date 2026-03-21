"use client";

import { Bot, MoreVertical, Archive, Trash2 } from "lucide-react";
import { useState } from "react";
import type { Session } from "@/types/session";

interface ChatHeaderProps {
  session: Session | null;
  onArchive?: () => void;
  onDelete?: () => void;
}

export function ChatHeader({ session, onArchive, onDelete }: ChatHeaderProps) {
  const [showMenu, setShowMenu] = useState(false);

  if (!session) {
    return (
      <div className="h-14 border-b border-gray-700 bg-gray-900 flex items-center justify-center">
        <Bot className="w-6 h-6 text-gray-500 mr-2" />
        <span className="text-gray-500">选择或创建一个对话</span>
      </div>
    );
  }

  const getAgentTypeLabel = (type: string) => {
    switch (type) {
      case "main":
        return "智能助手";
      case "guide":
        return "导游助手";
      case "planner":
        return "行程规划";
      default:
        return "助手";
    }
  };

  return (
    <div className="h-14 border-b border-gray-700 bg-gray-900 px-4 flex items-center justify-between">
      {/* Left: Title and info */}
      <div className="flex items-center gap-3">
        <Bot className="w-5 h-5 text-emerald-500" />
        <div>
          <h2 className="text-sm font-medium text-white">
            {session.title || "新对话"}
          </h2>
          <p className="text-xs text-gray-500">
            {getAgentTypeLabel(session.agent_type)} · {session.message_count} 条消息
          </p>
        </div>
      </div>

      {/* Right: Actions */}
      <div className="relative">
        <button
          onClick={() => setShowMenu(!showMenu)}
          className="p-2 hover:bg-gray-800 rounded-lg transition-colors"
        >
          <MoreVertical className="w-5 h-5 text-gray-400" />
        </button>

        {showMenu && (
          <>
            {/* Backdrop */}
            <div
              className="fixed inset-0 z-10"
              onClick={() => setShowMenu(false)}
            />

            {/* Menu */}
            <div className="absolute right-0 top-full mt-1 w-48 bg-gray-800 border border-gray-700 rounded-lg shadow-lg z-20 py-1">
              <button
                onClick={() => {
                  setShowMenu(false);
                  onArchive?.();
                }}
                className="w-full flex items-center gap-2 px-3 py-2 text-sm text-gray-300 hover:bg-gray-700"
              >
                <Archive className="w-4 h-4" />
                <span>归档对话</span>
              </button>
              <button
                onClick={() => {
                  setShowMenu(false);
                  onDelete?.();
                }}
                className="w-full flex items-center gap-2 px-3 py-2 text-sm text-red-400 hover:bg-gray-700"
              >
                <Trash2 className="w-4 h-4" />
                <span>删除对话</span>
              </button>
            </div>
          </>
        )}
      </div>
    </div>
  );
}