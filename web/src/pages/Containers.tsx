import { useMemo, useState } from "react";
import { useNavigate } from "react-router-dom";
import { AppIcon, StatePill, confidencePct } from "../components";
import { renderURL } from "../links";
import { useAppState } from "../store";

export default function Containers() {
  const { containers, status, loading, linkMode } = useAppState();
  const [query, setQuery] = useState("");
  const navigate = useNavigate();
  const identity = status?.identity ?? {};

  const rows = useMemo(() => {
    const q = query.trim().toLowerCase();
    return containers.filter((c) => {
      if (c.isSelf && !status?.settings.showSelf) return false;
      if (!q) return true;
      return `${c.name} ${c.image} ${c.networkMode} ${c.state}`
        .toLowerCase()
        .includes(q);
    });
  }, [containers, query, status]);

  return (
    <>
      <header className="page-head">
        <div>
          <h1>Containers</h1>
          <p className="sub">
            The complete Docker inventory — including containers without a web interface.
          </p>
        </div>
        <div className="actions">
          <div className="search">
            <span className="glyph" aria-hidden="true">⌕</span>
            <input
              type="search"
              placeholder="Filter inventory"
              aria-label="Filter containers"
              value={query}
              onChange={(e) => setQuery(e.target.value)}
            />
          </div>
        </div>
      </header>

      <div className="table-wrap">
        <table className="inventory">
          <thead>
            <tr>
              <th>Name</th>
              <th>Image</th>
              <th>State</th>
              <th>Health</th>
              <th>Ports</th>
              <th>Network</th>
              <th>Endpoint</th>
            </tr>
          </thead>
          <tbody>
            {loading && (
              <tr>
                <td colSpan={7} className="muted">
                  Loading inventory…
                </td>
              </tr>
            )}
            {!loading && rows.length === 0 && (
              <tr>
                <td colSpan={7} className="muted">
                  No containers match.
                </td>
              </tr>
            )}
            {rows.map((c) => (
              <tr
                key={c.id}
                onClick={() => navigate(`/containers/${c.key}/discovery`)}
                style={{ cursor: "pointer" }}
              >
                <td>
                  <span className="cell-name">
                    <AppIcon icon={c.icon} />
                    {c.name}
                  </span>
                </td>
                <td className="muted mono" title={c.image}>
                  {c.image.length > 42 ? c.image.slice(0, 42) + "…" : c.image}
                </td>
                <td>
                  <StatePill c={c} />
                </td>
                <td className="muted">{c.health || "—"}</td>
                <td className="mono muted">
                  {(c.ports ?? [])
                    .filter((p) => p.protocol === "tcp")
                    .slice(0, 3)
                    .map((p) => `${p.hostPort}→${p.containerPort}`)
                    .join(", ") || "—"}
                </td>
                <td className="muted">{c.networkMode === "default" ? "bridge" : c.networkMode}</td>
                <td className="mono">
                  {c.endpoint ? (
                    <>
                      {renderURL(c.endpoint, linkMode, identity)}{" "}
                      <span className={c.lowConfidence ? "sig-status present" : "sig-status used"}>
                        {confidencePct(c.confidence)}
                      </span>
                    </>
                  ) : (
                    <span className="faint">no web UI</span>
                  )}
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </>
  );
}
