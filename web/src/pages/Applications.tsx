import { useEffect, useMemo, useRef, useState } from "react";
import { Link } from "react-router-dom";
import type { ContainerView } from "../api";
import { AppIcon, StatePill, LinkModeSegment, ago } from "../components";
import { renderURL, sourceKind } from "../links";
import { useAppState } from "../store";

type ViewMode = "grouped" | "flat" | "state" | "network";

const GROUP_ORDER = [
  "Media", "Downloads", "Automation", "Home", "Network", "Security",
  "Development", "Storage", "Monitoring", "Databases", "Infrastructure", "Other",
];

export default function Applications() {
  const { containers, status, loading, error, linkMode, setLinkMode, refresh } =
    useAppState();
  const [query, setQuery] = useState("");
  const [viewMode, setViewMode] = useState<ViewMode>(
    () => (localStorage.getItem("arraydeck-view") as ViewMode) || "grouped",
  );
  const [collapsed, setCollapsed] = useState<Record<string, boolean>>(() => {
    try {
      return JSON.parse(localStorage.getItem("arraydeck-collapsed") || "{}");
    } catch {
      return {};
    }
  });
  const searchRef = useRef<HTMLInputElement>(null);
  const gridRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if ((e.metaKey || e.ctrlKey) && e.key.toLowerCase() === "k") {
        e.preventDefault();
        searchRef.current?.focus();
        searchRef.current?.select();
      }
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, []);

  const identity = status?.identity ?? {};
  const settings = status?.settings;
  const stats = status?.stats;

  const visible = useMemo(() => {
    const q = query.trim().toLowerCase();
    return containers.filter((c) => {
      if (c.isSelf && !settings?.showSelf) return false;
      if (c.hidden) return false;
      if (c.state !== "running" && settings && !settings.showStopped) return false;
      if (!q) return true;
      const url = c.endpoint ? renderURL(c.endpoint, linkMode, identity) : "";
      const ports = (c.ports ?? [])
        .map((p) => `${p.hostPort} ${p.containerPort}`)
        .join(" ");
      return `${c.displayName} ${c.name} ${c.image} ${c.category} ${ports} ${url} ${c.networkMode}`
        .toLowerCase()
        .includes(q);
    });
  }, [containers, query, settings, linkMode, identity]);

  const groups = useMemo(() => groupViews(visible, viewMode), [visible, viewMode]);

  const toggleGroup = (name: string) => {
    const next = { ...collapsed, [name]: !collapsed[name] };
    setCollapsed(next);
    localStorage.setItem("arraydeck-collapsed", JSON.stringify(next));
  };

  const changeViewMode = (m: ViewMode) => {
    setViewMode(m);
    localStorage.setItem("arraydeck-view", m);
  };

  const focusFirstCard = () => {
    gridRef.current
      ?.querySelector<HTMLAnchorElement>("a.url-row, a.card-open")
      ?.focus();
  };

  const docker = status?.sources.find((s) => s.name === "docker");
  const unraid = status?.sources.find((s) => s.name === "unraid-api");

  const sourceCounts = useMemo(() => {
    const counts = { exact: 0, template: 0, proxy: 0, probed: 0 };
    for (const c of visible) {
      const kind = sourceKind(c.source);
      if (kind === "exact" || kind === "manual") counts.exact++;
      else if (kind === "template") counts.template++;
      else if (kind === "proxy") counts.proxy++;
      else if (kind === "inferred" || kind === "catalog") counts.probed++;
    }
    return counts;
  }, [visible]);

  return (
    <>
      <header className="page-head">
        <div>
          <h1>Your applications</h1>
          <p className="sub">Automatically discovered from Docker and Unraid metadata.</p>
        </div>
        <div className="actions">
          <div className="search">
            <span className="glyph" aria-hidden="true">⌕</span>
            <input
              ref={searchRef}
              type="search"
              placeholder="Search containers, images or ports"
              aria-label="Search containers"
              value={query}
              onChange={(e) => setQuery(e.target.value)}
              onKeyDown={(e) => {
                if (e.key === "Enter") focusFirstCard();
              }}
            />
            <kbd>⌘K</kbd>
          </div>
          <button className="btn" onClick={() => void refresh()} title="Re-run discovery">
            ⟳ Refresh
          </button>
        </div>
      </header>

      {error && (
        <div className="banner danger" role="alert">
          <strong>ArrayDeck API unreachable.</strong> {error}
        </div>
      )}
      {docker && !docker.available && (
        <div className="banner danger" role="alert">
          <strong>Docker source unavailable.</strong> {docker.error || "The container inventory cannot refresh."}
        </div>
      )}
      {docker?.available && unraid && !unraid.available && (
        <div className="banner" role="status">
          <strong>Docker connected.</strong> Unraid API {unraid.detail || "unavailable"}; hostname, LAN address and
          system metrics may be incomplete.
        </div>
      )}

      <div className="toolbar">
        <span className="muted">Open links using</span>
        <LinkModeSegment
          mode={linkMode}
          hostname={identity.hostname || "Hostname"}
          onChange={setLinkMode}
        />
        <span className="hint">
          Detected address: <strong>{identity.lanAddress || "unknown"}</strong>
        </span>
        <span className="spacer" />
        {stats && (
          <>
            <span className="pill ok">
              <span className="dot" />
              {stats.running} running
            </span>
            <span className="pill neutral">{stats.noWebUi} no web UI</span>
            <span className="pill neutral" title="Last reconciliation">
              Synced {ago(stats.lastReconcile)} ago
            </span>
          </>
        )}
      </div>

      {stats && (
        <div className="tiles">
          <div className="tile">
            <div className="label">Containers</div>
            <div className="value">{stats.total}</div>
            <div className="detail">
              {stats.running} running · {stats.stopped} stopped
            </div>
          </div>
          <div className="tile">
            <div className="label">Web interfaces</div>
            <div className="value">{stats.withWebUi}</div>
            <div className="detail">
              {stats.exact} exact · {stats.inferred} inferred
            </div>
          </div>
          <div className="tile">
            <div className="label">Discovery health</div>
            <div className="value">{stats.discoveryScore}%</div>
            {stats.lowConfidence === 0 ? (
              <div className="detail ok">No manual links required</div>
            ) : (
              <div className="detail warn">
                {stats.lowConfidence} low-confidence link{stats.lowConfidence > 1 ? "s" : ""}
              </div>
            )}
          </div>
          <div className="tile">
            <div className="label">Server</div>
            <div className="value">{status?.server.name || identity.hostname?.split(".")[0] || "—"}</div>
            <div className="detail">
              {status?.server.unraidVersion
                ? `Unraid ${status.server.unraidVersion} · ${uptimeShort(status.server.uptimeSeconds)} uptime`
                : "Unraid API not connected"}
            </div>
          </div>
        </div>
      )}

      <div className="row" style={{ justifyContent: "flex-end", marginBottom: 10 }}>
        <label className="muted" htmlFor="viewmode" style={{ fontSize: 12.5 }}>
          View
        </label>
        <select
          id="viewmode"
          value={viewMode}
          onChange={(e) => changeViewMode(e.target.value as ViewMode)}
          style={{
            padding: "5px 9px",
            border: "1px solid var(--border)",
            borderRadius: 7,
            background: "var(--panel)",
            color: "var(--text)",
            font: "inherit",
            fontSize: 12.5,
          }}
        >
          <option value="grouped">Grouped</option>
          <option value="flat">Flat</option>
          <option value="state">By state</option>
          <option value="network">By network</option>
        </select>
      </div>

      <div ref={gridRef}>
        {loading ? (
          <div className="cards" aria-busy="true" aria-label="Loading applications">
            {Array.from({ length: 8 }).map((_, i) => (
              <div className="skeleton" key={i} />
            ))}
          </div>
        ) : visible.length === 0 ? (
          <EmptyState
            hasContainers={containers.length > 0}
            dockerAvailable={docker?.available ?? false}
            query={query}
          />
        ) : (
          groups.map((g, idx) => (
            <section className="group" key={g.name} aria-label={g.name}>
              <div className="group-head">
                <button
                  className="collapse"
                  onClick={() => toggleGroup(g.name)}
                  aria-expanded={!collapsed[g.name]}
                  aria-label={`${collapsed[g.name] ? "Expand" : "Collapse"} ${g.name}`}
                >
                  {collapsed[g.name] ? "▸" : "▾"}
                </button>
                <h2>{g.name}</h2>
                <span className="count">{g.items.length}</span>
                {idx === 0 && viewMode === "grouped" && (
                  <span className="origin">{g.origin}</span>
                )}
              </div>
              {!collapsed[g.name] && (
                <div className="cards">
                  {g.items.map((c) => (
                    <AppCard key={c.id} c={c} />
                  ))}
                </div>
              )}
            </section>
          ))
        )}
      </div>

      {!loading && stats && (
        <div className="trace-line">
          <span>
            Discovery trace: {sourceCounts.exact} URL{sourceCounts.exact === 1 ? "" : "s"} from Unraid WebUI
            metadata, {sourceCounts.template} from templates, {sourceCounts.proxy} from reverse-proxy labels,{" "}
            {sourceCounts.probed} from catalog or HTTP probes.
          </span>
          <Link to="/rules">Review low-confidence matches →</Link>
        </div>
      )}
    </>
  );
}

function uptimeShort(seconds: number | undefined): string {
  if (!seconds || seconds <= 0) return "—";
  const d = Math.floor(seconds / 86400);
  if (d > 0) return `${d}d`;
  return `${Math.floor(seconds / 3600)}h`;
}

function AppCard({ c }: { c: ContainerView }) {
  const { status, linkMode } = useAppState();
  const identity = status?.identity ?? {};
  const settings = status?.settings;
  const url = c.endpoint ? renderURL(c.endpoint, linkMode, identity) : null;
  const stopped = c.state !== "running";
  const openable = url != null && (!stopped || settings?.openStoppedLinks);
  const kind = sourceKind(c.source);
  const netName = c.networkMode === "default" ? "bridge" : c.networkMode;

  return (
    <article className={`card${stopped ? " stopped" : ""}`}>
      <div className="card-top">
        <AppIcon icon={c.icon} />
        <div className="card-title">
          <div className="name">{c.displayName}</div>
          <div className="image" title={c.image}>
            {c.image}
          </div>
        </div>
        <StatePill c={c} />
      </div>

      {url ? (
        openable ? (
          <a
            className="url-row"
            href={url}
            target="_blank"
            rel="noreferrer noopener"
            title={`Open ${url}`}
          >
            {url}
          </a>
        ) : (
          <span className="url-row" title="Container is stopped">
            {url}
          </span>
        )
      ) : (
        <span className="url-row none">No web interface detected</span>
      )}

      <div className="card-foot">
        <span className="meta">
          {netName}
          {kind ? ` · ${kind}` : ""}
          {c.ports?.length && !c.endpoint
            ? ` · ${c.ports[0].hostPort}/${c.ports[0].protocol}`
            : ""}
        </span>
        <span className="links">
          {openable && (
            <a className="card-open" href={url!} target="_blank" rel="noreferrer noopener">
              Open ›
            </a>
          )}
          <Link to={`/containers/${c.key}/discovery`}>Details</Link>
        </span>
      </div>

      {c.endpoint && (
        <div
          className={`confidence${c.lowConfidence ? " low" : ""}`}
          role="img"
          aria-label={`URL confidence ${Math.round((c.confidence ?? 0) * 100)}%${c.lowConfidence ? " (low)" : ""}`}
        >
          <i style={{ width: `${Math.round((c.confidence ?? 0) * 100)}%` }} />
        </div>
      )}
    </article>
  );
}

function EmptyState({
  hasContainers,
  dockerAvailable,
  query,
}: {
  hasContainers: boolean;
  dockerAvailable: boolean;
  query: string;
}) {
  if (!dockerAvailable) {
    return (
      <div className="empty">
        <h3>Docker source unavailable</h3>
        <p>
          ArrayDeck cannot reach the Docker engine. Check the socket mount or
          <code> DOCKER_HOST</code> and see Settings → Connections.
        </p>
      </div>
    );
  }
  if (hasContainers && query) {
    return (
      <div className="empty">
        <h3>No matches</h3>
        <p>No container matches “{query}”. Filters and search exclude everything.</p>
      </div>
    );
  }
  if (hasContainers) {
    return (
      <div className="empty">
        <h3>Everything is filtered out</h3>
        <p>Adjust the display settings to show stopped or hidden containers.</p>
      </div>
    );
  }
  return (
    <div className="empty">
      <h3>Docker returned zero containers</h3>
      <p>Deploy a container in Unraid and it will appear here automatically.</p>
    </div>
  );
}

function groupViews(views: ContainerView[], mode: ViewMode) {
  if (mode === "flat") {
    return views.length
      ? [{ name: "All applications", origin: "", items: views }]
      : [];
  }
  const buckets = new Map<string, ContainerView[]>();
  for (const v of views) {
    const key =
      mode === "grouped"
        ? v.category
        : mode === "state"
          ? v.state === "running"
            ? "Running"
            : "Stopped"
          : v.networkMode === "default"
            ? "bridge"
            : v.networkMode;
    if (!buckets.has(key)) buckets.set(key, []);
    buckets.get(key)!.push(v);
  }
  const names = [...buckets.keys()].sort((a, b) => {
    if (mode === "grouped") {
      const ia = GROUP_ORDER.indexOf(a);
      const ib = GROUP_ORDER.indexOf(b);
      return (ia < 0 ? 99 : ia) - (ib < 0 ? 99 : ib) || a.localeCompare(b);
    }
    return a.localeCompare(b);
  });
  return names.map((name) => ({
    name,
    origin: "Grouped from Unraid category metadata",
    items: buckets.get(name)!,
  }));
}
