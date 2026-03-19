import Link from "next/link";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Plus, MapPin, Globe } from "lucide-react";

// Mock data - will be replaced with API calls
const mockAgents = [
  {
    id: "abc123",
    name: "京都导游",
    destination: "京都, 日本",
    theme: "cultural",
    status: "ready",
    documentCount: 42,
  },
  {
    id: "def456",
    name: "巴黎导游",
    destination: "巴黎, 法国",
    theme: "art",
    status: "ready",
    documentCount: 38,
  },
];

export default function DestinationsPage() {
  return (
    <div className="min-h-screen">
      {/* Header */}
      <header className="sticky top-0 z-50 w-full border-b bg-background/95 backdrop-blur">
        <div className="container flex h-16 items-center justify-between">
          <Link href="/" className="flex items-center gap-2">
            <Globe className="h-6 w-6 text-primary" />
            <span className="text-xl font-bold">UTA Travel</span>
          </Link>
          <Link href="/destinations/create">
            <Button>
              <Plus className="h-4 w-4 mr-2" />
              创建 Agent
            </Button>
          </Link>
        </div>
      </header>

      {/* Main Content */}
      <main className="container py-8">
        <div className="flex items-center justify-between mb-8">
          <div>
            <h1 className="text-3xl font-bold">我的目的地</h1>
            <p className="text-muted-foreground mt-1">
              管理你创建的目的地 Agent
            </p>
          </div>
        </div>

        {/* Agent Grid */}
        <div className="grid md:grid-cols-2 lg:grid-cols-3 gap-6">
          {/* Create New Card */}
          <Link href="/destinations/create">
            <Card className="h-full cursor-pointer hover:border-primary transition-colors border-dashed">
              <CardContent className="flex flex-col items-center justify-center h-full py-12">
                <div className="h-16 w-16 rounded-full bg-primary/10 flex items-center justify-center mb-4">
                  <Plus className="h-8 w-8 text-primary" />
                </div>
                <p className="font-medium">创建新目的地 Agent</p>
                <p className="text-sm text-muted-foreground mt-1">
                  输入目的地，AI 自动构建知识库
                </p>
              </CardContent>
            </Card>
          </Link>

          {/* Agent Cards */}
          {mockAgents.map((agent) => (
            <Link key={agent.id} href={`/guide/${agent.id}`}>
              <Card className="h-full cursor-pointer hover:border-primary transition-colors">
                <CardHeader>
                  <div className="flex items-start justify-between">
                    <div className="h-12 w-12 rounded-lg bg-primary/10 flex items-center justify-center">
                      <MapPin className="h-6 w-6 text-primary" />
                    </div>
                    <span className="text-xs bg-green-100 text-green-700 px-2 py-1 rounded-full">
                      {agent.status === "ready" ? "就绪" : "创建中"}
                    </span>
                  </div>
                  <CardTitle className="mt-4">{agent.name}</CardTitle>
                  <CardDescription>{agent.destination}</CardDescription>
                </CardHeader>
                <CardContent>
                  <div className="flex items-center gap-4 text-sm text-muted-foreground">
                    <span>{agent.documentCount} 篇文档</span>
                    <span>•</span>
                    <span className="capitalize">{agent.theme}</span>
                  </div>
                </CardContent>
              </Card>
            </Link>
          ))}
        </div>
      </main>
    </div>
  );
}