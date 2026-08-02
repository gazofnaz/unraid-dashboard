import { Link } from "react-router-dom";
import { confidencePct } from "../components";
import { sourceKind } from "../links";
import { useAppState } from "../store";
import { sourceLabel } from "./System";

const RESOLVER_RULES = [
  { prio: 100, name: "Manual override", detail: "Always wins; discovered candidates stay visible and restorable." },
  { prio: 90, name: "Explicit URL labels", detail: "net.unraid.docker.webui, io.arraydeck.url and dashboard labels that clearly contain a URL." },
  { prio: 85, name: "dockerMan template <WebUI>", detail: "The Unraid template pattern with [IP] and [PORT:n] tokens resolved against real mappings." },
  { prio: 75, name: "Reverse-proxy routes", detail: "Traefik Host(...) rules and Caddy labels become explicit hosts preserved in Smart mode." },
  { prio: 60, name: "Application catalog", detail: "Known apps' default ports, only when the port is actually published." },
  { prio: 40, name: "Published-port probe", detail: "Plausible HTTP ports confirmed by a lightweight probe. Never links database or mail ports without proof." },
  { prio: 0, name: "No endpoint", detail: "A missing link is preferable to a confident-looking wrong link." },
];

export default function DiscoveryRules() {
  const { containers, status } = useAppState();

  const low = containers
    .filter((c) => c.endpoint && c.lowConfidence)
    .sort((a, b) => (a.confidence ?? 0) - (b.confidence ?? 0));
  const none = containers.filter((c) => !c.endpoint && !c.isSelf);

  return (
    <>
      <header className="page-head">
        <div>
          <h1>Discovery rules</h1>
          <p className="sub">
            How ArrayDeck derives every link — and which decisions deserve a second look.
          </p>
        </div>
      </header>

      <div className="panel">
        <h2>Source health</h2>
        {status?.sources.map((s) => (
          <div className="field-row" key={s.name}>
            <div>
              <div style={{ fontWeight: 600 }}>{sourceLabel(s.name)}</div>
              <div className="desc">{s.error || s.detail || ""}</div>
            </div>
            {s.available ? (
              <span className="pill ok"><span className="dot" />Available</span>
            ) : (
              <span className="pill neutral"><span className="dot" />Unavailable</span>
            )}
          </div>
        ))}
      </div>

      <div className="panel">
        <h2>Resolver priority</h2>
        <p className="panel-sub">
          Candidates are generated from every source, scored, and the highest
          deterministic priority wins. Confidence below 50% is never selected
          automatically.
        </p>
        {RESOLVER_RULES.map((r) => (
          <div className="field-row" key={r.prio}>
            <div>
              <div style={{ fontWeight: 600 }}>
                <span className="mono faint" style={{ marginRight: 8 }}>
                  P{r.prio}
                </span>
                {r.name}
              </div>
              <div className="desc">{r.detail}</div>
            </div>
          </div>
        ))}
      </div>

      <div className="panel">
        <h2>Low-confidence links</h2>
        <p className="panel-sub">
          These links work from weaker evidence. Open the trace to confirm or
          override them — the card is never marked unhealthy for low confidence.
        </p>
        {low.length === 0 && <p className="muted">Nothing needs review. 🎉</p>}
        {low.map((c) => (
          <div className="field-row" key={c.id}>
            <div>
              <div style={{ fontWeight: 600 }}>{c.displayName}</div>
              <div className="desc">
                {sourceKind(c.source)} · confidence {confidencePct(c.confidence)}
              </div>
            </div>
            <Link className="btn small" to={`/containers/${c.key}/discovery`}>
              Open trace
            </Link>
          </div>
        ))}
      </div>

      <div className="panel">
        <h2>No web interface</h2>
        <p className="panel-sub">
          Databases, workers and utility containers stay listed — the dashboard is
          also a lightweight inventory.
        </p>
        {none.length === 0 && <p className="muted">Every container has a web interface.</p>}
        {none.map((c) => (
          <div className="field-row" key={c.id}>
            <div>
              <div style={{ fontWeight: 600 }}>{c.displayName}</div>
              <div className="desc">{c.image}</div>
            </div>
            <Link className="btn small" to={`/containers/${c.key}/discovery`}>
              Why?
            </Link>
          </div>
        ))}
      </div>
    </>
  );
}
