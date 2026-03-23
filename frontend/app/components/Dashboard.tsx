"use client";

import {
  Globe,
  ArrowUp,
  ArrowDown,
  Timer,
  Activity,
} from "lucide-react";
import {
  BarChart,
  Bar,
  XAxis,
  YAxis,
  CartesianGrid,
  Tooltip,
  ResponsiveContainer,
  Cell,
} from "recharts";

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

interface DashboardProps {
  targets: MonitorJob[];
  results: PingResult[];
}

export default function Dashboard({ targets, results: rawResults }: DashboardProps) {
  // Filter results to only include those for active targets
  const activeTargetUrls = new Set(targets.map((t) => t.target));
  const results = rawResults.filter((r) => activeTargetUrls.has(r.job.target));

  // Compute per-site latest status
  const siteLatest: Record<string, PingResult> = {};
  results.forEach((r) => {
    if (!siteLatest[r.job.target]) {
      siteLatest[r.job.target] = r;
    }
  });

  const totalSites = targets.length;
  const sitesUp = Object.values(siteLatest).filter((r) => r.up).length;
  const sitesDown = Object.values(siteLatest).filter((r) => !r.up).length;
  const avgLatency =
    results.length > 0
      ? results.reduce((sum, r) => sum + r.latency / 1_000_000, 0) /
        results.length
      : 0;

  // Bar chart data: per-site UP count vs TOTAL count
  const siteCountMap: Record<string, { up: number; total: number }> = {};
  results.forEach((r) => {
    if (!siteCountMap[r.job.target]) {
      siteCountMap[r.job.target] = { up: 0, total: 0 };
    }
    siteCountMap[r.job.target].total++;
    if (r.up) siteCountMap[r.job.target].up++;
  });

  const barData = Object.entries(siteCountMap).map(([target, counts]) => ({
    name: target.length > 25 ? target.substring(0, 25) + "…" : target,
    uptime: Math.round((counts.up / counts.total) * 100),
    checks: counts.total,
  }));

  const statCards = [
    {
      label: "Total Sites",
      value: totalSites,
      icon: Globe,
      color: "text-indigo-400",
      bg: "bg-indigo-400/10",
      border: "border-indigo-400/20",
    },
    {
      label: "Sites Up",
      value: sitesUp,
      icon: ArrowUp,
      color: "text-emerald-400",
      bg: "bg-emerald-400/10",
      border: "border-emerald-400/20",
      glow: "stat-glow-green",
    },
    {
      label: "Sites Down",
      value: sitesDown,
      icon: ArrowDown,
      color: "text-red-400",
      bg: "bg-red-400/10",
      border: "border-red-400/20",
      glow: sitesDown > 0 ? "stat-glow-red" : "",
    },
    {
      label: "Avg Latency",
      value: `${avgLatency.toFixed(1)}ms`,
      icon: Timer,
      color: "text-amber-400",
      bg: "bg-amber-400/10",
      border: "border-amber-400/20",
    },
  ];

  return (
    <div className="flex-1 p-8 overflow-y-auto animate-fade-in">
      {/* Header */}
      <div className="mb-8">
        <div className="flex items-center gap-3 mb-2">
          <Activity className="w-7 h-7 text-indigo-400" />
          <h1 className="text-2xl font-bold text-gray-100">Dashboard Overview</h1>
        </div>
        <p className="text-sm text-gray-500">
          Real-time monitoring across all your sites
        </p>
      </div>

      {/* Stat Cards */}
      <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-4 mb-8">
        {statCards.map((card, idx) => (
          <div
            key={card.label}
            className={`glass-card p-5 animate-fade-in ${card.glow || ""}`}
            style={{ animationDelay: `${idx * 80}ms` }}
          >
            <div className="flex items-center justify-between mb-3">
              <span className="text-xs font-semibold text-gray-400 uppercase tracking-wider">
                {card.label}
              </span>
              <div className={`p-2 rounded-lg ${card.bg}`}>
                <card.icon className={`w-4 h-4 ${card.color}`} />
              </div>
            </div>
            <p className={`text-3xl font-bold ${card.color}`}>{card.value}</p>
          </div>
        ))}
      </div>

      {/* Uptime Bar Chart */}
      {barData.length > 0 && (
        <div className="glass-card p-6 mb-8">
          <h2 className="text-sm font-semibold text-gray-300 mb-4 uppercase tracking-wider">
            Uptime % by Site
          </h2>
          <div className="h-72">
            <ResponsiveContainer width="100%" height="100%">
              <BarChart
                data={barData}
                margin={{ top: 5, right: 20, left: 10, bottom: 60 }}
              >
                <CartesianGrid
                  strokeDasharray="3 3"
                  stroke="rgba(55, 65, 81, 0.3)"
                  vertical={false}
                />
                <XAxis
                  dataKey="name"
                  tick={{ fill: "#64748b", fontSize: 11 }}
                  angle={-35}
                  textAnchor="end"
                  interval={0}
                  axisLine={{ stroke: "rgba(55, 65, 81, 0.3)" }}
                />
                <YAxis
                  domain={[0, 100]}
                  tick={{ fill: "#64748b", fontSize: 11 }}
                  axisLine={{ stroke: "rgba(55, 65, 81, 0.3)" }}
                  tickFormatter={(v) => `${v}%`}
                />
                <Tooltip
                  contentStyle={{
                    background: "rgba(17, 24, 39, 0.95)",
                    border: "1px solid rgba(55, 65, 81, 0.5)",
                    borderRadius: "12px",
                    backdropFilter: "blur(12px)",
                    color: "#f1f5f9",
                    fontSize: "13px",
                  }}
                  formatter={(value: any) => [`${value}%`, "Uptime"]}
                />
                <Bar dataKey="uptime" radius={[6, 6, 0, 0]} maxBarSize={48}>
                  {barData.map((entry, index) => (
                    <Cell
                      key={`cell-${index}`}
                      fill={
                        entry.uptime >= 90
                          ? "#10b981"
                          : entry.uptime >= 50
                          ? "#f59e0b"
                          : "#ef4444"
                      }
                      fillOpacity={0.8}
                    />
                  ))}
                </Bar>
              </BarChart>
            </ResponsiveContainer>
          </div>
        </div>
      )}

      {/* Recent Activity Table */}
      <div className="glass-card overflow-hidden">
        <div className="px-6 py-4 border-b border-gray-800/60">
          <h2 className="text-sm font-semibold text-gray-300 uppercase tracking-wider">
            Recent Activity
          </h2>
        </div>
        <div className="overflow-x-auto">
          <table className="w-full text-left border-collapse">
            <thead>
              <tr className="text-xs text-gray-500 uppercase tracking-wider border-b border-gray-800/40">
                <th className="px-6 py-3 font-medium">Target</th>
                <th className="px-6 py-3 font-medium">Protocol</th>
                <th className="px-6 py-3 font-medium">Status</th>
                <th className="px-6 py-3 font-medium">Latency</th>
                <th className="px-6 py-3 font-medium">Time</th>
              </tr>
            </thead>
            <tbody>
              {results.length === 0 ? (
                <tr>
                  <td
                    colSpan={5}
                    className="px-6 py-12 text-center text-gray-600"
                  >
                    <Globe className="w-10 h-10 mx-auto mb-3 opacity-20" />
                    <p>No monitoring data yet. Add a site to get started.</p>
                  </td>
                </tr>
              ) : (
                results.slice(0, 20).map((ping, idx) => (
                  <tr
                    key={idx}
                    className="border-b border-gray-800/30 hover:bg-gray-800/30 transition-colors"
                  >
                    <td className="px-6 py-3">
                      <span className="font-mono text-sm text-gray-300">
                        {ping.job.target}
                      </span>
                    </td>
                    <td className="px-6 py-3">
                      <span className="text-xs text-gray-500 uppercase font-medium">
                        {ping.job.type}
                      </span>
                    </td>
                    <td className="px-6 py-3">
                      <span
                        className={`inline-flex items-center gap-1.5 px-2.5 py-1 rounded-full text-xs font-semibold ${
                          ping.up
                            ? "text-emerald-400 bg-emerald-400/10 border border-emerald-400/20"
                            : "text-red-400 bg-red-400/10 border border-red-400/20"
                        }`}
                      >
                        <span
                          className={`w-1.5 h-1.5 rounded-full ${
                            ping.up ? "bg-emerald-400" : "bg-red-400"
                          }`}
                        />
                        {ping.up ? "UP" : "DOWN"}
                      </span>
                    </td>
                    <td className="px-6 py-3">
                      <span className="font-mono text-sm text-amber-300/90">
                        {(ping.latency / 1_000_000).toFixed(2)} ms
                      </span>
                    </td>
                    <td className="px-6 py-3 text-sm text-gray-500">
                      {new Date(ping.timestamp).toLocaleTimeString()}
                    </td>
                  </tr>
                ))
              )}
            </tbody>
          </table>
        </div>
      </div>
    </div>
  );
}
