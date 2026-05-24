"use client";

import { useState, useEffect, useMemo, useCallback } from "react";
import {
  Globe,
  ArrowLeft,
  CheckCircle2,
  XCircle,
  Clock,
  Timer,
  Wifi,
  Lock,
  AlertTriangle,
  TrendingDown,
  TrendingUp,
  BarChart3,
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
import type { PingResult, Incident, SSLInfo } from "../types";

interface SiteDetailProps {
  target: string;
  logs: PingResult[];
  onBack: () => void;
}

function percentile(values: number[], p: number): number {
  if (values.length === 0) return 0;
  const sorted = [...values].sort((a, b) => a - b);
  const idx = Math.ceil((p / 100) * sorted.length) - 1;
  return sorted[Math.max(0, idx)];
}

function uptimeInWindow(logs: PingResult[], windowMs: number): number | null {
  const cutoff = Date.now() - windowMs;
  const inWindow = logs.filter((l) => new Date(l.timestamp).getTime() >= cutoff);
  if (inWindow.length === 0) return null;
  const up = inWindow.filter((l) => l.up).length;
  return (up / inWindow.length) * 100;
}

function formatDownDuration(ms: number): string {
  if (ms < 0) ms = 0;
  const totalMins = Math.floor(ms / 60000);
  const h = Math.floor(totalMins / 60);
  const m = totalMins % 60;
  if (h > 0) return `${h}h ${m}m`;
  if (m > 0) return `${m}m`;
  const s = Math.floor(ms / 1000);
  return `${s}s`;
}

function findDownSince(logs: PingResult[], openIncident: Incident | null): Date | null {
  if (openIncident?.status === "Active") {
    return new Date(openIncident.started_at);
  }
  if (logs.length === 0) return null;
  let downStart: Date | null = null;
  for (let i = 0; i < logs.length; i++) {
    if (!logs[i].up) {
      downStart = new Date(logs[i].timestamp);
    } else {
      break;
    }
  }
  return downStart;
}

function findLastSuccessfulCheck(logs: PingResult[]): PingResult | null {
  return logs.find((l) => l.up) ?? null;
}

export default function SiteDetail({ target, logs, onBack }: SiteDetailProps) {
  const siteId = target;
  const [incidents, setIncidents] = useState<Incident[]>([]);
  const [sslInfo, setSslInfo] = useState<SSLInfo | null>(null);
  const [now, setNow] = useState(() => Date.now());

  const latestPing = logs.length > 0 ? logs[0] : null;
  const protocol = latestPing?.job.type || "—";
  const isUp = latestPing?.up ?? false;

  const fetchIncidents = useCallback(async () => {
    try {
      const res = await fetch(
        `/api/incidents?siteId=${encodeURIComponent(siteId)}`
      );
      if (!res.ok) return;
      const data = await res.json();
      setIncidents(Array.isArray(data) ? data : []);
    } catch (err) {
      console.error("Failed to fetch incidents:", err);
    }
  }, [siteId]);

  const fetchSSL = useCallback(async () => {
    try {
      const res = await fetch(`/api/ssl?siteId=${encodeURIComponent(siteId)}`);
      if (!res.ok) return;
      const data = await res.json();
      setSslInfo(data);
    } catch (err) {
      console.error("Failed to fetch SSL:", err);
    }
  }, [siteId]);

  useEffect(() => {
    void fetchIncidents();
    void fetchSSL();
    const interval = setInterval(() => {
      void fetchIncidents();
      void fetchSSL();
    }, 5000);
    return () => clearInterval(interval);
  }, [fetchIncidents, fetchSSL]);

  useEffect(() => {
    if (!isUp) {
      const id = setInterval(() => setNow(Date.now()), 1000);
      return () => clearInterval(id);
    }
  }, [isUp]);

  const openIncident = incidents.find((i) => i.status === "Active") ?? null;
  const downSince = findDownSince(logs, openIncident);
  const lastSuccess = findLastSuccessfulCheck(logs);

  const totalChecks = logs.length;
  const upChecks = logs.filter((l) => l.up).length;
  const uptimePercent = totalChecks > 0 ? (upChecks / totalChecks) * 100 : 0;

  const latencies = useMemo(
    () => logs.map((l) => l.latency_ms),
    [logs]
  );
  const minLatency = latencies.length > 0 ? Math.min(...latencies) : 0;
  const maxLatency = latencies.length > 0 ? Math.max(...latencies) : 0;
  const p95Latency = percentile(latencies, 95);

  const avgLatency =
    totalChecks > 0
      ? logs.reduce((sum, l) => sum + l.latency_ms, 0) / totalChecks
      : 0;

  const availability = useMemo(
    () => ({
      h24: uptimeInWindow(logs, 24 * 60 * 60 * 1000),
      d7: uptimeInWindow(logs, 7 * 24 * 60 * 60 * 1000),
      d30: uptimeInWindow(logs, 30 * 24 * 60 * 60 * 1000),
    }),
    [logs]
  );

  const latencyData = [...logs]
    .reverse()
    .map((l) => ({
      time: new Date(l.timestamp).toLocaleTimeString([], {
        hour: "2-digit",
        minute: "2-digit",
      }),
      latency: parseFloat(l.latency_ms.toFixed(2)),
      up: l.up,
    }));

  const pieData = [
    { name: "Up", value: upChecks, color: "#10b981" },
    { name: "Down", value: totalChecks - upChecks, color: "#ef4444" },
  ].filter((d) => d.value > 0);

  const downDurationLabel =
    !isUp && downSince
      ? formatDownDuration(now - downSince.getTime())
      : null;

  const sslBadge = (() => {
    if (!sslInfo || sslInfo.status === "unavailable") return null;
    if (sslInfo.status === "expired") {
      return (
        <span className="inline-flex items-center gap-1 text-xs font-semibold text-red-400 bg-red-400/10 px-2 py-0.5 rounded border border-red-400/20">
          <Lock className="w-3 h-3" />
          SSL Expired
        </span>
      );
    }
    if (sslInfo.status === "warning") {
      return (
        <span className="inline-flex items-center gap-1 text-xs font-semibold text-amber-400 bg-amber-400/10 px-2 py-0.5 rounded border border-amber-400/20">
          <AlertTriangle className="w-3 h-3" />
          SSL {sslInfo.days_remaining}d
        </span>
      );
    }
    return (
      <span className="inline-flex items-center gap-1 text-xs font-semibold text-emerald-400 bg-emerald-400/10 px-2 py-0.5 rounded border border-emerald-400/20">
        <Lock className="w-3 h-3" />
        SSL Valid
      </span>
    );
  })();

  const uptimeColor = (pct: number | null) => {
    if (pct === null) return "text-gray-500";
    if (pct >= 99) return "text-emerald-400";
    if (pct >= 95) return "text-amber-400";
    return "text-red-400";
  };

  const progressBarColor = (pct: number | null) => {
    if (pct === null) return "bg-gray-700";
    if (pct >= 99) return "bg-emerald-500";
    if (pct >= 95) return "bg-amber-500";
    return "bg-red-500";
  };

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
              <div className="flex items-center gap-3 mt-1 flex-wrap">
                <span className="text-xs font-semibold text-gray-500 uppercase px-2 py-0.5 rounded bg-gray-800 border border-gray-700">
                  {protocol}
                </span>
                {sslBadge}
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
                  {!isUp && downDurationLabel && (
                    <span className="text-red-300/90 font-normal">
                      · Down for {downDurationLabel}
                    </span>
                  )}
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
              {lastSuccess && (
                <>
                  <p className="text-xs text-gray-500 mt-2">
                    Last successful check
                  </p>
                  <p className="text-sm text-emerald-400/90">
                    {new Date(lastSuccess.timestamp).toLocaleString()}
                  </p>
                </>
              )}
            </div>
          )}
        </div>
      </div>

      {/* Stat Cards Row */}
      <div className="grid grid-cols-1 sm:grid-cols-3 gap-4 mb-4">
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

      {/* Response Time Stats Row */}
      <div className="grid grid-cols-1 sm:grid-cols-3 gap-4 mb-8">
        <div className="glass-card p-5">
          <div className="flex items-center justify-between mb-2">
            <span className="text-xs font-semibold text-gray-400 uppercase tracking-wider">
              Min Response
            </span>
            <div className="p-1.5 rounded-lg bg-cyan-400/10">
              <TrendingDown className="w-4 h-4 text-cyan-400" />
            </div>
          </div>
          <p className="text-3xl font-bold text-cyan-400">
            {minLatency.toFixed(1)}
            <span className="text-lg font-normal text-cyan-400/60 ml-1">
              ms
            </span>
          </p>
        </div>

        <div className="glass-card p-5">
          <div className="flex items-center justify-between mb-2">
            <span className="text-xs font-semibold text-gray-400 uppercase tracking-wider">
              Max Response
            </span>
            <div className="p-1.5 rounded-lg bg-rose-400/10">
              <TrendingUp className="w-4 h-4 text-rose-400" />
            </div>
          </div>
          <p className="text-3xl font-bold text-rose-400">
            {maxLatency.toFixed(1)}
            <span className="text-lg font-normal text-rose-400/60 ml-1">
              ms
            </span>
          </p>
        </div>

        <div className="glass-card p-5">
          <div className="flex items-center justify-between mb-2">
            <span className="text-xs font-semibold text-gray-400 uppercase tracking-wider">
              P95 Response
            </span>
            <div className="p-1.5 rounded-lg bg-violet-400/10">
              <BarChart3 className="w-4 h-4 text-violet-400" />
            </div>
          </div>
          <p className="text-3xl font-bold text-violet-400">
            {p95Latency.toFixed(1)}
            <span className="text-lg font-normal text-violet-400/60 ml-1">
              ms
            </span>
          </p>
        </div>
      </div>

      {/* Availability Breakdown */}
      <div className="glass-card p-6 mb-8">
        <h2 className="text-sm font-semibold text-gray-300 mb-5 uppercase tracking-wider">
          Availability Breakdown
        </h2>
        <div className="space-y-4">
          {(
            [
              { label: "Last 24 hours", value: availability.h24 },
              { label: "Last 7 days", value: availability.d7 },
              { label: "Last 30 days", value: availability.d30 },
            ] as const
          ).map(({ label, value }) => (
            <div key={label}>
              <div className="flex items-center justify-between mb-1.5">
                <span className="text-sm text-gray-400">{label}</span>
                <span className={`text-sm font-semibold ${uptimeColor(value)}`}>
                  {value !== null ? `${value.toFixed(2)}%` : "No data"}
                </span>
              </div>
              <div className="h-2 rounded-full bg-gray-800 overflow-hidden">
                <div
                  className={`h-full rounded-full transition-all duration-500 ${progressBarColor(value)}`}
                  style={{ width: value !== null ? `${Math.min(value, 100)}%` : "0%" }}
                />
              </div>
            </div>
          ))}
        </div>
      </div>

      {/* Charts Row */}
      <div className="grid grid-cols-1 lg:grid-cols-3 gap-6 mb-8">
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

      {/* Ping Log History */}
      <div className="glass-card overflow-hidden mb-8">
        <div className="px-6 py-4 border-b border-gray-800/60">
          <h2 className="text-sm font-semibold text-gray-300 uppercase tracking-wider">
            Ping Log History
          </h2>
        </div>
        <div className="overflow-x-auto max-h-96 overflow-y-auto">
          <table className="w-full text-left border-collapse">
            <thead
              className="sticky top-0"
              style={{ background: "rgba(17, 24, 39, 0.95)" }}
            >
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
                        {log.latency_ms.toFixed(2)} ms
                      </span>
                    </td>
                  </tr>
                ))
              )}
            </tbody>
          </table>
        </div>
      </div>

      {/* Incident History */}
      <div className="glass-card overflow-hidden">
        <div className="px-6 py-4 border-b border-gray-800/60">
          <h2 className="text-sm font-semibold text-gray-300 uppercase tracking-wider">
            Incident History
          </h2>
        </div>
        <div className="overflow-x-auto">
          <table className="w-full text-left border-collapse">
            <thead style={{ background: "rgba(17, 24, 39, 0.95)" }}>
              <tr className="text-xs text-gray-500 uppercase tracking-wider border-b border-gray-800/40">
                <th className="px-6 py-3 font-medium">Incident #</th>
                <th className="px-6 py-3 font-medium">Started</th>
                <th className="px-6 py-3 font-medium">Resolved</th>
                <th className="px-6 py-3 font-medium">Duration</th>
                <th className="px-6 py-3 font-medium">Status</th>
              </tr>
            </thead>
            <tbody>
              {incidents.length === 0 ? (
                <tr>
                  <td
                    colSpan={5}
                    className="px-6 py-12 text-center text-gray-600"
                  >
                    No incidents recorded for this site.
                  </td>
                </tr>
              ) : (
                incidents.map((inc) => {
                  const isActive = inc.status === "Active";
                  return (
                    <tr
                      key={inc.id}
                      className={`border-b border-gray-800/30 transition-colors ${
                        isActive
                          ? "bg-red-500/10 hover:bg-red-500/15"
                          : "hover:bg-gray-800/30"
                      }`}
                    >
                      <td className="px-6 py-2.5 text-sm font-mono text-gray-300">
                        #{inc.incident_number}
                      </td>
                      <td className="px-6 py-2.5 text-sm text-gray-400">
                        {new Date(inc.started_at).toLocaleString()}
                      </td>
                      <td className="px-6 py-2.5 text-sm text-gray-400">
                        {inc.resolved_at
                          ? new Date(inc.resolved_at).toLocaleString()
                          : "—"}
                      </td>
                      <td className="px-6 py-2.5 text-sm text-gray-400 font-mono">
                        {isActive && downSince
                          ? formatDownDuration(now - downSince.getTime())
                          : inc.duration}
                      </td>
                      <td className="px-6 py-2.5">
                        <span
                          className={`inline-flex items-center gap-1.5 px-2.5 py-0.5 rounded-full text-xs font-semibold ${
                            isActive
                              ? "text-red-400 bg-red-400/15 border border-red-400/30"
                              : "text-gray-400 bg-gray-800/60"
                          }`}
                        >
                          {inc.status}
                        </span>
                      </td>
                    </tr>
                  );
                })
              )}
            </tbody>
          </table>
        </div>
      </div>
    </div>
  );
}
