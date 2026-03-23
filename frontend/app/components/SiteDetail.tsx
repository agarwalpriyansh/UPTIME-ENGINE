"use client";

import {
  Globe,
  ArrowLeft,
  CheckCircle2,
  XCircle,
  Clock,
  Timer,
  Wifi,
} from "lucide-react";
import {
  AreaChart,
  Area,
  XAxis,
  YAxis,
  CartesianGrid,
  Tooltip,
  ResponsiveContainer,
  PieChart,
  Pie,
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
  error_msg?: string;
  timestamp: string;
}

interface SiteDetailProps {
  target: string;
  logs: PingResult[];
  onBack: () => void;
}

export default function SiteDetail({ target, logs, onBack }: SiteDetailProps) {
  const latestPing = logs.length > 0 ? logs[0] : null;
  const protocol = latestPing?.job.type || "—";
  const isUp = latestPing?.up ?? false;

  // Uptime percentage
  const totalChecks = logs.length;
  const upChecks = logs.filter((l) => l.up).length;
  const uptimePercent = totalChecks > 0 ? (upChecks / totalChecks) * 100 : 0;

  // Average latency
  const avgLatency =
    totalChecks > 0
      ? logs.reduce((sum, l) => sum + l.latency / 1_000_000, 0) / totalChecks
      : 0;

  // Latency chart data (reversed so oldest is left)
  const latencyData = [...logs]
    .reverse()
    .map((l) => ({
      time: new Date(l.timestamp).toLocaleTimeString([], {
        hour: "2-digit",
        minute: "2-digit",
      }),
      latency: parseFloat((l.latency / 1_000_000).toFixed(2)),
      up: l.up,
    }));

  // Pie chart data
  const pieData = [
    { name: "Up", value: upChecks, color: "#10b981" },
    { name: "Down", value: totalChecks - upChecks, color: "#ef4444" },
  ].filter((d) => d.value > 0);

  return (
    <div className="flex-1 p-8 overflow-y-auto animate-fade-in">
      {/* Back + Header */}
      <div className="mb-8">
        <button
          onClick={onBack}
          className="flex items-center gap-2 text-gray-500 hover:text-gray-300 transition-colors mb-4 text-sm"
        >
          <ArrowLeft className="w-4 h-4" />
          Back to Dashboard
        </button>

        <div className="flex items-center justify-between">
          <div className="flex items-center gap-4">
            <div
              className={`p-3 rounded-xl ${
                isUp ? "bg-emerald-400/10" : "bg-red-400/10"
              }`}
            >
              <Globe
                className={`w-7 h-7 ${
                  isUp ? "text-emerald-400" : "text-red-400"
                }`}
              />
            </div>
            <div>
              <h1 className="text-xl font-bold text-gray-100">{target}</h1>
              <div className="flex items-center gap-3 mt-1">
                <span className="text-xs font-semibold text-gray-500 uppercase px-2 py-0.5 rounded bg-gray-800 border border-gray-700">
                  {protocol}
                </span>
                <span
                  className={`inline-flex items-center gap-1.5 text-xs font-semibold ${
                    isUp ? "text-emerald-400" : "text-red-400"
                  }`}
                >
                  {isUp ? (
                    <CheckCircle2 className="w-3.5 h-3.5" />
                  ) : (
                    <XCircle className="w-3.5 h-3.5" />
                  )}
                  {isUp ? "Operational" : "Down"}
                </span>
              </div>
            </div>
          </div>
          {latestPing && (
            <div className="text-right">
              <p className="text-xs text-gray-500">Last checked</p>
              <p className="text-sm text-gray-400">
                {new Date(latestPing.timestamp).toLocaleString()}
              </p>
            </div>
          )}
        </div>
      </div>

      {/* Stat Cards Row */}
      <div className="grid grid-cols-1 sm:grid-cols-3 gap-4 mb-8">
        <div className="glass-card p-5">
          <div className="flex items-center justify-between mb-2">
            <span className="text-xs font-semibold text-gray-400 uppercase tracking-wider">
              Uptime
            </span>
            <div className="p-1.5 rounded-lg bg-emerald-400/10">
              <Wifi className="w-4 h-4 text-emerald-400" />
            </div>
          </div>
          <p
            className={`text-3xl font-bold ${
              uptimePercent >= 90
                ? "text-emerald-400"
                : uptimePercent >= 50
                ? "text-amber-400"
                : "text-red-400"
            }`}
          >
            {uptimePercent.toFixed(1)}%
          </p>
          <p className="text-xs text-gray-500 mt-1">
            {upChecks}/{totalChecks} checks passed
          </p>
        </div>

        <div className="glass-card p-5">
          <div className="flex items-center justify-between mb-2">
            <span className="text-xs font-semibold text-gray-400 uppercase tracking-wider">
              Avg Latency
            </span>
            <div className="p-1.5 rounded-lg bg-amber-400/10">
              <Timer className="w-4 h-4 text-amber-400" />
            </div>
          </div>
          <p className="text-3xl font-bold text-amber-400">
            {avgLatency.toFixed(1)}
            <span className="text-lg font-normal text-amber-400/60 ml-1">
              ms
            </span>
          </p>
        </div>

        <div className="glass-card p-5">
          <div className="flex items-center justify-between mb-2">
            <span className="text-xs font-semibold text-gray-400 uppercase tracking-wider">
              Total Checks
            </span>
            <div className="p-1.5 rounded-lg bg-indigo-400/10">
              <Clock className="w-4 h-4 text-indigo-400" />
            </div>
          </div>
          <p className="text-3xl font-bold text-indigo-400">{totalChecks}</p>
          <p className="text-xs text-gray-500 mt-1">ping records</p>
        </div>
      </div>

      {/* Charts Row */}
      <div className="grid grid-cols-1 lg:grid-cols-3 gap-6 mb-8">
        {/* Latency Area Chart */}
        <div className="glass-card p-6 lg:col-span-2">
          <h2 className="text-sm font-semibold text-gray-300 mb-4 uppercase tracking-wider">
            Latency Over Time
          </h2>
          {latencyData.length > 0 ? (
            <div className="h-64">
              <ResponsiveContainer width="100%" height="100%">
                <AreaChart
                  data={latencyData}
                  margin={{ top: 5, right: 20, left: 10, bottom: 5 }}
                >
                  <defs>
                    <linearGradient
                      id="latencyGradient"
                      x1="0"
                      y1="0"
                      x2="0"
                      y2="1"
                    >
                      <stop
                        offset="0%"
                        stopColor="#6366f1"
                        stopOpacity={0.4}
                      />
                      <stop
                        offset="95%"
                        stopColor="#6366f1"
                        stopOpacity={0}
                      />
                    </linearGradient>
                  </defs>
                  <CartesianGrid
                    strokeDasharray="3 3"
                    stroke="rgba(55, 65, 81, 0.3)"
                    vertical={false}
                  />
                  <XAxis
                    dataKey="time"
                    tick={{ fill: "#64748b", fontSize: 11 }}
                    axisLine={{ stroke: "rgba(55, 65, 81, 0.3)" }}
                    interval="preserveStartEnd"
                  />
                  <YAxis
                    tick={{ fill: "#64748b", fontSize: 11 }}
                    axisLine={{ stroke: "rgba(55, 65, 81, 0.3)" }}
                    tickFormatter={(v) => `${v}ms`}
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
                    formatter={(value) => [`${value} ms`, "Latency"]}
                  />
                  <Area
                    type="monotone"
                    dataKey="latency"
                    stroke="#6366f1"
                    strokeWidth={2}
                    fill="url(#latencyGradient)"
                    dot={false}
                    activeDot={{ r: 4, fill: "#6366f1", stroke: "#fff" }}
                  />
                </AreaChart>
              </ResponsiveContainer>
            </div>
          ) : (
            <div className="h-64 flex items-center justify-center text-gray-600">
              No data available
            </div>
          )}
        </div>

        {/* Uptime Pie Chart */}
        <div className="glass-card p-6 flex flex-col items-center justify-center">
          <h2 className="text-sm font-semibold text-gray-300 mb-4 uppercase tracking-wider self-start">
            Uptime Ratio
          </h2>
          {pieData.length > 0 ? (
            <div className="h-48 w-full">
              <ResponsiveContainer width="100%" height="100%">
                <PieChart>
                  <Pie
                    data={pieData}
                    cx="50%"
                    cy="50%"
                    innerRadius={50}
                    outerRadius={72}
                    paddingAngle={3}
                    dataKey="value"
                    strokeWidth={0}
                  >
                    {pieData.map((entry, index) => (
                      <Cell key={`cell-${index}`} fill={entry.color} />
                    ))}
                  </Pie>
                  <Tooltip
                    contentStyle={{
                      background: "rgba(17, 24, 39, 0.95)",
                      border: "1px solid rgba(55, 65, 81, 0.5)",
                      borderRadius: "12px",
                      color: "#f1f5f9",
                      fontSize: "13px",
                    }}
                  />
                </PieChart>
              </ResponsiveContainer>
            </div>
          ) : (
            <div className="h-48 flex items-center justify-center text-gray-600">
              No data
            </div>
          )}
          <div className="flex gap-4 mt-2">
            <div className="flex items-center gap-1.5">
              <span className="w-2.5 h-2.5 rounded-full bg-emerald-400" />
              <span className="text-xs text-gray-400">Up</span>
            </div>
            <div className="flex items-center gap-1.5">
              <span className="w-2.5 h-2.5 rounded-full bg-red-400" />
              <span className="text-xs text-gray-400">Down</span>
            </div>
          </div>
        </div>
      </div>

      {/* Logs Table */}
      <div className="glass-card overflow-hidden">
        <div className="px-6 py-4 border-b border-gray-800/60">
          <h2 className="text-sm font-semibold text-gray-300 uppercase tracking-wider">
            Ping Log History
          </h2>
        </div>
        <div className="overflow-x-auto max-h-96 overflow-y-auto">
          <table className="w-full text-left border-collapse">
            <thead className="sticky top-0" style={{ background: "rgba(17, 24, 39, 0.95)" }}>
              <tr className="text-xs text-gray-500 uppercase tracking-wider border-b border-gray-800/40">
                <th className="px-6 py-3 font-medium">Time</th>
                <th className="px-6 py-3 font-medium">Status</th>
                <th className="px-6 py-3 font-medium">HTTP Code</th>
                <th className="px-6 py-3 font-medium">Latency</th>

              </tr>
            </thead>
            <tbody>
              {logs.length === 0 ? (
                <tr>
                  <td
                    colSpan={4}
                    className="px-6 py-12 text-center text-gray-600"
                  >
                    No logs for this site yet.
                  </td>
                </tr>
              ) : (
                logs.map((log, idx) => (
                  <tr
                    key={idx}
                    className="border-b border-gray-800/30 hover:bg-gray-800/30 transition-colors"
                  >
                    <td className="px-6 py-2.5 text-sm text-gray-400">
                      {new Date(log.timestamp).toLocaleString()}
                    </td>
                    <td className="px-6 py-2.5">
                      <span
                        className={`inline-flex items-center gap-1.5 px-2.5 py-0.5 rounded-full text-xs font-semibold ${
                          log.up
                            ? "text-emerald-400 bg-emerald-400/10"
                            : "text-red-400 bg-red-400/10"
                        }`}
                      >
                        <span
                          className={`w-1.5 h-1.5 rounded-full ${
                            log.up ? "bg-emerald-400" : "bg-red-400"
                          }`}
                        />
                        {log.up ? "UP" : "DOWN"}
                      </span>
                    </td>
                    <td className="px-6 py-2.5 text-sm text-gray-500 font-mono">
                      {log.status_code || "—"}
                    </td>
                    <td className="px-6 py-2.5">
                      <span className="font-mono text-sm text-amber-300/90">
                        {(log.latency / 1_000_000).toFixed(2)} ms
                      </span>
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
