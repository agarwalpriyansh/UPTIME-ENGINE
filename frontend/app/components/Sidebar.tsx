"use client";

import {
  Activity,
  Globe,
  Plus,
  Trash2,
  X,
  Menu,
  Signal,
  SignalZero,
} from "lucide-react";
import { useState } from "react";

interface MonitorJob {
  type: string;
  target: string;
}

interface SidebarProps {
  targets: MonitorJob[];
  selectedSite: string | null;
  siteStatuses: Record<string, boolean>;
  onSelectSite: (target: string | null) => void;
  onAddSite: (protocol: string, url: string) => void;
  onDeleteSite: (url: string) => void;
}

export default function Sidebar({
  targets,
  selectedSite,
  siteStatuses,
  onSelectSite,
  onAddSite,
  onDeleteSite,
}: SidebarProps) {
  const [newUrl, setNewUrl] = useState("");
  const [newProtocol, setNewProtocol] = useState("https");
  const [isSubmitting, setIsSubmitting] = useState(false);
  const [collapsed, setCollapsed] = useState(false);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!newUrl.trim()) return;
    setIsSubmitting(true);
    await onAddSite(newProtocol, newUrl.trim());
    setNewUrl("");
    setIsSubmitting(false);
  };

  if (collapsed) {
    return (
      <aside
        className="w-16 flex flex-col items-center py-6 border-r border-gray-800/80"
        style={{ background: "rgba(10, 14, 26, 0.95)" }}
      >
        <button
          onClick={() => setCollapsed(false)}
          className="p-2 rounded-lg hover:bg-gray-800 transition-colors text-gray-400 hover:text-white mb-6"
        >
          <Menu className="w-5 h-5" />
        </button>
        <Activity className="w-6 h-6 text-indigo-400 mb-6" />
        <div className="flex flex-col gap-2 flex-1 overflow-y-auto w-full px-2">
          {targets.map((t) => {
            const isUp = siteStatuses[t.target] !== false;
            const isSelected = selectedSite === t.target;
            return (
              <button
                key={t.target}
                onClick={() => onSelectSite(t.target)}
                className={`p-2 rounded-lg transition-all ${
                  isSelected
                    ? "bg-indigo-500/20 border border-indigo-500/30"
                    : "hover:bg-gray-800 border border-transparent"
                }`}
                title={t.target}
              >
                <span
                  className={`block w-2.5 h-2.5 rounded-full mx-auto ${
                    isUp ? "bg-emerald-400" : "bg-red-400"
                  }`}
                />
              </button>
            );
          })}
        </div>
      </aside>
    );
  }

  return (
    <aside
      className="w-80 flex flex-col border-r border-gray-800/80 animate-slide-in"
      style={{ background: "rgba(10, 14, 26, 0.95)" }}
    >
      {/* Header */}
      <div className="p-5 border-b border-gray-800/80">
        <div className="flex items-center justify-between mb-1">
          <div className="flex items-center gap-2.5">
            <div className="p-1.5 rounded-lg bg-indigo-500/10">
              <Activity className="w-5 h-5 text-indigo-400" />
            </div>
            <h1 className="text-lg font-bold gradient-text">Uptime Engine</h1>
          </div>
          <button
            onClick={() => setCollapsed(true)}
            className="p-1.5 rounded-lg hover:bg-gray-800 transition-colors text-gray-500 hover:text-gray-300"
          >
            <X className="w-4 h-4" />
          </button>
        </div>
        <p className="text-xs text-gray-500 ml-10">Monitor Dashboard</p>
      </div>

      {/* Add Site Form */}
      <div className="p-4 border-b border-gray-800/80">
        <p className="text-xs font-semibold text-gray-400 uppercase tracking-wider mb-3">
          Add New Site
        </p>
        <form onSubmit={handleSubmit} className="space-y-2.5">
          <div className="flex gap-2">
            <select
              value={newProtocol}
              onChange={(e) => setNewProtocol(e.target.value)}
              className="bg-gray-900/80 border border-gray-700 text-gray-300 text-xs rounded-lg px-2 py-2 outline-none focus:border-indigo-500/50 transition-colors w-24 cursor-pointer"
            >
              <option value="https">HTTPS</option>
              <option value="http">HTTP</option>
              <option value="tcp">TCP</option>
            </select>
            <input
              type="text"
              value={newUrl}
              onChange={(e) => setNewUrl(e.target.value)}
              placeholder="google.com"
              className="flex-1 bg-gray-900/80 border border-gray-700 text-gray-200 text-sm rounded-lg px-3 py-2 outline-none focus:border-indigo-500/50 transition-colors placeholder:text-gray-600"
              required
            />
          </div>
          <button
            type="submit"
            disabled={isSubmitting}
            className="w-full flex items-center justify-center gap-2 text-white bg-indigo-600 hover:bg-indigo-500 disabled:bg-indigo-800 disabled:cursor-not-allowed rounded-lg text-sm px-4 py-2 transition-all font-medium"
          >
            <Plus className="w-3.5 h-3.5" />
            {isSubmitting ? "Adding..." : "Add Monitor"}
          </button>
        </form>
      </div>

      {/* Sites List */}
      <div className="flex-1 overflow-y-auto p-3">
        <div className="flex items-center justify-between mb-3 px-1">
          <p className="text-xs font-semibold text-gray-400 uppercase tracking-wider">
            Monitored Sites
          </p>
          <span className="text-xs text-gray-600 tabular-nums">
            {targets.length}
          </span>
        </div>

        {targets.length === 0 ? (
          <div className="text-center py-8">
            <Globe className="w-8 h-8 mx-auto mb-2 text-gray-700" />
            <p className="text-xs text-gray-600">No sites yet</p>
          </div>
        ) : (
          <div className="space-y-1">
            {targets.map((t, idx) => {
              const isUp = siteStatuses[t.target] !== false;
              const isSelected = selectedSite === t.target;
              return (
                <div
                  key={t.target}
                  onClick={() => onSelectSite(t.target)}
                  className={`group flex items-center gap-3 px-3 py-2.5 rounded-xl cursor-pointer transition-all animate-fade-in ${
                    isSelected
                      ? "bg-indigo-500/10 border border-indigo-500/25"
                      : "hover:bg-gray-800/60 border border-transparent"
                  }`}
                  style={{ animationDelay: `${idx * 40}ms` }}
                >
                  {/* Status Dot */}
                  <div className="relative flex-shrink-0">
                    {isUp ? (
                      <Signal className="w-4 h-4 text-emerald-400" />
                    ) : (
                      <SignalZero className="w-4 h-4 text-red-400" />
                    )}
                  </div>

                  {/* Site Info */}
                  <div className="flex-1 min-w-0">
                    <p className="text-sm text-gray-200 truncate font-medium">
                      {t.target}
                    </p>
                    <p className="text-xs text-gray-500 uppercase">
                      {t.type}
                    </p>
                  </div>

                  {/* Delete */}
                  <button
                    onClick={(e) => {
                      e.stopPropagation();
                      onDeleteSite(t.target);
                    }}
                    className="p-1.5 rounded-lg text-gray-600 hover:text-red-400 hover:bg-red-400/10 transition-all opacity-0 group-hover:opacity-100"
                    title="Remove monitor"
                  >
                    <Trash2 className="w-3.5 h-3.5" />
                  </button>
                </div>
              );
            })}
          </div>
        )}
      </div>

      {/* Footer */}
      <div className="p-4 border-t border-gray-800/80">
        <div className="flex items-center gap-2">
          <span className="relative flex h-2 w-2">
            <span className="animate-ping absolute inline-flex h-full w-full rounded-full bg-emerald-400 opacity-75" />
            <span className="relative inline-flex rounded-full h-2 w-2 bg-emerald-500" />
          </span>
          <span className="text-xs text-gray-500 font-medium">
            Auto-refreshing every 5s
          </span>
        </div>
      </div>
    </aside>
  );
}
