"use client";

import { Plus, MessageSquare } from "lucide-react";
import { SessionItem } from "./SessionItem";
import type { Session, SessionGroup } from "@/types/session";

interface SessionSidebarProps {
  sessions: Session[];
  grouped: SessionGroup;
  activeId: string | null;
  onSelect: (id: string) => void;
  onCreate: () => void;
  onDelete: (id: string) => void;
  onRename: (id: string, title: string) => void;
  loading?: boolean;
}

export function SessionSidebar({
  sessions,
  grouped,
  activeId,
  onSelect,
  onCreate,
  onDelete,
  onRename,
  loading,
}: SessionSidebarProps) {
  return (
    <div className="w-64 h-full bg-gray-900 border-r border-gray-700 flex flex-col">
      {/* Header */}
      <div className="p-3 border-b border-gray-700">
        <button
          onClick={onCreate}
          className="w-full flex items-center justify-center gap-2 px-3 py-2 bg-blue-600 hover:bg-blue-700 text-white rounded-lg transition-colors"
        >
          <Plus className="w-4 h-4" />
          <span className="text-sm font-medium">新建对话</span>
        </button>
      </div>

      {/* Session List */}
      <div className="flex-1 overflow-y-auto">
        {loading ? (
          <div className="flex items-center justify-center py-8 text-gray-500">
            <div className="animate-spin w-5 h-5 border-2 border-gray-500 border-t-transparent rounded-full" />
          </div>
        ) : sessions.length === 0 ? (
          <div className="flex flex-col items-center justify-center py-8 text-gray-500">
            <MessageSquare className="w-8 h-8 mb-2 opacity-50" />
            <p className="text-sm">暂无对话</p>
            <p className="text-xs mt-1">点击上方按钮开始新对话</p>
          </div>
        ) : (
          <>
            {/* Today */}
            {grouped.today.length > 0 && (
              <div className="p-2">
                <h3 className="text-xs text-gray-500 px-2 py-1 font-medium">今天</h3>
                <div className="space-y-1">
                  {grouped.today.map((session) => (
                    <SessionItem
                      key={session.id}
                      session={session}
                      isActive={activeId === session.id}
                      onSelect={() => onSelect(session.id)}
                      onDelete={() => onDelete(session.id)}
                      onRename={(title) => onRename(session.id, title)}
                    />
                  ))}
                </div>
              </div>
            )}

            {/* Yesterday */}
            {grouped.yesterday.length > 0 && (
              <div className="p-2">
                <h3 className="text-xs text-gray-500 px-2 py-1 font-medium">昨天</h3>
                <div className="space-y-1">
                  {grouped.yesterday.map((session) => (
                    <SessionItem
                      key={session.id}
                      session={session}
                      isActive={activeId === session.id}
                      onSelect={() => onSelect(session.id)}
                      onDelete={() => onDelete(session.id)}
                      onRename={(title) => onRename(session.id, title)}
                    />
                  ))}
                </div>
              </div>
            )}

            {/* Previous */}
            {grouped.previous.length > 0 && (
              <div className="p-2">
                <h3 className="text-xs text-gray-500 px-2 py-1 font-medium">过去 7 天</h3>
                <div className="space-y-1">
                  {grouped.previous.map((session) => (
                    <SessionItem
                      key={session.id}
                      session={session}
                      isActive={activeId === session.id}
                      onSelect={() => onSelect(session.id)}
                      onDelete={() => onDelete(session.id)}
                      onRename={(title) => onRename(session.id, title)}
                    />
                  ))}
                </div>
              </div>
            )}
          </>
        )}
      </div>

      {/* Footer */}
      <div className="p-3 border-t border-gray-700 text-xs text-gray-500 text-center">
        UTA Travel Agent v0.6.0
      </div>
    </div>
  );
}