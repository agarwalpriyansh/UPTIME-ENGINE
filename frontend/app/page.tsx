"use client";

import { useState, useEffect, useCallback, useRef } from "react";
import Sidebar from "./components/Sidebar";
import Dashboard from "./components/Dashboard";
import SiteDetail from "./components/SiteDetail";
import type { MonitorJob, PingResult } from "./types";

export default function Home() {
  const [targets, setTargets] = useState<MonitorJob[]>([]);
  const [results, setResults] = useState<PingResult[]>([]);
  const [selectedSite, setSelectedSite] = useState<string | null>(null);
  const [siteLogs, setSiteLogs] = useState<PingResult[]>([]);
  const [siteStatuses, setSiteStatuses] = useState<Record<string, boolean>>({});
  const alive = useRef(true);

  const fetchTargets = useCallback(async () => {
    try {
      const res = await fetch("/api/targets");
      if (!res.ok) return;
      const data = await res.json();
      if (!alive.current) return;
      setTargets(Array.isArray(data) ? data : []);
    } catch (err) {
      console.error("Failed to fetch targets:", err);
    }
  }, []);

  const fetchStatus = useCallback(async () => {
    try {
      const res = await fetch("/api/status?limit=800");
      if (!res.ok) return;
      const data: PingResult[] = await res.json();
      if (!alive.current) return;
      const list = Array.isArray(data) ? data : [];
      setResults(list);

      const statuses: Record<string, boolean> = {};
      for (const r of list) {
        const key = r.job.target;
        if (!(key in statuses)) {
          statuses[key] = r.up;
        }
      }
      setSiteStatuses(statuses);
    } catch (err) {
      console.error("Failed to fetch status:", err);
    }
  }, []);

  const fetchSiteLogs = useCallback(async (url: string) => {
    try {
      const res = await fetch(
        `/api/logs?url=${encodeURIComponent(url)}&limit=200`
      );
      if (!res.ok) return;
      const data = await res.json();
      if (!alive.current) return;
      setSiteLogs(Array.isArray(data) ? data : []);
    } catch (err) {
      console.error("Failed to fetch site logs:", err);
    }
  }, []);

  const handleAddSite = async (
    protocol: string,
    url: string,
    email?: string
  ): Promise<boolean> => {
    try {
      const res = await fetch("/api/monitor", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          type: protocol,
          target: url,
          owner_email: email ?? "",
        }),
      });
      if (res.status === 409) {
        const data = (await res.json().catch(() => null)) as {
          message?: string;
        } | null;
        console.warn(data?.message ?? "Target is already monitored");
        return false;
      }
      if (!res.ok) {
        console.error("Failed to add monitor:", res.status, await res.text());
        return false;
      }
      void fetchTargets();
      void fetchStatus();
      return true;
    } catch (err) {
      console.error("Failed to add monitor:", err);
      return false;
    }
  };

  const handleDeleteSite = async (url: string): Promise<boolean> => {
    try {
      const res = await fetch(`/api/monitor?url=${encodeURIComponent(url)}`, {
        method: "DELETE",
      });
      if (!res.ok) {
        console.error("Failed to delete monitor:", res.status);
        return false;
      }
      if (selectedSite === url) {
        setSelectedSite(null);
        setSiteLogs([]);
      }
      void fetchTargets();
      void fetchStatus();
      return true;
    } catch (err) {
      console.error("Failed to delete monitor:", err);
      return false;
    }
  };

  const handleSelectSite = (target: string | null) => {
    if (target === selectedSite) {
      setSelectedSite(null);
      setSiteLogs([]);
    } else {
      setSelectedSite(target);
      if (target) void fetchSiteLogs(target);
    }
  };

  useEffect(() => {
    alive.current = true;
    void fetchTargets();
    void fetchStatus();

    const interval = setInterval(() => {
      void fetchTargets();
      void fetchStatus();
      if (selectedSite) void fetchSiteLogs(selectedSite);
    }, 5000);

    return () => {
      alive.current = false;
      clearInterval(interval);
    };
  }, [fetchTargets, fetchStatus, fetchSiteLogs, selectedSite]);

  useEffect(() => {
    if (selectedSite) {
      void fetchSiteLogs(selectedSite);
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
