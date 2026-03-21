import { useState, useEffect, useRef, useCallback } from "react";
import Link from "next/link";
import { useRouter } from "next/navigation";

interface DeleteConfirmModalProps {
  isOpen: boolean;
  agentName: string;
  onConfirm: () => void;
  onCancel: () => void;
}

export function DeleteConfirmModal({ isOpen, agentName, onConfirm, onCancel }: DeleteConfirmModalProps) {
  if (!isOpen) return null;

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center">
      {/* Backdrop */}
      <div
        className="absolute inset-0 bg-black/50 backdrop-blur-sm"
        onClick={onCancel}
      />

      {/* Modal */}
      <div className="relative bg-card rounded-lg shadow-xl p-6 w-full max-w-md mx-4 animate-in fade-in zoom-in duration-200">
        <div className="flex items-start gap-4">
          <div className="h-12 w-12 rounded-full bg-destructive/10 flex items-center justify-center flex-shrink-0">
            <svg
              className="h-6 w-6 text-destructive"
              fill="none"
              viewBox="0 0 24 24"
              stroke="currentColor"
              strokeWidth={2}
            >
              <path
                strokeLinecap="round"
                strokeLinejoin="round"
                d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z"
              />
            </svg>
          </div>
          <div className="flex-1">
            <h2 className="text-lg font-semibold">Delete Agent</h2>
            <p className="text-sm text-muted-foreground mt-1">
              Are you sure you want to delete <span className="font-medium text-foreground">"{agentName}"</span>?
            </p>
            <p className="text-sm text-muted-foreground mt-2">
              This will also delete all associated documents and chat history. This action cannot be undone.
            </p>
          </div>
        </div>

        <div className="flex justify-end gap-3 mt-6">
          <button
            onClick={onCancel}
            className="px-4 py-2 text-sm font-medium rounded-md border border-input bg-background hover:bg-accent hover:text-accent-foreground transition-colors"
          >
            Cancel
          </button>
          <button
            onClick={onConfirm}
            className="px-4 py-2 text-sm font-medium rounded-md bg-destructive text-white hover:bg-destructive/90 transition-colors"
          >
            Delete
          </button>
        </div>
      </div>
    </div>
  );
}
