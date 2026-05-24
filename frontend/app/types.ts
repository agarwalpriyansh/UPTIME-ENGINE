export interface MonitorJob {
  type: string;
  target: string;
  owner_email?: string;
}

export interface PingResult {
  job: MonitorJob;
  status_code: number;
  latency_ms: number;
  up: boolean;
  error_msg?: string;
  timestamp: string;
}

export interface Incident {
  id: number;
  incident_number: number;
  started_at: string;
  resolved_at: string | null;
  duration: string;
  duration_seconds: number;
  status: "Active" | "Resolved";
}

export interface SSLInfo {
  valid: boolean;
  expires_at?: string;
  days_remaining: number;
  status: "valid" | "warning" | "expired" | "unavailable";
  host?: string;
  error?: string;
}
