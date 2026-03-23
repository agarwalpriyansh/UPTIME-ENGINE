import { Activity } from "lucide-react";

export default function Home() {
  return (
    <main className="min-h-screen p-8 max-w-7xl mx-auto">
      <div className="flex items-center gap-3 mb-8 border-b border-gray-800 pb-6">
        <Activity className="w-8 h-8 text-blue-500" />
        <h1 className="text-3xl font-bold text-gray-100">
          Uptime Engine Dashboard
        </h1>
      </div>
      
      <p className="text-gray-400">Next.js Frontend Initialized Successfully.</p>
    </main>
  );
}