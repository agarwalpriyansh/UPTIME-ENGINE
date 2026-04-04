"use client";

import { useState, useEffect, useCallback } from "react";
import Sidebar from "./components/Sidebar";
import Dashboard from "./components/Dashboard";
import SiteDetail from "./components/SiteDetail";

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

export default function Home() {
  const [targets, setTargets] = useState<MonitorJob[]>([]);
  const [results, setResults] = useState<PingResult[]>([]);
  const [selectedSite, setSelectedSite] = useState<string | null>(null);
  const [siteLogs, setSiteLogs] = useState<PingResult[]>([]);
  const [siteStatuses, setSiteStatuses] = useState<Record<string, boolean>>({});

  // Fetch all active monitors
  const fetchTargets = useCallback(async () => {
    try {
      const res = await fetch("/api/targets");
      const data = await res.json();
      setTargets(data || []);
    } catch (err) {
      console.error("Failed to fetch targets:", err);
    }
  }, []);

  // Fetch recent results (for dashboard)
  const fetchStatus = useCallback(async () => {
    try {
      const res = await fetch("/api/status");
      const data: PingResult[] = await res.json();
      setResults(data || []);

      // Compute latest status per site
      const statuses: Record<string, boolean> = {};
      (data || []).forEach((r) => {
        if (!(r.job.target in statuses)) {
          statuses[r.job.target] = r.up;
        }
      });
      setSiteStatuses(statuses);
    } catch (err) {
      console.error("Failed to fetch status:", err);
    }
  }, []);

  // Fetch logs for a specific site
  const fetchSiteLogs = useCallback(async (url: string) => {
    try {
      const res = await fetch(
        `/api/logs?url=${encodeURIComponent(url)}&limit=200`
      );
      const data = await res.json();
      setSiteLogs(data || []);
    } catch (err) {
      console.error("Failed to fetch site logs:", err);
    }
  }, []);

  // Add a new monitor
  const handleAddSite = async (protocol: string, url: string, email?: string) => {
    try {
      await fetch("/api/monitor", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ type: protocol, target: url, email }),
      });
      fetchTargets();
      fetchStatus();
    } catch (err) {
      console.error("Failed to add monitor:", err);
    }
  };

  // Delete a monitor
  const handleDeleteSite = async (url: string) => {
    try {
      await fetch(`/api/monitor?url=${encodeURIComponent(url)}`, {
        method: "DELETE",
      });
      if (selectedSite === url) {
        setSelectedSite(null);
        setSiteLogs([]);
      }
      fetchTargets();
      fetchStatus();
    } catch (err) {
      console.error("Failed to delete monitor:", err);
    }
  };

  // Select a site
  const handleSelectSite = (target: string | null) => {
    if (target === selectedSite) {
      // Deselect → go back to dashboard
      setSelectedSite(null);
      setSiteLogs([]);
    } else {
      setSelectedSite(target);
      if (target) fetchSiteLogs(target);
    }
  };

  // Initial load + polling
  useEffect(() => {
    fetchTargets();
    fetchStatus();

    const interval = setInterval(() => {
      fetchTargets();
      fetchStatus();
      if (selectedSite) fetchSiteLogs(selectedSite);
    }, 5000);

    return () => clearInterval(interval);
  }, [fetchTargets, fetchStatus, fetchSiteLogs, selectedSite]);

  // Fetch logs when site changes
  useEffect(() => {
    if (selectedSite) {
      fetchSiteLogs(selectedSite);
    }
  }, [selectedSite, fetchSiteLogs]);

  return (
    <div className="flex h-screen overflow-hidden">
      <Sidebar
        targets={targets}
        selectedSite={selectedSite}
        siteStatuses={siteStatuses}
        onSelectSite={handleSelectSite}
        onAddSite={handleAddSite}
        onDeleteSite={handleDeleteSite}
      />

      {selectedSite ? (
        <SiteDetail
          target={selectedSite}
          logs={siteLogs}
          onBack={() => {
            setSelectedSite(null);
            setSiteLogs([]);
          }}
        />
      ) : (
        <Dashboard targets={targets} results={results} />
      )}
    </div>
  );
}