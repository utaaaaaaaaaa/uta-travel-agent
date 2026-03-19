import Link from "next/link";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { MapPin, Compass, Plane, Sparkles } from "lucide-react";

export default function Home() {
  return (
    <div className="flex flex-col">
      {/* Header */}
      <header className="sticky top-0 z-50 w-full border-b bg-background/95 backdrop-blur supports-[backdrop-filter]:bg-background/60">
        <div className="container flex h-16 items-center justify-between">
          <div className="flex items-center gap-2">
            <Compass className="h-6 w-6 text-primary" />
            <span className="text-xl font-bold">UTA Travel</span>
          </div>
          <nav className="flex items-center gap-4">
            <Link href="/chat">
              <Button variant="ghost">AI 助手</Button>
            </Link>
            <Link href="/destinations">
              <Button variant="ghost">我的目的地</Button>
            </Link>
            <Link href="/destinations/create">
              <Button>创建 Agent</Button>
            </Link>
          </nav>
        </div>
      </header>

      {/* Hero Section */}
      <section className="relative py-20 md:py-32">
        <div className="container flex flex-col items-center text-center">
          <div className="inline-flex items-center gap-2 rounded-full bg-primary/10 px-4 py-2 text-sm text-primary mb-6">
            <Sparkles className="h-4 w-4" />
            <span>AI 驱动的智能导游</span>
          </div>
          <h1 className="text-4xl font-bold tracking-tight sm:text-5xl md:text-6xl lg:text-7xl">
            探索世界
            <br />
            <span className="text-primary">AI 导游随行</span>
          </h1>
          <p className="mt-6 max-w-2xl text-lg text-muted-foreground">
            创建专属目的地 Agent，获取深度文化讲解。实地旅游时，AI 导游实时为你讲解景点背后的故事。
          </p>
          <div className="mt-10 flex gap-4">
            <Link href="/chat">
              <Button size="lg" className="gap-2">
                <MapPin className="h-4 w-4" />
                开始对话
              </Button>
            </Link>
            <Link href="/destinations/create">
              <Button size="lg" variant="outline">
                创建目的地 Agent
              </Button>
            </Link>
          </div>
        </div>
      </section>

      {/* Features */}
      <section className="py-20 bg-muted/50">
        <div className="container">
          <h2 className="text-3xl font-bold text-center mb-12">核心功能</h2>
          <div className="grid md:grid-cols-3 gap-6">
            <Card>
              <CardHeader>
                <div className="h-12 w-12 rounded-lg bg-primary/10 flex items-center justify-center mb-4">
                  <MapPin className="h-6 w-6 text-primary" />
                </div>
                <CardTitle>智能目的地 Agent</CardTitle>
                <CardDescription>
                  输入目的地，AI 自动搜索整理信息，构建专属知识库
                </CardDescription>
              </CardHeader>
            </Card>
            <Card>
              <CardHeader>
                <div className="h-12 w-12 rounded-lg bg-primary/10 flex items-center justify-center mb-4">
                  <Plane className="h-6 w-6 text-primary" />
                </div>
                <CardTitle>实时导游讲解</CardTitle>
                <CardDescription>
                  实地旅游时，基于位置和图片识别景点，提供文化背景讲解
                </CardDescription>
              </CardHeader>
            </Card>
            <Card>
              <CardHeader>
                <div className="h-12 w-12 rounded-lg bg-primary/10 flex items-center justify-center mb-4">
                  <Compass className="h-6 w-6 text-primary" />
                </div>
                <CardTitle>个性化行程</CardTitle>
                <CardDescription>
                  根据你的偏好，智能规划旅行路线和时间安排
                </CardDescription>
              </CardHeader>
            </Card>
          </div>
        </div>
      </section>

      {/* CTA */}
      <section className="py-20">
        <div className="container text-center">
          <h2 className="text-3xl font-bold mb-4">开始你的旅程</h2>
          <p className="text-muted-foreground mb-8">
            创建你的第一个目的地 Agent，开启智能旅行体验
          </p>
          <Link href="/destinations/create">
            <Button size="lg" className="gap-2">
              <Sparkles className="h-4 w-4" />
              立即开始
            </Button>
          </Link>
        </div>
      </section>

      {/* Footer */}
      <footer className="border-t py-8">
        <div className="container flex items-center justify-between text-sm text-muted-foreground">
          <div className="flex items-center gap-2">
            <Compass className="h-4 w-4" />
            <span>UTA Travel Agent</span>
          </div>
          <div>
            Powered by Multi-Agent Architecture
          </div>
        </div>
      </footer>
    </div>
  );
}