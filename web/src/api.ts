// API types mirroring the Go backend JSON, plus fetch helpers and the SSE
// subscription.

export type LinkMode = "hostname" | "lan-ip" | "smart";

export interface PortBinding {
  containerPort: number;
  protocol: string;
  hostIp: string;
  hostPort: number;
}

export interface NetworkAttachment {
  name: string;
  ipAddress?: string;
}

export interface TemplateMeta {
  file: string;
  name: string;
  repository: string;
  webUi?: string;
  icon?: string;
  category?: string;
  project?: string;
}

export interface Endpoint {
  scheme: string;
  addressPolicy: "unraid-host" | "container-lan-ip" | "explicit-host";
  explicitHost?: string;
  containerPort?: number;
  hostPort?: number;
  path: string;
  confidence: number;
  source: string;
}

export interface ProbeResult {
  attempted: boolean;
  ok: boolean;
  statusClass?: string;
  statusCode?: number;
  redirectTo?: string;
  contentType?: string;
  error?: string;
}

export interface Candidate {
  endpoint: Endpoint;
  priority: number;
  source: string;
  evidence?: string[];
  explanation: string;
  probe?: ProbeResult;
  rejected?: boolean;
  rejectReason?: string;
  dismissed?: boolean;
  identity?: string;
}

export interface TraceStep {
  title: string;
  detail: string;
  value?: string;
}

export interface SignalUse {
  name: string;
  value?: string;
  status: "used" | "present" | "absent" | "rejected";
}

export interface Decision {
  containerKey: string;
  containerId: string;
  winner?: Candidate;
  candidates?: Candidate[];
  steps?: TraceStep[];
  signals?: SignalUse[];
  resolvedAt: string;
}

export interface IconRef {
  kind: "proxy" | "initials";
  url?: string;
  initials: string;
  hue: number;
}

export interface ContainerView {
  id: string;
  name: string;
  image: string;
  state: string;
  health?: string;
  networkMode: string;
  labels?: Record<string, string>;
  ports?: PortBinding[];
  exposed?: { port: number; protocol: string }[];
  networks?: NetworkAttachment[];
  createdAt: string;
  startedAt?: string;
  isSelf?: boolean;
  template?: TemplateMeta;
  key: string;
  displayName: string;
  category: string;
  categorySource: string;
  endpoint?: Endpoint;
  confidence?: number;
  lowConfidence?: boolean;
  source?: string;
  icon: IconRef;
  favorite?: boolean;
  hidden?: boolean;
  hasOverride?: boolean;
  candidateCount: number;
}

export interface SourceStatus {
  name: string;
  available: boolean;
  detail?: string;
  error?: string;
}

export interface HostIdentity {
  hostname?: string;
  hostnameSource?: string;
  lanAddress?: string;
  lanSource?: string;
  candidates?: string[];
}

export interface ServerInfo {
  name?: string;
  unraidVersion?: string;
  uptimeSeconds?: number;
}

export interface Stats {
  total: number;
  running: number;
  stopped: number;
  withWebUi: number;
  exact: number;
  inferred: number;
  lowConfidence: number;
  noWebUi: number;
  discoveryScore: number;
  lastReconcile: string;
  durationMs: number;
}

export interface Settings {
  serverHostname: string;
  lanAddress: string;
  preferredInterface: string;
  linkMode: LinkMode;
  showStopped: boolean;
  showSelf: boolean;
  openStoppedLinks: boolean;
  probeEnabled: boolean;
  theme: string;
}

export interface StatusPayload {
  version: string;
  sources: SourceStatus[];
  identity: HostIdentity;
  server: ServerInfo;
  stats: Stats;
  settings: Settings;
}

export interface Patch {
  upserts?: ContainerView[];
  removals?: string[];
  stats: Stats;
}

export interface Override {
  url?: string;
  name?: string;
  icon?: string;
  category?: string;
  hidden?: boolean;
  favorite?: boolean;
  dismissedUrls?: string[];
}

export interface DiscoveryPayload {
  decision: Decision;
  container?: ContainerView;
  override?: Override;
}

async function request<T>(path: string, init?: RequestInit): Promise<T> {
  const res = await fetch(path, {
    headers: { "Content-Type": "application/json" },
    ...init,
  });
  if (!res.ok) {
    let message = `${res.status} ${res.statusText}`;
    try {
      const body = await res.json();
      if (body?.error) message = body.error;
    } catch {
      /* keep the status text */
    }
    throw new Error(message);
  }
  return res.json() as Promise<T>;
}

export const api = {
  status: () => request<StatusPayload>("/api/v1/status"),
  containers: () =>
    request<{ containers: ContainerView[] }>("/api/v1/containers"),
  discovery: (id: string) =>
    request<DiscoveryPayload>(
      `/api/v1/containers/${encodeURIComponent(id)}/discovery`,
    ),
  setOverride: (id: string, o: Override) =>
    request(`/api/v1/containers/${encodeURIComponent(id)}/override`, {
      method: "POST",
      body: JSON.stringify(o),
    }),
  clearOverride: (id: string) =>
    request(`/api/v1/containers/${encodeURIComponent(id)}/override`, {
      method: "DELETE",
    }),
  settings: () => request<Settings>("/api/v1/settings"),
  saveSettings: (s: Settings) =>
    request<Settings>("/api/v1/settings", {
      method: "PUT",
      body: JSON.stringify(s),
    }),
  reconcile: () =>
    request("/api/v1/discovery/reconcile", { method: "POST" }),
};

/** Opens the SSE stream; the browser reconnects automatically. */
export function subscribe(handlers: {
  onPatch: (p: Patch) => void;
  onStatus: (s: StatusPayload) => void;
  onConnectionChange?: (ok: boolean) => void;
}): () => void {
  const source = new EventSource("/api/v1/events");
  source.addEventListener("patch", (e) =>
    handlers.onPatch(JSON.parse((e as MessageEvent).data)),
  );
  source.addEventListener("status", (e) =>
    handlers.onStatus(JSON.parse((e as MessageEvent).data)),
  );
  source.onopen = () => handlers.onConnectionChange?.(true);
  source.onerror = () => handlers.onConnectionChange?.(false);
  return () => source.close();
}
