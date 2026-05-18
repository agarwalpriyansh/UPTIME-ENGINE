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
