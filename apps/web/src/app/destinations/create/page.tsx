"use client";

import { useState } from "react";
import { useRouter } from "next/navigation";
import Link from "next/link";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Globe, ArrowRight, Check, Loader2 } from "lucide-react";

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
  const [step, setStep] = useState(1);
  const [destination, setDestination] = useState("");
  const [selectedTheme, setSelectedTheme] = useState("cultural");
  const [selectedLanguages, setSelectedLanguages] = useState<string[]>(["zh"]);
  const [isCreating, setIsCreating] = useState(false);
  const [progress, setProgress] = useState(0);
  const [currentTask, setCurrentTask] = useState("");

  const handleCreate = async () => {
    setIsCreating(true);
    setStep(2);

    // Simulate creation process
    const tasks = [
      { text: "搜索目的地信息", progress: 20 },
      { text: "提取文化背景", progress: 40 },
      { text: "构建知识图谱", progress: 60 },
      { text: "向量化存储", progress: 80 },
      { text: "创建对话系统", progress: 100 },
    ];

    for (const task of tasks) {
      setCurrentTask(task.text);
      setProgress(task.progress);
      await new Promise((resolve) => setTimeout(resolve, 1500));
    }

    // Navigate to guide page
    setTimeout(() => {
      router.push("/destinations");
    }, 1000);
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
        {step === 1 && (
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

        {step === 2 && (
          <div className="space-y-8">
            <div className="text-center">
              <div className="h-16 w-16 rounded-full bg-primary/10 flex items-center justify-center mx-auto mb-4">
                {progress < 100 ? (
                  <Loader2 className="h-8 w-8 text-primary animate-spin" />
                ) : (
                  <Check className="h-8 w-8 text-primary" />
                )}
              </div>
              <h1 className="text-2xl font-bold">
                {progress < 100 ? "正在创建中..." : "创建完成！"}
              </h1>
              <p className="text-muted-foreground mt-2">
                为你构建 {destination} 导游 Agent
              </p>
            </div>

            {/* Progress Bar */}
            <div className="space-y-2">
              <div className="h-2 bg-muted rounded-full overflow-hidden">
                <div
                  className="h-full bg-primary transition-all duration-500"
                  style={{ width: `${progress}%` }}
                />
              </div>
              <p className="text-sm text-center text-muted-foreground">{progress}%</p>
            </div>

            {/* Task List */}
            <div className="space-y-3">
              {[
                { text: "搜索目的地信息", done: progress >= 20 },
                { text: "提取文化背景", done: progress >= 40 },
                { text: "构建知识图谱", done: progress >= 60 },
                { text: "向量化存储", done: progress >= 80 },
                { text: "创建对话系统", done: progress >= 100 },
              ].map((task, index) => (
                <div
                  key={index}
                  className={`flex items-center gap-3 p-3 rounded-lg ${
                    task.done ? "bg-green-50" : currentTask === task.text ? "bg-primary/5" : ""
                  }`}
                >
                  {task.done ? (
                    <Check className="h-5 w-5 text-green-600" />
                  ) : currentTask === task.text ? (
                    <Loader2 className="h-5 w-5 text-primary animate-spin" />
                  ) : (
                    <div className="h-5 w-5 rounded-full border-2" />
                  )}
                  <span className={task.done ? "text-green-700" : ""}>{task.text}</span>
                </div>
              ))}
            </div>
          </div>
        )}
      </main>
    </div>
  );
}