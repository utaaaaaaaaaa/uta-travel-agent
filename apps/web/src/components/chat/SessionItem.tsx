"use client";

import { MessageSquare, Trash2, Edit2, Check, X } from "lucide-react";
import { useState } from "react";
import type { Session } from "@/types/session";

// Simple time ago formatter
function formatTimeAgo(dateString: string): string {
  const date = new Date(dateString);
  const now = new Date();
  const diffMs = now.getTime() - date.getTime();
  const diffMins = Math.floor(diffMs / 60000);
  const diffHours = Math.floor(diffMs / 3600000);
  const diffDays = Math.floor(diffMs / 86400000);

  if (diffMins < 1) return "刚刚";
  if (diffMins < 60) return `${diffMins} 分钟前`;
  if (diffHours < 24) return `${diffHours} 小时前`;
  if (diffDays < 7) return `${diffDays} 天前`;
  return date.toLocaleDateString("zh-CN");
}

interface SessionItemProps {
  session: Session;
  isActive: boolean;
  onSelect: () => void;
  onDelete: () => void;
  onRename: (title: string) => void;
}

export function SessionItem({
  session,
  isActive,
  onSelect,
  onDelete,
  onRename,
}: SessionItemProps) {
  const [isEditing, setIsEditing] = useState(false);
  const [editTitle, setEditTitle] = useState(session.title || "新对话");
  const [showActions, setShowActions] = useState(false);

  const timeAgo = formatTimeAgo(session.last_active_at || session.created_at);

  const handleRename = () => {
    if (editTitle.trim() && editTitle !== session.title) {
      onRename(editTitle.trim());
    }
    setIsEditing(false);
  };

  const handleKeyDown = (e: React.KeyboardEvent) => {
    if (e.key === "Enter") {
      handleRename();
    } else if (e.key === "Escape") {
      setEditTitle(session.title || "新对话");
      setIsEditing(false);
    }
  };

  return (
    <div
      className={`
        group relative flex items-center gap-2 px-3 py-2 rounded-lg cursor-pointer
        transition-colors duration-200
        ${isActive ? "bg-gray-700 text-white" : "hover:bg-gray-800 text-gray-300"}
      `}
      onClick={() => !isEditing && onSelect()}
      onMouseEnter={() => setShowActions(true)}
      onMouseLeave={() => {
        setShowActions(false);
        if (isEditing) {
          setEditTitle(session.title || "新对话");
          setIsEditing(false);
        }
      }}
    >
      <MessageSquare className="w-4 h-4 flex-shrink-0 opacity-60" />

      <div className="flex-1 min-w-0">
        {isEditing ? (
          <div className="flex items-center gap-1">
            <input
              type="text"
              value={editTitle}
              onChange={(e) => setEditTitle(e.target.value)}
              onKeyDown={handleKeyDown}
              className="w-full bg-gray-900 text-sm px-2 py-1 rounded border border-gray-600 focus:border-blue-500 focus:outline-none"
              autoFocus
              onClick={(e) => e.stopPropagation()}
            />
            <button
              onClick={(e) => {
                e.stopPropagation();
                handleRename();
              }}
              className="p-1 hover:bg-gray-600 rounded"
            >
              <Check className="w-3 h-3 text-green-500" />
            </button>
            <button
              onClick={(e) => {
                e.stopPropagation();
                setEditTitle(session.title || "新对话");
                setIsEditing(false);
              }}
              className="p-1 hover:bg-gray-600 rounded"
            >
              <X className="w-3 h-3 text-red-500" />
            </button>
          </div>
        ) : (
          <>
            <p className="text-sm truncate font-medium">
              {session.title || "新对话"}
            </p>
            <p className="text-xs opacity-50 truncate">{timeAgo}</p>
          </>
        )}
      </div>

      {showActions && !isEditing && (
        <div className="flex items-center gap-1">
          <button
            onClick={(e) => {
              e.stopPropagation();
              setEditTitle(session.title || "新对话");
              setIsEditing(true);
            }}
            className="p-1 hover:bg-gray-600 rounded opacity-0 group-hover:opacity-100 transition-opacity"
            title="重命名"
          >
            <Edit2 className="w-3 h-3" />
          </button>
          <button
            onClick={(e) => {
              e.stopPropagation();
              onDelete();
            }}
            className="p-1 hover:bg-gray-600 rounded opacity-0 group-hover:opacity-100 transition-opacity"
            title="删除"
          >
            <Trash2 className="w-3 h-3 text-red-400" />
          </button>
        </div>
      )}
    </div>
  );
}
