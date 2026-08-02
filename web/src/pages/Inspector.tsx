import { useCallback, useEffect, useState } from "react";
import { Link, useParams } from "react-router-dom";
import {
  api,
  type Candidate,
  type DiscoveryPayload,
  type LinkMode,
} from "../api";
import { AppIcon, StatePill, confidencePct } from "../components";
import { renderURL } from "../links";
import { useAppState } from "../store";

export default function Inspector() {
  const { id = "" } = useParams();
  const { status, linkMode, reload } = useAppState();
  const [payload, setPayload] = useState<DiscoveryPayload | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [overrideURL, setOverrideURL] = useState("");
  const [busy, setBusy] = useState(false);

  const load = useCallback(async () => {
    try {
      const p = await api.discovery(id);
      setPayload(p);
      setOverrideURL(p.override?.url ?? "");
      setError(null);
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err));
    }
  }, [id]);

  useEffect(() => {
    void load();
  }, [load]);

  const rerun = async () => {
    setBusy(true);
    try {
      await api.reconcile();
      // Give the reconcile a moment, then refetch the trace.
      await new Promise((r) => setTimeout(r, 1200));
      await load();
      await reload();
    } finally {
      setBusy(false);
    }
  };

  const mutateOverride = async (fn: () => Promise<unknown>) => {
    setBusy(true);
    setError(null);
    try {
      await fn();
      await new Promise((r) => setTimeout(r, 800));
      await load();
      await reload();
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err));
    } finally {
      setBusy(false);
    }
  };

  const saveOverride = () =>
    mutateOverride(() =>
      api.setOverride(id, { ...payload?.override, url: overrideURL.trim() }),
    );

  const clearOverride = () => mutateOverride(() => api.clearOverride(id));

  const dismissCandidate = (c: Candidate) => {
    if (!c.identity) return;
    const existing = payload?.override?.dismissedUrls ?? [];
    return mutateOverride(() =>
      api.setOverride(id, {
        ...payload?.override,
        dismissedUrls: [...existing.filter((d) => d !== c.identity), c.identity!],
      }),
    );
  };

  const restoreCandidate = (c: Candidate) => {
    const existing = payload?.override?.dismissedUrls ?? [];
    return mutateOverride(() =>
      api.setOverride(id, {
        ...payload?.override,
        dismissedUrls: existing.filter((d) => d !== c.identity),
      }),
    );
  };

  if (error && !payload) {
    return (
      <>
        <div className="crumbs">
          <Link to="/">Applications</Link> / discovery
        </div>
        <div className="banner danger" role="alert">
          <strong>Cannot load the discovery trace.</strong> {error}
        </div>
      </>
    );
  }
  if (!payload) {
    return <div className="skeleton" style={{ height: 320 }} aria-label="Loading trace" />;
  }

  const { decision, container } = payload;
  const identity = status?.identity ?? {};
  const winner = decision.winner;
  const selectedURL = winner ? renderURL(winner.endpoint, linkMode, identity) : null;
  const candidates = decision.candidates ?? [];
  const firstIP = container?.networks?.find((n) => n.ipAddress)?.ipAddress;

  const modes: { key: LinkMode; label: string; text: string }[] = [
    {
      key: "hostname",
      label: "Hostname",
      text: winner ? renderURL(winner.endpoint, "hostname", identity) : "—",
    },
    {
      key: "lan-ip",
      label: "LAN IP",
      text: winner ? renderURL(winner.endpoint, "lan-ip", identity) : "—",
    },
    {
      key: "smart",
      label: "Smart",
      text: winner
        ? winner.endpoint.addressPolicy === "explicit-host"
          ? renderURL(winner.endpoint, "smart", identity)
          : "Use reverse proxy when known"
        : "—",
    },
  ];

  return (
    <>
      <header className="page-head">
        <div>
          <div className="crumbs">
            <Link to="/">Applications</Link> / {container?.displayName ?? decision.containerKey} / Discovery
          </div>
          <h1>Why this link was selected</h1>
          <p className="sub">Every automatic decision remains inspectable and overrideable.</p>
        </div>
        <div className="actions">
          <button className="btn" onClick={() => void rerun()} disabled={busy}>
            {busy ? "Working…" : "Re-run discovery"}
          </button>
        </div>
      </header>

      {error && (
        <div className="banner danger" role="alert">
          {error}
        </div>
      )}

      <div className="inspector-grid">
        <div className="panel">
          <div className="row" style={{ justifyContent: "space-between", marginBottom: 12 }}>
            <h2 style={{ margin: 0 }}>Container metadata</h2>
            {container && <StatePill c={container} />}
          </div>

          {container && (
            <div
              className="row"
              style={{
                border: "1px solid var(--border)",
                borderRadius: 10,
                padding: "10px 12px",
                marginBottom: 14,
              }}
            >
              <AppIcon icon={container.icon} />
              <div>
                <div style={{ fontWeight: 650 }}>{container.displayName}</div>
                <div className="faint" style={{ fontSize: 12 }}>
                  {container.image}
                </div>
              </div>
            </div>
          )}

          <dl className="kv">
            <dt>Network mode</dt>
            <dd>{container?.networkMode ?? "—"}</dd>
            <dt>Container address</dt>
            <dd>{firstIP || "—"}</dd>
            <dt>Published ports</dt>
            <dd>
              {container?.ports?.length
                ? container.ports
                    .map((p) => `${p.containerPort}/${p.protocol} → ${p.hostIp || "0.0.0.0"}:${p.hostPort}`)
                    .join(", ")
                : "none"}
            </dd>
            <dt>Unraid template</dt>
            <dd>{container?.template?.file ?? "—"}</dd>
            <dt>Category</dt>
            <dd>
              {container?.category}
              {container?.categorySource ? ` (${container.categorySource})` : ""}
            </dd>
          </dl>

          <div className="nav-section" style={{ padding: "18px 0 6px" }}>
            Signals
          </div>
          {(decision.signals ?? []).map((sig, i) => (
            <div className="signal-row" key={i}>
              <span className="sig-name">{sig.name}</span>
              <span className="sig-value">{sig.value || "—"}</span>
              <span className={`sig-status ${sig.status}`}>
                {sig.status === "used"
                  ? "Used"
                  : sig.status === "rejected"
                    ? "Rejected"
                    : sig.status === "present"
                      ? "Present"
                      : "—"}
              </span>
            </div>
          ))}
        </div>

        <div className="panel">
          <div className="row" style={{ justifyContent: "space-between", marginBottom: 12 }}>
            <h2 style={{ margin: 0 }}>Resolution trace</h2>
            <span className="faint" style={{ fontSize: 12 }}>
              Resolver v1
            </span>
          </div>

          {(decision.steps ?? []).map((step, i) => (
            <div className="step" key={i}>
              <span className="step-num">{i + 1}</span>
              <div className="step-body">
                <div className="step-title">{step.title}</div>
                <div className="step-detail">{step.detail}</div>
                {step.value && <div className="value-box">{step.value}</div>}
              </div>
            </div>
          ))}

          {winner && selectedURL ? (
            <div className="selected-endpoint">
              <div className="head">
                <span>Selected endpoint</span>
                <span className="conf">{confidencePct(winner.endpoint.confidence)} confidence</span>
              </div>
              <a className="url" href={selectedURL} target="_blank" rel="noreferrer noopener">
                {selectedURL}
              </a>
            </div>
          ) : (
            <div className="selected-endpoint none">
              <div className="head">
                <span>No web interface detected</span>
              </div>
              Ports and network details remain available above.
            </div>
          )}

          {winner && (
            <div className="preview-grid">
              {modes.map((m) => (
                <div className={`preview${linkMode === m.key ? " current" : ""}`} key={m.key}>
                  <div className="mode">{m.label}</div>
                  <div className="addr">{m.text}</div>
                </div>
              ))}
            </div>
          )}

          <h2 style={{ fontSize: 14.5, margin: "4px 0 8px" }}>Manual override</h2>
          <div className="row" style={{ marginBottom: 16 }}>
            <input
              type="text"
              placeholder="https://qb.home.example/"
              aria-label="Override URL"
              value={overrideURL}
              onChange={(e) => setOverrideURL(e.target.value)}
              style={{
                flex: 1,
                minWidth: 200,
                padding: "8px 11px",
                border: "1px solid var(--border)",
                borderRadius: 8,
                background: "var(--panel)",
                color: "var(--text)",
                font: "inherit",
              }}
            />
            <button
              className="btn primary small"
              onClick={() => void saveOverride()}
              disabled={busy || !overrideURL.trim()}
            >
              Save override
            </button>
            {payload.override && !isEmptyOverride(payload.override) && (
              <button className="btn small danger-ghost" onClick={() => void clearOverride()} disabled={busy}>
                Reset to discovered
              </button>
            )}
          </div>

          <h2 style={{ fontSize: 14.5, margin: "4px 0 8px" }}>
            All candidates ({candidates.length})
          </h2>
          {candidates.map((c, i) => (
            <div className={`candidate${c.rejected ? " rejected" : ""}`} key={i}>
              <div className="head">
                <span className="src">{c.source}</span>
                <span className="pill neutral">P{c.priority}</span>
                {c.endpoint.confidence > 0 && (
                  <span className={`pill ${c.endpoint.confidence >= 0.75 ? "ok" : "warn"}`}>
                    {confidencePct(c.endpoint.confidence)}
                  </span>
                )}
                {c.dismissed && <span className="pill warn">Dismissed</span>}
                {c.rejected && <span className="pill danger">Rejected</span>}
                {!c.rejected && !c.dismissed && winner && c.identity === winner.identity && (
                  <span className="pill ok">
                    <span className="dot" />
                    Selected
                  </span>
                )}
                <span className="grow" />
                {!c.rejected && c.identity && !c.dismissed && (!winner || c.identity !== winner.identity) && (
                  <button className="btn small" onClick={() => void dismissCandidate(c)} disabled={busy}>
                    Dismiss
                  </button>
                )}
                {c.dismissed && (
                  <button className="btn small" onClick={() => void restoreCandidate(c)} disabled={busy}>
                    Restore
                  </button>
                )}
              </div>
              <div className="why">{c.explanation}</div>
              {c.rejected && c.rejectReason && <div className="reject">{c.rejectReason}</div>}
              {c.probe?.attempted && (
                <div className="faint" style={{ fontSize: 11.5, marginTop: 3 }}>
                  probe: {c.probe.ok ? `ok (${c.probe.statusClass})` : c.probe.error || c.probe.statusClass}
                </div>
              )}
            </div>
          ))}
        </div>
      </div>
    </>
  );
}

function isEmptyOverride(o: NonNullable<DiscoveryPayload["override"]>): boolean {
  return (
    !o.url &&
    !o.name &&
    !o.icon &&
    !o.category &&
    o.hidden == null &&
    o.favorite == null &&
    !(o.dismissedUrls?.length)
  );
}
