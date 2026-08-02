import { uptime } from "../components";
import { useAppState } from "../store";

export default function System() {
  const { status } = useAppState();
  const identity = status?.identity;
  const server = status?.server;
  const unraid = status?.sources.find((s) => s.name === "unraid-api");

  return (
    <>
      <header className="page-head">
        <div>
          <h1>System</h1>
          <p className="sub">Server identity and discovery source health.</p>
        </div>
      </header>

      <div className="panel">
        <h2>Server</h2>
        <p className="panel-sub">
          {unraid?.available
            ? "Read from the Unraid GraphQL API with a read-only key."
            : "The Unraid API is not connected; identity falls back to configuration and the browser origin."}
        </p>
        <dl className="kv">
          <dt>Hostname</dt>
          <dd>
            {identity?.hostname || "—"}
            {identity?.hostnameSource ? `  (${identity.hostnameSource})` : ""}
          </dd>
          <dt>LAN address</dt>
          <dd>
            {identity?.lanAddress || "—"}
            {identity?.lanSource ? `  (${identity.lanSource})` : ""}
          </dd>
          <dt>Address candidates</dt>
          <dd>{identity?.candidates?.length ? identity.candidates.join(", ") : "—"}</dd>
          <dt>Unraid version</dt>
          <dd>{server?.unraidVersion || "—"}</dd>
          <dt>Uptime</dt>
          <dd>{uptime(server?.uptimeSeconds)}</dd>
          <dt>ArrayDeck version</dt>
          <dd>{status?.version || "—"}</dd>
        </dl>
      </div>

      <div className="panel">
        <h2>Sources</h2>
        <p className="panel-sub">Each discovery source degrades independently.</p>
        {status?.sources.map((s) => (
          <div className="field-row" key={s.name}>
            <div>
              <div style={{ fontWeight: 600 }}>{sourceLabel(s.name)}</div>
              <div className="desc">{s.error || s.detail || ""}</div>
            </div>
            {s.available ? (
              <span className="pill ok">
                <span className="dot" />
                Available
              </span>
            ) : (
              <span className="pill neutral">
                <span className="dot" />
                Unavailable
              </span>
            )}
          </div>
        ))}
      </div>

      <div className="panel">
        <h2>Coming later</h2>
        <p className="panel-sub">
          This area is reserved for array and parity status, disk temperatures and
          SMART summaries, cache pool utilization, VM inventory and Unraid
          notifications — reading the same Unraid API connection.
        </p>
      </div>
    </>
  );
}

export function sourceLabel(name: string): string {
  switch (name) {
    case "docker":
      return "Docker engine";
    case "unraid-api":
      return "Unraid GraphQL API";
    case "templates":
      return "dockerMan templates";
    default:
      return name;
  }
}
