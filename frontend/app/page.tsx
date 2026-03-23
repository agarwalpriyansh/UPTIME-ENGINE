"use client";

import { useState, useEffect } from "react";
import { Activity, Globe, ServerCrash, Trash2, Plus } from "lucide-react";

interface MonitorJob {
  type: string;
  target: string;
}

interface PingResult {
  job: MonitorJob;
  status_code: number;
  latency: number;
  up: boolean;
  timestamp: string;
}

export default function Home() {
  const [results, setResults] = useState<PingResult[]>([]);
  const [loading, setLoading] = useState(true);

  // Form State
  const [newUrl, setNewUrl] = useState("");
  const [newProtocol, setNewProtocol] = useState("https");
  const [isSubmitting, setIsSubmitting] = useState(false);

  // 1. Fetch Data (Read)
  const fetchStatus = async () => {
    try {
      const res = await fetch("/api/status");
      const data = await res.json();
      setResults(data || []);
    } catch (error) {
      console.error("Failed to fetch status:", error);
    } finally {
      setLoading(false);
    }
  };

  // 2. Add New Monitor (Create)
  const handleAddMonitor = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!newUrl) return;

    setIsSubmitting(true);
    try {
      await fetch("/api/monitor", {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
        },
        body: JSON.stringify({
          type: newProtocol,
          target: newUrl,
        }),
      });
      
      // Clear the form and instantly refresh the table
      setNewUrl("");
      fetchStatus(); 
    } catch (error) {
      console.error("Failed to add monitor:", error);
    } finally {
      setIsSubmitting(false);
    }
  };

  // 3. Delete Monitor (Delete)
  const handleDeleteMonitor = async (targetUrl: string) => {
    try {
      // We use encodeURIComponent in case the URL has special characters
      await fetch(`/api/monitor?url=${encodeURIComponent(targetUrl)}`, {
        method: "DELETE",
      });
      
      // Instantly refresh the table to show it's gone
      fetchStatus();
    } catch (error) {
      console.error("Failed to delete monitor:", error);
    }
  };

  // Real-Time Polling Engine
  useEffect(() => {
    fetchStatus();
    const interval = setInterval(() => {
      fetchStatus();
    }, 5000);
    return () => clearInterval(interval);
  }, []);

  return (
    <main className="min-h-screen p-8 max-w-6xl mx-auto">
      {/* Header section */}
      <div className="flex items-center justify-between mb-8 border-b border-gray-800 pb-6">
        <div className="flex items-center gap-3">
          <Activity className="w-8 h-8 text-blue-500" />
          <h1 className="text-3xl font-bold text-gray-100">Uptime Engine</h1>
        </div>
        <div className="flex items-center gap-2">
          <span className="relative flex h-3 w-3">
            <span className="animate-ping absolute inline-flex h-full w-full rounded-full bg-green-400 opacity-75"></span>
            <span className="relative inline-flex rounded-full h-3 w-3 bg-green-500"></span>
          </span>
          <span className="text-sm text-green-400 font-medium tracking-wide">SYSTEM LIVE</span>
        </div>
      </div>

      {/* NEW: The Management Panel (Add Monitor Form) */}
      <div className="bg-gray-800/40 rounded-xl border border-gray-700 p-6 mb-8 shadow-lg">
        <h2 className="text-lg font-semibold text-gray-200 mb-4">Add New Monitor</h2>
        <form onSubmit={handleAddMonitor} className="flex gap-4 items-center">
          <select 
            value={newProtocol}
            onChange={(e) => setNewProtocol(e.target.value)}
            className="bg-gray-900 border border-gray-600 text-white text-sm rounded-lg focus:ring-blue-500 focus:border-blue-500 block p-2.5 outline-none"
          >
            <option value="https">HTTPS</option>
            <option value="http">HTTP</option>
            <option value="tcp">TCP</option>
          </select>
          
          <input 
            type="text" 
            value={newUrl}
            onChange={(e) => setNewUrl(e.target.value)}
            placeholder="e.g., google.com or 192.168.1.1:80" 
            className="bg-gray-900 border border-gray-600 text-white text-sm rounded-lg focus:ring-blue-500 focus:border-blue-500 block w-full p-2.5 outline-none"
            required
          />
          
          <button 
            type="submit" 
            disabled={isSubmitting}
            className="flex items-center gap-2 text-white bg-blue-600 hover:bg-blue-700 disabled:bg-blue-800 disabled:cursor-not-allowed focus:ring-4 focus:outline-none focus:ring-blue-800 font-medium rounded-lg text-sm px-5 py-2.5 transition-colors"
          >
            <Plus className="w-4 h-4" />
            {isSubmitting ? "Adding..." : "Add Target"}
          </button>
        </form>
      </div>

      {/* The Data Table */}
      <div className="bg-gray-800/50 rounded-xl border border-gray-700 overflow-hidden shadow-2xl">
        <table className="w-full text-left border-collapse">
          <thead>
            <tr className="bg-gray-800/80 text-gray-400 text-xs uppercase tracking-wider border-b border-gray-700">
              <th className="p-5 font-medium">Target URL</th>
              <th className="p-5 font-medium">Protocol</th>
              <th className="p-5 font-medium">Status</th>
              <th className="p-5 font-medium">Latency</th>
              <th className="p-5 font-medium">Last Checked</th>
              <th className="p-5 font-medium text-right">Actions</th> {/* NEW: Actions Column */}
            </tr>
          </thead>
          <tbody>
            {loading ? (
              <tr>
                <td colSpan={6} className="p-8 text-center text-gray-500 animate-pulse">
                  Connecting to Go backend...
                </td>
              </tr>
            ) : results.length === 0 ? (
              <tr>
                <td colSpan={6} className="p-12 text-center text-gray-500">
                  <ServerCrash className="w-12 h-12 mx-auto mb-3 opacity-20" />
                  No monitoring data found. Add a target above to begin.
                </td>
              </tr>
            ) : (
              results.map((ping, idx) => (
                <tr 
                  key={idx} 
                  className="border-b border-gray-700/50 hover:bg-gray-700/30 transition-colors group"
                >
                  <td className="p-5">
                    <div className="flex items-center gap-2 text-gray-200">
                      <Globe className="w-4 h-4 text-gray-500" />
                      <span className="font-mono text-sm">{ping.job.target}</span>
                    </div>
                  </td>
                  <td className="p-5 text-gray-400 text-sm">{ping.job.type.toUpperCase()}</td>
                  <td className="p-5">
                    <span className={`px-3 py-1 rounded-full text-xs font-bold flex items-center gap-2 w-max ${
                      ping.up ? "text-green-400 bg-green-400/10 border border-green-400/20" : "text-red-400 bg-red-400/10 border border-red-400/20"
                    }`}>
                      <span className={`w-1.5 h-1.5 rounded-full ${ping.up ? "bg-green-400" : "bg-red-400"}`}></span>
                      {ping.up ? "UP" : "DOWN"}
                    </span>
                  </td>
                  <td className="p-5">
                    <span className="font-mono text-sm text-yellow-200/90">
                      {(ping.latency / 1000000).toFixed(2)} ms
                    </span>
                  </td>
                  <td className="p-5 text-gray-400 text-sm">
                    {new Date(ping.timestamp).toLocaleTimeString()}
                  </td>
                  <td className="p-5 text-right">
                    {/* NEW: Delete Button */}
                    <button 
                      onClick={() => handleDeleteMonitor(ping.job.target)}
                      className="p-2 text-gray-500 hover:text-red-400 hover:bg-red-400/10 rounded transition-colors opacity-0 group-hover:opacity-100 focus:opacity-100"
                      title="Delete Monitor"
                    >
                      <Trash2 className="w-4 h-4" />
                    </button>
                  </td>
                </tr>
              ))
            )}
          </tbody>
        </table>
      </div>
    </main>
  );
}