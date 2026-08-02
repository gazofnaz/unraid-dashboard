import { useEffect, useState } from "react";
import { Link } from "react-router-dom";
import { api, type Settings as SettingsModel } from "../api";
import { Switch } from "../components";
import { applyTheme, currentTheme, useAppState, type ThemeChoice } from "../store";
import { sourceLabel } from "./System";

export default function Settings() {
  const { status, reload } = useAppState();
  const [form, setForm] = useState<SettingsModel | null>(null);
  const [saving, setSaving] = useState(false);
  const [saved, setSaved] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [theme, setTheme] = useState<ThemeChoice>(currentTheme());

  useEffect(() => {
    if (status && !form) setForm(status.settings);
  }, [status, form]);

  const patch = (p: Partial<SettingsModel>) =>
    setForm((f) => (f ? { ...f, ...p } : f));

  const save = async () => {
    if (!form) return;
    setSaving(true);
    setError(null);
    try {
      await api.saveSettings({ ...form, theme });
      await reload();
      setSaved(true);
      setTimeout(() => setSaved(false), 2500);
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err));
    } finally {
      setSaving(false);
    }
  };

  const changeTheme = (t: ThemeChoice) => {
    setTheme(t);
    applyTheme(t);
  };

  if (!form) {
    return (
      <header className="page-head">
        <div>
          <h1>Settings</h1>
          <p className="sub">Loading…</p>
        </div>
      </header>
    );
  }

  const identity = status?.identity;

  return (
    <>
      <header className="page-head">
        <div>
          <h1>Settings</h1>
          <p className="sub">Connections, addressing, display and security posture.</p>
        </div>
        <div className="actions">
          {saved && <span className="pill ok"><span className="dot" />Saved</span>}
          <button className="btn primary" onClick={() => void save()} disabled={saving}>
            {saving ? "Saving…" : "Save settings"}
          </button>
        </div>
      </header>

      {error && (
        <div className="banner danger" role="alert">
          <strong>Save failed.</strong> {error}
        </div>
      )}

      <div className="panel">
        <h2>Connections</h2>
        <p className="panel-sub">
          Sources are configured through the container environment
          (<code className="mono">DOCKER_HOST</code>, <code className="mono">UNRAID_API_URL</code>,{" "}
          <code className="mono">UNRAID_API_KEY</code>, <code className="mono">TEMPLATES_DIR</code>) and
          reported here. The API key is never sent to the browser.
        </p>
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
        <div className="field-row">
          <div>
            <div style={{ fontWeight: 600 }}>Discovery rules</div>
            <div className="desc">Resolver priorities and the low-confidence review queue.</div>
          </div>
          <Link className="btn small" to="/rules">Open</Link>
        </div>
      </div>

      <div className="panel">
        <h2>Address &amp; links</h2>
        <p className="panel-sub">
          Detected: hostname{" "}
          <span className="mono">{identity?.hostname || "unknown"}</span> · LAN{" "}
          <span className="mono">{identity?.lanAddress || "unknown"}</span>. Overrides
          win over detection and never switch silently.
        </p>
        <div className="field">
          <label htmlFor="hostname">Server hostname override</label>
          <input
            id="hostname"
            type="text"
            placeholder="tower.local"
            value={form.serverHostname}
            onChange={(e) => patch({ serverHostname: e.target.value.trim() })}
          />
          <span className="help">Used by the Hostname link mode. Leave empty to auto-detect.</span>
        </div>
        <div className="field">
          <label htmlFor="lan">LAN address override</label>
          <input
            id="lan"
            type="text"
            placeholder="192.168.0.253"
            value={form.lanAddress}
            onChange={(e) => patch({ lanAddress: e.target.value.trim() })}
            list="lan-candidates"
          />
          <datalist id="lan-candidates">
            {identity?.candidates?.map((c) => <option value={c} key={c} />)}
          </datalist>
          <span className="help">
            {identity?.candidates?.length
              ? `Discovered candidates: ${identity.candidates.join(", ")}`
              : "Used by the LAN IP link mode and for HTTP probes."}
          </span>
        </div>
        <div className="field">
          <label htmlFor="iface">Preferred interface</label>
          <input
            id="iface"
            type="text"
            placeholder="br0"
            value={form.preferredInterface}
            onChange={(e) => patch({ preferredInterface: e.target.value.trim() })}
          />
          <span className="help">When the Unraid API reports several interfaces, prefer this one.</span>
        </div>
        <div className="field">
          <label htmlFor="linkmode">Default link mode (all browsers)</label>
          <select
            id="linkmode"
            value={form.linkMode}
            onChange={(e) => patch({ linkMode: e.target.value as SettingsModel["linkMode"] })}
          >
            <option value="hostname">Hostname</option>
            <option value="lan-ip">LAN IP</option>
            <option value="smart">Smart</option>
          </select>
          <span className="help">Each browser can still switch modes locally from the toolbar.</span>
        </div>
        <div className="field-row">
          <div>
            <div style={{ fontWeight: 600 }}>Confirm endpoints with HTTP probes</div>
            <div className="desc">Short, credential-free HEAD/GET requests against local addresses only.</div>
          </div>
          <Switch
            checked={form.probeEnabled}
            onChange={(v) => patch({ probeEnabled: v })}
            label="Enable HTTP probes"
          />
        </div>
        <div className="field-row">
          <div>
            <div style={{ fontWeight: 600 }}>Allow opening stopped containers</div>
            <div className="desc">A reverse proxy or dependent service may still answer.</div>
          </div>
          <Switch
            checked={form.openStoppedLinks}
            onChange={(v) => patch({ openStoppedLinks: v })}
            label="Allow opening stopped containers"
          />
        </div>
      </div>

      <div className="panel">
        <h2>Display</h2>
        <div className="field">
          <label htmlFor="theme">Theme</label>
          <select
            id="theme"
            value={theme}
            onChange={(e) => changeTheme(e.target.value as ThemeChoice)}
          >
            <option value="light">Light (default)</option>
            <option value="dark">Dark</option>
            <option value="system">Match system</option>
          </select>
          <span className="help">Applies immediately in this browser; saved as the server default.</span>
        </div>
        <div className="field-row">
          <div>
            <div style={{ fontWeight: 600 }}>Show stopped containers</div>
            <div className="desc">Stopped cards keep their last-known endpoint, clearly marked.</div>
          </div>
          <Switch
            checked={form.showStopped}
            onChange={(v) => patch({ showStopped: v })}
            label="Show stopped containers"
          />
        </div>
        <div className="field-row">
          <div>
            <div style={{ fontWeight: 600 }}>Show the ArrayDeck container</div>
            <div className="desc">Hidden by default to keep the grid about your applications.</div>
          </div>
          <Switch
            checked={form.showSelf}
            onChange={(v) => patch({ showSelf: v })}
            label="Show the ArrayDeck container"
          />
        </div>
      </div>

      <div className="panel">
        <h2>Security posture</h2>
        <p className="panel-sub">
          ArrayDeck is read-only by design: it exposes no Docker mutations and no
          generic Docker API passthrough. Direct socket access still grants the
          process broad host power — for hardened deployments point{" "}
          <code className="mono">DOCKER_HOST</code> at a filtered socket proxy and keep
          ArrayDeck LAN-only or behind an authenticated reverse proxy. Probes never
          send credentials; the Unraid API key never reaches this browser.
        </p>
      </div>
    </>
  );
}
