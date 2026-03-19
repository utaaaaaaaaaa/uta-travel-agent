"use client";

import { useState } from "react";
import { useRouter } from "next/navigation";
import Link from "next/link";
import { Button } from "@/components/ui/button";
import { Card, CardContent } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Globe, ArrowRight, Check, Loader2, AlertCircle, Sparkles } from "lucide-react";
import { ExecutionProgress } from "@/components/destination/execution-progress";
import { useAgentCreation } from "@/hooks/useAgentCreation";

const themes = [
  { id: "cultural", label: "文化之旅", icon: "🏛️", description: "历史遗迹、博物馆、传统文化" },
  { id: "food", label: "美食探索", icon: "🍜", description: "当地美食、餐厅推荐、烹饪体验" },
  { id: "adventure", label: "户外探险", icon: "🏔️", description: "徒步、攀岩、自然风光" },
  { id: "art", label: "艺术之旅", icon: "🎨", description: "美术馆、街头艺术、设计" },
];

const languages = [
  { code: "zh", label: "中文" },
  { code: "en", label: "English" },
  { code: "ja", label: "日本語" },
];

export default function CreateDestinationPage() {
  const router = useRouter();
  const {
    status,
    steps,
    progress,
    error,
    agentId,
    create,
    reset,
  } = useAgentCreation();

  const [destination, setDestination] = useState("");
  const [selectedTheme, setSelectedTheme] = useState("cultural");
  const [selectedLanguages, setSelectedLanguages] = useState<string[]>(["zh"]);

  const handleCreate = async () => {
    try {
      await create({
        destination,
        theme: selectedTheme,
        languages: selectedLanguages,
      });
    } catch {
      // Error is handled by the hook
    }
  };

  const handleViewAgent = () => {
    if (agentId) {
      router.push(`/guide/${agentId}`);
    }
  };

  const handleCreateAnother = () => {
    reset();
    setDestination("");
    setSelectedTheme("cultural");
    setSelectedLanguages(["zh"]);
  };

  const toggleLanguage = (code: string) => {
    setSelectedLanguages((prev) =>
      prev.includes(code) ? prev.filter((l) => l !== code) : [...prev, code]
    );
  };

  return (
    <div className="min-h-screen">
      {/* Header */}
      <header className="sticky top-0 z-50 w-full border-b bg-background/95 backdrop-blur">
        <div className="container flex h-16 items-center justify-between">
          <Link href="/" className="flex items-center gap-2">
            <Globe className="h-6 w-6 text-primary" />
            <span className="text-xl font-bold">UTA Travel</span>
          </Link>
        </div>
      </header>

      <main className="container py-8 max-w-2xl">
        {/* Step 1: Form */}
        {status === 'idle' && (
          <div className="space-y-8">
            <div>
              <h1 className="text-3xl font-bold">创建你的专属导游 Agent</h1>
              <p className="text-muted-foreground mt-2">
                输入目的地信息，AI 将自动为你构建专属知识库
              </p>
            </div>

            {/* Destination Input */}
            <div className="space-y-2">
              <label className="text-sm font-medium">目的地名称</label>
              <Input
                placeholder="例如：京都, Japan"
                value={destination}
                onChange={(e) => setDestination(e.target.value)}
                className="text-lg"
              />
            </div>

            {/* Theme Selection */}
            <div className="space-y-3">
              <label className="text-sm font-medium">主题选择</label>
              <div className="grid grid-cols-2 gap-3">
                {themes.map((theme) => (
                  <Card
                    key={theme.id}
                    className={`cursor-pointer transition-all ${
                      selectedTheme === theme.id
                        ? "border-primary bg-primary/5"
                        : "hover:border-gray-300"
                    }`}
                    onClick={() => setSelectedTheme(theme.id)}
                  >
                    <CardContent className="p-4">
                      <div className="flex items-center gap-3">
                        <span className="text-2xl">{theme.icon}</span>
                        <div>
                          <p className="font-medium">{theme.label}</p>
                          <p className="text-xs text-muted-foreground">
                            {theme.description}
                          </p>
                        </div>
                      </div>
                    </CardContent>
                  </Card>
                ))}
              </div>
            </div>

            {/* Language Selection */}
            <div className="space-y-3">
              <label className="text-sm font-medium">支持语言</label>
              <div className="flex gap-2">
                {languages.map((lang) => (
                  <Button
                    key={lang.code}
                    variant={selectedLanguages.includes(lang.code) ? "default" : "outline"}
                    onClick={() => toggleLanguage(lang.code)}
                  >
                    {lang.label}
                  </Button>
                ))}
              </div>
            </div>

            {/* Submit */}
            <Button
              size="lg"
              className="w-full"
              disabled={!destination.trim() || selectedLanguages.length === 0}
              onClick={handleCreate}
            >
              开始创建
              <ArrowRight className="ml-2 h-4 w-4" />
            </Button>
          </div>
        )}

        {/* Step 2: Creation Progress */}
        {status === 'creating' && (
          <div className="space-y-8">
            <div className="text-center">
              <div className="h-16 w-16 rounded-full bg-primary/10 flex items-center justify-center mx-auto mb-4">
                <Loader2 className="h-8 w-8 text-primary animate-spin" />
              </div>
              <h1 className="text-2xl font-bold">正在创建中...</h1>
              <p className="text-muted-foreground mt-2">
                为你构建 <span className="font-medium text-foreground">{destination}</span> 导游 Agent
              </p>
            </div>

            {/* Execution Progress */}
            <Card>
              <CardContent className="p-6">
                <ExecutionProgress
                  steps={steps}
                  progress={progress}
                />
              </CardContent>
            </Card>

            {/* Error */}
            {error && (
              <div className="text-center">
                <div className="flex items-center justify-center gap-2 text-destructive mb-4">
                  <AlertCircle className="h-4 w-4" />
                  <span>{error}</span>
                </div>
                <Button variant="outline" onClick={reset}>
                  重试
                </Button>
              </div>
            )}
          </div>
        )}

        {/* Step 3: Completion */}
        {status === 'completed' && (
          <div className="space-y-8">
            <div className="text-center">
              <div className="h-20 w-20 rounded-full bg-green-100 dark:bg-green-900/30 flex items-center justify-center mx-auto mb-4">
                <Check className="h-10 w-10 text-green-600 dark:text-green-400" />
              </div>
              <h1 className="text-2xl font-bold text-green-700 dark:text-green-400">创建完成！</h1>
              <p className="text-muted-foreground mt-2">
                <span className="font-medium text-foreground">{destination}</span> 导游 Agent 已准备就绪
              </p>
            </div>

            {/* Summary Card */}
            <Card className="bg-gradient-to-br from-primary/5 to-primary/10 border-primary/20">
              <CardContent className="p-6">
                <div className="flex items-center gap-4 mb-4">
                  <div className="h-12 w-12 rounded-lg bg-primary/20 flex items-center justify-center">
                    <Sparkles className="h-6 w-6 text-primary" />
                  </div>
                  <div>
                    <h3 className="font-semibold">{destination}</h3>
                    <p className="text-sm text-muted-foreground">
                      {steps.find(s => s.id === 'researcher')?.details || '信息已收集'}
                    </p>
                  </div>
                </div>

                <div className="grid grid-cols-3 gap-4 text-center">
                  {steps.map((step) => (
                    <div key={step.id} className="space-y-1">
                      <p className="text-xs text-muted-foreground">{step.name}</p>
                      <p className="text-sm font-medium text-green-600 dark:text-green-400">
                        <Check className="h-3 w-3 inline mr-1" />
                        完成
                      </p>
                    </div>
                  ))}
                </div>
              </CardContent>
            </Card>

            {/* Action Buttons */}
            <div className="flex gap-3">
              <Button
                size="lg"
                className="flex-1"
                onClick={handleViewAgent}
              >
                开始使用
                <ArrowRight className="ml-2 h-4 w-4" />
              </Button>
              <Button
                size="lg"
                variant="outline"
                onClick={handleCreateAnother}
              >
                创建新 Agent
              </Button>
            </div>
          </div>
        )}

        {/* Error State */}
        {status === 'error' && (
          <div className="space-y-8 text-center">
            <div className="h-20 w-20 rounded-full bg-destructive/10 flex items-center justify-center mx-auto">
              <AlertCircle className="h-10 w-10 text-destructive" />
            </div>
            <div>
              <h1 className="text-2xl font-bold text-destructive">创建失败</h1>
              <p className="text-muted-foreground mt-2">{error}</p>
            </div>
            <Button variant="outline" size="lg" onClick={reset}>
              重试
            </Button>
          </div>
        )}
      </main>
    </div>
  );
}