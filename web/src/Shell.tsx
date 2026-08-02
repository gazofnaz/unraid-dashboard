import { NavLink, Outlet } from "react-router-dom";
import { useAppState } from "./store";

const workspace = [
  { to: "/", label: "Applications", end: true },
  { to: "/containers", label: "Containers" },
  { to: "/system", label: "System" },
];
const manage = [
  { to: "/rules", label: "Discovery rules" },
  { to: "/settings", label: "Settings" },
];

export default function Shell() {
  const { status } = useAppState();
  const unraidOK = status?.sources.find((s) => s.name === "unraid-api")?.available;
  const hostname = status?.identity.hostname;

  const item = (l: { to: string; label: string; end?: boolean }) => (
    <NavLink
      key={l.to}
      to={l.to}
      end={l.end}
      className={({ isActive }) => `nav-item${isActive ? " active" : ""}`}
    >
      <span className="nav-dot" />
      {l.label}
    </NavLink>
  );

  return (
    <div className="shell">
      <aside className="sidebar">
        <div className="brand">
          <span className="brand-mark">A</span>
          ArrayDeck
        </div>
        <div className="nav-section">Workspace</div>
        {workspace.map(item)}
        <div className="nav-section">Manage</div>
        {manage.map(item)}
        <div className="sidebar-server">
          <div className="label">Connected server</div>
          <div className="host">{hostname || "unknown"}</div>
          {unraidOK ? (
            <span className="pill ok">
              <span className="dot" />
              Unraid API connected
            </span>
          ) : (
            <span className="pill neutral">
              <span className="dot" />
              Docker metadata only
            </span>
          )}
        </div>
      </aside>

      <main className="main">
        <Outlet />
      </main>

      <nav className="bottom-nav" aria-label="Primary">
        {[
          { to: "/", label: "Apps", end: true },
          { to: "/containers", label: "Containers" },
          { to: "/system", label: "System" },
          { to: "/settings", label: "Settings" },
        ].map((l) => (
          <NavLink
            key={l.to}
            to={l.to}
            end={l.end}
            className={({ isActive }) => (isActive ? "active" : "")}
          >
            {l.label}
          </NavLink>
        ))}
      </nav>
    </div>
  );
}
