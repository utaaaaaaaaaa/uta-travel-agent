"use client";

import { Check, Loader2, Circle } from "lucide-react";
import { cn } from "@/lib/utils";

export interface ExecutionStep {
  id: string;
  name: string;
  description: string;
  status: "pending" | "running" | "completed" | "error";
}

interface ExecutionProgressProps {
  steps: ExecutionStep[];
  currentStep?: number;
  progress: number;
}

export function ExecutionProgress({ steps, progress }: ExecutionProgressProps) {
  return (
    <div className="w-full space-y-4">
      {/* Steps */}
      <div className="space-y-3">
        {steps.map((step, index) => (
          <div
            key={step.id}
            className={cn(
              "flex items-start gap-3 p-3 rounded-lg transition-colors",
              step.status === "running" && "bg-primary/5 border border-primary/20",
              step.status === "completed" && "bg-green-50 dark:bg-green-950/20",
              step.status === "error" && "bg-destructive/5 border border-destructive/20"
            )}
          >
            {/* Status Icon */}
            <div className="flex-shrink-0 mt-0.5">
              {step.status === "pending" && (
                <Circle className="h-5 w-5 text-muted-foreground" />
              )}
              {step.status === "running" && (
                <Loader2 className="h-5 w-5 text-primary animate-spin" />
              )}
              {step.status === "completed" && (
                <div className="h-5 w-5 rounded-full bg-green-500 flex items-center justify-center">
                  <Check className="h-3 w-3 text-white" />
                </div>
              )}
              {step.status === "error" && (
                <Circle className="h-5 w-5 text-destructive fill-destructive" />
              )}
            </div>

            {/* Content */}
            <div className="flex-1 min-w-0">
              <div className="flex items-center justify-between">
                <p className={cn(
                  "font-medium text-sm",
                  step.status === "pending" && "text-muted-foreground",
                  step.status === "running" && "text-primary",
                  step.status === "completed" && "text-green-700 dark:text-green-400",
                  step.status === "error" && "text-destructive"
                )}>
                  {step.name}
                </p>
                {step.status === "running" && (
                  <span className="text-xs text-muted-foreground animate-pulse">
                    处理中...
                  </span>
                )}
              </div>
              <p className="text-xs text-muted-foreground mt-0.5">
                {step.description}
              </p>

              {/* Running step progress bar */}
              {step.status === "running" && (
                <div className="mt-2 h-1 bg-muted rounded-full overflow-hidden">
                  <div
                    className="h-full bg-primary transition-all duration-300"
                    style={{ width: `${progress}%` }}
                  />
                </div>
              )}
            </div>
          </div>
        ))}
      </div>

      {/* Overall Progress */}
      <div className="pt-4 border-t">
        <div className="flex items-center justify-between text-sm mb-2">
          <span className="text-muted-foreground">总进度</span>
          <span className="font-medium">{progress}%</span>
        </div>
        <div className="h-2 bg-muted rounded-full overflow-hidden">
          <div
            className="h-full bg-primary transition-all duration-500"
            style={{ width: `${progress}%` }}
          />
        </div>
      </div>
    </div>
  );
}
